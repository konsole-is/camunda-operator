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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DatabaseEngine identifies the database engine of a server.
// +kubebuilder:validation:Enum=postgres
type DatabaseEngine string

// DatabaseEnginePostgres is the PostgreSQL engine, currently the only engine
// the Database controller can bootstrap against.
const DatabaseEnginePostgres DatabaseEngine = "postgres"

// PITRCapability declares a server's point-in-time-recovery capability: that it
// performs continuous WAL archiving with the given retention.
// +kubebuilder:validation:XValidation:rule="!self.enabled || (has(self.retentionPeriodDays) && self.retentionPeriodDays >= 1)",message="retentionPeriodDays of at least 1 is required when enabled is true"
type PITRCapability struct {
	// Enabled reports whether the server performs continuous WAL archiving.
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// RetentionPeriodDays is how many days into the past a point-in-time
	// restore can target. Required when enabled is true.
	// +optional
	RetentionPeriodDays *int32 `json:"retentionPeriodDays,omitempty"`
}

// DatabaseServerConfigSpec describes a database server: engine, endpoint,
// admin credentials, and point-in-time-recovery capability.
type DatabaseServerConfigSpec struct {
	// Engine is the database engine of the server. See DatabaseEngine for
	// the accepted values.
	Engine DatabaseEngine `json:"engine"`
	// Host the server is reachable at.
	// +kubebuilder:validation:MinLength=1
	Host string `json:"host"`
	// Port the server listens on.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`
	// AdminCredentialsSecretRef names an admin user with permission to create
	// databases and roles; used by the Database controller to bootstrap. The
	// Secret lives in the namespace of this contract.
	AdminCredentialsSecretRef LocalCredentialsSecretRef `json:"adminCredentialsSecretRef"`
	// PITR declares the server's point-in-time-recovery capability.
	// +optional
	PITR *PITRCapability `json:"pitr,omitempty"`
}

// DatabaseServerConfigStatus is the observed validation state of the contract.
// It holds what the operator read from the server the last time it reached it.
type DatabaseServerConfigStatus struct {
	// ObservedGeneration is the last generation reconciled by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// ServerVersion is the major version that the server reported the last
	// time the operator reached it, for example "17". A dump of a database on
	// this server runs client tools of this major, so a backup waits until
	// the operator publishes it.
	// +optional
	ServerVersion string `json:"serverVersion,omitempty"`
	// SystemIdentifier is the identity of the PostgreSQL instance behind this
	// endpoint, as the server reported it on the last probe. It names the
	// server itself, so two contracts that describe one server under
	// different hosts publish one value. The Database controller keys the
	// uniqueness of a logical database on it.
	// +optional
	SystemIdentifier string `json:"systemIdentifier,omitempty"`
	// ProbedAt is when the operator last reached the server and read
	// ServerVersion and SystemIdentifier. The operator probes the server again
	// when this is older than the probe interval, when the spec changed, or
	// when the admin credentials Secret changed. A reconcile in between leaves
	// it untouched.
	// +optional
	ProbedAt *metav1.Time `json:"probedAt,omitempty"`
	// ProbedSecretVersion is the resourceVersion of the admin credentials
	// Secret that the last probe used. The operator probes a changed Secret
	// again before the interval, so it validates rotated credentials promptly.
	// +optional
	ProbedSecretVersion string `json:"probedSecretVersion,omitempty"`
	// Conditions represent the current validation state. The Ready condition
	// carries reasons Healthy, MissingSecret, or ConnectionFailed.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.status.serverVersion`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// DatabaseServerConfig is the contract CRD that describes a database server —
// engine, endpoint, admin credentials, and point-in-time-recovery capability —
// for controllers that bootstrap databases on it or validate its declared
// capabilities. A consumer resolves it in its own namespace, so the whole
// RDBMS chain of a cluster lives with that cluster.
type DatabaseServerConfig struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of DatabaseServerConfig
	// +required
	Spec DatabaseServerConfigSpec `json:"spec"`

	// status defines the observed state of DatabaseServerConfig
	// +optional
	Status DatabaseServerConfigStatus `json:"status,omitzero"`
}

// GetStatusConditions returns a pointer to the status conditions. The
// component framework stages conditions on the resource through it.
func (in *DatabaseServerConfig) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// GetKind returns the CRD kind. The component framework uses it for event and
// metric recording.
func (in *DatabaseServerConfig) GetKind() string { return "DatabaseServerConfig" }

// SetObservedGeneration records the last reconciled generation in status.
func (in *DatabaseServerConfig) SetObservedGeneration(generation int64) {
	in.Status.ObservedGeneration = generation
}

// +kubebuilder:object:root=true

// DatabaseServerConfigList contains a list of DatabaseServerConfig
type DatabaseServerConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []DatabaseServerConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DatabaseServerConfig{}, &DatabaseServerConfigList{})
}
