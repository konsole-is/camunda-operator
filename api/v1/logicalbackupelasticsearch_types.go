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
// paused, so a person must act. Only LogicalBackupElasticsearch reports it,
// because only the Elasticsearch backup pauses exporting.
const ReasonResumeFailed = "ResumeFailed"

// LogicalBackupElasticsearchStep is the current step of the backup. After a
// crash or a restart of the operator, the backup continues at this step.
// The request to pause exporting can be sent again after a restart.
// +kubebuilder:validation:Enum=PauseExporting;BackupHistory;SnapshotRecords;BackupRuntime;ResumeExporting
type LogicalBackupElasticsearchStep string

// The steps of the Elasticsearch backup procedure, in order. After a failure
// in a step, the backup goes to StepResumeExporting. A cluster with paused
// exporting cannot compact its log, so the resume always runs before a final
// phase.
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
	// State is the state of this part.
	// +optional
	State BackupPartState `json:"state,omitempty"`
	// FailureReason is set when State is Failed.
	// +optional
	FailureReason string `json:"failureReason,omitempty"`
}

// PinnedStorage is the destination of a backup set, recorded when the backup
// starts. It names the storage contract and the Elasticsearch endpoint that
// hold the snapshots, and the backup bucket that holds the runtime backup.
// status.repository records the name of the snapshot repository. Every step
// checks the destination before it writes, and the operator checks it before
// it deletes. Thus a destination that moves during the backup does not split
// the set or send a deletion to the wrong place.
type PinnedStorage struct {
	// SecondaryStorageConfig is the name of the storage contract, in the
	// namespace of the backup.
	SecondaryStorageConfig string `json:"secondaryStorageConfig"`
	// Endpoint is the Elasticsearch endpoint that the contract named.
	Endpoint string `json:"endpoint"`
	// BucketRef is the ObjectStorageConfig that the cluster used for backups
	// when the backup started: its spec.backupStorageRef at that time. The
	// runtime backup goes to that bucket.
	BucketRef string `json:"bucketRef"`
	// BucketLocation is where that contract pointed: the storage type, the
	// bucket, the base path, and the endpoint. The steps write, and the
	// operator deletes, only while the contract still points there.
	BucketLocation string `json:"bucketLocation"`
}

// LogicalBackupElasticsearchSpec identifies the cluster to back up. The whole
// spec is immutable. A backup runs one time. To try again, create a new
// resource.
type LogicalBackupElasticsearchSpec struct {
	// ClusterRef references the CamundaCluster to back up, in the namespace
	// of this backup. Its secondary storage must be Elasticsearch.
	// +required
	ClusterRef ClusterRef `json:"clusterRef"`
}

