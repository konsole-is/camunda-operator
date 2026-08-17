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

// ReasonMissingCredentials means the backup bucket uses static credentials
// and their local copy in the cluster namespace does not resolve. Only a
// LogicalBackupRDBMS reports it: the dump Job mounts those credentials.
const ReasonMissingCredentials = "MissingCredentials"

// LogicalBackupRDBMSStep is the resume marker of the backup procedure. A
// reconcile that re-enters after a crash continues at the recorded step
// instead of repeating one that already ran.
// +kubebuilder:validation:Enum=Dumping;PrimaryBackup
type LogicalBackupRDBMSStep string

// The steps of a relational backup, in order.
const (
	// StepDumping runs the Job that writes the logical database to the
	// backup bucket.
	StepDumping LogicalBackupRDBMSStep = "Dumping"
	// StepPrimaryBackup requests one cluster-generated primary-storage
	// backup, so the dump pairs with a Zeebe backup taken right after it.
	StepPrimaryBackup LogicalBackupRDBMSStep = "PrimaryBackup"
)

// LogicalBackupRDBMSSpec identifies the cluster to back up. It is immutable:
// a backup is one operation, and a retry is a new CR.
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable; retry a backup with a new CR"
type LogicalBackupRDBMSSpec struct {
	// ClusterRef references the CamundaCluster to back up. The cluster must
	// store its data in a relational database and have a backupStorageRef.
	// +required
	ClusterRef ClusterRef `json:"clusterRef"`
	// Dump replaces the cluster's spec.backup.dump block as a whole for this
	// backup. Unset means the block of the cluster. The two never merge.
	// +optional
	Dump *BackupDumpSpec `json:"dump,omitempty"`
}

// LogicalBackupRDBMSStatus is the observed state of one backup operation.
type LogicalBackupRDBMSStatus struct {
	// Phase tracks the one-shot operation. Completed and Failed are
	// terminal.
	// +optional
	Phase LogicalBackupPhase `json:"phase,omitempty"`
	// Step is where the procedure stands and where a crashed reconcile
	// resumes.
	// +optional
	Step LogicalBackupRDBMSStep `json:"step,omitempty"`
	// BackupID identifies the dump object in the bucket. It is allocated
	// once, when the backup leaves Pending.
	// +optional
	BackupID int64 `json:"backupId,omitempty"`
	// JobName is the Job that dumps and uploads the database.
	// +optional
	JobName string `json:"jobName,omitempty"`
	// ObjectKey is the full key of the dump in the backup bucket.
	// +optional
	ObjectKey string `json:"objectKey,omitempty"`
	// PrimaryBackupID is the id of the primary-storage backup that the
	// cluster generated after the dump. Unset until that backup is
	// requested.
	// +optional
	PrimaryBackupID *int64 `json:"primaryBackupId,omitempty"`
	// PrimaryBackupRequestedAt is when the primary-storage backup was
	// requested. It bounds how long the poll tolerates a backup the cluster
	// does not report yet.
	// +optional
	PrimaryBackupRequestedAt *metav1.Time `json:"primaryBackupRequestedAt,omitempty"`
	// FirstFailedAt is when a dependency of the running backup first stopped
	// resolving, or the management API first stopped answering. The mid-run
	// grace is measured from it; it clears when the backup recovers.
	// +optional
	FirstFailedAt *metav1.Time `json:"firstFailedAt,omitempty"`
	// BucketRef pins the ObjectStorageConfig the dump was written through,
	// so deletion cleans up against the bucket that actually holds the
	// object, even after the cluster's backupStorageRef moved elsewhere.
	// +optional
	BucketRef string `json:"bucketRef,omitempty"`
	// BucketGeneration is the generation of the pinned config when the
	// backup started. A different generation at deletion time means the
	// bucket coordinates may have changed under the object.
	// +optional
	BucketGeneration int64 `json:"bucketGeneration,omitempty"`
	// StorageSizes are the effective restore sizes recorded when the backup
	// started. The RDBMS kind records the Zeebe size only.
	// +optional
	StorageSizes LogicalBackupStorageSizes `json:"storageSizes,omitzero"`
	// FailureMessage is why the backup failed, set with a Failed phase. The
	// Ready condition carries the same message and is re-staged from this
	// field, so a write conflict can never lose it.
	// +optional
	FailureMessage string `json:"failureMessage,omitempty"`
	// CompletionTime is when the backup reached a terminal phase.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
	// ObservedGeneration is the last generation reconciled by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the current state; the Ready condition carries
	// the phase as its reason and names a failing step in its message.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=logicalbackuprdbmses,shortName=lbrdbms
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Step",type=string,JSONPath=`.status.step`
// +kubebuilder:printcolumn:name="Backup ID",type=integer,JSONPath=`.status.backupId`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LogicalBackupRDBMS is one backup of a relational orchestration cluster: a
// dump of the entire logical database, uploaded to the backup bucket, paired
// with one cluster-generated primary-storage backup. A restore reads the
// exporter position from the restored dump and picks the primary-storage
// backups that match it, so the pair is a complete restore point.
type LogicalBackupRDBMS struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of LogicalBackupRDBMS
	// +required
	Spec LogicalBackupRDBMSSpec `json:"spec"`

	// status defines the observed state of LogicalBackupRDBMS
	// +optional
	Status LogicalBackupRDBMSStatus `json:"status,omitzero"`
}

// GetStatusConditions returns a pointer to the status conditions. The
// component framework stages conditions on the resource through it.
func (in *LogicalBackupRDBMS) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// GetKind returns the CRD kind. The component framework uses it for event and
// metric recording.
func (in *LogicalBackupRDBMS) GetKind() string { return "LogicalBackupRDBMS" }

// SetObservedGeneration records the last reconciled generation in status.
func (in *LogicalBackupRDBMS) SetObservedGeneration(generation int64) {
	in.Status.ObservedGeneration = generation
}

// Terminal reports whether the backup can never transition again.
func (in *LogicalBackupRDBMS) Terminal() bool {
	return in.Status.Phase == LogicalBackupCompleted || in.Status.Phase == LogicalBackupFailed
}

// +kubebuilder:object:root=true

// LogicalBackupRDBMSList contains a list of LogicalBackupRDBMS
type LogicalBackupRDBMSList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []LogicalBackupRDBMS `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LogicalBackupRDBMS{}, &LogicalBackupRDBMSList{})
}
