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

package controller

import (
	"maps"

	commonv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/common/v1"
	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/secret"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/serviceaccount"
	unstructuredstatic "github.com/sourcehawk/operator-component-framework/pkg/primitives/unstructured/static"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/eckelasticsearch"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/secondarystorageconfig"
)

const (
	// clusterLabelKey labels every managed resource — and the Elasticsearch
	// pods and data PVCs through the ECK templates — with the owning CR's
	// name, so extensions such as PVCAutoResize can discover them.
	clusterLabelKey = "camunda.io/cluster"
	// componentLabelKey labels every managed resource with the component it
	// belongs to.
	componentLabelKey = "camunda.io/component"
)

const (
	// escComponentLabel is the componentLabelKey value on everything an
	// ElasticsearchCluster manages.
	escComponentLabel = "elasticsearch"
	// escUserSecretSuffix appended to the CR name yields the file-realm user
	// Secret's name.
	escUserSecretSuffix = "-es-user"
	// escServiceAccountSuffix appended to the CR name yields the pods'
	// ServiceAccount name.
	escServiceAccountSuffix = "-es"
	// escHTTPServiceSuffix appended to the CR name yields the name of the
	// HTTPS service ECK creates for the cluster.
	escHTTPServiceSuffix = "-es-http"
	// escCertsSecretSuffix appended to the CR name yields the name of the
	// Secret ECK publishes the cluster's CA certificate in.
	escCertsSecretSuffix = "-es-http-certs-public"
	// escCACertKey is the CA certificate's key inside the ECK certs Secret.
	escCACertKey = "ca.crt"
	// escNodeSetName names the single node set the operator renders.
	escNodeSetName = "default"
	// escDataVolumeClaimName is ECK's fixed claim name for the data volume;
	// a volumeClaimTemplate under this name overrides ECK's default claim.
	escDataVolumeClaimName = "elasticsearch-data"
	// escContainerName is ECK's fixed name for the Elasticsearch container.
	escContainerName = "elasticsearch"
	// escUsername is the file-realm user the operator provisions for Camunda.
	escUsername = "camunda"
	// escUserRole is the role granted to the Camunda user. It is superuser
	// for now: snapshot-repository registration by the CamundaCluster
	// controller needs cluster-manage rights; narrowing is deferred until
	// that flow lands (Batch C).
	escUserRole = "superuser"
	// escUsernameKey is the username's key in the user Secret.
	escUsernameKey = "username"
	// escPasswordKey is the password's key in the user Secret.
	escPasswordKey = "password"
	// escRolesKey is the roles' key in the user Secret.
	escRolesKey = "roles"
)

const (
	// escConditionCredentials is the credentials component's condition type.
	escConditionCredentials = "CredentialsReady"
	// escConditionElasticsearch is the elasticsearch component's condition
	// type.
	escConditionElasticsearch = "ElasticsearchReady"
	// escConditionStorageContract is the storage-contract component's
	// condition type.
	escConditionStorageContract = "StorageContractReady"
)

// escLabels returns the discovery labels every resource an
// ElasticsearchCluster manages carries.
func escLabels(cluster *v1.ElasticsearchCluster) map[string]string {
	return map[string]string{
		clusterLabelKey:   cluster.Name,
		componentLabelKey: escComponentLabel,
	}
}

// escUserSecretName returns the name of the file-realm user Secret.
func escUserSecretName(cluster *v1.ElasticsearchCluster) string {
	return cluster.Name + escUserSecretSuffix
}

// escCredentialsComponent builds the credentials component: the basic-auth
// style file-realm Secret carrying the Camunda user, the given password, and
// the superuser role grant, consumed by ECK through spec.auth.fileRealm.
func escCredentialsComponent(cluster *v1.ElasticsearchCluster, password string) (*component.Component, error) {
	userSecret, err := secret.NewBuilder(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      escUserSecretName(cluster),
			Namespace: cluster.Namespace,
			Labels:    escLabels(cluster),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			escUsernameKey: []byte(escUsername),
			escPasswordKey: []byte(password),
			escRolesKey:    []byte(escUserRole),
		},
	}).Build()
	if err != nil {
		return nil, err
	}

	return component.NewComponentBuilder().
		WithName("credentials").
		WithConditionType(escConditionCredentials).
		WithResource(userSecret).
		Build()
}

