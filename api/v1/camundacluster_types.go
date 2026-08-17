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

// The per-process conditions of a CamundaCluster. Every process reports one.
// The condition of an embedded gateway, an embedded web application, or
// disabled connectors reads True with reason Disabled and stays out of Ready;
// the condition of the host process covers an embedded web application.
const (
	// ConditionZeebeReady reports whether every broker replica is ready.
	ConditionZeebeReady = "ZeebeReady"
	// ConditionGatewayReady reports whether every gateway replica is ready.
	// It reads Disabled when the gateway is embedded.
	ConditionGatewayReady = "GatewayReady"
	// ConditionOperateReady reports whether every standalone Operate replica
	// is ready. It reads Disabled when Operate is embedded.
	ConditionOperateReady = "OperateReady"
	// ConditionTasklistReady reports whether every standalone Tasklist replica
	// is ready. It reads Disabled when Tasklist is embedded.
	ConditionTasklistReady = "TasklistReady"
	// ConditionAdminReady reports whether every standalone Admin replica is
	// ready. It reads Disabled when Admin is embedded.
	ConditionAdminReady = "AdminReady"
	// ConditionConnectorsReady reports whether every connectors replica is
	// ready. It reads Disabled when connectors are not enabled.
	ConditionConnectorsReady = "ConnectorsReady"
	// ConditionAdminSecretReady reports whether the admin credentials Secret
	// of a basic-auth cluster is applied. It takes part in Ready under basic
	// authentication and reads Disabled under OIDC.
	ConditionAdminSecretReady = "AdminSecretReady"
	// ConditionMirroredSecretsReady reports whether every referenced Secret
	// that lives outside the cluster namespace is copied into it. It takes
	// part in Ready only when such a Secret is referenced and reads Disabled
	// when none is.
	ConditionMirroredSecretsReady = "MirroredSecretsReady"
)

// ComponentMode says where a process of the unified binary runs.
// +kubebuilder:validation:Enum=Standalone;Embedded
type ComponentMode string

const (
	// ComponentModeStandalone runs the process as its own Deployment of the
	// unified binary.
	ComponentModeStandalone ComponentMode = "Standalone"
	// ComponentModeEmbedded runs the process inside the nearest standalone
	// host up the chain: the gateway when it is standalone, otherwise zeebe.
	ComponentModeEmbedded ComponentMode = "Embedded"
)

// WorkloadSpec is the override surface that every component block shares. It
// tunes the size, the environment, the pod metadata, and the scheduling of
// one process.
type WorkloadSpec struct {
	// Replicas is the number of pods of this process. Defaults to 1. It has
	// no effect on an embedded web application.
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// Resources are the CPU and memory of the container of this process.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// ExtraEnv are extra environment variables of the container of this
	// process. A per-component entry wins over a top-level entry with the
	// same name. For an embedded web application the entries apply to the
	// host process.
	// +optional
	ExtraEnv []corev1.EnvVar `json:"extraEnv,omitempty"`
	// ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) of the
	// container of this process.
	// +optional
	ExtraEnvFrom []corev1.EnvFromSource `json:"extraEnvFrom,omitempty"`
	// PodLabels are extra labels of the pods of this process.
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`
	// PodAnnotations are extra annotations of the pods of this process.
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`
	// Scheduling constraints of the pods of this process. When set, it
	// replaces the top-level scheduling block and the scheduling block of a
	// preset for this process entirely (no merge).
	// +optional
	Scheduling *SchedulingSpec `json:"scheduling,omitempty"`
}

// ZeebeSpec configures the brokers. Zeebe is always a standalone StatefulSet
// with persistent volumes.
type ZeebeSpec struct {
	WorkloadSpec `json:",inline"`
	// Partitions is the number of partitions. Defaults to 1. Once set, it can
	// neither be decreased nor removed.
	// +kubebuilder:validation:Minimum=1
	// +optional
	Partitions *int32 `json:"partitions,omitempty"`
	// ReplicationFactor is the number of brokers that hold a copy of each
	// partition. Defaults to 1. It must not exceed replicas.
	// +kubebuilder:validation:Minimum=1
	// +optional
	ReplicationFactor *int32 `json:"replicationFactor,omitempty"`
	// StorageClassName is the StorageClass of the broker volumes. Defaults to
	// the default StorageClass of the Kubernetes cluster. It is immutable
	// after creation, because a StatefulSet volume claim template cannot
	// change its storage class. The CEL transition rule that rejects a change
	// sits on the spec field of CamundaCluster ("zeebe.storageClassName is
	// immutable"), not here: this type is shared with CamundaClusterPreset,
	// and a preset baseline stays free to change.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
	// StorageSize is the size of the data volume of each broker. Defaults to
	// 10Gi. It can only grow. Admission rejects a lower inline value on a
	// CamundaCluster through a CEL transition rule. That rule does not bind
	// this shared field, so a preset baseline can be resized freely: a
	// cluster that applied a larger size keeps it and records a
	// StorageShrinkIgnored event. On growth the operator expands the existing
	// claims in place, so the storage class must support volume expansion.
	// +optional
	StorageSize *resource.Quantity `json:"storageSize,omitempty"`
	// PersistentVolumeClaimRetentionPolicy says what happens to the broker
	// volumes when the CamundaCluster is deleted. A scale-down and a
	// suspension always keep them.
	// +optional
	PersistentVolumeClaimRetentionPolicy *PersistentVolumeClaimRetentionPolicy `json:"persistentVolumeClaimRetentionPolicy,omitempty"`
}

