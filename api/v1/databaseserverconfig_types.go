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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DatabaseEngine identifies the database engine of a server.
// +kubebuilder:validation:Enum=postgres
type DatabaseEngine string

// DatabaseEnginePostgres is the PostgreSQL engine. At this time, it is the
// only engine on which the operator can create databases.
const DatabaseEnginePostgres DatabaseEngine = "postgres"

// RecoveryMode says who rolls the server back to a point in time.
// +kubebuilder:validation:Enum=operator;external
type RecoveryMode string

const (
	// RecoveryModeOperator means that the publisher of this contract rolls
	// the server back when spec.recovery asks for it.
	RecoveryModeOperator RecoveryMode = "operator"
	// RecoveryModeExternal means that nobody answers spec.recovery. You roll
	// the server back manually, before the restore starts.
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

// RecoveryRequest asks the publisher of this contract to roll the server back
// to a point in time. A consumer writes it with its own field manager. The
// publisher of the contract never writes this field, so the two writers
// never conflict.
type RecoveryRequest struct {
	// RequestID identifies this request, and only this one. A controller sets
	// the uid of the resource that asks. A manual request can use any UUID.
	// Two requests can name the same resource and the same point. The
	// RequestID tells them apart. A resource that is deleted and created
	// again with the same name is another requester. The answer to the first
	// request says nothing about the state that the second requester asks
	// for.
	// +kubebuilder:validation:Pattern=`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
	RequestID string `json:"requestID"`
	// RequestedBy is the namespace and the name of the resource that asks, as
	// "<namespace>/<name>". It comes back in pitr.lastRecovery. Thus the
	// requester can find its own answer.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?/[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=507
	RequestedBy string `json:"requestedBy"`
	// TargetTime is the point to roll back to, as RFC 3339 with a zone, for
	// example 2026-08-20T14:30:00Z. The API server refuses a timestamp
	// without a zone. PostgreSQL reads such a timestamp as the local time of
	// the server, which can be another point than the writer intended.
	// +kubebuilder:validation:Format=date-time
	// +kubebuilder:validation:Pattern=`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$`
	TargetTime string `json:"targetTime"`
}

// RecoveryOutcome is how a recovery request ended. It repeats the request
// that it answers. Thus a consumer knows whether it is the answer to its own
// request.
type RecoveryOutcome struct {
	// RequestID is the requestID of the request that this outcome answers.
	// +kubebuilder:validation:Pattern=`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
	RequestID string `json:"requestID"`
	// RequestedBy is the requestedBy of the request that this outcome
	// answers.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?/[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=507
	RequestedBy string `json:"requestedBy"`
	// TargetTime is the targetTime of the request that this outcome answers,
	// in the same form as in the request. Thus a consumer can compare the
	// two as text.
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

// PITRCapability declares the point-in-time recovery capability of a server:
// continuous WAL archiving with the given retention.
// +kubebuilder:validation:XValidation:rule="!self.enabled || (has(self.retentionPeriodDays) && self.retentionPeriodDays >= 1)",message="retentionPeriodDays of at least 1 is required when enabled is true"
// +kubebuilder:validation:XValidation:rule="self.recovery != 'operator' || self.enabled",message="recovery: operator requires enabled: true"
type PITRCapability struct {
	// Enabled reports whether the server performs continuous WAL archiving.
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// RetentionPeriodDays is how many days into the past a point-in-time
	// restore can go. Required when enabled is true.
	//
	// The maximum is 36500 days, which is a hundred years. A reader counts
	// the reachable window in nanoseconds, and that count overflows at about
	// 292 years. The maximum stays well below that limit.
	// +kubebuilder:validation:Maximum=36500
	// +optional
	RetentionPeriodDays *int32 `json:"retentionPeriodDays,omitempty"`
	// Recovery says who rolls the server back to a point in time. Defaults to
	// external. The value operator means that the publisher of this contract
	// answers spec.recovery. It requires enabled: true. The value external
	// means that nobody answers, and you roll the server back manually.
	// +kubebuilder:default=external
	// +optional
	Recovery RecoveryMode `json:"recovery,omitempty"`
	// LastRecovery is how the last recovery request ended. It is unset until
	// the first answer. The answer to each later request replaces it.
	// +optional
	LastRecovery *RecoveryOutcome `json:"lastRecovery,omitempty"`
}

// DatabaseServerConfigSpec describes a database server: the engine, the
// endpoint, the admin credentials, and the point-in-time recovery
// capability.
type DatabaseServerConfigSpec struct {
	// Engine is the database engine of the server. See DatabaseEngine for
	// the accepted values.
	Engine DatabaseEngine `json:"engine"`
	// Host is the host name or the address of the server.
	// +kubebuilder:validation:MinLength=1
	Host string `json:"host"`
	// Port is the port of the server.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`
	// AdminCredentialsSecretRef names an admin user that can create databases
	// and roles. The operator uses it to probe the server and to create the
	// database and the roles of a Database. The Secret is in the namespace of
	// this contract.
	AdminCredentialsSecretRef LocalCredentialsSecretRef `json:"adminCredentialsSecretRef"`
	// PITR declares the point-in-time recovery capability of the server.
	// +optional
	PITR *PITRCapability `json:"pitr,omitempty"`
	// Recovery asks for a rollback of the server to a point in time. A
	// consumer writes it. The answer is in pitr.lastRecovery, only when
	// pitr.recovery is operator. The request stays on the contract after the
	// answer, as the record of the last request.
	// +optional
	Recovery *RecoveryRequest `json:"recovery,omitempty"`
}

// DatabaseServerConfigStatus is the observed validation state of the contract.
// It holds what the operator read from the server the last time that it
// reached the server.
type DatabaseServerConfigStatus struct {
	// ObservedGeneration is the last generation that the operator processed.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// ServerVersion is the major version that the server reported the last
	// time that the operator reached it, for example "17". A dump of a
	// database on this server uses client tools of this major version. Thus
	// a backup waits until the operator publishes it.
	// +optional
	ServerVersion string `json:"serverVersion,omitempty"`
	// SystemIdentifier is the identity of the PostgreSQL instance behind this
	// endpoint, as the server reported it on the last probe. It identifies
	// the server itself. Thus two contracts that describe one server with
	// different hosts publish the same value. The operator uses it to keep
	// each logical database unique. A change to the endpoint or to
	// adminCredentialsSecretRef clears it.
	// +optional
	SystemIdentifier string `json:"systemIdentifier,omitempty"`
	// ProbedAt is when the operator last reached the server and read
	// ServerVersion and SystemIdentifier. The operator probes the server
	// again when this is older than the probe interval, or when the admin
	// credentials Secret changed. Between probes, it does not change.
	//
	// A change to the endpoint or to adminCredentialsSecretRef clears this
	// field, and also ServerVersion, SystemIdentifier, ProbedEndpoint,
	// ProbedSecretName, ProbedSecretKeys, and ProbedSecretVersion. These
	// fields describe the server of the old spec. A change to another field,
	// for example a recovery request, does not clear them, because it cannot
	// move the server.
	// +optional
	ProbedAt *metav1.Time `json:"probedAt,omitempty"`
	// ProbedEndpoint is the host and the port that the last probe reached, as
	// "<host>:<port>". With it, the operator knows whether a spec change
	// moves the server.
	// +optional
	ProbedEndpoint string `json:"probedEndpoint,omitempty"`
	// ProbedSecretName is the admin credentials Secret that the last probe
	// read. When the spec names another Secret, the operator clears the
	// record of the probe.
	// +optional
	ProbedSecretName string `json:"probedSecretName,omitempty"`
	// ProbedSecretKeys are the keys of that Secret that the last probe read,
	// as "<usernameKey>/<passwordKey>". One Secret can hold the credentials
	// of more than one user, so the keys identify the user.
	// +optional
	ProbedSecretKeys string `json:"probedSecretKeys,omitempty"`
	// ProbedSecretVersion is the resourceVersion of the admin credentials
	// Secret that the last probe used. When the Secret changes, the operator
	// probes again before the interval ends, so it validates new credentials
	// quickly.
	// +optional
	ProbedSecretVersion string `json:"probedSecretVersion,omitempty"`
	// Conditions represent the current validation state. The Ready condition
	// has the reasons Healthy, MissingSecret, or ConnectionFailed.
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

// DatabaseServerConfig is the contract CRD that describes a database server:
// the engine, the endpoint, the admin credentials, and the point-in-time
// recovery capability. The operator creates databases on the server and
// validates the declared capabilities. A consumer reads it in its own
// namespace. Thus all the RDBMS resources of a cluster are in the namespace
// of that cluster.
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

// ProbedForCurrentSpec reports whether the record of the last probe describes
// the server that the spec names now. After a change of host, port, or admin
// Secret, the record stays as it was until the contract controller notices
// the change and clears it, and it stays empty until the next probe succeeds.
// In both windows ServerVersion and SystemIdentifier do not describe the
// server the spec names. A consumer that keys on either one waits until this
// reports true.
func (in *DatabaseServerConfig) ProbedForCurrentSpec() bool {
	if in.Status.ProbedAt == nil {
		return false
	}

	ref := in.Spec.AdminCredentialsSecretRef

	return in.Status.ProbedEndpoint == fmt.Sprintf("%s:%d", in.Spec.Host, in.Spec.Port) &&
		in.Status.ProbedSecretName == ref.Name &&
		in.Status.ProbedSecretKeys == ref.UsernameKey+"/"+ref.PasswordKey
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