// LogicalBackupElasticsearchStatus is the progress of the backup to a final
// phase.
type LogicalBackupElasticsearchStatus struct {
	// Phase is the phase of the backup. Completed and Failed are final.
	// +optional
	Phase LogicalBackupPhase `json:"phase,omitempty"`
	// Step is the current step of the backup. After an interruption, the
	// backup continues at this step.
	// +optional
	Step LogicalBackupElasticsearchStep `json:"step,omitempty"`
	// BackupID identifies every part of the backup set: the web-application
	// snapshots, the record snapshot, and the partition backup. A restore
	// finds the set by this ID.
	// +optional
	BackupID int64 `json:"backupId,omitempty"`
	// PartitionsCount is the partition count of the cluster when the backup
	// started. A restore must match it.
	// +optional
	PartitionsCount int32 `json:"partitionsCount,omitempty"`
	// StorageSizes are the effective restore sizes. The operator computes
	// them when the backup starts. If a value is not available then, the
	// operator adds it later while exporting runs: before the pause, or after
	// the resume. It does not add a value while exporting is paused. Thus an
	// absent value can still appear later in the backup.
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
	// web-application indices. The operator records the names as soon as the
	// management API gives them. Thus the deletion of the backup and a
	// restore can find the snapshots after the cluster is gone.
	// +optional
	HistorySnapshots []string `json:"historySnapshots,omitempty"`
	// Repository records the snapshot repository of every part of the set.
	// The operator records it when the backup starts, and every later step
	// and the deletion use this name. Thus the whole set goes to one
	// repository, also when the storage contract changes its repository
	// during the backup. The deletion also goes to the correct repository.
	// +optional
	Repository string `json:"repository,omitempty"`
	// Storage records the Elasticsearch destination of the set: the storage
	// contract and its endpoint when the backup started. The repository name
	// alone does not identify a cluster. If the storage contract or the
	// endpoint changes during the backup, the step fails. The deletion never
	// runs against another cluster.
	// +optional
	Storage *PinnedStorage `json:"storage,omitempty"`
	// WorkloadConfigHash records the configuration of Zeebe when the backup
	// started. It is the config hash of the Zeebe pod template. The backup
	// starts only after the cluster runs its current spec and Zeebe runs the
	// Elasticsearch endpoint of the storage contract. Until the runtime
	// backup is final, each step compares the current hash with this value,
	// after it reads the state of its part. If the hash differs, the step
	// fails at once. The generation of the cluster is not sufficient, because
	// a change of a referenced object changes the hash but not the generation.
	// +optional
	WorkloadConfigHash string `json:"workloadConfigHash,omitempty"`
	// ClusterUID records the identity of the CamundaCluster of the backup. A
	// cluster that is deleted and created again with the same name is
	// another cluster. This backup never paused the exporting of the new
	// cluster. Every management call after the start compares the cluster
	// with this UID. If they differ, the backup ends and does not change the
	// new cluster.
	// +optional
	ClusterUID string `json:"clusterUID,omitempty"`
	// Version is the Camunda version of the cluster when the backup started,
	// as the management binding reported it. A restore compares it with the
	// version of its target. An Elasticsearch backup restores only to the
	// same version. A restore can read the version only here, because a
	// suspended cluster has no management binding.
	// +optional
	Version string `json:"version,omitempty"`
	// HistoryRequestedTime is when the operator decided to request the
	// backup of the web-application indices. The operator writes it before
	// it sends the request, so the decision stays after a lost response or a
	// restart. It shows that this backup intended to send the request. It
	// does not prove that a history backup with this ID belongs to this
	// backup.
	// +optional
	HistoryRequestedTime *metav1.Time `json:"historyRequestedTime,omitempty"`
	// HistoryAcceptedTime is when the cluster accepted the history backup
	// request of this backup. Only this field shows that the history backup
	// with this ID belongs to this backup. If such a history backup exists
	// without this field, the step fails, and the operator does not delete
	// its snapshots. A crash between the request and the write of this field
	// fails the backup. The cluster can then keep a history backup with this
	// ID, which you remove manually.
	// +optional
	HistoryAcceptedTime *metav1.Time `json:"historyAcceptedTime,omitempty"`
	// RuntimeRequestedTime is when the operator decided to request the
	// runtime backup. The operator writes it before it sends the request, so
	// the decision stays after a lost response or a restart. It shows that
	// this backup intended to send the request. It does not prove that a
	// runtime backup with this ID belongs to this backup.
	// +optional
	RuntimeRequestedTime *metav1.Time `json:"runtimeRequestedTime,omitempty"`
	// RuntimeAcceptedTime is when the cluster accepted the runtime backup
	// request of this backup. Only this field shows that the runtime backup
	// with this ID belongs to this backup. A runtime backup can exist without
	// this field, after a lost response or when another client used the ID
	// first. Then the step fails, and the operator does not delete that
	// runtime backup. A crash between the request and the write of this
	// field fails the backup. The cluster can then keep such a runtime
	// backup, which you remove manually.
	//
	// The cluster registers the backup some time after it accepts it. For a
	// short grace period after this time, the operator waits for an absent
	// backup. After the grace period, an absent backup fails the step.
	// +optional
	RuntimeAcceptedTime *metav1.Time `json:"runtimeAcceptedTime,omitempty"`
	// UnreachableSince is when a step first failed to reach its endpoint:
	// the management API or Elasticsearch. Exporting can be paused during
	// every step, so the retries have a time limit. After the limit, the step
	// fails, and the backup resumes exporting. The field clears when all
	// calls succeed again.
	// +optional
	UnreachableSince *metav1.Time `json:"unreachableSince,omitempty"`
	// FailureMessage names the failed step and its error. The operator
	// records it when a step fails and exporting must still resume. Thus the
	// final condition shows the reason after the resume.
	// +optional
	FailureMessage string `json:"failureMessage,omitempty"`
	// ResumeStartedTime is the start of the resume deadline. Only the time
	// of active resume attempts counts against the deadline. A time in which
	// the backup waits, for example for a suspended cluster or an
	// unpublished binding, moves the start forward and does not count. The
	// value stays after a restart of the operator.
	// +optional
	ResumeStartedTime *metav1.Time `json:"resumeStartedTime,omitempty"`
	// LastResumeAttemptTime is when the last resume attempt ended. The time
	// from it to the start of the next attempt decides whether the start of
	// the deadline moves. The time inside an attempt always counts.
	// +optional
	LastResumeAttemptTime *metav1.Time `json:"lastResumeAttemptTime,omitempty"`
	// TerminalReason is the Ready reason that the operator recorded when the
	// backup reached its final phase: Completed, Failed, or ResumeFailed.
	// +optional
	TerminalReason string `json:"terminalReason,omitempty"`
	// ResumeFailureMessage is the last error of the resume of exporting,
	// when the backup stopped the attempts. A backup that failed a step and
	// then failed to resume reports this field and FailureMessage.
	// +optional
	ResumeFailureMessage string `json:"resumeFailureMessage,omitempty"`
	// CompletionTime is when the backup reached a final phase.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
	// ObservedGeneration is the last generation that the operator processed.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the current state. The Ready condition shows the
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

// LogicalBackupElasticsearch is one backup of a CamundaCluster with
// Elasticsearch secondary storage. The backup is one set under one backup ID:
// the web-application indices, the exported Zeebe record indices, and the
// Zeebe partitions. The cluster continues to run during the backup, with
// exporting soft-paused. A restore reads a completed backup by its backup ID
// and its recorded snapshot names.
//
// When you delete the resource, the operator tries to delete the stored
// backup data. The deletion waits while the cluster publishes no management
// binding, for example while it is suspended. It also waits while the pinned
// bucket points elsewhere. The operator removes the resource and can leave
// the data when the cluster is gone or was created again. The same applies
// when the pinned bucket is gone. It also applies when the management client
// cannot be built and the backup holds no pause of exporting.
type LogicalBackupElasticsearch struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec identifies the cluster to back up. It is immutable. A backup runs
	// one time. To try again, create a new resource.
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