// GatewaySpec configures the gateway.
type GatewaySpec struct {
	// Mode selects a Deployment of the unified binary (Standalone) or the
	// embedded gateway of the brokers (Embedded). Defaults to Standalone.
	// The workload fields have an effect only when the mode is Standalone,
	// except extraEnv and extraEnvFrom, which apply to the brokers when the
	// mode is Embedded.
	// +optional
	Mode         ComponentMode `json:"mode,omitempty"`
	WorkloadSpec `              json:",inline"`
}

// WebAppSpec configures one of the web applications Operate, Tasklist, and
// Admin.
type WebAppSpec struct {
	// Mode selects a Deployment of the unified binary that serves only this
	// application (Standalone) or the nearest standalone host up the chain
	// (Embedded). Defaults to Embedded. The workload fields have an effect
	// only when the mode is Standalone, except extraEnv and extraEnvFrom,
	// which apply to the host process when the mode is Embedded.
	// +optional
	Mode         ComponentMode `json:"mode,omitempty"`
	WorkloadSpec `              json:",inline"`
}

// ConnectorsSpec configures the connectors runtime. It is a separate
// application, never part of the unified binary, and always its own
// Deployment.
type ConnectorsSpec struct {
	// Enabled runs the connectors runtime when true. Defaults to false.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// Version is the version of the connectors bundle image, as a full
	// semantic version. The bundle has its own patch line, so it does not
	// follow the cluster version. Required when connectors are enabled,
	// unless the resolved preset provides it.
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+$`
	// +optional
	Version      string `json:"version,omitempty"`
	WorkloadSpec `       json:",inline"`
}

// ClusterAuthSpec holds the OIDC client credentials of one cluster, and the
// identities that get its admin role. The credentials override the defaults
// of the platform config and of the preset.
type ClusterAuthSpec struct {
	// ClientID is the OIDC client ID of this cluster.
	// +optional
	ClientID string `json:"clientId,omitempty"`
	// Audience is the audience that access tokens must carry. Defaults to
	// the clientId.
	// +optional
	Audience string `json:"audience,omitempty"`
	// ClientSecretRef names the Secret that holds the OIDC client secret of
	// this cluster.
	// +optional
	ClientSecretRef *SecretKeyRef `json:"clientSecretRef,omitempty"`
	// Admin holds the identities that get the admin role of this cluster. It
	// applies under OIDC only. Basic authentication seeds its own
	// administrator and ignores this block.
	// +optional
	Admin *ClusterAdminSpec `json:"admin,omitempty"`
}

// ClusterAdminSpec holds the identities that get the admin role of one
// cluster under OIDC. The identity provider authenticates a caller, and
// nothing else authorizes one, so an OIDC cluster without this block has no
// administrator. Basic authentication seeds its own administrator and ignores
// the block.
type ClusterAdminSpec struct {
	// Users are the values of the username claim that get the admin role.
	// +kubebuilder:validation:items:MinLength=1
	// +optional
	Users []string `json:"users,omitempty"`
	// Clients are the values of the client id claim that get the admin role.
	// A client entry matches only when the platform config sets
	// auth.oidc.clientIdClaim.
	// +kubebuilder:validation:items:MinLength=1
	// +optional
	Clients []string `json:"clients,omitempty"`
	// MappingRules give the admin role to every token that holds a claim with
	// a given value.
	// +optional
	MappingRules []AdminMappingRule `json:"mappingRules,omitempty"`
}

// AdminMappingRule gives the admin role to every token in which the claim
// ClaimName holds the value ClaimValue.
type AdminMappingRule struct {
	// ID names the rule inside the cluster. The Admin web application lists it
	// under Mapping Rules.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	ID string `json:"id"`
	// ClaimName is the name of a claim, or a JSONPath expression that points
	// at a claim.
	// +kubebuilder:validation:MinLength=1
	ClaimName string `json:"claimName"`
	// ClaimValue is the value that the claim must hold for the rule to match.
	// +kubebuilder:validation:MinLength=1
	ClaimValue string `json:"claimValue"`
}

// ClusterMonitoringSpec groups the monitoring integrations of a
// CamundaCluster.
type ClusterMonitoringSpec struct {
	// ServiceMonitor configures the Prometheus ServiceMonitors. When
	// enabled, the operator creates one ServiceMonitor per process, named
	// like the workload, that scrapes /actuator/prometheus on the management
	// port 9600 of a unified process and on the HTTP port 8080 of
	// connectors.
	// +optional
	ServiceMonitor *ServiceMonitorSpec `json:"serviceMonitor,omitempty"`
}

// CamundaClusterSpec defines the desired state of CamundaCluster.
//
// The type doubles as the configuration baseline of a CamundaClusterPreset,
// so fields that are required on a CamundaCluster (storageRef,
// platformConfigRef) are optional at the schema level here and enforced on
// the CamundaCluster usage instead. The instance-bound fields are cluster-only
// and rejected in a preset.
type CamundaClusterSpec struct {
	// PlatformConfigRef names the cluster-scoped CamundaPlatformConfig that
	// provides auth, license, and image registry. Required on a
	// CamundaCluster, forbidden in a preset.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=253
	// +optional
	PlatformConfigRef string `json:"platformConfigRef,omitempty"`
	// PresetRef names a cluster-scoped CamundaClusterPreset used as the
	// configuration baseline. The preset doc lists the merge rules.
	// +optional
	PresetRef string `json:"presetRef,omitempty"`
	// Version is the Camunda version to deploy, as a full semantic version.
	// The floor of 8.9.0 is enforced by the controller on the preset-merged
	// result, the schema pins only the three-segment shape. Required unless
	// the resolved preset provides it.
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+$`
	// +optional
	Version string `json:"version,omitempty"`
	// ExternalURL is the external base URL of the cluster. The operator uses
	// it for OIDC redirect URLs and web application links. It creates no
	// Ingress.
	// +optional
	ExternalURL string `json:"externalUrl,omitempty"`
	// ServiceAccount configures the ServiceAccount of every workload. Bucket
	// access for backupStorageRef and documentStorageRef flows from this
	// identity.
	// +optional
	ServiceAccount *ServiceAccountSpec `json:"serviceAccount,omitempty"`
	// Auth holds the OIDC client credentials of this cluster and the
	// identities that get its admin role.
	// +optional
	Auth *ClusterAuthSpec `json:"auth,omitempty"`
	// Zeebe configures the brokers.
	// +optional
	Zeebe *ZeebeSpec `json:"zeebe,omitempty"`
	// Gateway configures the gateway.
	// +optional
	Gateway *GatewaySpec `json:"gateway,omitempty"`
	// Operate configures the Operate web application.
	// +optional
	Operate *WebAppSpec `json:"operate,omitempty"`
	// Tasklist configures the Tasklist web application.
	// +optional
	Tasklist *WebAppSpec `json:"tasklist,omitempty"`
	// Admin configures the Admin web application (Orchestration Cluster
	// Identity before Camunda 8.9). Its Spring profile is admin.
	// +optional
	Admin *WebAppSpec `json:"admin,omitempty"`
	// Connectors configures the connectors runtime.
	// +optional
	Connectors *ConnectorsSpec `json:"connectors,omitempty"`
	// ExtraEnv are extra environment variables of every workload. A
	// per-component entry wins over an entry here with the same name.
	// +optional
	ExtraEnv []corev1.EnvVar `json:"extraEnv,omitempty"`
	// ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) of
	// every workload.
	// +optional
	ExtraEnvFrom []corev1.EnvFromSource `json:"extraEnvFrom,omitempty"`
	// PodLabels are extra labels of every workload pod.
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`
	// PodAnnotations are extra annotations of every workload pod.
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`
	// Scheduling constraints of every workload, unless a component sets its
	// own. When set, it replaces the scheduling block of a preset entirely
	// (no merge).
	// +optional
	Scheduling *SchedulingSpec `json:"scheduling,omitempty"`
	// StorageRef names the SecondaryStorageConfig, in the namespace of this
	// cluster, that describes the secondary storage backend. Required on a
	// CamundaCluster, forbidden in a preset.
	// +optional
	StorageRef string `json:"storageRef,omitempty"`
	// BackupStorageRef names a cluster-scoped ObjectStorageConfig for
	// backups.
	// +optional
	BackupStorageRef string `json:"backupStorageRef,omitempty"`
	// DocumentStorageRef names a cluster-scoped ObjectStorageConfig for
	// document storage.
	// +optional
	DocumentStorageRef string `json:"documentStorageRef,omitempty"`
	// Monitoring configures the monitoring integrations.
	// +optional
	Monitoring *ClusterMonitoringSpec `json:"monitoring,omitempty"`
	// Suspend scales every workload to zero and keeps the data. Defaults to
	// false.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
	// Pause halts the reconciliation of this cluster entirely and leaves the
	// workloads as they are. Defaults to false.
	// +optional
	Pause bool `json:"pause,omitempty"`
}

