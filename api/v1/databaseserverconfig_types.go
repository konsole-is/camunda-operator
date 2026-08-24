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

// RecoveryMode says who rolls the server back to a point in time.
// +kubebuilder:validation:Enum=operator;external
type RecoveryMode string

const (
	// RecoveryModeOperator means that whoever publishes this contract rolls
	// the server back when spec.recovery asks for it.
	RecoveryModeOperator RecoveryMode = "operator"
	// RecoveryModeExternal means that nobody answers spec.recovery. The
	// server is rolled back by hand, before the restore starts.
	RecoveryModeExternal RecoveryMode = "external"
)

// RecoveryResult is how a recovery request ended.
// +kubebuilder:validation:Enum=Completed;Failed;Unavailable
type RecoveryResult string

const (
	// RecoveryResultCompleted means that the server now holds the state of
	// the requested point.
	RecoveryResultCompleted RecoveryResult = "Completed"
	// RecoveryResultFailed means that the recovery started and did not
	// finish.
	RecoveryResultFailed RecoveryResult = "Failed"
	// RecoveryResultUnavailable means that the server holds no copy of the
	// requested point, so it attempted no recovery.
	RecoveryResultUnavailable RecoveryResult = "Unavailable"
)

// RecoveryRequest asks whoever publishes this contract to roll the server back
// to a point in time. A consumer writes it under a field manager of its own,
// and the publisher of the contract never carries the field, so the two
// writers never meet on it.
type RecoveryRequest struct {
	// RequestID identifies this request, and only this one. A controller sets
	// the uid of the resource that asks. A request written by hand carries
	// any UUID. It is what tells two requests apart that name one resource
	// and one point: a resource that is deleted and created again under its
	// name is another requester, and the answer to the request of the first
	// says nothing about the state the second asks for.
	// +kubebuilder:validation:Pattern=`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
	RequestID string `json:"requestID"`
	// RequestedBy is the namespace and the name of the resource that asks, as
	// "<namespace>/<name>". It comes back in pitr.lastRecovery, which is how
	// the requester tells its own answer from somebody else's.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?/[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=507
	RequestedBy string `json:"requestedBy"`
	// TargetTime is the point to roll back to, as RFC 3339 with a zone, for
	// example 2026-08-20T14:30:00Z. A timestamp without a zone is rejected:
	// PostgreSQL reads it as the local time of the server, so a request that
	// means one point to the writer means another to the server.
	// +kubebuilder:validation:Format=date-time
	// +kubebuilder:validation:Pattern=`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$`
	TargetTime string `json:"targetTime"`
}

// RecoveryOutcome is how a recovery request ended. It repeats the request it
// answers, so a consumer knows whether the answer is the answer to its own
// request.
type RecoveryOutcome struct {
	// RequestID is the requestID of the request this outcome answers.
	// +kubebuilder:validation:Pattern=`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
	RequestID string `json:"requestID"`
	// RequestedBy is the requestedBy of the request this outcome answers.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?/[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=507
	RequestedBy string `json:"requestedBy"`
	// TargetTime is the targetTime of the request this outcome answers. It
	// carries the shape of the request, so a consumer that compares the two
	// as text compares two values of one form.
	// +kubebuilder:validation:Format=date-time
	// +kubebuilder:validation:Pattern=`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$`
	TargetTime string `json:"targetTime"`
	// CompletedAt is when the request ended.
	CompletedAt metav1.Time `json:"completedAt"`
	// Result is how the request ended. See RecoveryResult for the values.
	Result RecoveryResult `json:"result"`
	// Message says what happened. It is empty for a result of Completed.
	// +optional
	Message string `json:"message,omitempty"`
}

// AnsweredBy reports whether outcome answers this request. It compares the
// whole request, requestID included, so the answer to an earlier request of
// the same resource and the same point is not read as the answer to this one.
func (r RecoveryRequest) AnsweredBy(outcome *RecoveryOutcome) bool {
	return outcome != nil &&
		outcome.RequestID == r.RequestID &&
		outcome.RequestedBy == r.RequestedBy &&
		outcome.TargetTime == r.TargetTime
}

// PITRCapability declares a server's point-in-time-recovery capability: that it
// performs continuous WAL archiving with the given retention.
// +kubebuilder:validation:XValidation:rule="!self.enabled || (has(self.retentionPeriodDays) && self.retentionPeriodDays >= 1)",message="retentionPeriodDays of at least 1 is required when enabled is true"
// +kubebuilder:validation:XValidation:rule="self.recovery != 'operator' || self.enabled",message="recovery: operator requires enabled: true"
type PITRCapability struct {
	// Enabled reports whether the server performs continuous WAL archiving.
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// RetentionPeriodDays is how many days into the past a point-in-time
	// restore can target. Required when enabled is true.
	// +optional
	RetentionPeriodDays *int32 `json:"retentionPeriodDays,omitempty"`
	// Recovery says who rolls the server back to a point in time. operator
	// means that whoever publishes this contract answers spec.recovery, and
	// it requires enabled: true. Defaults to external, which means that
	// nobody does and the server is rolled back by hand.
	// +kubebuilder:default=external
	// +optional
	Recovery RecoveryMode `json:"recovery,omitempty"`
	// LastRecovery is how the last recovery request ended. It is unset until
	// the first request is answered, and it is replaced by the answer to
	// every later one.
	// +optional
	LastRecovery *RecoveryOutcome `json:"lastRecovery,omitempty"`
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
	// Recovery asks for the server to be rolled back to a point in time. A
	// consumer writes it. It is answered in pitr.lastRecovery, and only when
	// pitr.recovery is operator. It stays on the contract after the answer,
	// as the record of the last request.
	// +optional
	Recovery *RecoveryRequest `json:"recovery,omitempty"`
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
	// uniqueness of a logical database on it. A change to the endpoint or to
	// the admin credentials Secret clears it.
	// +optional
	SystemIdentifier string `json:"systemIdentifier,omitempty"`
	// ProbedAt is when the operator last reached the server and read
	// ServerVersion and SystemIdentifier. The operator probes the server again
	// when this is older than the probe interval, or when the admin
	// credentials Secret changed. A reconcile in between leaves it untouched.
	// A change to the endpoint or to the admin credentials Secret clears it,
	// together with ServerVersion, SystemIdentifier, ProbedEndpoint,
	// ProbedSecretName, and ProbedSecretVersion, because the whole record
	// describes the server the old spec named. A change to any other field,
	// for example a recovery request, leaves the record alone: it cannot move
	// the server.
	// +optional
	ProbedAt *metav1.Time `json:"probedAt,omitempty"`
	// ProbedEndpoint is the host and the port that the last probe reached, as
	// "<host>:<port>". It is what tells a spec change that moves the server
	// from one that does not.
	// +optional
	ProbedEndpoint string `json:"probedEndpoint,omitempty"`
	// ProbedSecretName is the admin credentials Secret that the last probe
	// read. A spec that names another Secret names other credentials, so the
	// record of the probe goes with it.
	// +optional
	ProbedSecretName string `json:"probedSecretName,omitempty"`
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

// OperatorRecovers reports whether the contract declares that whoever
// publishes it rolls the server back on request. A consumer reads it before it
// writes spec.recovery, and the producer reads it before it takes one.
func (in *DatabaseServerConfig) OperatorRecovers() bool {
	return in.Spec.PITR != nil && in.Spec.PITR.Recovery == RecoveryModeOperator
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
