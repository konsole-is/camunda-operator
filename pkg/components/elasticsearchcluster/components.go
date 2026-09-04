/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package elasticsearchcluster renders the resources that an
// ElasticsearchCluster CR publishes. It merges the preset into the spec,
// validates the merged spec, and assembles the ocf components: the file-realm
// credentials Secret, the ECK Elasticsearch CR, the SecondaryStorageConfig
// binding, and the optional metrics exporter. Everything here is pure: spec in, resources out, no API calls. The
// controller in internal/controller drives it.
package elasticsearchcluster

import (
	"maps"
	"strings"

	commonv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/common/v1"
	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	"github.com/sourcehawk/operator-component-framework/pkg/mutation/editors"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/secret"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/serviceaccount"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/credentials"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/eckelasticsearch"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/secondarystorageconfig"
)

const (
	// componentLabel is the labels.ComponentKey value on everything that an
	// ElasticsearchCluster manages.
	componentLabel = "elasticsearch"
	// userSecretSuffix appended to the CR name yields the name of the
	// file-realm user Secret.
	userSecretSuffix = "-es-user"
	// serviceAccountSuffix appended to the CR name yields the name of the
	// ServiceAccount of the pods.
	serviceAccountSuffix = "-es"
	// httpServiceSuffix appended to the CR name yields the name of the
	// HTTPS service that ECK creates for the cluster.
	httpServiceSuffix = "-es-http"
	// certsSecretSuffix appended to the CR name yields the name of the
	// Secret in which ECK publishes the CA certificate of the cluster.
	certsSecretSuffix = "-es-http-certs-public"
	// ECKClusterNameLabel is the label that ECK puts on every object of an
	// Elasticsearch cluster, with the cluster name as its value.
	ECKClusterNameLabel = "elasticsearch.k8s.elastic.co/cluster-name"
	// caCertKey is the key of the CA certificate inside the ECK certs Secret.
	caCertKey = "ca.crt"
	// nodeSetName names the single node set that the operator renders.
	nodeSetName = "default"
	// DataVolumeClaimName is the fixed claim name of ECK for the data volume.
	// A volumeClaimTemplate under this name overrides the default claim of
	// ECK.
	DataVolumeClaimName = "elasticsearch-data"
	// username is the file-realm user that the operator provisions for
	// Camunda.
	username = "camunda"
	// userRole is the role that the Camunda user holds. It is the custom role
	// that rolesSecretSuffix defines, not a built-in one: no built-in role
	// covers the Camunda privileges and the snapshot privileges together, and
	// superuser grants far more than either.
	userRole = "camunda"
	// rolesSecretSuffix appended to the CR name yields the name of the Secret
	// that holds the definition of the Camunda role.
	rolesSecretSuffix = "-es-roles"
	// rolesFileKey is the key that ECK reads role definitions from.
	rolesFileKey = "roles.yml"
	// usernameKey is the key of the username in the user Secret.
	usernameKey = "username"
	// PasswordKey is the key of the password in the user Secret.
	PasswordKey = "password"
	// rolesKey is the key of the roles in the user Secret.
	rolesKey = "roles"
	// repositorySeparator joins the namespace and the name of a cluster into
	// the name of its snapshot repository. A Kubernetes namespace is a
	// DNS-1123 label and holds no dot, so the first dot of a repository name
	// splits it back into the two parts that give the base path.
	repositorySeparator = "."
)

