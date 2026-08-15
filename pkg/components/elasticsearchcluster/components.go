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
// credentials Secret, the ECK Elasticsearch CR, and the SecondaryStorageConfig
// binding. Everything here is pure: spec in, resources out, no API calls. The
// controller in internal/controller drives it.
package elasticsearchcluster

import (
	"maps"

	commonv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/common/v1"
	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	"github.com/sourcehawk/operator-component-framework/pkg/mutation/editors"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/secret"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/serviceaccount"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/eckelasticsearch"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/secondarystorageconfig"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/servicemonitor"
)

const (
	// clusterLabelKey labels every managed resource with the name of the
	// owning CR. Through the ECK templates it also labels the Elasticsearch
	// pods and data PVCs, so extensions such as PVCAutoResize can discover
	// them.
	clusterLabelKey = "camunda.io/cluster"
	// componentLabelKey labels every managed resource with the component it
	// belongs to.
	componentLabelKey = "camunda.io/component"
)

const (
	// componentLabel is the componentLabelKey value on everything that an
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
	// userRole is the role that the Camunda user holds. It is superuser for
	// now, because snapshot-repository registration by the CamundaCluster
	// controller needs cluster-manage rights. The role is narrowed when that
	// flow lands (Batch C).
	userRole = "superuser"
	// usernameKey is the key of the username in the user Secret.
	usernameKey = "username"
	// PasswordKey is the key of the password in the user Secret.
	PasswordKey = "password"
	// rolesKey is the key of the roles in the user Secret.
	rolesKey = "roles"
)

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
)

// clusterLabels returns the discovery labels that every resource of an
// ElasticsearchCluster carries.
func clusterLabels(cluster *v1.ElasticsearchCluster) map[string]string {
	return map[string]string{
		clusterLabelKey:   cluster.Name,
		componentLabelKey: componentLabel,
	}
}

// UserSecretName returns the name of the file-realm user Secret.
func UserSecretName(cluster *v1.ElasticsearchCluster) string {
	return cluster.Name + userSecretSuffix
}

