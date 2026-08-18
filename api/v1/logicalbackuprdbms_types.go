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
// +kubebuilder:validation:Enum=Dumping;ZeebeBackup
type LogicalBackupRDBMSStep string

// The steps of a relational backup, in order.
const (
	// StepDumping runs the Job that writes the logical database to the
	// backup bucket.
	StepDumping LogicalBackupRDBMSStep = "Dumping"
	// StepZeebeBackup requests one Zeebe backup — Camunda's own backup of
	// its primary storage, the Zeebe log and snapshots — right after the
	// dump, so the two pair into one restore point.
	StepZeebeBackup LogicalBackupRDBMSStep = "ZeebeBackup"
)

// LogicalBackupRDBMSSpec identifies the cluster to back up. It is immutable:
// a backup is one operation, and a retry is a new CR.
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable; retry a backup with a new CR"
type LogicalBackupRDBMSSpec struct {
	// ClusterRef references the CamundaCluster to back up. The cluster must
	// store its data in a relational database and have a backupStorageRef.
	// +required
	ClusterRef ClusterRef `json:"clusterRef"`
	// Dump replaces the pod settings of the cluster's spec.backup.dump block
	// as a whole for this backup. Unset means the settings of the cluster.
	// The two never merge. The image that runs the dump is not among them:
	// the Job runs under the cluster's ServiceAccount, so the executable is
	// the cluster owner's choice and always comes from the cluster block.
	// +optional
	Dump *DumpPodSpec `json:"dump,omitempty"`
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
	// JobName is the Job that dumps and uploads the database, while it
	// exists. It clears once the dump is recorded and the Job released; a
	// failed Job stays, and its name with it, until the backup is deleted.
	// +optional
	JobName string `json:"jobName,omitempty"`
	// ObjectKey is the full key of the dump in the backup bucket.
	// +optional
	ObjectKey string `json:"objectKey,omitempty"`
	// ZeebeBackupID is the id of the Zeebe backup that the cluster generated
	// after the dump. Unset until that backup is requested.
	// +optional
	ZeebeBackupID *int64 `json:"zeebeBackupId,omitempty"`
	// ZeebeBackupRequestedAt is when the Zeebe backup was requested. It
	// bounds how long the poll tolerates a backup the cluster does not
	// report yet.
	// +optional
	ZeebeBackupRequestedAt *metav1.Time `json:"zeebeBackupRequestedAt,omitempty"`
	// WorkloadConfigHash pins the configuration Zeebe ran when the backup
	// started: the config hash of the live Zeebe pod template. The Zeebe
	// backup is requested only while the hash is unchanged — a database
	// swapped in between would pair the dump with a Zeebe backup of another
	// configuration, and the cluster's generation alone cannot tell, because
	// mutable referents enter the hash without bumping it.
	// +optional
	WorkloadConfigHash string `json:"workloadConfigHash,omitempty"`
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
	// BucketLocation pins where the object was written: the storage type,
	// bucket, base path, and endpoint of the ObjectStorageConfig at the
	// start, as ObjectStorageConfig.Location renders them. Deletion runs
	// only while the contract still points there; a retargeted contract
	// leaves the object behind instead of deleting a stranger's object at
	// the same key.
	// +optional
	BucketLocation string `json:"bucketLocation,omitempty"`
	// BucketGeneration is the generation of the pinned config when the
	// backup started, for reference; BucketLocation is what decides whether
	// deletion may run.
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
// with one Zeebe backup. Camunda calls the Zeebe log and snapshots its
// "primary storage" and the exported relational data its "secondary
// storage"; a Zeebe backup is Camunda's own backup of that primary storage
// to the backup bucket, requested through the management API. A restore
// reads the exporter position from the restored dump and picks the Zeebe
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