// escElasticsearchComponent builds the elasticsearch component from the
// preset-merged spec: the pods' ServiceAccount (gated on
// spec.serviceAccount), the ECK Elasticsearch CR, and the ServiceMonitor
// (gated on monitoring.serviceMonitor.enabled). serviceMonitorSupported
// reports whether the cluster serves the ServiceMonitor kind; when false the
// resource is omitted entirely so reconciliation never touches a missing
// kind. spec.suspend suspends the component, which scales the node set to
// zero through the ECK wrapper's suspend mutation.
func escElasticsearchComponent(
	cluster *v1.ElasticsearchCluster,
	merged v1.ElasticsearchClusterSpec,
	serviceMonitorSupported bool,
) (*component.Component, error) {
	account, err := serviceaccount.NewBuilder(escServiceAccount(cluster, merged)).Build()
	if err != nil {
		return nil, err
	}

	elasticsearch, err := eckelasticsearch.NewBuilder(escElasticsearch(cluster, merged)).
		WithMutation(escElasticsearchMutations(merged)...).
		Build()
	if err != nil {
		return nil, err
	}

	builder := component.NewComponentBuilder().
		WithName("elasticsearch").
		WithConditionType(escConditionElasticsearch).
		WithResource(account, component.GatedBy(feature.NewBooleanGate(merged.ServiceAccount != nil))).
		WithResource(elasticsearch).
		Suspend(merged.Suspend)

	if serviceMonitorSupported {
		serviceMonitor, err := unstructuredstatic.NewBuilder(escServiceMonitor(cluster, merged)).Build()
		if err != nil {
			return nil, err
		}
		enabled := merged.Monitoring != nil &&
			merged.Monitoring.ServiceMonitor != nil &&
			merged.Monitoring.ServiceMonitor.Enabled
		builder.WithResource(serviceMonitor, component.GatedBy(feature.NewBooleanGate(enabled)))
	}

	return builder.Build()
}

// escStorageContractComponent builds the storage-contract component: the
// SecondaryStorageConfig named by the spec, published in the CR's own
// namespace with the in-cluster HTTPS endpoint, the user Secret credentials
// reference, and the ECK CA certificate reference. The contract is guarded on
// the credentials Secret existing through a read-only registration that
// blocks on its absence.
func escStorageContractComponent(
	cluster *v1.ElasticsearchCluster,
	merged v1.ElasticsearchClusterSpec,
) (*component.Component, error) {
	userSecret, err := secret.NewBuilder(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      escUserSecretName(cluster),
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
			Labels:    escLabels(cluster),
		},
		Spec: v1.SecondaryStorageConfigSpec{
			Type: v1.SecondaryStorageTypeElasticsearch,
			Elasticsearch: &v1.ElasticsearchStorage{
				Endpoint: "https://" + cluster.Name + escHTTPServiceSuffix + "." + cluster.Namespace + ".svc:9200",
				CredentialsSecretRef: v1.CredentialsSecretRef{
					Name:        escUserSecretName(cluster),
					Namespace:   cluster.Namespace,
					UsernameKey: escUsernameKey,
					PasswordKey: escPasswordKey,
				},
				CASecretRef: &v1.SecretKeyRef{
					Name:      cluster.Name + escCertsSecretSuffix,
					Namespace: cluster.Namespace,
					Key:       escCACertKey,
				},
			},
		},
	}).Build()
	if err != nil {
		return nil, err
	}

	return component.NewComponentBuilder().
		WithName("storage-contract").
		WithConditionType(escConditionStorageContract).
		WithResource(userSecret, component.ReadOnly(), component.BlockOnAbsence(), component.Auxiliary()).
		WithResource(contract).
		Build()
}

// escServiceAccount renders the pods' ServiceAccount carrying the
// workload-identity annotations from spec.serviceAccount.
func escServiceAccount(cluster *v1.ElasticsearchCluster, merged v1.ElasticsearchClusterSpec) *corev1.ServiceAccount {
	var annotations map[string]string
	if merged.ServiceAccount != nil {
		annotations = merged.ServiceAccount.Annotations
	}

	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cluster.Name + escServiceAccountSuffix,
			Namespace:   cluster.Namespace,
			Labels:      escLabels(cluster),
			Annotations: annotations,
		},
	}
}