// camundaRole is the definition of the role that the Camunda user holds, in
// the file-based format that ECK loads through spec.auth.roles.
//
// Every privilege is one that Camunda documents as required, except manage:
// see below. The cluster privileges: monitor for the cluster health check and
// the node statistics that size a restore; manage_index_templates and
// manage_ilm to create the index templates and lifecycle policies on first
// start and on an upgrade; manage_pipeline for the data migration of an
// upgrade; create_snapshot and monitor_snapshot for taking a backup and
// reading its status.
//
// manage is here for one reason only: deleting a snapshot, which the backup
// finalizer does. create_snapshot's own definition is "create snapshots for
// existing repositories. Can also list and view details on existing
// repositories and snapshots" — creation and viewing, not deletion. The
// Delete Snapshot API documents its requirement as the manage cluster
// privilege, and no narrower named privilege for delete exists. manage grants
// more than deletion alone (it "builds on monitor and adds cluster operations
// that change values in the cluster"), but it is the documented way to get
// this one operation, so it stands as a deliberate, wider-than-ideal grant
// rather than an unverified raw action name.
//
// Registering the repository is deliberately absent: it needs
// cluster:admin/repository, and the operator does it with the elastic user of
// ECK instead.
const camundaRole = `camunda:
  cluster:
    - monitor
    - manage_index_templates
    - manage_ilm
    - manage_pipeline
    - create_snapshot
    - monitor_snapshot
    - manage
  indices:
    - names: [ "*" ]
      privileges:
        - create_index
        - delete_index
        - read
        - write
        - manage
        - manage_ilm
`

const (
	// ConditionCredentials is the condition type of the credentials
	// component.
	ConditionCredentials = "CredentialsReady"
	// ConditionElasticsearch is the condition type of the elasticsearch
	// component.
	ConditionElasticsearch = "ElasticsearchReady"
	// ConditionStorageContract is the condition type of the storage-contract
	// component.
	ConditionStorageContract = "StorageContractReady"
	// ConditionKeystore is the condition type of the keystore component.
	ConditionKeystore = "KeystoreReady"
	// ConditionSnapshotRepository reports whether the snapshot repository of
	// the cluster is registered in Elasticsearch. The controller sets it
	// directly: registration is a call against Elasticsearch, not a Kubernetes
	// resource, so no component owns it.
	ConditionSnapshotRepository = "SnapshotRepositoryReady"
)

// CACertKey is the key of the CA certificate inside the Secret that
// CACertSecretName names.
const CACertKey = caCertKey

// CredentialsComponent builds the credentials component: the basic-auth style
// file-realm Secret with the Camunda user, the given password, and the Camunda
// role, plus the Secret that defines that role. ECK consumes them through
// spec.auth.fileRealm and spec.auth.roles.
//
// A reused password carries its apply precondition onto the user Secret, so a
// delete of that Secret always rotates the password. The controller must
// reconcile the component through credentials.NewApplyClient for the
// precondition to hold.
func CredentialsComponent(
	cluster *v1.ElasticsearchCluster,
	password credentials.Password,
) (*component.Component, error) {
	userSecret, err := secret.NewBuilder(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        UserSecretName(cluster),
			Namespace:   cluster.Namespace,
			Labels:      managedLabels(cluster),
			Annotations: password.PreconditionAnnotations(),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			usernameKey: []byte(username),
			PasswordKey: []byte(password.Value),
			rolesKey:    []byte(userRole),
		},
	}).Build()
	if err != nil {
		return nil, err
	}

	rolesSecret, err := secret.NewBuilder(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RolesSecretName(cluster),
			Namespace: cluster.Namespace,
			Labels:    managedLabels(cluster),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{rolesFileKey: []byte(camundaRole)},
	}).Build()
	if err != nil {
		return nil, err
	}

	return component.NewComponentBuilder().
		WithName("credentials").
		WithConditionType(ConditionCredentials).
		WithResource(userSecret).
		WithResource(rolesSecret).
		Build()
}

// UserSecretName returns the name of the file-realm user Secret.
func UserSecretName(cluster *v1.ElasticsearchCluster) string {
	return cluster.Name + userSecretSuffix
}

// managedLabels returns the labels of a resource that the operator applies
// for cluster.
func managedLabels(cluster *v1.ElasticsearchCluster) map[string]string {
	return labels.Managed(labels.ElasticsearchCluster(cluster.Name), componentLabel)
}

// RolesSecretName returns the name of the Secret that defines the Camunda
// role.
func RolesSecretName(cluster *v1.ElasticsearchCluster) string {
	return cluster.Name + rolesSecretSuffix
}

