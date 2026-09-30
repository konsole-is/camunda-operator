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

// ReasonMissingCredentials means that the backup bucket uses static
// credentials, and their copy in the namespace of the cluster does not
// resolve. The dump Job mounts those credentials. Thus only a
// LogicalBackupRDBMS and a LogicalRestoreRDBMS report this reason.
const ReasonMissingCredentials = "MissingCredentials"

// LogicalBackupRDBMSStep is the current step of the backup. After an
// interruption, the backup continues at this step. It does not repeat a step
// that already ran.
// +kubebuilder:validation:Enum=Dumping;ZeebeBackup
type LogicalBackupRDBMSStep string

// The steps of a relational backup, in order.
const (
	// StepDumping runs the Job that writes the logical database to the
	// backup bucket.
	StepDumping LogicalBackupRDBMSStep = "Dumping"
	// StepZeebeBackup requests one Zeebe backup directly after the dump, so
	// that the two make one restore point. A Zeebe backup is the backup that
	// Camunda takes of its primary storage: the Zeebe log and snapshots.
	StepZeebeBackup LogicalBackupRDBMSStep = "ZeebeBackup"
)

// LogicalBackupRDBMSSpec identifies the cluster to back up. It is immutable.
// A backup runs one time. To try again, create a new CR.
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable; retry a backup with a new CR"
type LogicalBackupRDBMSSpec struct {
	// ClusterRef references the CamundaCluster to back up. The cluster must
	// store its data in a relational database and have a backupStorageRef.
	// +required
	ClusterRef ClusterRef `json:"clusterRef"`
	// Dump replaces all the pod settings of the spec.backup.dump block of
	// the cluster for this backup. When unset, the settings of the cluster
	// apply. The two never merge. The image of the dump is not one of these
	// settings. The Job runs under the ServiceAccount of the cluster, so the
	// image always comes from the block of the cluster.
	//
	// The environment has limits too. The extraEnv of a backup cannot set a
	// name that starts with PG or UPLOAD_. Every extraEnvFrom source needs a
	// prefix that cannot make such a name. libpq prefers PGHOSTADDR over the
	// PGHOST of the Job. Without the limit, a source can send the dump, with
	// the credentials of the Job, to another host. The environment of this
	// block reaches only the dump container, never the upload container.
	// Cloud SDKs read endpoint, proxy, and configuration variables from the
	// environment, and a backup must not change where its dump goes. The
	// block of the cluster has no prefix limit, and its environment reaches
	// every container.
	// +kubebuilder:validation:XValidation:rule="!has(self.extraEnvFrom) || self.extraEnvFrom.all(s, has(s.prefix) && s.prefix != '' && !s.prefix.startsWith('PG') && !'PG'.startsWith(s.prefix) && !s.prefix.startsWith('UPLOAD_') && !'UPLOAD_'.startsWith(s.prefix))",message="every extraEnvFrom source of a backup needs a prefix, and one that cannot spell a PG* or UPLOAD_* name"
	// +optional
	Dump *DumpPodSpec `json:"dump,omitempty"`
}

