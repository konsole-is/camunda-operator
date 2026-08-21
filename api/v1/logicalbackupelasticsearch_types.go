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

// ReasonResumeFailed means that exporting did not resume before the deadline
// after a backup. The cluster cannot compact its log while exporting is
// paused, so it needs the attention of an operator. Only
// LogicalBackupElasticsearch reports it. To pause exporting is a step of the
// Elasticsearch procedure alone.
const ReasonResumeFailed = "ResumeFailed"

// LogicalBackupElasticsearchStep is the resume marker of the backup
// procedure. A crash or an operator restart re-enters at the recorded step.
// Every step queries the current state before it acts, so no call is
// repeated.
// +kubebuilder:validation:Enum=PauseExporting;BackupHistory;SnapshotRecords;BackupRuntime;ResumeExporting
type LogicalBackupElasticsearchStep string

// The steps of the Elasticsearch backup procedure, in order. A failure in any
// step routes to StepResumeExporting. A cluster with paused exporting cannot
// compact its log, so resume always runs before a terminal phase.
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

// PinnedStorage is the destination of a backup set, recorded when the backup
// starts. It names the storage contract and the Elasticsearch endpoint that
// hold the snapshots. It names the backup bucket that holds the runtime
// backup. The snapshot repository is pinned by name in status.repository.
// Every step verifies the destination before it writes. The finalizer
// verifies it before it deletes. A destination that moved mid-run therefore
// splits no set and aims no delete at the wrong place.
type PinnedStorage struct {
	// SecondaryStorageConfig is the name of the storage contract, in the
	// namespace of the backup.
	SecondaryStorageConfig string `json:"secondaryStorageConfig"`
	// Endpoint is the Elasticsearch endpoint that the contract named.
	Endpoint string `json:"endpoint"`
	// BucketRef is the ObjectStorageConfig that the cluster backed up
	// through when the backup started: its spec.backupStorageRef then. The
	// runtime backup lands in that bucket.
	BucketRef string `json:"bucketRef"`
	// BucketLocation is where that contract pointed: the storage type,
	// bucket, base path, and endpoint. The steps write, and the finalizer
	// deletes, only while the contract still points there.
	BucketLocation string `json:"bucketLocation"`
}