// ElasticsearchComponent builds the elasticsearch component from the
// preset-merged spec: the ServiceAccount of the pods (gated on
// spec.serviceAccount) and the ECK Elasticsearch CR. spec.suspend suspends the
// component, which deletes the ECK CR with its data volumes retained.
func ElasticsearchComponent(
	cluster *v1.ElasticsearchCluster,
	merged v1.ElasticsearchClusterSpec,
	storage *SnapshotStorage,
) (*component.Component, error) {
	account, err := serviceaccount.NewBuilder(serviceAccount(cluster, merged, storage)).Build()
	if err != nil {
		return nil, err
	}

	elasticsearch, err := eckelasticsearch.NewBuilder(elasticsearch(cluster, merged)).
		WithMutation(elasticsearchMutations(cluster, merged, storage)...).
		Build()
	if err != nil {
		return nil, err
	}

	return component.NewComponentBuilder().
		WithName("elasticsearch").
		WithConditionType(ConditionElasticsearch).
		// A pre-existing ServiceAccount is excluded, not gated: a gated-off
		// resource is a deletion target, and the operator must never delete
		// an account it does not own. With create true the gate cleans up
		// the owned account when nothing uses one anymore.
		IncludeWhen(
			merged.ServiceAccount.Creates(),
			func() component.Resource { return account },
			component.GatedBy(feature.NewBooleanGate(usesServiceAccount(merged, storage))),
		).
		WithResource(elasticsearch).
		Suspend(merged.Suspend).
		Build()
}

// serviceAccount renders the ServiceAccount of the pods. Its annotations are
// the identity of the snapshot bucket with the annotations of
// spec.serviceAccount layered over it, so a value the user states wins over
// the derived one on the same key.
func serviceAccount(
	cluster *v1.ElasticsearchCluster,
	merged v1.ElasticsearchClusterSpec,
	storage *SnapshotStorage,
) *corev1.ServiceAccount {
	var user map[string]string
	if merged.ServiceAccount != nil {
		user = merged.ServiceAccount.Annotations
	}

	// Not labels.Merge: that helper lets the operator win, because selectors
	// depend on operator labels. Here the user wins, so an identity stated on
	// the cluster overrides the one derived from the bucket.
	var annotations map[string]string
	if derived := storage.identityAnnotations(); len(derived) > 0 || len(user) > 0 {
		annotations = make(map[string]string, len(derived)+len(user))
		maps.Copy(annotations, derived)
		maps.Copy(annotations, user)
	}

	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ServiceAccountName(cluster, merged),
			Namespace:   cluster.Namespace,
			Labels:      managedLabels(cluster),
			Annotations: annotations,
		},
	}
}

// ServiceAccountName returns the name of the ServiceAccount of the pods: the
// name that the spec sets, or the name derived from the cluster otherwise. It
// is the principal that a workload identity without an annotation binds, so it
// is part of the contract with the cloud provider.
func ServiceAccountName(cluster *v1.ElasticsearchCluster, merged v1.ElasticsearchClusterSpec) string {
	if merged.ServiceAccount != nil && merged.ServiceAccount.Name != "" {
		return merged.ServiceAccount.Name
	}

	return cluster.Name + serviceAccountSuffix
}

// elasticsearch renders the baseline ECK Elasticsearch CR: the version, the
// single default node set with the merged replica count, the labeled pod
// template and data volume claim, and the file-realm auth block.
// elasticsearchMutations layers the optional concerns (resources, env, pod
// metadata, scheduling, service account) on top.
func elasticsearch(cluster *v1.ElasticsearchCluster, merged v1.ElasticsearchClusterSpec) *esv1.Elasticsearch {
	return &esv1.Elasticsearch{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
			Labels:    managedLabels(cluster),
		},
		Spec: esv1.ElasticsearchSpec{
			Version: merged.Version,
			// The ECK default: the data volumes go with the cluster. The
			// VolumeRetention mutation switches to retention for a Retain
			// policy, and suspension switches to it before it deletes the CR.
			VolumeClaimDeletePolicy: esv1.DeleteOnScaledownAndClusterDeletionPolicy,
			Auth: esv1.Auth{
				FileRealm: []esv1.FileRealmSource{
					{SecretRef: commonv1.SecretRef{SecretName: UserSecretName(cluster)}},
				},
				Roles: []esv1.RoleSource{
					{SecretRef: commonv1.SecretRef{SecretName: RolesSecretName(cluster)}},
				},
			},
			NodeSets: []esv1.NodeSet{{
				Name:  nodeSetName,
				Count: *merged.Replicas,
				PodTemplate: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: discoveryLabels(cluster)},
				},
				VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
					ObjectMeta: metav1.ObjectMeta{
						Name:   DataVolumeClaimName,
						Labels: discoveryLabels(cluster),
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceStorage: *merged.StorageSize},
						},
						StorageClassName: merged.StorageClassName,
					},
				}},
			}},
		},
	}
}