// CamundaClusterStatus is the observed state of a CamundaCluster.
type CamundaClusterStatus struct {
	// ObservedGeneration is the last generation reconciled by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Volumes lists the bound broker PersistentVolumeClaims and the capacity
	// that each one reports, sorted by name.
	// +listType=map
	// +listMapKey=name
	// +optional
	Volumes []VolumeStatus `json:"volumes,omitempty"`
	// Conditions represent the current state. Ready carries a pre-check
	// reason, or it is derived from the conditions of the components that the
	// cluster needs. The per-process conditions (ZeebeReady, GatewayReady,
	// OperateReady, TasklistReady, AdminReady, ConnectorsReady) also appear
	// here.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// CamundaCluster describes one Camunda orchestration cluster: the Zeebe
// brokers, the gateway, the web applications, and optionally the connectors
// runtime. The operator turns it into StatefulSets, Deployments, and Services
// and keeps them converged.
type CamundaCluster struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of CamundaCluster
	// +kubebuilder:validation:XValidation:rule="has(self.storageRef) && self.storageRef != ''",message="spec.storageRef is required"
	// +kubebuilder:validation:XValidation:rule="has(self.platformConfigRef) && self.platformConfigRef != ''",message="spec.platformConfigRef is required"
	// +kubebuilder:validation:XValidation:rule="!has(oldSelf.zeebe) || !has(oldSelf.zeebe.partitions) || (has(self.zeebe) && has(self.zeebe.partitions) && self.zeebe.partitions >= oldSelf.zeebe.partitions)",message="zeebe.partitions cannot be decreased or removed once set"
	// +kubebuilder:validation:XValidation:rule="!has(oldSelf.zeebe) || !has(oldSelf.zeebe.storageClassName) || (has(self.zeebe) && has(self.zeebe.storageClassName) && self.zeebe.storageClassName == oldSelf.zeebe.storageClassName)",message="zeebe.storageClassName is immutable"
	// +kubebuilder:validation:XValidation:rule="!has(oldSelf.zeebe) || !has(oldSelf.zeebe.storageSize) || !has(self.zeebe) || !has(self.zeebe.storageSize) || !quantity(string(self.zeebe.storageSize)).isLessThan(quantity(string(oldSelf.zeebe.storageSize)))",message="zeebe.storageSize cannot be shrunk"
	// +kubebuilder:validation:XValidation:rule="!has(self.zeebe) || !has(self.zeebe.replicas) || !has(self.zeebe.replicationFactor) || self.zeebe.replicationFactor <= self.zeebe.replicas",message="zeebe.replicationFactor must not exceed zeebe.replicas"
	// +required
	Spec CamundaClusterSpec `json:"spec"`

	// status defines the observed state of CamundaCluster
	// +optional
	Status CamundaClusterStatus `json:"status,omitzero"`
}

// GetStatusConditions returns a pointer to the status conditions. The
// component framework stages per-component conditions on the resource through
// it.
func (in *CamundaCluster) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// GetKind returns the CRD kind. The component framework uses it for event
// and metric recording and derives its per-component SSA field managers
// (CamundaCluster/<component>) from it.
func (in *CamundaCluster) GetKind() string { return "CamundaCluster" }

// SetObservedGeneration records the last reconciled generation in status.
func (in *CamundaCluster) SetObservedGeneration(generation int64) {
	in.Status.ObservedGeneration = generation
}

// +kubebuilder:object:root=true

// CamundaClusterList contains a list of CamundaCluster
type CamundaClusterList struct {
	metav1.TypeMeta `                 json:",inline"`
	metav1.ListMeta `                 json:"metadata,omitzero"`
	Items           []CamundaCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CamundaCluster{}, &CamundaClusterList{})
}
