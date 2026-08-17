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

	commonv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/common/v1"
	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	"github.com/sourcehawk/operator-component-framework/pkg/mutation/editors"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/secret"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/serviceaccount"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
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
	// keystoreSecretSuffix appended to the CR name yields the name of the
	// Secret that holds the bucket credentials of the node keystore.
	keystoreSecretSuffix = "-es-snapshot-keystore"
	// accessKeyKeystorePath and secretKeyKeystorePath are the keystore entries
	// that the s3 repository client reads its credentials from. Elasticsearch
	// accepts them nowhere else: the settings of a repository never carry
	// credentials.
	accessKeyKeystorePath = "s3.client.default.access_key"
	secretKeyKeystorePath = "s3.client.default.secret_key"
	// usernameKey is the key of the username in the user Secret.
	usernameKey = "username"
	// PasswordKey is the key of the password in the user Secret.
	PasswordKey = "password"
	// rolesKey is the key of the roles in the user Secret.
	rolesKey = "roles"
)

// camundaRole is the definition of the role that the Camunda user holds, in
// the file-based format that ECK loads through spec.auth.roles.
//
// Every privilege is one that Camunda documents as required. The cluster
// privileges: monitor for the cluster health check and the node statistics
// that size a restore; manage_index_templates and manage_ilm to create the
// index templates and lifecycle policies on first start and on an upgrade;
// manage_pipeline for the data migration of an upgrade; create_snapshot and
// monitor_snapshot for the backups, where create_snapshot also covers deleting
// a snapshot, which is what the backup finalizer does. Registering the
// repository is deliberately absent: it needs cluster:admin/repository, and
// the operator does it with the elastic user of ECK instead.
const camundaRole = `camunda:
  cluster:
    - monitor
    - manage_index_templates
    - manage_ilm
    - manage_pipeline
    - create_snapshot
    - monitor_snapshot
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

// managedLabels returns the labels of a resource that the operator applies
// for cluster.
func managedLabels(cluster *v1.ElasticsearchCluster) map[string]string {
	return labels.Managed(labels.ElasticsearchCluster(cluster.Name), componentLabel)
}

// discoveryLabels returns the labels of the pods and data volumes that ECK
// runs from the template of cluster. They carry the owner and component, so
// extensions such as PVCAutoResize can discover them, but not the manager
// label: ECK manages those objects.
func discoveryLabels(cluster *v1.ElasticsearchCluster) map[string]string {
	return labels.Discovery(labels.ElasticsearchCluster(cluster.Name), componentLabel)
}

// UserSecretName returns the name of the file-realm user Secret.
func UserSecretName(cluster *v1.ElasticsearchCluster) string {
	return cluster.Name + userSecretSuffix
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

// CACertKey is the key of the CA certificate inside the Secret that
// CACertSecretName names.
const CACertKey = caCertKey

// RolesSecretName returns the name of the Secret that defines the Camunda
// role.
func RolesSecretName(cluster *v1.ElasticsearchCluster) string {
	return cluster.Name + rolesSecretSuffix
}

// KeystoreSecretName returns the name of the Secret that carries the bucket
// credentials into the keystore of every node.
func KeystoreSecretName(cluster *v1.ElasticsearchCluster) string {
	return cluster.Name + keystoreSecretSuffix
}

// RepositoryName returns the name of the snapshot repository that the operator
// registers for the cluster. Consumers read it from the snapshotRepository
// field of the published SecondaryStorageConfig rather than deriving it.
func RepositoryName(cluster *v1.ElasticsearchCluster) string {
	return cluster.Name
}

// repositoryName returns the repository name to publish in the contract, or
// the empty string when the cluster references no bucket and therefore takes
// no part in backups.
func repositoryName(cluster *v1.ElasticsearchCluster, storage *SnapshotStorage) string {
	if !storage.repository() {
		return ""
	}

	return RepositoryName(cluster)
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

// SnapshotStorage is the resolved snapshot bucket of a cluster: the contract
// that spec.snapshotStorageRef names, and the keys read from its Secret when
// it holds static credentials. A nil SnapshotStorage means the spec names no
// bucket, so the cluster takes no part in backups.
type SnapshotStorage struct {
	// Config is the referenced contract.
	Config *v1.ObjectStorageConfig
	// Credentials are the static keys of the bucket, or nil when the contract
	// uses workload identity.
	Credentials *objectstore.Credentials
}

// identityAnnotations returns the ServiceAccount annotations that bind the
// identity of the bucket, or nil when there is no bucket or no identity.
func (s *SnapshotStorage) identityAnnotations() map[string]string {
	if s == nil {
		return nil
	}

	return s.Config.WorkloadIdentityAnnotations()
}

// keystore reports whether the nodes need a keystore Secret: the bucket holds
// static credentials, which Elasticsearch reads from the keystore alone.
func (s *SnapshotStorage) keystore() bool {
	return s != nil && s.Credentials != nil
}

// repository reports whether the cluster has a snapshot repository to
// register and to publish.
func (s *SnapshotStorage) repository() bool {
	return s != nil && s.Config != nil
}

// rendersServiceAccount reports whether the operator renders the
// ServiceAccount of the pods: the spec asks for one, or a bucket identity has
// to be annotated onto it, and the spec does not name a foreign one.
func rendersServiceAccount(merged v1.ElasticsearchClusterSpec, storage *SnapshotStorage) bool {
	if !merged.ServiceAccount.Creates() {
		return false
	}

	return merged.ServiceAccount != nil || len(storage.identityAnnotations()) > 0
}

// usesServiceAccount reports whether the pods run under a named
// ServiceAccount, whether or not the operator renders it.
func usesServiceAccount(merged v1.ElasticsearchClusterSpec, storage *SnapshotStorage) bool {
	return merged.ServiceAccount != nil || len(storage.identityAnnotations()) > 0
}

// CredentialsComponent builds the credentials component: the basic-auth style
// file-realm Secret with the Camunda user, the given password, and the Camunda
// role, plus the Secret that defines that role. ECK consumes them through
// spec.auth.fileRealm and spec.auth.roles.
func CredentialsComponent(cluster *v1.ElasticsearchCluster, password string) (*component.Component, error) {
	userSecret, err := secret.NewBuilder(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      UserSecretName(cluster),
			Namespace: cluster.Namespace,
			Labels:    managedLabels(cluster),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			usernameKey: []byte(username),
			PasswordKey: []byte(password),
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

// KeystoreComponent builds the keystore component: the Secret whose entries
// ECK loads into the keystore of every node, holding the credentials of the
// snapshot bucket. Elasticsearch reads the credentials of a repository from
// the keystore alone, never from the settings of the repository. The component
// is gated on a bucket with static credentials: with workload identity the
// nodes authenticate through their ServiceAccount and the Secret is deleted.
func KeystoreComponent(
	cluster *v1.ElasticsearchCluster,
	storage *SnapshotStorage,
) (*component.Component, error) {
	data := map[string][]byte{}
	if storage.keystore() {
		data[accessKeyKeystorePath] = []byte(storage.Credentials.AccessKeyID)
		data[secretKeyKeystorePath] = []byte(storage.Credentials.SecretAccessKey)
	}

	keystoreSecret, err := secret.NewBuilder(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      KeystoreSecretName(cluster),
			Namespace: cluster.Namespace,
			Labels:    managedLabels(cluster),
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}).Build()
	if err != nil {
		return nil, err
	}

	return component.NewComponentBuilder().
		WithName("keystore").
		WithConditionType(ConditionKeystore).
		WithResource(keystoreSecret, component.GatedBy(feature.NewBooleanGate(storage.keystore()))).
		Build()
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
		WithResource(
			account,
			component.GatedBy(feature.NewBooleanGate(rendersServiceAccount(merged, storage))),
		).
		WithResource(elasticsearch).
		Suspend(merged.Suspend).
		Build()
}

// StorageContractComponent builds the storage-contract component: the
// SecondaryStorageConfig that the spec names, published in the namespace of
// the CR. It carries the in-cluster HTTPS endpoint, the reference to the user
// Secret credentials, and the reference to the ECK CA certificate. A
// read-only registration guards the contract on the credentials Secret, and
// blocks while that Secret is absent.
func StorageContractComponent(
	cluster *v1.ElasticsearchCluster,
	merged v1.ElasticsearchClusterSpec,
	storage *SnapshotStorage,
) (*component.Component, error) {
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
				CredentialsSecretRef: v1.CredentialsSecretRef{
					Name:        UserSecretName(cluster),
					Namespace:   cluster.Namespace,
					UsernameKey: usernameKey,
					PasswordKey: PasswordKey,
				},
				CASecretRef: &v1.SecretKeyRef{
					Name:      CACertSecretName(cluster),
					Namespace: cluster.Namespace,
					Key:       CACertKey,
				},
				SnapshotRepository: repositoryName(cluster, storage),
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

// elasticsearchMutations layers the optional concerns of the merged spec onto
// the baseline ECK CR. Each mutation is gated on its field.
func elasticsearchMutations(
	cluster *v1.ElasticsearchCluster,
	merged v1.ElasticsearchClusterSpec,
	storage *SnapshotStorage,
) []eckelasticsearch.Mutation {
	secureSettings := secureSettings(cluster, merged, storage)

	return []eckelasticsearch.Mutation{
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
