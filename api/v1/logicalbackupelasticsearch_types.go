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

// ReasonResumeFailed means exporting could not be resumed before the deadline
// after a backup. The cluster cannot compact its log while exporting is
// paused, so it needs an operator's attention. Only LogicalBackupElasticsearch
// reports it: pausing exporting is a step of the Elasticsearch procedure.
const ReasonResumeFailed = "ResumeFailed"

// LogicalBackupElasticsearchStep is the resume marker of the backup
// procedure. A crash or an operator restart re-enters at the recorded step,
// and every step first queries the current state before it acts, so no call
// is ever repeated.
// +kubebuilder:validation:Enum=PauseExporting;BackupHistory;SnapshotRecords;BackupRuntime;ResumeExporting
type LogicalBackupElasticsearchStep string

// The steps of the Elasticsearch backup procedure, in order. A failure in any
// step routes to StepResumeExporting: a cluster left with paused exporting
// cannot compact its log, so resume always runs before a terminal phase.
const (
	StepPauseExporting  LogicalBackupElasticsearchStep = "PauseExporting"
	StepBackupHistory   LogicalBackupElasticsearchStep = "BackupHistory"
	StepSnapshotRecords LogicalBackupElasticsearchStep = "SnapshotRecords"
	StepBackupRuntime   LogicalBackupElasticsearchStep = "BackupRuntime"
	StepResumeExporting LogicalBackupElasticsearchStep = "ResumeExporting"
)

// BackupPartState is the state of one part of the backup set.
// +kubebuilder:validation:Enum=Pending;InProgress;Completed;Failed
type BackupPartState string

// The states of one backup part.
const (
	BackupPartPending    BackupPartState = "Pending"
	BackupPartInProgress BackupPartState = "InProgress"
	BackupPartCompleted  BackupPartState = "Completed"
	BackupPartFailed     BackupPartState = "Failed"
)

// BackupPart is the observed state of one part of the backup set: the
// web-application indices, the exported record indices, or the Zeebe
// partitions.
type BackupPart struct {
	// State of this part.
	// +optional
	State BackupPartState `json:"state,omitempty"`
	// FailureReason is set when State is Failed.
	// +optional
	FailureReason string `json:"failureReason,omitempty"`
}

// LogicalBackupElasticsearchSpec identifies the cluster to back up. The whole
// spec is immutable: a backup is a one-shot operation, retried by creating a
// new resource.
type LogicalBackupElasticsearchSpec struct {
	// ClusterRef references the CamundaCluster to back up. Its secondary
	// storage must be Elasticsearch.
	// +required
	ClusterRef ClusterRef `json:"clusterRef"`
}

// LogicalBackupElasticsearchStatus tracks the one-shot backup procedure to
// completion.
type LogicalBackupElasticsearchStatus struct {
	// Phase of the backup. Completed and Failed are terminal.
	// +optional
	Phase LogicalBackupPhase `json:"phase,omitempty"`
	// Step is the resume marker of the running procedure.
	// +optional
	Step LogicalBackupElasticsearchStep `json:"step,omitempty"`
	// BackupID keys every part of the backup set: the web-application
	// snapshots, the record snapshot, and the partition backup. A restore
	// locates the set by it.
	// +optional
	BackupID int64 `json:"backupId,omitempty"`
	// PartitionsCount is the partition count of the cluster when the backup
	// started. A restore must match it.
	// +optional
	PartitionsCount int32 `json:"partitionsCount,omitempty"`
	// StorageSizes are the effective restore sizes, recorded best effort when
	// the backup starts.
	// +optional
	StorageSizes LogicalBackupStorageSizes `json:"storageSizes,omitempty"`
	// History is the backup of the web-application indices.
	// +optional
	History BackupPart `json:"history,omitempty"`
	// Records is the snapshot of the exported Zeebe record indices.
	// +optional
	Records BackupPart `json:"records,omitempty"`
	// Runtime is the backup of the Zeebe partitions.
	// +optional
	Runtime BackupPart `json:"runtime,omitempty"`
	// HistorySnapshots names the Elasticsearch snapshots of the
	// web-application indices. The names are recorded as soon as the
	// management API reports them, so the finalizer and a restore can locate
	// the snapshots after the cluster is gone.
	// +optional
	HistorySnapshots []string `json:"historySnapshots,omitempty"`
	// FailureMessage names the failing step and its error. It is recorded
	// when a step fails and exporting still has to be resumed, so the reason
	// survives the resume and reaches the terminal condition.
	// +optional
	FailureMessage string `json:"failureMessage,omitempty"`
	// ResumeStartedTime is when the procedure entered ResumeExporting. The
	// resume deadline counts from it, so the deadline survives an operator
	// restart.
	// +optional
	ResumeStartedTime *metav1.Time `json:"resumeStartedTime,omitempty"`
	// CompletionTime is when the backup reached a terminal phase.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
	// ObservedGeneration is the last generation reconciled by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the current state; the Ready condition tracks the
	// backup with reasons Progressing, Completed, Failed, ResumeFailed,
	// ClusterSuspended, BackupInProgress, StorageTypeMismatch,
	// InvalidReference, and ConnectionFailed.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=logicalbackupelasticsearches,shortName=lbes
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Step",type=string,JSONPath=`.status.step`
// +kubebuilder:printcolumn:name="Backup ID",type=integer,JSONPath=`.status.backupId`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LogicalBackupElasticsearch is one backup of an Elasticsearch-backed
// CamundaCluster: the web-application indices, the exported Zeebe record
// indices, and the Zeebe partitions, as one coordinated set under one backup
// ID, taken hot with exporting soft-paused. LogicalRestore consumes a
// completed backup. Deleting the resource deletes the stored artifacts
// through a finalizer.
type LogicalBackupElasticsearch struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec identifies the cluster to back up. It is immutable: a backup is a
	// one-shot operation, retried by creating a new resource.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable: a backup is one-shot, retried by creating a new resource"
	Spec LogicalBackupElasticsearchSpec `json:"spec"`

	// status defines the observed state of the backup
	// +optional
	Status LogicalBackupElasticsearchStatus `json:"status,omitzero"`
}

// GetStatusConditions returns a pointer to the status conditions. The
// component framework stages conditions on the resource through it.
func (in *LogicalBackupElasticsearch) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// GetKind returns the CRD kind. The component framework uses it for event and
// metric recording.
func (in *LogicalBackupElasticsearch) GetKind() string { return "LogicalBackupElasticsearch" }

// SetObservedGeneration records the last reconciled generation in status.
func (in *LogicalBackupElasticsearch) SetObservedGeneration(generation int64) {
	in.Status.ObservedGeneration = generation
}

// Terminal reports whether the backup reached a phase it never leaves.
func (in *LogicalBackupElasticsearch) Terminal() bool {
	return in.Status.Phase == LogicalBackupCompleted || in.Status.Phase == LogicalBackupFailed
}

// EffectiveClusterNamespace returns the namespace of the referenced cluster:
// the clusterRef namespace, or the backup's own when unset.
func (in *LogicalBackupElasticsearch) EffectiveClusterNamespace() string {
	if in.Spec.ClusterRef.Namespace != "" {
		return in.Spec.ClusterRef.Namespace
	}
	return in.Namespace
}

// +kubebuilder:object:root=true

// LogicalBackupElasticsearchList contains a list of LogicalBackupElasticsearch
type LogicalBackupElasticsearchList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []LogicalBackupElasticsearch `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LogicalBackupElasticsearch{}, &LogicalBackupElasticsearchList{})
}
