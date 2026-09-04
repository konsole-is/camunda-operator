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

// ServiceAccountSpec configures the ServiceAccount of the pods a controller
// manages.
type ServiceAccountSpec struct {
	// Name of the ServiceAccount. Empty means the name that the controller
	// derives from the name of the resource, which each CRD doc states. The
	// name is part of the contract with the cloud provider: a workload
	// identity that needs no annotation, such as EKS Pod Identity, binds the
	// principal system:serviceaccount:<namespace>:<name>.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=253
	// +optional
	Name string `json:"name,omitempty"`
	// Create renders and owns the ServiceAccount. Defaults to true. False
	// names one that already exists: the operator neither creates, annotates,
	// nor owns it, and a ServiceAccount that is absent then fails the
	// pre-check instead of leaving the pods unschedulable. The operator never
	// adopts a foreign ServiceAccount, because an owned one is deleted with
	// the resource.
	// +optional
	Create *bool `json:"create,omitempty"`
	// Annotations to set on the ServiceAccount, typically workload-identity
	// annotations (IRSA, GCP Workload Identity, ...) granting the pods access
	// to cloud resources such as the snapshot bucket used for backups. A
	// controller that resolves a bucket contract derives the annotation of
	// that contract's identity as well; an annotation set here wins over the
	// derived one on the same key.
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

// SecureSettingsSource references a Secret whose contents ECK loads into the
// keystore of every Elasticsearch node. Elasticsearch reads credentials from
// the keystore only, never from the settings of a snapshot repository.
type SecureSettingsSource struct {
	// SecretName is the Secret, in the namespace of the ElasticsearchCluster.
	// +kubebuilder:validation:MinLength=1
	SecretName string `json:"secretName"`
	// Entries projects single keys to keystore entries. An empty list loads
	// every key of the Secret under its own name.
	// +optional
	Entries []SecureSettingEntry `json:"entries,omitempty"`
}

// SchedulingSpec groups the scheduling constraints applied to the pods a
// controller manages. A consumer resolving a preset replaces the preset's
// entire scheduling block when it sets its own; the blocks are never merged
// field by field.
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
// that runs workloads. The owning kind says what is scraped: an
// ElasticsearchCluster deploys the prometheus-community elasticsearch_exporter
// (Elasticsearch serves no Prometheus endpoint itself) and scrapes it; a
// CamundaCluster scrapes /actuator/prometheus of every process. The
// ServiceMonitor is created only when the Kubernetes cluster serves the kind.
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
// The type doubles as the configuration baseline of an
// ElasticsearchClusterPreset, so the field that is required on an
// ElasticsearchCluster, secondaryStorageConfig, is optional at the schema
// level here and enforced on the ElasticsearchCluster usage instead. The
// instance-bound fields are cluster-only and rejected in a preset, and so is
// the version, which belongs to a CamundaRelease.
type ElasticsearchClusterSpec struct {
	// PresetRef names a cluster-scoped ElasticsearchClusterPreset used as the
	// configuration baseline; fields set inline override the preset's value
	// for that field wholesale.
	// +optional
	PresetRef string `json:"presetRef,omitempty"`
	// ReleaseRef names a cluster-scoped CamundaRelease that provides the
	// Elasticsearch version. It merges over the preset and under this spec.
	// Forbidden in a preset.
	// +optional
	ReleaseRef string `json:"releaseRef,omitempty"`
	// Version is the Elasticsearch version to deploy, as a full semantic
	// version. Camunda 8.9 supports Elasticsearch 8.19+ and 9.2+. The
	// controller enforces that floor on the merged result, and the schema
	// pins only the three-segment shape. Required unless the resolved release
	// provides it, and forbidden in a preset.
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
	// StorageSize is the size of the data volume of each node. It cannot
	// shrink, because Elasticsearch data volumes cannot be reduced in place.
	// Admission rejects a lower inline value on an ElasticsearchCluster
	// through a CEL transition rule. That rule does not bind this shared
	// field, so a preset baseline can be resized freely: a cluster that
	// applied a larger size keeps it and records a StorageShrinkIgnored
	// event. Required unless the resolved preset provides it.
	// +optional
	StorageSize *resource.Quantity `json:"storageSize,omitempty"`
	// StorageClassName is the StorageClass for the data volumes. Defaults to
	// the cluster's default StorageClass.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
	// ServiceAccount configures the Elasticsearch pods' ServiceAccount,
	// applied through the ECK podTemplate.
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
	// Scheduling constraints for the Elasticsearch pods; when set, it replaces
	// the preset's scheduling block entirely (no merge).
	// +optional
	Scheduling *SchedulingSpec `json:"scheduling,omitempty"`
	// SnapshotStorageRef names an ObjectStorageConfig, in the namespace of
	// this cluster, that holds the bucket of the snapshot repository of this
	// cluster. When it is set,
	// the operator owns the whole Elasticsearch side of that bucket: it gives
	// the nodes their credentials, registers the repository, and publishes the
	// repository name in the SecondaryStorageConfig it produces. Backups of a
	// CamundaCluster on this storage need it. The bucket must be the one that
	// the CamundaCluster references as well.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=253
	// +optional
	SnapshotStorageRef string `json:"snapshotStorageRef,omitempty"`
	// SecureSettings are Secrets that ECK loads into the keystore of every
	// node. The operator adds the credentials of snapshotStorageRef to the
	// keystore on its own, so this field is for everything else a keystore
	// holds.
	// +optional
	SecureSettings []SecureSettingsSource `json:"secureSettings,omitempty"`
	// SecondaryStorageConfig names the SecondaryStorageConfig the operator
	// creates in this CR's own namespace with the connection details and
	// generated credentials. Required on an ElasticsearchCluster, forbidden in
	// a preset.
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
	// The operator sets the volume claim delete policy of the ECK
	// Elasticsearch resource to DeleteOnScaledownOnly, waits until ECK has
	// observed it, and deletes the resource. Setting the field back to
	// false recreates the resource, and ECK reattaches the volumes.
	// Defaults to false.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// ReasonECKNotInstalled is the Ready reason of an ElasticsearchCluster on a
// cluster that did not serve the ECK Elasticsearch kind when the operator
// started. The operator decides at start whether it watches ECK resources,
// so it must restart after ECK is installed.
const ReasonECKNotInstalled = "ECKNotInstalled"

// PersistentVolumeClaimRetentionPolicyType is what happens to the data
// volumes of a resource (an ElasticsearchCluster, the brokers of a
// CamundaCluster) when the resource is deleted.
// +kubebuilder:validation:Enum=Retain;Delete
type PersistentVolumeClaimRetentionPolicyType string

const (
	// RetainPersistentVolumeClaimRetentionPolicyType keeps the data volumes.
	// Removing the data is a manual act. For an ElasticsearchCluster the ECK
	// resource carries the volume claim delete policy DeleteOnScaledownOnly;
	// for a CamundaCluster the broker StatefulSet carries whenDeleted Retain.
	RetainPersistentVolumeClaimRetentionPolicyType PersistentVolumeClaimRetentionPolicyType = "Retain"
	// DeletePersistentVolumeClaimRetentionPolicyType deletes the data volumes
	// with the resource. For an ElasticsearchCluster the ECK resource carries
	// the volume claim delete policy DeleteOnScaledownAndClusterDeletion, the
	// ECK default; for a CamundaCluster the broker StatefulSet carries
	// whenDeleted Delete.
	DeletePersistentVolumeClaimRetentionPolicyType PersistentVolumeClaimRetentionPolicyType = "Delete"
)

// PersistentVolumeClaimRetentionPolicy mirrors the StatefulSet field of the
// same name. Only whenDeleted exists: ECK deletes the volume of every
// Elasticsearch node that it scales away, and the operator always keeps the
// volume of a broker that is scaled away, so there is no whenScaled choice.
type PersistentVolumeClaimRetentionPolicy struct {
	// WhenDeleted is what happens to the data volumes when the resource is
	// deleted. Delete removes them with the resource. Retain keeps them, and
	// a later resource with the same name reattaches them. Defaults to
	// Delete.
	// +kubebuilder:default=Delete
	// +optional
	WhenDeleted PersistentVolumeClaimRetentionPolicyType `json:"whenDeleted,omitempty"`
}

// ElasticsearchClusterStatus is the observed state of an ElasticsearchCluster.
type ElasticsearchClusterStatus struct {
	// ObservedGeneration is the last generation reconciled by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Volumes lists the bound data PersistentVolumeClaims of the cluster and
	// the capacity that each one reports, sorted by name.
	// +listType=map
	// +listMapKey=name
	// +optional
	Volumes []VolumeStatus `json:"volumes,omitempty"`
	// SnapshotRepository is the snapshot repository that the operator has
	// registered in Elasticsearch for this cluster. It is the name that the
	// published SecondaryStorageConfig carries, and it is empty until the
	// first registration converges. A cluster whose name a later operator
	// version derives differently therefore publishes nothing until the new
	// repository exists, rather than a name that Elasticsearch does not hold.
	// +optional
	SnapshotRepository string `json:"snapshotRepository,omitempty"`
	// Conditions represent the current state. Ready carries a pre-check
	// reason (InvalidReference, MissingSecret, ECKNotInstalled), or it is
	// derived from the component conditions. The per-component conditions
	// (CredentialsReady, ElasticsearchReady, StorageContractReady) also
	// appear here.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ElasticsearchCluster provisions and operates an Elasticsearch cluster for
// use as secondary storage, deployed through the external ECK operator, and
// publishes the connection details as a SecondaryStorageConfig with generated
// credentials.
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