// discoveryLabels returns the labels of the pods and data volumes that ECK
// runs from the template of cluster. They carry the owner and component, so
// an extension can discover them, but not the manager label: ECK manages
// those objects.
func discoveryLabels(cluster *v1.ElasticsearchCluster) map[string]string {
	return labels.Discovery(labels.ElasticsearchCluster(cluster.Name), componentLabel)
}

// elasticsearchMutations layers the optional concerns of the merged spec onto
// the baseline ECK CR. Each mutation is gated on its field.
func elasticsearchMutations(
	cluster *v1.ElasticsearchCluster,
	merged v1.ElasticsearchClusterSpec,
	storage *SnapshotStorage,
) []eckelasticsearch.Mutation {
	secureSettings := secureSettings(cluster, merged, storage)

	mutations := []eckelasticsearch.Mutation{
		{
			Name:    "SecureSettings",
			Feature: feature.NewBooleanGate(len(secureSettings) > 0),
			Mutate: func(m *eckelasticsearch.Mutator) error {
				m.Edit(func(es *esv1.Elasticsearch) error {
					es.Spec.SecureSettings = secureSettings
					return nil
				})
				return nil
			},
		},
		{
			Name:    "NodeResources",
			Feature: feature.NewBooleanGate(merged.Resources != nil),
			Mutate: func(m *eckelasticsearch.Mutator) error {
				m.EditContainer(nodeSetName, func(c *editors.ContainerEditor) error {
					c.SetResources(*merged.Resources)
					return nil
				})
				return nil
			},
		},
		{
			Name:    "ExtraEnvironment",
			Feature: feature.NewBooleanGate(len(merged.ExtraEnv) > 0 || len(merged.ExtraEnvFrom) > 0),
			Mutate: func(m *eckelasticsearch.Mutator) error {
				m.EditContainer(nodeSetName, func(c *editors.ContainerEditor) error {
					c.EnsureEnvVars(merged.ExtraEnv)
					c.Raw().EnvFrom = merged.ExtraEnvFrom
					return nil
				})
				return nil
			},
		},
		{
			Name:    "PodMetadata",
			Feature: feature.NewBooleanGate(len(merged.PodLabels) > 0 || len(merged.PodAnnotations) > 0),
			Mutate: func(m *eckelasticsearch.Mutator) error {
				m.Edit(func(es *esv1.Elasticsearch) error {
					template := &es.Spec.NodeSets[0].PodTemplate
					template.Labels = labels.Merge(
						labels.Merge(template.Labels, merged.PodLabels),
						discoveryLabels(cluster),
					)
					if len(merged.PodAnnotations) > 0 {
						if template.Annotations == nil {
							template.Annotations = map[string]string{}
						}
						maps.Copy(template.Annotations, merged.PodAnnotations)
					}
					return nil
				})
				return nil
			},
		},
		{
			Name:    "SchedulingConstraints",
			Feature: feature.NewBooleanGate(merged.Scheduling != nil),
			Mutate: func(m *eckelasticsearch.Mutator) error {
				m.Edit(func(es *esv1.Elasticsearch) error {
					pod := &es.Spec.NodeSets[0].PodTemplate.Spec
					if merged.Scheduling.NodeAffinity != nil || merged.Scheduling.PodAffinity != nil {
						pod.Affinity = &corev1.Affinity{
							NodeAffinity: merged.Scheduling.NodeAffinity,
							PodAffinity:  merged.Scheduling.PodAffinity,
						}
					}
					pod.Tolerations = merged.Scheduling.Tolerations
					return nil
				})
				return nil
			},
		},
		{
			Name:    "VolumeRetention",
			Feature: feature.NewBooleanGate(retainsVolumes(merged)),
			Mutate: func(m *eckelasticsearch.Mutator) error {
				m.RetainVolumesOnDeletion()
				return nil
			},
		},
		{
			Name:    "ServiceAccount",
			Feature: feature.NewBooleanGate(usesServiceAccount(merged, storage)),
			Mutate: func(m *eckelasticsearch.Mutator) error {
				m.Edit(func(es *esv1.Elasticsearch) error {
					es.Spec.NodeSets[0].PodTemplate.Spec.ServiceAccountName = ServiceAccountName(cluster, merged)
					return nil
				})
				return nil
			},
		},
	}

	return append(mutations, snapshotStorageMutations(storage)...)
}

