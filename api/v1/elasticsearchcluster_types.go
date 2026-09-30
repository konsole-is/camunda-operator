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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceAccountSpec configures the ServiceAccount of the pods of a resource.
type ServiceAccountSpec struct {
	// Name is the name of the ServiceAccount. When empty, the operator
	// derives the name from the name of the resource, as the doc of each kind
	// states. The cloud provider uses this name. A workload identity that
	// needs no annotation, such as EKS Pod Identity, binds the principal
	// system:serviceaccount:<namespace>:<name>.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=253
	// +optional
	Name string `json:"name,omitempty"`
	// Create makes the operator create and own the ServiceAccount. Defaults
	// to true. False names a ServiceAccount that already exists. The operator
	// then does not create, annotate, or own it. If it does not exist, Ready
	// reports InvalidReference, except on a suspended ElasticsearchCluster. The operator never takes ownership of a
	// ServiceAccount that it did not create, because it deletes an owned one
	// with the resource.
	// +optional
	Create *bool `json:"create,omitempty"`
	// Annotations to set on the ServiceAccount. Usually these are
	// workload-identity annotations (IRSA, GCP Workload Identity, and more)
	// that give the pods access to cloud resources, such as the snapshot
	// bucket for backups. The operator also adds the identity annotation of
	// each bucket contract that names an identity. An annotation set here
	// wins over the operator annotation with the same key.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// Creates reports whether the operator renders and owns the ServiceAccount.
// An absent spec and an unset create both mean true.
func (in *ServiceAccountSpec) Creates() bool {
	return in == nil || in.Create == nil || *in.Create
}

// SecureSettingEntry projects one key of a Secret to a named entry of the
// Elasticsearch keystore.
type SecureSettingEntry struct {
	// Key in the Secret.
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
	// Path is the keystore entry that the key becomes, for example
	// s3.client.default.access_key.
	// +kubebuilder:validation:MinLength=1
	Path string `json:"path"`
}

// SecureSettingsSource references a Secret that ECK loads into the keystore
// of every Elasticsearch node. Elasticsearch reads credentials only from the
// keystore, never from the settings of a snapshot repository.
type SecureSettingsSource struct {
	// SecretName is the Secret, in the namespace of the ElasticsearchCluster.
	// +kubebuilder:validation:MinLength=1
	SecretName string `json:"secretName"`
	// Entries maps single keys to keystore entries. An empty list loads
	// every key of the Secret under its own name.
	// +optional
	Entries []SecureSettingEntry `json:"entries,omitempty"`
}

// SchedulingSpec holds the scheduling constraints of the pods of a resource.
// When a resource sets its own block, it replaces the complete scheduling
// block of its preset. The two blocks never merge field by field.
type SchedulingSpec struct {
	// NodeAffinity rules for the pods.
	// +optional
	NodeAffinity *corev1.NodeAffinity `json:"nodeAffinity,omitempty"`
	// PodAffinity rules for the pods.
	// +optional
	PodAffinity *corev1.PodAffinity `json:"podAffinity,omitempty"`
	// Tolerations for the pods.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// ServiceMonitorSpec configures the Prometheus ServiceMonitors of a resource
// that runs workloads. The kind of the resource sets what Prometheus scrapes.
// Elasticsearch serves no Prometheus endpoint, so an ElasticsearchCluster
// deploys the prometheus-community elasticsearch_exporter and scrapes it. A
// CamundaCluster scrapes /actuator/prometheus of every process. The operator
// creates the ServiceMonitor only when the Kubernetes cluster serves the kind.
type ServiceMonitorSpec struct {
	// Enabled creates the ServiceMonitors (and, for an ElasticsearchCluster,
	// the exporter) when true. Defaults to false.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// Labels are extra labels applied to the ServiceMonitor.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations are extra annotations applied to the ServiceMonitor.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ExporterSpec tunes the elasticsearch_exporter Deployment that
// ServiceMonitorSpec.Enabled deploys.
type ExporterSpec struct {
	// Image overrides the exporter image. Defaults to the pinned
	// quay.io/prometheuscommunity/elasticsearch-exporter release of the
	// operator.
	// +optional
	Image string `json:"image,omitempty"`
	// Resources are the CPU and memory of the exporter container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// MonitoringSpec groups the Prometheus scraping integration of the
// Elasticsearch cluster.
type MonitoringSpec struct {
	// ServiceMonitor configures the exporter and the Prometheus
	// ServiceMonitor.
	// +optional
	ServiceMonitor *ServiceMonitorSpec `json:"serviceMonitor,omitempty"`
	// Exporter tunes the exporter Deployment.
	// +optional
	Exporter *ExporterSpec `json:"exporter,omitempty"`
}

// ElasticsearchClusterSpec defines the desired state of ElasticsearchCluster.
//
// An ElasticsearchClusterPreset uses the same type as its baseline. For this
// reason, the schema marks secondaryStorageConfig as optional, and the
// ElasticsearchCluster requires it. A preset refuses the fields that belong
// to one cluster only. A preset also refuses the version, which belongs to a
// CamundaRelease.
type ElasticsearchClusterSpec struct {
	// PresetRef names a cluster-scoped ElasticsearchClusterPreset to use as
	// the configuration baseline. A field set on the cluster replaces the
	// value of the preset for that field completely.
	// +optional
	PresetRef string `json:"presetRef,omitempty"`
	// ReleaseRef names a cluster-scoped CamundaRelease that provides the
	// Elasticsearch version. It merges over the preset and under this spec.
	// Forbidden in a preset.
	// +optional
	ReleaseRef string `json:"releaseRef,omitempty"`
	// Version is the Elasticsearch version to deploy, as a full semantic
	// version. Camunda 8.9 supports Elasticsearch 8.19 and later, and 9.2 and
	// later. The operator refuses a lower version, also when it comes from
	// the release, with Ready reason InvalidReference. Required unless the
	// resolved release provides it. Forbidden in a preset.
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+$`
	// +optional
	Version string `json:"version,omitempty"`
	// Replicas is the number of Elasticsearch nodes. Required unless the
	// resolved preset provides it.
	// +kubebuilder:validation:Minimum=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// Resources are the CPU and memory for each Elasticsearch node.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// StorageSize is the size of the data volume of each node. Required
	// unless the resolved preset provides it. It cannot shrink, because an
	// Elasticsearch data volume cannot become smaller in place. The API
	// server refuses a change of this field in an ElasticsearchCluster to a
	// smaller value. A smaller value is accepted when the field was not set
	// before, or when a preset lowers the size. A cluster whose volumes are
	// already larger then keeps that size and records a StorageShrinkIgnored
	// event.
	// +optional
	StorageSize *resource.Quantity `json:"storageSize,omitempty"`
	// StorageClassName is the StorageClass of the data volumes. Defaults to
	// the default StorageClass of the Kubernetes cluster.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
	// ServiceAccount configures the ServiceAccount of the Elasticsearch pods.
	// +optional
	ServiceAccount *ServiceAccountSpec `json:"serviceAccount,omitempty"`
	// ExtraEnv are extra environment variables for every Elasticsearch node.
	// +optional
	ExtraEnv []corev1.EnvVar `json:"extraEnv,omitempty"`
	// ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) for
	// every Elasticsearch node.
	// +optional
	ExtraEnvFrom []corev1.EnvFromSource `json:"extraEnvFrom,omitempty"`
	// PodLabels are extra labels applied to the Elasticsearch pods.
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`
	// PodAnnotations are extra annotations applied to the Elasticsearch pods.
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`
	// Scheduling holds the scheduling constraints of the Elasticsearch pods.
	// When set, it replaces the scheduling block of the preset, with no merge.
	// +optional
	Scheduling *SchedulingSpec `json:"scheduling,omitempty"`
	// SnapshotStorageRef names an ObjectStorageConfig, in the namespace of
	// this cluster, for the bucket of the snapshot repository of this
	// cluster. When it is set, the operator does all the Elasticsearch work
	// for that bucket. It gives the nodes their credentials, registers the
	// repository, and publishes the repository name in the
	// SecondaryStorageConfig. A CamundaCluster on this storage needs it for
	// backups. The CamundaCluster must reference the same bucket.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=253
	// +optional
	SnapshotStorageRef string `json:"snapshotStorageRef,omitempty"`
	// SecureSettings are Secrets that ECK loads into the keystore of every
	// node. The operator adds the credentials of snapshotStorageRef to the
	// keystore itself. Use this field for all other keystore entries.
	// +optional
	SecureSettings []SecureSettingsSource `json:"secureSettings,omitempty"`
	// SecondaryStorageConfig names the SecondaryStorageConfig that the
	// operator creates in the namespace of this cluster. It holds the
	// connection details and the generated credentials. Required on an
	// ElasticsearchCluster, forbidden in a preset.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=253
	// +optional
	SecondaryStorageConfig string `json:"secondaryStorageConfig,omitempty"`
	// Monitoring configures the Prometheus scraping integration.
	// +optional
	Monitoring *MonitoringSpec `json:"monitoring,omitempty"`
	// PersistentVolumeClaimRetentionPolicy says what happens to the data
	// volumes when the ElasticsearchCluster is deleted. Suspension always
	// keeps them.
	// +optional
	PersistentVolumeClaimRetentionPolicy *PersistentVolumeClaimRetentionPolicy `json:"persistentVolumeClaimRetentionPolicy,omitempty"`
	// Suspend stops the Elasticsearch cluster and keeps its data volumes.
	// Defaults to false. The operator deletes the ECK Elasticsearch resource
	// and keeps the volumes. When you set the field back to false, the
	// operator creates the resource again, and ECK attaches the volumes
	// again.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// ReasonECKNotInstalled is the Ready reason of an ElasticsearchCluster on a
// cluster that did not serve the ECK Elasticsearch kind when the operator
// started. Restart the operator after you install ECK.
const ReasonECKNotInstalled = "ECKNotInstalled"

// PersistentVolumeClaimRetentionPolicyType is what happens to the data
// volumes of a resource (an ElasticsearchCluster, the brokers of a
// CamundaCluster) when the resource is deleted.
// +kubebuilder:validation:Enum=Retain;Delete
type PersistentVolumeClaimRetentionPolicyType string

const (
	// RetainPersistentVolumeClaimRetentionPolicyType keeps the data volumes.
	// You remove the data manually. For an ElasticsearchCluster, the ECK
	// resource has the volume claim delete policy DeleteOnScaledownOnly. For
	// a CamundaCluster, the broker StatefulSet has whenDeleted Retain.
	RetainPersistentVolumeClaimRetentionPolicyType PersistentVolumeClaimRetentionPolicyType = "Retain"
	// DeletePersistentVolumeClaimRetentionPolicyType deletes the data volumes
	// with the resource. For an ElasticsearchCluster, the ECK resource has
	// the volume claim delete policy DeleteOnScaledownAndClusterDeletion, the
	// ECK default. For a CamundaCluster, the broker StatefulSet has
	// whenDeleted Delete.
	DeletePersistentVolumeClaimRetentionPolicyType PersistentVolumeClaimRetentionPolicyType = "Delete"
)

// PersistentVolumeClaimRetentionPolicy is like the StatefulSet field of the
// same name, with only whenDeleted. There is no whenScaled choice. ECK
// deletes the volume of each Elasticsearch node that it removes in a scale
// down. The operator always keeps the volume of a broker that it removes in
// a scale down.
type PersistentVolumeClaimRetentionPolicy struct {
	// WhenDeleted is what happens to the data volumes when the resource is
	// deleted. Defaults to Delete. Delete removes them with the resource.
	// Retain keeps them, and a later resource with the same name attaches
	// them again.
	// +kubebuilder:default=Delete
	// +optional
	WhenDeleted PersistentVolumeClaimRetentionPolicyType `json:"whenDeleted,omitempty"`
}

// ElasticsearchClusterStatus is the observed state of an ElasticsearchCluster.
type ElasticsearchClusterStatus struct {
	// ObservedGeneration is the last generation that the operator processed.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Version is the effective Elasticsearch version of the cluster, as a
	// full semantic version. It comes from the merged spec, so it is correct
	// when the release or the cluster gives the version. During
	// an upgrade, or when the operator cannot apply the change, the old
	// version can still run. It is empty
	// until the operator resolves the references of the cluster for the first
	// time.
	// +optional
	Version string `json:"version,omitempty"`
	// Volumes lists the bound data PersistentVolumeClaims of the cluster and
	// the capacity that each one reports, sorted by name.
	// +listType=map
	// +listMapKey=name
	// +optional
	Volumes []VolumeStatus `json:"volumes,omitempty"`
	// SnapshotRepository is the snapshot repository that the operator
	// registered in Elasticsearch for this cluster. The published
	// SecondaryStorageConfig holds the same name. It is empty until the first
	// registration succeeds. It shows the last registration that succeeded,
	// not a new check that the repository still exists.
	// +optional
	SnapshotRepository string `json:"snapshotRepository,omitempty"`
	// Conditions represent the current state. Ready holds the reason of a
	// failed pre-check (InvalidReference, MissingSecret, ECKNotInstalled).
	// Otherwise it follows the component conditions and, when the cluster or
	// its preset sets snapshotStorageRef, SnapshotRepositoryReady. The
	// per-component conditions (CredentialsReady, KeystoreReady,
	// ElasticsearchReady, StorageContractReady) also appear here.
	// MetricsReady reports the exporter and never affects Ready.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.status.version`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ElasticsearchCluster runs an Elasticsearch cluster for secondary storage,
// through the external ECK operator. It publishes the connection details and
// generated credentials as a SecondaryStorageConfig.
type ElasticsearchCluster struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ElasticsearchCluster
	// +kubebuilder:validation:XValidation:rule="has(self.secondaryStorageConfig)",message="secondaryStorageConfig is required"
	// +kubebuilder:validation:XValidation:rule="!has(oldSelf.storageSize) || !has(self.storageSize) || !quantity(string(self.storageSize)).isLessThan(quantity(string(oldSelf.storageSize)))",message="storageSize may not be shrunk"
	// +required
	Spec ElasticsearchClusterSpec `json:"spec"`

	// status defines the observed state of ElasticsearchCluster
	// +optional
	Status ElasticsearchClusterStatus `json:"status,omitzero"`
}

// GetStatusConditions returns a pointer to the status conditions. The
// component framework stages per-component conditions on the resource through
// it.
func (in *ElasticsearchCluster) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// GetKind returns the CRD kind. The component framework uses it for event
// and metric recording and derives its per-component SSA field managers
// (ElasticsearchCluster/<component>) from it.
func (in *ElasticsearchCluster) GetKind() string { return "ElasticsearchCluster" }

// SetObservedGeneration records the last reconciled generation in status.
func (in *ElasticsearchCluster) SetObservedGeneration(generation int64) {
	in.Status.ObservedGeneration = generation
}

// +kubebuilder:object:root=true

// ElasticsearchClusterList contains a list of ElasticsearchCluster
type ElasticsearchClusterList struct {
	metav1.TypeMeta `                       json:",inline"`
	metav1.ListMeta `                       json:"metadata,omitzero"`
	Items           []ElasticsearchCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ElasticsearchCluster{}, &ElasticsearchClusterList{})
}
