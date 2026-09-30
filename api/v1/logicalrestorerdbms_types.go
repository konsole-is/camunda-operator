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

// LogicalRestoreRDBMSSpec names the backup to restore and the cluster to
// restore into. The whole spec is immutable. A restore runs one time. To try
// again, create a new resource.
type LogicalRestoreRDBMSSpec struct {
	// BackupRef references the completed LogicalBackupRDBMS to restore from,
	// in the namespace of this restore.
	// +required
	BackupRef LogicalBackupRef `json:"backupRef"`
	// TargetClusterRef references the CamundaCluster to restore into. It
	// must name the cluster of the backup, because the restore application
	// reads the primary-storage backup under the prefix of that cluster. The
	// cluster must be suspended for the whole restore.
	// +required
	TargetClusterRef ClusterRef `json:"targetClusterRef"`
}

// LogicalRestoreRDBMSStatus is the progress of the restore to a final phase.
type LogicalRestoreRDBMSStatus struct {
	// Phase is the phase of the restore. After an interruption, the restore
	// continues at this phase.
	// +optional
	Phase LogicalRestorePhase `json:"phase,omitempty"`
	// BackupID is the Zeebe backup id that the restore reads. The operator
	// records it when the restore starts. A backup that is deleted and
	// created again with the same name has another id, and this restore does
	// not read it. Later phases of the restore still read the backup
	// resource, so keep it until the restore completes.
	// +optional
	BackupID int64 `json:"backupId,omitempty"`
	// Backend is the logical database that the restore writes, as the host,
	// the port, and the database name. The operator records it when the
	// restore starts. From then until the final phase, no other
	// CamundaCluster starts on this backend. The restore waits while its
	// target does not hold the backend.
	// +optional
	Backend string `json:"backend,omitempty"`
	// Contract is the DatabaseServerConfig and the database name of Backend
	// when the restore started. When the DatabaseServerConfig moves to
	// another address, the restore still holds this contract.
	// +optional
	Contract string `json:"contract,omitempty"`
	// SecondaryJobName is the Job that runs pg_restore, while the Job exists.
	// +optional
	SecondaryJobName string `json:"secondaryJobName,omitempty"`
	// RestoreProgress is the part of the status that every restore kind has.
	// Its Ready condition has the reasons Progressing, Completed, Failed,
	// ClusterNotSuspended, ClusterClaimed, IncompatibleTarget,
	// StorageAlreadyAttached, WaitingForHandover, InvalidReference,
	// MissingSecret, and MissingCredentials.
	RestoreProgress `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=logicalrestorerdbmses,shortName=lrrdbms
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Backup",type=string,JSONPath=`.spec.backupRef.name`
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.targetClusterRef.name`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LogicalRestoreRDBMS restores one completed LogicalBackupRDBMS into one
// suspended CamundaCluster. The operator writes the dump into the logical
// database of the target with pg_restore. Then it gives the brokers empty
// data volumes and runs the Camunda restore application on them, one time
// for each broker.
//
// The restore prepares the target itself. It suspends the target, waits for
// its brokers to stop, and sets spec.version to the Camunda version of the
// backup. When it completes, it removes the suspension, but only a
// suspension that it applied itself. A failed restore leaves the target
// suspended. A restore that somebody deletes while it runs also leaves the
// target suspended. Empty or half-written broker volumes cause more damage
// under running brokers.
//
// The restore keeps spec.version, with the field manager
// camunda-operator/restore-version. The target runs the version of the
// backup until another manager takes over or removes that field. A manifest
// without spec.version does not change it, because server-side apply
// removes a field only for the manager that set it. Thus a target of a newer
// minor stays on the minor of the backup, and the owner must upgrade it
// again.
type LogicalRestoreRDBMS struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec names the backup to read and the cluster to restore into. It is
	// immutable. A restore runs one time. To try again, create a new
	// resource.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable: a restore is one-shot, retried by creating a new resource"
	Spec LogicalRestoreRDBMSSpec `json:"spec"`

	// status defines the observed state of the restore
	// +optional
	Status LogicalRestoreRDBMSStatus `json:"status,omitzero"`
}

// GetStatusConditions returns a pointer to the status conditions. The
// component framework stages conditions on the resource through it.
func (in *LogicalRestoreRDBMS) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// GetKind returns the CRD kind. The component framework uses it for event and
// metric recording.
func (in *LogicalRestoreRDBMS) GetKind() string { return "LogicalRestoreRDBMS" }

// SetObservedGeneration records the last reconciled generation in status.
func (in *LogicalRestoreRDBMS) SetObservedGeneration(generation int64) {
	in.Status.ObservedGeneration = generation
}

// Terminal reports whether the restore reached a phase it never leaves.
func (in *LogicalRestoreRDBMS) Terminal() bool {
	return in.Status.Phase == LogicalRestoreCompleted ||
		in.Status.Phase == LogicalRestoreFailed
}

// +kubebuilder:object:root=true

// LogicalRestoreRDBMSList contains a list of LogicalRestoreRDBMS
type LogicalRestoreRDBMSList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []LogicalRestoreRDBMS `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LogicalRestoreRDBMS{}, &LogicalRestoreRDBMSList{})
}