// secureSettings returns the keystore sources of the cluster: the Secret with
// the bucket credentials first, then the sources of the spec. ECK loads every
// one of them into the keystore of every node.
func secureSettings(
	cluster *v1.ElasticsearchCluster,
	merged v1.ElasticsearchClusterSpec,
	storage *SnapshotStorage,
) []commonv1.SecretSource {
	var sources []commonv1.SecretSource
	if storage.keystore() {
		// No entries: the keys of the Secret are already the keystore entry
		// names, so ECK projects each one under its own name.
		sources = append(sources, commonv1.SecretSource{SecretName: KeystoreSecretName(cluster)})
	}

	for _, source := range merged.SecureSettings {
		entries := make([]commonv1.KeyToPath, 0, len(source.Entries))
		for _, entry := range source.Entries {
			entries = append(entries, commonv1.KeyToPath{Key: entry.Key, Path: entry.Path})
		}

		mapped := commonv1.SecretSource{SecretName: source.SecretName}
		if len(entries) > 0 {
			mapped.Entries = entries
		}
		sources = append(sources, mapped)
	}

	return sources
}

// retainsVolumes reports whether the retention policy of the merged spec keeps
// the data volumes when the cluster is deleted.
func retainsVolumes(merged v1.ElasticsearchClusterSpec) bool {
	policy := merged.PersistentVolumeClaimRetentionPolicy
	return policy != nil && policy.WhenDeleted == v1.RetainPersistentVolumeClaimRetentionPolicyType
}

// usesServiceAccount reports whether the pods run under a named
// ServiceAccount, whether or not the operator renders it: the spec asks for
// one, or the bucket authenticates through workload identity — with or
// without an annotation, because an annotation-less identity still binds the
// ServiceAccount by name on the cloud side. The name the pods use is the
// documented principal, so it must exist and be referenced whenever a
// workload-identity bucket is.
func usesServiceAccount(merged v1.ElasticsearchClusterSpec, storage *SnapshotStorage) bool {
	return merged.ServiceAccount != nil || storage.workloadIdentity()
}

// StorageContractComponent builds the storage-contract component: the
// SecondaryStorageConfig that the spec names, published in the namespace of
// the CR. It carries the in-cluster HTTPS endpoint, the reference to the user
// Secret credentials, and the reference to the ECK CA certificate. A
// read-only registration guards the contract on the credentials Secret, and
// blocks while that Secret is absent.
//
// registeredName is the snapshot repository that Elasticsearch last
// confirmed, which the caller reads from status.snapshotRepository. The
// contract carries it only when it is the repository this cluster needs: see
// publishedRepositoryName.
func StorageContractComponent(
	cluster *v1.ElasticsearchCluster,
	merged v1.ElasticsearchClusterSpec,
	storage *SnapshotStorage,
	registeredName string,
) (*component.Component, error) {
	suspended := merged.Suspend
	userSecret, err := secret.NewBuilder(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      UserSecretName(cluster),
			Namespace: cluster.Namespace,
		},
	}).Build()
	if err != nil {
		return nil, err
	}

	contract, err := secondarystorageconfig.NewBuilder(&v1.SecondaryStorageConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      merged.SecondaryStorageConfig,
			Namespace: cluster.Namespace,
			Labels:    managedLabels(cluster),
		},
		Spec: v1.SecondaryStorageConfigSpec{
			Type: v1.SecondaryStorageTypeElasticsearch,
			Elasticsearch: &v1.ElasticsearchStorage{
				Endpoint: HTTPEndpoint(cluster),
				CredentialsSecretRef: v1.LocalCredentialsSecretRef{
					Name:        UserSecretName(cluster),
					UsernameKey: usernameKey,
					PasswordKey: PasswordKey,
				},
				CASecretRef: &v1.LocalSecretKeyRef{
					Name: CACertSecretName(cluster),
					Key:  CACertKey,
				},
				SnapshotRepository: publishedRepositoryName(cluster, storage, registeredName, suspended),
			},
		},
	}).Build()
	if err != nil {
		return nil, err
	}

	return component.NewComponentBuilder().
		WithName("storage-contract").
		WithConditionType(ConditionStorageContract).
		WithResource(userSecret, component.ReadOnly(), component.BlockOnAbsence(), component.Auxiliary()).
		WithResource(contract).
		Build()
}