// CredentialsComponent builds the credentials component: the basic-auth style
// file-realm Secret with the Camunda user, the given password, and the
// superuser role. ECK consumes it through spec.auth.fileRealm.
func CredentialsComponent(cluster *v1.ElasticsearchCluster, password string) (*component.Component, error) {
	userSecret, err := secret.NewBuilder(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      UserSecretName(cluster),
			Namespace: cluster.Namespace,
			Labels:    clusterLabels(cluster),
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

	return component.NewComponentBuilder().
		WithName("credentials").
		WithConditionType(ConditionCredentials).
		WithResource(userSecret).
		Build()
}

// ElasticsearchComponent builds the elasticsearch component from the
// preset-merged spec: the ServiceAccount of the pods (gated on
// spec.serviceAccount), the ECK Elasticsearch CR, and the ServiceMonitor
// (gated on monitoring.serviceMonitor.enabled). serviceMonitorSupported
// reports whether the cluster serves the ServiceMonitor kind. When it is
// false, the resource is omitted, so the reconcile never touches a missing
// kind. spec.suspend suspends the component, which scales the node set to
// zero through the suspend mutation of the ECK wrapper.
func ElasticsearchComponent(
	cluster *v1.ElasticsearchCluster,
	merged v1.ElasticsearchClusterSpec,
	serviceMonitorSupported bool,
) (*component.Component, error) {
	account, err := serviceaccount.NewBuilder(serviceAccount(cluster, merged)).Build()
	if err != nil {
		return nil, err
	}

	elasticsearch, err := eckelasticsearch.NewBuilder(elasticsearch(cluster, merged)).
		WithMutation(elasticsearchMutations(merged)...).
		Build()
	if err != nil {
		return nil, err
	}

	monitor, err := servicemonitor.NewBuilder(serviceMonitor(cluster, merged)).Build()
	if err != nil {
		return nil, err
	}
	monitorEnabled := merged.Monitoring != nil &&
		merged.Monitoring.ServiceMonitor != nil &&
		merged.Monitoring.ServiceMonitor.Enabled

	// The ServiceMonitor is included only where the cluster serves the kind
	// and never deleted by inclusion alone: when the CRD goes away, the API
	// server removes every instance, and a delete against a missing kind
	// fails. While the kind is served, the gate deletes it when monitoring is
	// disabled.
	return component.NewComponentBuilder().
		WithName("elasticsearch").
		WithConditionType(ConditionElasticsearch).
		WithResource(account, component.GatedBy(feature.NewBooleanGate(merged.ServiceAccount != nil))).
		WithResource(elasticsearch).
		IncludeWhen(
			serviceMonitorSupported,
			func() component.Resource { return monitor },
			component.GatedBy(feature.NewBooleanGate(monitorEnabled)),
		).
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
			Labels:    clusterLabels(cluster),
		},
		Spec: v1.SecondaryStorageConfigSpec{
			Type: v1.SecondaryStorageTypeElasticsearch,
			Elasticsearch: &v1.ElasticsearchStorage{
				Endpoint: "https://" + cluster.Name + httpServiceSuffix + "." + cluster.Namespace + ".svc:9200",
				CredentialsSecretRef: v1.CredentialsSecretRef{
					Name:        UserSecretName(cluster),
					Namespace:   cluster.Namespace,
					UsernameKey: usernameKey,
					PasswordKey: PasswordKey,
				},
				CASecretRef: &v1.SecretKeyRef{
					Name:      cluster.Name + certsSecretSuffix,
					Namespace: cluster.Namespace,
					Key:       caCertKey,
				},
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

// serviceAccount renders the ServiceAccount of the pods with the
// workload-identity annotations from spec.serviceAccount.
func serviceAccount(cluster *v1.ElasticsearchCluster, merged v1.ElasticsearchClusterSpec) *corev1.ServiceAccount {
	var annotations map[string]string
	if merged.ServiceAccount != nil {
		annotations = merged.ServiceAccount.Annotations
	}

	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cluster.Name + serviceAccountSuffix,
			Namespace:   cluster.Namespace,
			Labels:      clusterLabels(cluster),
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
	labels := clusterLabels(cluster)

	return &esv1.Elasticsearch{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
			Labels:    labels,
		},
		Spec: esv1.ElasticsearchSpec{
			Version: merged.Version,
			// The data volumes outlive the Elasticsearch CR: suspension deletes
			// the CR and re-creation reattaches the volumes, and deleting the
			// ElasticsearchCluster never erases data on its own.
			VolumeClaimDeletePolicy: esv1.DeleteOnScaledownOnlyPolicy,
			Auth: esv1.Auth{
				FileRealm: []esv1.FileRealmSource{
					{SecretRef: commonv1.SecretRef{SecretName: UserSecretName(cluster)}},
				},
			},
			NodeSets: []esv1.NodeSet{{
				Name:  nodeSetName,
				Count: *merged.Replicas,
				PodTemplate: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: maps.Clone(labels)},
				},
				VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
					ObjectMeta: metav1.ObjectMeta{
						Name:   DataVolumeClaimName,
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

// elasticsearchMutations layers the optional concerns of the merged spec onto
// the baseline ECK CR. Each mutation is gated on its field.
func elasticsearchMutations(merged v1.ElasticsearchClusterSpec) []eckelasticsearch.Mutation {
	return []eckelasticsearch.Mutation{
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
					es.Spec.NodeSets[0].PodTemplate.Spec.ServiceAccountName = es.Name + serviceAccountSuffix
					return nil
				})
				return nil
			},
		},
	}
}

// serviceMonitor renders the Prometheus ServiceMonitor for the ECK HTTPS
// service. It scrapes the https port with the basic auth of the Camunda user
// and checks TLS against the CA that ECK publishes.
func serviceMonitor(cluster *v1.ElasticsearchCluster, merged v1.ElasticsearchClusterSpec) *monitoringv1.ServiceMonitor {
	labels := clusterLabels(cluster)
	var annotations map[string]string
	if merged.Monitoring != nil && merged.Monitoring.ServiceMonitor != nil {
		maps.Copy(labels, merged.Monitoring.ServiceMonitor.Labels)
		annotations = merged.Monitoring.ServiceMonitor.Annotations
	}

	return &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cluster.Name + serviceAccountSuffix,
			Namespace:   cluster.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{MatchLabels: map[string]string{
				"common.k8s.elastic.co/type": componentLabel,
				ECKClusterNameLabel:          cluster.Name,
			}},
			Endpoints: []monitoringv1.Endpoint{{
				Port: "https",
				// Lowercase: the CRD enum of every prometheus-operator release
				// accepts it, and older releases accept nothing else.
				Scheme: new(monitoringv1.Scheme("https")),
				HTTPConfigWithProxyAndTLSFiles: monitoringv1.HTTPConfigWithProxyAndTLSFiles{
					HTTPConfigWithTLSFiles: monitoringv1.HTTPConfigWithTLSFiles{
						HTTPConfigWithoutTLS: monitoringv1.HTTPConfigWithoutTLS{
							BasicAuth: &monitoringv1.BasicAuth{
								Username: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: UserSecretName(cluster)},
									Key:                  usernameKey,
								},
								Password: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: UserSecretName(cluster)},
									Key:                  PasswordKey,
								},
							},
						},
						TLSConfig: &monitoringv1.TLSConfig{
							SafeTLSConfig: monitoringv1.SafeTLSConfig{
								ServerName: new(cluster.Name + httpServiceSuffix + "." + cluster.Namespace + ".svc"),
								CA: monitoringv1.SecretOrConfigMap{
									Secret: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: cluster.Name + certsSecretSuffix,
										},
										Key: caCertKey,
									},
								},
							},
						},
					},
				},
			}},
		},
	}
}
