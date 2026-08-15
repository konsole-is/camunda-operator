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
	// Annotations to set on the ServiceAccount, typically workload-identity
	// annotations (IRSA, GCP Workload Identity, ...) granting the pods access
	// to cloud resources such as the snapshot bucket used for backups.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
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

// ServiceMonitorSpec configures the optional Prometheus ServiceMonitor for a
// managed workload's service.
type ServiceMonitorSpec struct {
	// Enabled creates the ServiceMonitor when true. Defaults to false.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// Labels are extra labels applied to the ServiceMonitor.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations are extra annotations applied to the ServiceMonitor.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// MonitoringSpec groups the Prometheus scraping integration of a managed
// workload.
type MonitoringSpec struct {
	// ServiceMonitor configures the Prometheus ServiceMonitor.
	// +optional
	ServiceMonitor *ServiceMonitorSpec `json:"serviceMonitor,omitempty"`
}

// ElasticsearchClusterSpec defines the desired state of ElasticsearchCluster.
//
// The type doubles as the configuration baseline of an
// ElasticsearchClusterPreset, so fields that are required on an
// ElasticsearchCluster — secondaryStorageConfig — are optional at the schema
// level here and enforced on the ElasticsearchCluster usage instead.
type ElasticsearchClusterSpec struct {
	// PresetRef names a cluster-scoped ElasticsearchClusterPreset used as the
	// configuration baseline; fields set inline override the preset's value
	// for that field wholesale.
	// +optional
	PresetRef string `json:"presetRef,omitempty"`
	// Version is the Elasticsearch version to deploy, as a full semantic
	// version. Camunda 8.9 supports Elasticsearch 8.19+ and 9.2+; the floor is
	// enforced by the controller on the preset-merged result, the schema pins
	// only the three-segment shape. Required unless the resolved preset
	// provides it.
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
	// Suspend scales the Elasticsearch node set to zero while keeping all
	// data volumes. Defaults to false.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// ElasticsearchClusterStatus is the observed state of an ElasticsearchCluster.
type ElasticsearchClusterStatus struct {
	// ObservedGeneration is the last generation reconciled by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// StorageSize is the data volume size that the cluster has. It is the
	// smallest capacity that the data PersistentVolumeClaims of the cluster
	// report, so a resize outside the spec, for example by an auto-resize
	// controller, shows here. Until a claim reports a capacity it is the size
	// that the applied Elasticsearch CR requests.
	// +optional
	StorageSize *resource.Quantity `json:"storageSize,omitempty"`
	// Conditions represent the current state. Ready carries the pre-check
	// reason InvalidReference or mirrors the representative component
	// condition. The per-component conditions (CredentialsReady,
	// ElasticsearchReady, StorageContractReady) also appear here.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

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

// +kubebuilder:object:root=true

// ElasticsearchClusterList contains a list of ElasticsearchCluster
type ElasticsearchClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ElasticsearchCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ElasticsearchCluster{}, &ElasticsearchClusterList{})
}