// HTTPEndpoint returns the in-cluster HTTPS endpoint of the cluster, the
// address that both the published contract and the operator itself use.
func HTTPEndpoint(cluster *v1.ElasticsearchCluster) string {
	return "https://" + cluster.Name + httpServiceSuffix + "." + cluster.Namespace + ".svc:9200"
}

// CACertSecretName returns the name of the Secret in which ECK publishes the
// CA certificate of the cluster. CACertKey is the key inside it.
func CACertSecretName(cluster *v1.ElasticsearchCluster) string {
	return cluster.Name + certsSecretSuffix
}

// publishedRepositoryName returns the repository name to publish in the
// contract, or the empty string when the cluster references no bucket or the
// repository this cluster needs is not registered. The GoDoc of the contract
// field promises a registered repository. A name published before the
// registration converges would send the first snapshot of a consumer into a
// repository that does not exist.
//
// registeredName is the repository that Elasticsearch last confirmed, which
// the cluster records in its status. It is compared against the name this
// cluster needs, not merely tested for emptiness: an operator version that
// derives the name differently must publish nothing until the repository
// under the new name exists.
//
// The record is the last observed convergence, not this reconcile's, so it
// also holds through a suspension. A suspended cluster resolves no bucket
// (storage is nil) and registers nothing, while the repository persists in
// the cluster state that the data volumes retain. Only a cluster that dropped
// its bucket reference while running clears the field.
func publishedRepositoryName(
	cluster *v1.ElasticsearchCluster,
	storage *SnapshotStorage,
	registeredName string,
	suspended bool,
) string {
	name := RepositoryName(cluster)
	if registeredName != name {
		return ""
	}
	if !storage.repository() && !suspended {
		return ""
	}

	return name
}

// RepositoryName returns the name of the snapshot repository that the operator
// registers for the cluster: "<namespace>.<name>". Consumers read it from the
// snapshotRepository field of the published SecondaryStorageConfig rather than
// deriving it.
//
// A snapshot repository is a name on one Elasticsearch server, and two
// clusters of one name in two namespaces can reach one server, so the name
// carries the namespace. RepositoryBasePath reads it back: a registration
// under this name therefore carries one base path, whichever controller
// writes it.
func RepositoryName(cluster *v1.ElasticsearchCluster) string {
	return cluster.Namespace + repositorySeparator + cluster.Name
}

// RepositoryBasePath returns the base path that the snapshot repository named
// repository holds in the bucket whose base path is bucketPath. It reports
// true only when repository can be a name that RepositoryName produced: a
// namespace and a cluster name that Kubernetes itself accepts. Any other name
// holds a prefix that only its own registration knows, so a caller must read
// that registration instead of writing one of its own.
func RepositoryBasePath(bucketPath, repository string) (string, bool) {
	namespace, name, found := strings.Cut(repository, repositorySeparator)
	if !found ||
		len(validation.IsDNS1123Label(namespace)) > 0 ||
		len(validation.IsDNS1123Subdomain(name)) > 0 {
		return "", false
	}

	return logicalbackup.ClusterPrefix(bucketPath, namespace, name), true
}