// LogicalBackupElasticsearchSpec identifies the cluster to back up. The whole
// spec is immutable: a backup is a one-shot operation, retried by creating a
// new resource.
type LogicalBackupElasticsearchSpec struct {
	// ClusterRef references the CamundaCluster to back up, in the namespace
	// of this backup. Its secondary storage must be Elasticsearch.
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
	// StorageSizes are the effective restore sizes, recorded best effort.
	// They are computed when the backup starts. A value that was not
	// available then is backfilled while exporting runs: before the pause,
	// and after the resume. No value is backfilled while exporting is
	// paused. A value that is absent can therefore still arrive later in
	// the run.
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
	// management API names them. The answer to the start names the scheduled
	// snapshots, and every status report names them again. The finalizer and
	// a restore can then locate the snapshots after the cluster is gone.
	// +optional
	HistorySnapshots []string `json:"historySnapshots,omitempty"`
	// Repository pins the snapshot repository that every part of the set is
	// written to. It is recorded when the backup starts. Every later step and
	// the finalizer use the pinned name. A repository that changes on the
	// storage contract mid-run can then neither split the set nor aim the
	// deletion at the wrong repository.
	// +optional
	Repository string `json:"repository,omitempty"`
	// Storage pins the Elasticsearch destination of the set: the storage
	// contract and the endpoint it named when the backup started. The
	// repository name alone does not identify a cluster. A storage contract
	// or endpoint that changes mid-run fails the step, and the finalizer
	// never deletes against a different cluster.
	// +optional
	Storage *PinnedStorage `json:"storage,omitempty"`
	// ClusterUID pins the identity of the CamundaCluster that the backup
	// started against. A cluster that is deleted and recreated under the
	// same name is a different cluster. Its exporting was never paused by
	// this backup, and its artifacts are not this backup's. Every
	// management call after the start verifies the live cluster against
	// this UID. A mismatch ends the backup without touching the
	// replacement.
	// +optional
	ClusterUID string `json:"clusterUID,omitempty"`
	// Version is the Camunda version of the cluster when the backup started,
	// as the management binding reported it. A restore compares it against
	// the version of its target: an Elasticsearch backup restores only with
	// the exact same version, and a relational backup restores with the same
	// version or one minor newer. It is the only place a restore can read
	// the version, because the management binding of a suspended cluster is
	// unset.
	// +optional
	Version string `json:"version,omitempty"`
	// HistoryRequestedTime is when the controller decided to request the
	// backup of the web-application indices. It is written before the
	// request is sent, so the intent survives a lost response or a restart.
	// It proves that this backup meant to request, not that a history
	// backup under its ID is its own.
	// +optional
	HistoryRequestedTime *metav1.Time `json:"historyRequestedTime,omitempty"`
	// HistoryAcceptedTime is when the cluster accepted the history backup
	// request of this backup, as this controller observed it. It is the
	// only evidence that the history backup under this ID is this backup's.
	// A history backup that exists without it is not adopted: the step
	// fails, and the finalizer does not delete its snapshots. A crash
	// between the request and the write of this field fails the backup
	// safely. It can leave a history backup under this ID in the cluster
	// for the user to remove by hand.
	// +optional
	HistoryAcceptedTime *metav1.Time `json:"historyAcceptedTime,omitempty"`
	// RuntimeRequestedTime is when the controller decided to request the
	// runtime backup. It is written before the request is sent, so the
	// intent survives a lost response or a restart. It proves that this
	// backup meant to request, not that a runtime backup under its ID is
	// its own.
	// +optional
	RuntimeRequestedTime *metav1.Time `json:"runtimeRequestedTime,omitempty"`
	// RuntimeAcceptedTime is when the cluster accepted the request of this
	// backup, as this controller observed it. It is the only evidence that
	// the runtime backup under this ID is this backup's. A runtime backup
	// that exists without it, after a lost response or because another
	// actor won the ID, is not adopted. The step fails, and the finalizer
	// leaves that runtime backup alone. A crash between the request and
	// the write of this field fails the backup safely. It can leave such a
	// runtime backup in the cluster for the user to remove by hand. The
	// cluster registers the backup asynchronously and can report it absent
	// for a moment after the acceptance. Within a registration grace after
	// this time, an absent backup is polled. After the grace, an absent
	// backup fails the step.
	// +optional
	RuntimeAcceptedTime *metav1.Time `json:"runtimeAcceptedTime,omitempty"`
	// UnreachableSince is when a working step first found its endpoint
	// unreachable. The endpoint is the management API or Elasticsearch,
	// whichever the step calls. Exporting can be paused at every working
	// step, so the retry is bounded. After the bound the step fails and the
	// procedure resumes exporting. It clears once every call of a reconcile
	// answered.
	// +optional
	UnreachableSince *metav1.Time `json:"unreachableSince,omitempty"`
	// FailureMessage names the failing step and its error. It is recorded
	// when a step fails and exporting still has to be resumed. The reason
	// then survives the resume and reaches the terminal condition.
	// +optional
	FailureMessage string `json:"failureMessage,omitempty"`
	// ResumeStartedTime anchors the resume deadline. Only the accumulated
	// time of active resume attempts counts against the deadline. A gap in
	// which the procedure was parked slides the anchor forward and does not
	// count, for example a suspended cluster or an unpublished binding. The
	// anchor survives an operator restart.
	// +optional
	ResumeStartedTime *metav1.Time `json:"resumeStartedTime,omitempty"`
	// LastResumeAttemptTime is when the last resume attempt ended. The gap
	// from it to the start of the next attempt decides whether the deadline
	// anchor slides. The time inside an attempt always counts.
	// +optional
	LastResumeAttemptTime *metav1.Time `json:"lastResumeAttemptTime,omitempty"`
	// TerminalReason is the Ready reason recorded at the terminal
	// transition: Completed, Failed, or ResumeFailed. The controller
	// re-stages the terminal condition from it when a write conflict
	// restored an older one.
	// +optional
	TerminalReason string `json:"terminalReason,omitempty"`
	// ResumeFailureMessage is the last error of resume-exporting when the
	// procedure gave up on it. It stands beside FailureMessage, so a backup
	// that failed a step and then failed to resume reports both.
	// +optional
	ResumeFailureMessage string `json:"resumeFailureMessage,omitempty"`
	// CompletionTime is when the backup reached a terminal phase.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
	// ObservedGeneration is the last generation reconciled by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the current state. The Ready condition tracks the
	// backup with the reasons Progressing, Completed, Failed, ResumeFailed,
	// ClusterSuspended, BackupInProgress, StorageTypeMismatch,
	// InvalidReference, MissingSecret, and ConnectionFailed.
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
// CamundaCluster. The backup is one coordinated set under one backup ID: the
// web-application indices, the exported Zeebe record indices, and the Zeebe
// partitions. It is taken hot, with exporting soft-paused. A restore reads a
// completed backup by its backup ID and its recorded snapshot names. When you
// delete the resource, a finalizer deletes the stored artifacts.
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
