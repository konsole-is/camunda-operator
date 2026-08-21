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

// The condition reasons that only a PointInTimeRestore reports.
const (
	// ReasonPitrUnavailable means that the database server does not declare
	// point-in-time recovery, or that spec.timestamp lies outside its
	// retention period.
	ReasonPitrUnavailable = "PitrUnavailable"
	// ReasonSharedServer means that more than one Database references the
	// database server. Engine-level point-in-time recovery rolls back the
	// whole server, so a shared server rolls back unrelated databases too.
	ReasonSharedServer = "SharedServer"
	// ReasonDatabaseNotRestored means that the database is ahead of
	// spec.timestamp, or that it reports no exporter position for a
	// partition. The restore holds in Pending and touches no volume.
	ReasonDatabaseNotRestored = "DatabaseNotRestored"
	// ReasonExporterPositionNotCovered means that the restore application
	// found no primary-storage checkpoint that covers the exporter position
	// of the restored database, so it restored no partition. The restore
	// fails, and the remedy is to roll the database back further and create
	// a new restore. The operator reads this cause from the log of the failed
	// restore Job, because only the restore application compares the two.
	ReasonExporterPositionNotCovered = "ExporterPositionNotCovered"
)

// PointInTimeRestorePhase tracks the one-shot restore. Completed and Failed
// are terminal. A retry is a new resource.
// +kubebuilder:validation:Enum=Pending;ValidatingDatabaseState;RestoringPrimaryStorage;Completed;Failed
type PointInTimeRestorePhase string

// The phases of a point-in-time restore, in order.
const (
	// PointInTimeRestorePending means that the restore did not start real
	// work. A pre-check still holds it: the cluster runs, the storage chain
	// does not resolve, or the database is ahead of spec.timestamp.
	PointInTimeRestorePending PointInTimeRestorePhase = "Pending"
	// PointInTimeRestoreValidatingDatabaseState means that the operator reads
	// the exporter position of every partition from the restored database.
	// It runs before the operator touches a volume.
	PointInTimeRestoreValidatingDatabaseState PointInTimeRestorePhase = "ValidatingDatabaseState"
	// PointInTimeRestoreRestoringPrimaryStorage means that the operator
	// recreated the broker data volumes and runs the restore application on
	// them.
	PointInTimeRestoreRestoringPrimaryStorage PointInTimeRestorePhase = "RestoringPrimaryStorage"
	// PointInTimeRestoreCompleted means that the restore finished. The
	// cluster can be unsuspended.
	PointInTimeRestoreCompleted PointInTimeRestorePhase = "Completed"
	// PointInTimeRestoreFailed means that the restore failed. The Ready
	// condition names the failing phase.
	PointInTimeRestoreFailed PointInTimeRestorePhase = "Failed"
)

// PointInTimeRestoreSpec names the cluster to roll back and the point it was
// rolled back to. The whole spec is immutable.
type PointInTimeRestoreSpec struct {
	// ClusterRef references the CamundaCluster to align, in the namespace of
	// this restore. Its secondary storage must be a relational database.
	// +required
	ClusterRef ClusterRef `json:"clusterRef"`
	// Timestamp is the point the database was already restored to. It must
	// lie within the retention period the database server declares. It must
	// not lie in the future either, and the operator checks that at reconcile
	// time, because a CEL rule has no clock.
	// +required
	Timestamp metav1.Time `json:"timestamp"`
}

// PartitionPosition is the exporter position of one partition, as the
// pre-check read it from the restored database.
type PartitionPosition struct {
	// PartitionID is the Zeebe partition.
	PartitionID int32 `json:"partitionId"`
	// LastUpdated is the LAST_UPDATED value of the partition's row in the
	// EXPORTER_POSITION table.
	LastUpdated metav1.Time `json:"lastUpdated"`
}

