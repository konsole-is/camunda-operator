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
	"k8s.io/apimachinery/pkg/types"
)

// ReasonIncompatibleTarget means that the target cluster cannot hold the
// backup: the secondary storage types differ, the partition counts differ,
// the backup bucket differs, or the Camunda versions break the version rule.
// Only a LogicalRestore reports it.
const ReasonIncompatibleTarget = "IncompatibleTarget"

// LogicalBackupKind names one of the two logical backup kinds.
// +kubebuilder:validation:Enum=LogicalBackupElasticsearch;LogicalBackupRDBMS
type LogicalBackupKind string

// The logical backup kinds that a LogicalRestore reads from.
const (
	LogicalBackupKindElasticsearch LogicalBackupKind = "LogicalBackupElasticsearch"
	LogicalBackupKindRDBMS         LogicalBackupKind = "LogicalBackupRDBMS"
)

// LogicalBackupRef references a completed logical backup in the namespace of
// the restore. The reference never crosses a namespace.
type LogicalBackupRef struct {
	// Kind of the backup.
	// +required
	Kind LogicalBackupKind `json:"kind"`
	// Name of the backup, in the namespace of this restore.
	// +required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// LogicalRestorePhase tracks the one-shot restore. Completed and Failed are
// terminal. A retry is a new resource.
// +kubebuilder:validation:Enum=Pending;ValidatingCompatibility;RestoringSecondaryStorage;RestoringPrimaryStorage;Completed;Failed
type LogicalRestorePhase string

// The phases of a logical restore, in order.
const (
	// LogicalRestorePending means that the restore did not start real work.
	// A pre-check still holds it: the target cluster runs, or the backup is
	// not completed.
	LogicalRestorePending LogicalRestorePhase = "Pending"
	// LogicalRestoreValidatingCompatibility means that the operator compares
	// the backup against the target: the storage type, the partition count,
	// the backup bucket, and the Camunda version.
	LogicalRestoreValidatingCompatibility LogicalRestorePhase = "ValidatingCompatibility"
	// LogicalRestoreRestoringSecondaryStorage means that the operator writes
	// the backup into the target's secondary storage.
	LogicalRestoreRestoringSecondaryStorage LogicalRestorePhase = "RestoringSecondaryStorage"
	// LogicalRestoreRestoringPrimaryStorage means that the operator recreated
	// the broker data volumes and runs the restore application on them.
	LogicalRestoreRestoringPrimaryStorage LogicalRestorePhase = "RestoringPrimaryStorage"
	// LogicalRestoreCompleted means that the restore finished. The target can
	// be unsuspended.
	LogicalRestoreCompleted LogicalRestorePhase = "Completed"
	// LogicalRestoreFailed means that the restore failed. The Ready condition
	// names the failing phase.
	LogicalRestoreFailed LogicalRestorePhase = "Failed"
)

// LogicalRestoreSpec names the backup to restore and the cluster to restore
// into. The whole spec is immutable: a restore is one operation, retried by
// creating a new resource.
type LogicalRestoreSpec struct {
	// BackupRef references the completed backup to restore from.
	// +required
	BackupRef LogicalBackupRef `json:"backupRef"`
	// TargetClusterRef references the CamundaCluster to restore into. It can
	// differ from the cluster the backup was taken from. The cluster must be
	// suspended for the whole restore.
	// +required
	TargetClusterRef ClusterRef `json:"targetClusterRef"`
}

// LogicalRestoreStatus tracks the restore to a terminal phase.
type LogicalRestoreStatus struct {
	// Phase of the restore. It is the resume marker: a reconcile that
	// re-enters after a crash continues at the recorded phase.
	// +optional
	Phase LogicalRestorePhase `json:"phase,omitempty"`
	// BackupID is the backup id that the restore reads, pinned when the
	// restore starts. The backup can be deleted afterwards without moving the
	// restore to another set of artifacts.
	// +optional
	BackupID int64 `json:"backupId,omitempty"`
	// StorageType is the secondary storage type of the backup, pinned with
	// the backup id. It decides which restore procedure runs.
	// +optional
	StorageType SecondaryStorageType `json:"storageType,omitempty"`
	// TargetClusterUID pins the identity of the target cluster. A cluster
	// that is deleted and created again under the same name is another
	// cluster, and this restore is not its restore.
	// +optional
	TargetClusterUID types.UID `json:"targetClusterUID,omitempty"`
	// Brokers is the broker count read from the live broker StatefulSet when
	// the restore entered RestoringPrimaryStorage. It fixes how many volumes
	// are recreated and how many Jobs run.
	// +optional
	Brokers int32 `json:"brokers,omitempty"`
	// Repository is the Elasticsearch snapshot repository the restore reads
	// from, registered on the target's Elasticsearch. It is unset on the
	// relational path.
	// +optional
	Repository string `json:"repository,omitempty"`
	// RestoredSnapshots names every snapshot the restore asked Elasticsearch
	// to restore. It is unset on the relational path.
	// +optional
	RestoredSnapshots []string `json:"restoredSnapshots,omitempty"`
	// SecondaryJobName is the Job that runs pg_restore on the relational
	// path, while it exists. It is unset on the Elasticsearch path.
	// +optional
	SecondaryJobName string `json:"secondaryJobName,omitempty"`
	// PrimaryJobNames are the per-broker restore-application Jobs, in broker
	// order.
	// +optional
	PrimaryJobNames []string `json:"primaryJobNames,omitempty"`
	// RecreatedClaims names the broker data claims that the restore deleted
	// and created again. A reconcile that re-enters does not delete a claim
	// twice.
	// +optional
	RecreatedClaims []string `json:"recreatedClaims,omitempty"`
	// FirstFailedAt is when a dependency of the running restore first stopped
	// resolving. The operator measures the mid-run grace from it, and clears
	// it when the restore recovers.
	// +optional
	FirstFailedAt *metav1.Time `json:"firstFailedAt,omitempty"`
	// FailureMessage names the failing phase and its error. The Ready
	// condition carries the same message, and the operator stages the
	// condition again from this field, so a write conflict cannot lose it.
	// +optional
	FailureMessage string `json:"failureMessage,omitempty"`
	// CompletionTime is when the restore reached a terminal phase.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
	// ObservedGeneration is the last generation reconciled by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the current state. The Ready condition carries the
	// reasons Progressing, Completed, Failed, ClusterNotSuspended,
	// InvalidReference, IncompatibleTarget, MissingSecret, and
	// ConnectionFailed.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=logicalrestores,shortName=lr
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Backup",type=string,JSONPath=`.spec.backupRef.name`
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.targetClusterRef.name`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LogicalRestore restores one completed logical backup into a suspended
// CamundaCluster. The target can be the cluster the backup came from, or
// another cluster that reads the same backup bucket. The restore writes the
// backup into secondary storage, deletes and creates the broker data volumes
// again, and runs the Camunda restore application once per broker. The
// operator only reads spec.suspend of the target. Whoever owns the cluster
// suspends it before the restore and unsuspends it after.
type LogicalRestore struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec names the backup to restore and the cluster to restore into. It is
	// immutable: a restore is one-shot, retried by creating a new resource.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable: a restore is one-shot, retried by creating a new resource"
	Spec LogicalRestoreSpec `json:"spec"`

	// status defines the observed state of the restore
	// +optional
	Status LogicalRestoreStatus `json:"status,omitzero"`
}

// GetStatusConditions returns a pointer to the status conditions. The
// component framework stages conditions on the resource through it.
func (in *LogicalRestore) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// GetKind returns the CRD kind. The component framework uses it for event and
// metric recording.
func (in *LogicalRestore) GetKind() string { return "LogicalRestore" }

// SetObservedGeneration records the last reconciled generation in status.
func (in *LogicalRestore) SetObservedGeneration(generation int64) {
	in.Status.ObservedGeneration = generation
}

// Terminal reports whether the restore reached a phase it never leaves.
func (in *LogicalRestore) Terminal() bool {
	return in.Status.Phase == LogicalRestoreCompleted || in.Status.Phase == LogicalRestoreFailed
}

// +kubebuilder:object:root=true

// LogicalRestoreList contains a list of LogicalRestore
type LogicalRestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []LogicalRestore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LogicalRestore{}, &LogicalRestoreList{})
}