// LogicalBackupRDBMSStatus is the observed state of one backup operation.
type LogicalBackupRDBMSStatus struct {
	// Phase is the phase of the backup. Completed and Failed are final.
	// +optional
	Phase LogicalBackupPhase `json:"phase,omitempty"`
	// Step is the current step of the backup. After an interruption, the
	// backup continues at this step.
	// +optional
	Step LogicalBackupRDBMSStep `json:"step,omitempty"`
	// BackupID identifies the dump object in the bucket. The operator sets
	// it one time, when the backup leaves Pending.
	// +optional
	BackupID int64 `json:"backupId,omitempty"`
	// JobName is the Job that dumps and uploads the database, while the Job
	// exists. It clears when the operator recorded the dump and removed the
	// Job. A failed Job stays until the backup is deleted, and its name stays
	// too.
	// +optional
	JobName string `json:"jobName,omitempty"`
	// ObjectKey is the full key of the dump in the backup bucket:
	// <basePath>/<namespace>/<cluster>/<backupId>/<uid>/camunda.dump. The
	// uid is the UID of this resource. Thus a backup id that is used again
	// never names the dump of another backup.
	// +optional
	ObjectKey string `json:"objectKey,omitempty"`
	// ZeebeBackupID is the id of the Zeebe backup that the cluster generated
	// after the dump. It is unset until the operator requests that backup.
	// +optional
	ZeebeBackupID *int64 `json:"zeebeBackupId,omitempty"`
	// ZeebeBackupRequestedAt is when the operator requested the Zeebe
	// backup. It limits how long the operator waits for the cluster to report
	// the backup.
	// +optional
	ZeebeBackupRequestedAt *metav1.Time `json:"zeebeBackupRequestedAt,omitempty"`
	// WorkloadConfigHash records the configuration of Zeebe when the backup
	// started. It is the config hash of the Zeebe pod template. The operator
	// requests the Zeebe backup only while the hash is unchanged. Otherwise,
	// for example after a change of the database, the dump pairs with a Zeebe
	// backup of another configuration. The generation of the cluster is not
	// sufficient, because a change of a referenced object changes the hash
	// but not the generation.
	// +optional
	WorkloadConfigHash string `json:"workloadConfigHash,omitempty"`
	// ClusterUID records the CamundaCluster of the backup. A cluster that is
	// deleted and created again with the same name is another cluster, with
	// other primary storage. If the UID of the cluster changes, the backup
	// fails. Thus a dump never pairs with the Zeebe backup of another
	// cluster.
	// +optional
	ClusterUID types.UID `json:"clusterUID,omitempty"`
	// Version is the Camunda version of the cluster when the backup started,
	// as the management binding reported it. A restore compares it with the
	// version of its target. A relational backup restores to the same Camunda
	// minor or to one minor newer. A restore can read the version only here,
	// because a suspended cluster has no management binding.
	// +optional
	Version string `json:"version,omitempty"`
	// FirstFailedAt is when a dependency of the running backup first stopped
	// resolving, or the management API first stopped answering. The grace
	// period starts at this time. It clears when the backup recovers.
	// +optional
	FirstFailedAt *metav1.Time `json:"firstFailedAt,omitempty"`
	// BucketRef records the ObjectStorageConfig through which the Job wrote
	// the dump. On deletion, the operator removes the object from that
	// bucket, also when the backupStorageRef of the cluster changed.
	// +optional
	BucketRef string `json:"bucketRef,omitempty"`
	// BucketLocation records where the Job wrote the object: the storage
	// type, the bucket, the base path, and the endpoint of the
	// ObjectStorageConfig at the start. On deletion, the operator removes the
	// object only while the contract still points there. If the contract
	// points somewhere else, the object stays. Thus the operator never
	// deletes an unrelated object with the same key.
	// +optional
	BucketLocation string `json:"bucketLocation,omitempty"`
	// BucketGeneration is the generation of the recorded ObjectStorageConfig
	// when the backup started. It is information only. BucketLocation
	// decides whether a deletion can run.
	// +optional
	BucketGeneration int64 `json:"bucketGeneration,omitempty"`
	// StorageSizes are the effective restore sizes that the operator
	// recorded when the backup started. The RDBMS kind records only the Zeebe
	// size.
	// +optional
	StorageSizes LogicalBackupStorageSizes `json:"storageSizes,omitzero"`
	// FailureMessage tells why the backup failed. It is set with the Failed
	// phase. The Ready condition has the same message. The operator sets the
	// condition again from this field, so a write conflict never loses it.
	// +optional
	FailureMessage string `json:"failureMessage,omitempty"`
	// CompletionTime is when the backup reached a final phase.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
	// ObservedGeneration is the last generation that the operator processed.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the current state. The Ready condition has the
	// phase as its reason. Its message names a failed step.
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

// LogicalBackupRDBMS is one backup of an orchestration cluster with
// relational secondary storage. It is a dump of the whole logical database in
// the backup bucket, with one Zeebe backup. Camunda calls the Zeebe log and
// snapshots its "primary storage", and the exported relational data its
// "secondary storage". A Zeebe backup is the backup of the primary storage
// that Camunda writes to the backup bucket, on a request through the
// management API. A restore reads the exporter position from the restored
// dump and selects the Zeebe backups that match it. Thus the pair is a
// complete restore point.
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