// PointInTimeRestoreStorage is the identity of the storage chain that the
// restore validated: the contracts it resolved, the logical database it read,
// and the server that holds it. Every link of the chain is mutable, and the
// rules of the server and the state of the database are checked once, before
// the restore deletes anything. A later look that disagrees with this record
// is another database, so the restore ends instead of acting on it.
type PointInTimeRestoreStorage struct {
	// SecondaryStorageConfig is the storage contract of the cluster.
	SecondaryStorageConfig string `json:"secondaryStorageConfig"`
	// SecondaryStorageConfigUID pins the identity of that contract, so a
	// contract that was deleted and created again under one name is caught.
	// +optional
	SecondaryStorageConfigUID types.UID `json:"secondaryStorageConfigUID,omitempty"`
	// DatabaseConfig is the contract of the logical database.
	DatabaseConfig string `json:"databaseConfig"`
	// DatabaseConfigUID pins the identity of that contract.
	// +optional
	DatabaseConfigUID types.UID `json:"databaseConfigUID,omitempty"`
	// DatabaseServerConfig is the contract of the server that holds the
	// database and declares its point-in-time recovery.
	DatabaseServerConfig string `json:"databaseServerConfig"`
	// DatabaseServerConfigUID pins the identity of that contract.
	// +optional
	DatabaseServerConfigUID types.UID `json:"databaseServerConfigUID,omitempty"`
	// DatabaseName is the logical database whose exporter position the
	// pre-check read.
	DatabaseName string `json:"databaseName"`
	// Endpoint is the host and port of the server, as the pre-check reached
	// it. A server that is repointed in place is another server.
	Endpoint string `json:"endpoint"`
}

// PointInTimeRestoreStatus tracks the restore to a terminal phase.
type PointInTimeRestoreStatus struct {
	// Phase of the restore. It is the resume marker.
	// +optional
	Phase PointInTimeRestorePhase `json:"phase,omitempty"`
	// Storage pins the storage chain that the restore validated. The operator
	// records it once, before it reads the database, and fails the restore
	// when a later look disagrees.
	// +optional
	Storage *PointInTimeRestoreStorage `json:"storage,omitempty"`
	// ObservedPositions are the exporter positions the pre-check read, in
	// partition order. They record what the operator saw when it let the
	// restore past the database-state check, or what held it.
	// +optional
	// +listType=map
	// +listMapKey=partitionId
	ObservedPositions []PartitionPosition `json:"observedPositions,omitempty"`
	// RestoreProgress is the part of the status that every restore kind has.
	// Its Ready condition carries the reasons Progressing, Completed, Failed,
	// ClusterNotSuspended, ClusterClaimed, InvalidReference, PitrUnavailable,
	// SharedServer, DatabaseNotRestored, MissingSecret, and ConnectionFailed.
	RestoreProgress `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=pointintimerestores,shortName=pitr
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=`.spec.clusterRef.name`
// +kubebuilder:printcolumn:name="Timestamp",type=string,JSONPath=`.spec.timestamp`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// PointInTimeRestore aligns the primary storage of a suspended,
// relational-backed CamundaCluster with a database that was already restored
// to a point in time. The operator never restores the database server. It
// reads the exporter position of every partition from the restored database,
// deletes and creates the broker data volumes again, and runs the Camunda
// restore application with the requested point once per broker. The operator
// only reads spec.suspend of the cluster. Whoever owns the cluster suspends
// it before the restore and unsuspends it after.
type PointInTimeRestore struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec names the cluster to align and the point its database holds. It is
	// immutable: a restore is one-shot, retried by creating a new resource.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable: a restore is one-shot, retried by creating a new resource"
	Spec PointInTimeRestoreSpec `json:"spec"`

	// status defines the observed state of the restore
	// +optional
	Status PointInTimeRestoreStatus `json:"status,omitzero"`
}

// GetStatusConditions returns a pointer to the status conditions. The
// component framework stages conditions on the resource through it.
func (in *PointInTimeRestore) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// GetKind returns the CRD kind. The component framework uses it for event and
// metric recording.
func (in *PointInTimeRestore) GetKind() string { return "PointInTimeRestore" }

// SetObservedGeneration records the last reconciled generation in status.
func (in *PointInTimeRestore) SetObservedGeneration(generation int64) {
	in.Status.ObservedGeneration = generation
}

// Terminal reports whether the restore reached a phase it never leaves.
func (in *PointInTimeRestore) Terminal() bool {
	return in.Status.Phase == PointInTimeRestoreCompleted ||
		in.Status.Phase == PointInTimeRestoreFailed
}

// +kubebuilder:object:root=true

// PointInTimeRestoreList contains a list of PointInTimeRestore
type PointInTimeRestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []PointInTimeRestore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PointInTimeRestore{}, &PointInTimeRestoreList{})
}