// escElasticsearch renders the baseline ECK Elasticsearch CR: version, the
// single default node set with the merged replica count, the labeled pod
// template and data volume claim, and the file-realm auth block. Optional
// concerns (resources, env, pod metadata, scheduling, service account) are
// layered by escElasticsearchMutations.
func escElasticsearch(cluster *v1.ElasticsearchCluster, merged v1.ElasticsearchClusterSpec) *esv1.Elasticsearch {
	labels := escLabels(cluster)

	return &esv1.Elasticsearch{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
			Labels:    labels,
		},
		Spec: esv1.ElasticsearchSpec{
			Version: merged.Version,
			Auth: esv1.Auth{
				FileRealm: []esv1.FileRealmSource{
					{SecretRef: commonv1.SecretRef{SecretName: escUserSecretName(cluster)}},
				},
			},
			NodeSets: []esv1.NodeSet{{
				Name:  escNodeSetName,
				Count: *merged.Replicas,
				PodTemplate: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: maps.Clone(labels)},
				},
				VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
					ObjectMeta: metav1.ObjectMeta{
						Name:   escDataVolumeClaimName,
						Labels: maps.Clone(labels),
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

// escElasticsearchMutations layers the optional concerns of the merged spec
// onto the baseline ECK CR, each gated on its field being set.
func escElasticsearchMutations(merged v1.ElasticsearchClusterSpec) []eckelasticsearch.Mutation {
	return []eckelasticsearch.Mutation{
		{
			Name:    "NodeResources",
			Feature: feature.NewBooleanGate(merged.Resources != nil),
			Mutate: func(m *eckelasticsearch.Mutator) error {
				m.Edit(func(es *esv1.Elasticsearch) error {
					ensureESContainer(&es.Spec.NodeSets[0].PodTemplate.Spec).Resources = *merged.Resources
					return nil
				})
				return nil
			},
		},
		{
			Name:    "ExtraEnvironment",
			Feature: feature.NewBooleanGate(len(merged.ExtraEnv) > 0 || len(merged.ExtraEnvFrom) > 0),
			Mutate: func(m *eckelasticsearch.Mutator) error {
				m.Edit(func(es *esv1.Elasticsearch) error {
					container := ensureESContainer(&es.Spec.NodeSets[0].PodTemplate.Spec)
					container.Env = merged.ExtraEnv
					container.EnvFrom = merged.ExtraEnvFrom
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
					maps.Copy(template.Labels, merged.PodLabels)
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
			Name:    "ServiceAccount",
			Feature: feature.NewBooleanGate(merged.ServiceAccount != nil),
			Mutate: func(m *eckelasticsearch.Mutator) error {
				m.Edit(func(es *esv1.Elasticsearch) error {
					es.Spec.NodeSets[0].PodTemplate.Spec.ServiceAccountName = es.Name + escServiceAccountSuffix
					return nil
				})
				return nil
			},
		},
	}
}

// ensureESContainer returns the pod spec's elasticsearch container, adding
// the entry when the template does not carry one yet. The returned pointer
// aliases the slice element, so edits land on the pod spec.
func ensureESContainer(pod *corev1.PodSpec) *corev1.Container {
	for i := range pod.Containers {
		if pod.Containers[i].Name == escContainerName {
			return &pod.Containers[i]
		}
	}

	pod.Containers = append(pod.Containers, corev1.Container{Name: escContainerName})
	return &pod.Containers[len(pod.Containers)-1]
}

// escServiceMonitor renders the Prometheus ServiceMonitor for the ECK HTTPS
// service as unstructured content, since the operator does not depend on the
// prometheus-operator API types. It scrapes the https port with the Camunda
// user's basic auth and verifies TLS against the ECK-published CA.
func escServiceMonitor(cluster *v1.ElasticsearchCluster, merged v1.ElasticsearchClusterSpec) *unstructured.Unstructured {
	labels := map[string]any{}
	for k, v := range escLabels(cluster) {
		labels[k] = v
	}
	annotations := map[string]any{}
	if merged.Monitoring != nil && merged.Monitoring.ServiceMonitor != nil {
		for k, v := range merged.Monitoring.ServiceMonitor.Labels {
			labels[k] = v
		}
		for k, v := range merged.Monitoring.ServiceMonitor.Annotations {
			annotations[k] = v
		}
	}

	metadata := map[string]any{
		"name":      cluster.Name + escServiceAccountSuffix,
		"namespace": cluster.Namespace,
		"labels":    labels,
	}
	if len(annotations) > 0 {
		metadata["annotations"] = annotations
	}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "monitoring.coreos.com/v1",
		"kind":       "ServiceMonitor",
		"metadata":   metadata,
		"spec": map[string]any{
			"selector": map[string]any{
				"matchLabels": map[string]any{
					"common.k8s.elastic.co/type":                escComponentLabel,
					"elasticsearch.k8s.elastic.co/cluster-name": cluster.Name,
				},
			},
			"endpoints": []any{map[string]any{
				"port":   "https",
				"scheme": "https",
				"basicAuth": map[string]any{
					"username": map[string]any{"name": escUserSecretName(cluster), "key": escUsernameKey},
					"password": map[string]any{"name": escUserSecretName(cluster), "key": escPasswordKey},
				},
				"tlsConfig": map[string]any{
					"serverName": cluster.Name + escHTTPServiceSuffix + "." + cluster.Namespace + ".svc",
					"ca": map[string]any{
						"secret": map[string]any{
							"name": cluster.Name + escCertsSecretSuffix,
							"key":  escCACertKey,
						},
					},
				},
			}},
		},
	}}
}
