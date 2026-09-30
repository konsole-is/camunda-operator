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
	// point-in-time recovery, or that spec.timestamp is outside its retention
	// period.
	ReasonPitrUnavailable = "PitrUnavailable"
	// ReasonSharedServer means that more than one Database has a logical
	// database on the database server. Point-in-time recovery rolls back the
	// whole server, so on a shared server it also rolls back other databases.
	ReasonSharedServer = "SharedServer"
	// ReasonDatabaseNotRestored means that the database is ahead of
	// spec.timestamp, or that it reports no exporter position for a
	// partition. The restore stays in Pending and does not change a volume.
	ReasonDatabaseNotRestored = "DatabaseNotRestored"
	// ReasonExporterPositionNotCovered means that the restore application
	// found no primary-storage checkpoint that covers the exporter position
	// of the restored database, so it restored no partition. The restore
	// fails. To recover, roll the database back further and create a new
	// restore. The operator reads this cause from the log of the failed
	// restore Job.
	ReasonExporterPositionNotCovered = "ExporterPositionNotCovered"
)

// PointInTimeRestorePhase is the phase of a restore that runs one time.
// Completed and Failed are final. To try again, create a new resource.
// +kubebuilder:validation:Enum=Pending;RestoringDatabase;ValidatingDatabaseState;RestoringPrimaryStorage;Completed;Failed
type PointInTimeRestorePhase string

// The phases of a point-in-time restore, in order.
const (
	// PointInTimeRestorePending means that the restore did not start its
	// work. A pre-check stops it: the cluster runs, the storage chain does
	// not resolve, or the database is ahead of spec.timestamp.
	PointInTimeRestorePending PointInTimeRestorePhase = "Pending"
	// PointInTimeRestoreRestoringDatabase means that the operator asked the
	// database server to roll back to spec.timestamp and waits for the
	// answer. The restore has this phase only when the DatabaseServerConfig
	// declares pitr.recovery: operator. A server that declares external rolls
	// back before you create the restore. The restore then goes directly to
	// ValidatingDatabaseState.
	PointInTimeRestoreRestoringDatabase PointInTimeRestorePhase = "RestoringDatabase"
	// PointInTimeRestoreValidatingDatabaseState means that the operator reads
	// the exporter position of every partition from the restored database.
	// This happens before the operator changes a volume.
	PointInTimeRestoreValidatingDatabaseState PointInTimeRestorePhase = "ValidatingDatabaseState"
	// PointInTimeRestoreRestoringPrimaryStorage means that the operator
	// created the broker data volumes again and runs the restore application
	// on them.
	PointInTimeRestoreRestoringPrimaryStorage PointInTimeRestorePhase = "RestoringPrimaryStorage"
	// PointInTimeRestoreCompleted means that the restore finished. The
	// operator removes the Jobs of the brokers, so their pods release the
	// broker data volumes. It removes the suspension that it applied, so the
	// cluster runs again, unless its owner suspended it.
	PointInTimeRestoreCompleted PointInTimeRestorePhase = "Completed"
	// PointInTimeRestoreFailed means that the restore failed. The Ready
	// condition names the failed phase. The operator keeps the Jobs of the
	// brokers, because their logs show the cause. Thus a restore that reached
	// RestoringPrimaryStorage holds the broker data volumes until you delete
	// it. A restore that failed in an earlier phase records no Job in
	// PrimaryJobNames and holds nothing.
	PointInTimeRestoreFailed PointInTimeRestorePhase = "Failed"
)

// PointInTimeRestoreSpec names the cluster to roll back and the point in time
// to roll it back to. The whole spec is immutable.
type PointInTimeRestoreSpec struct {
	// ClusterRef references the CamundaCluster to restore, in the namespace
	// of this restore. Its secondary storage must be a relational database.
	// +required
	ClusterRef ClusterRef `json:"clusterRef"`
	// Timestamp is the point to restore to.
	// DatabaseServerConfig.spec.pitr.recovery sets who rolls the database
	// server back to it. With operator, the restore asks the server to roll
	// back to this point. With external, you roll the server back to it
	// before you create the restore.
	//
	// Choose a point at least one backup interval before the cluster stopped
	// writing. Also choose a point inside the window in which Zeebe keeps its
	// primary-storage backups. The CamundaCluster sets these with
	// spec.backup.primaryStorage.schedule and retention.window. The defaults
	// are one hour and seven days. If no backup covers the point, the restore
	// fails after it erased the broker volumes.
	//
	// The point must be inside the retention period of the database server,
	// and it must not be in the future. The operator does these two checks.
	// +required
	Timestamp metav1.Time `json:"timestamp"`
}

// PartitionPosition is the exporter position of one partition, as the
// pre-check read it from the restored database.
type PartitionPosition struct {
	// PartitionID is the Zeebe partition.
	PartitionID int32 `json:"partitionId"`
	// LastUpdated is the LAST_UPDATED value of the row of the partition in the
	// EXPORTER_POSITION table.
	LastUpdated metav1.Time `json:"lastUpdated"`
}

// PointInTimeRestoreStorage is the identity of the storage chain that the
// restore validated. It holds the contracts that the restore resolved, the
// logical database that it read, and the server of that database. Each part
// of the chain can change. The restore checks the server and the database
// one time, before it deletes anything. If a later read does not agree with
// this record, it is another database, and the restore fails.
type PointInTimeRestoreStorage struct {
	// SecondaryStorageConfig is the storage contract of the cluster.
	SecondaryStorageConfig string `json:"secondaryStorageConfig"`
	// SecondaryStorageConfigUID records the identity of that contract. Thus
	// the restore finds a contract that was deleted and created again with
	// the same name.
	// +optional
	SecondaryStorageConfigUID types.UID `json:"secondaryStorageConfigUID,omitempty"`
	// DatabaseConfig is the contract of the logical database.
	DatabaseConfig string `json:"databaseConfig"`
	// DatabaseConfigUID records the identity of that contract.
	// +optional
	DatabaseConfigUID types.UID `json:"databaseConfigUID,omitempty"`
	// DatabaseServerConfig is the contract of the server that holds the
	// database and declares its point-in-time recovery.
	DatabaseServerConfig string `json:"databaseServerConfig"`
	// DatabaseServerConfigUID records the identity of that contract.
	// +optional
	DatabaseServerConfigUID types.UID `json:"databaseServerConfigUID,omitempty"`
	// DatabaseName is the logical database whose exporter position the
	// pre-check read.
	DatabaseName string `json:"databaseName"`
	// Endpoint is the host and port of the server, as the pre-check used it.
	// A contract that now names another endpoint names another server.
	Endpoint string `json:"endpoint"`
	// SystemIdentifier is the identity of the PostgreSQL instance behind that
	// endpoint, as the contract published it. The rule for a dedicated server
	// counted this identity. An endpoint that later reports another identity
	// is another instance.
	SystemIdentifier string `json:"systemIdentifier"`
}

// PointInTimeRestoreStatus tracks the restore to a terminal phase.
type PointInTimeRestoreStatus struct {
	// Phase is the phase of the restore. The restore continues from it after
	// an interruption.
	// +optional
	Phase PointInTimeRestorePhase `json:"phase,omitempty"`
	// Storage records the storage chain that the restore validated. The
	// operator records it one time, before it reads the database. If a later
	// read does not agree, the restore fails.
	// +optional
	Storage *PointInTimeRestoreStorage `json:"storage,omitempty"`
	// Backend names the database that the restore holds while its server
	// rolls back: the host, the port, and the database name. The operator
	// records it just before it asks for the rollback. It follows each
	// endpoint that the contract names. From then until the final phase, no
	// other CamundaCluster starts on this database, also when the contract
	// names another endpoint. A restore whose server
	// rolls back outside the operator records no backend.
	// +optional
	Backend string `json:"backend,omitempty"`
	// ObservedPositions are the exporter positions that the pre-check read,
	// in partition order. They show what the operator saw when the restore
	// passed the database-state check, or what stopped it.
	// +optional
	// +listType=map
	// +listMapKey=partitionId
	ObservedPositions []PartitionPosition `json:"observedPositions,omitempty"`
	// RestoreProgress is the part of the status that every restore kind has.
	// Its Ready condition has the reasons Progressing, Completed, Failed,
	// ClusterNotSuspended, ClusterClaimed, StorageAlreadyAttached,
	// WaitingForHandover, InvalidReference, PitrUnavailable, SharedServer,
	// DatabaseNotRestored, MissingSecret, and ConnectionFailed.
	RestoreProgress `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=pointintimerestores,shortName=pitr
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=`.spec.clusterRef.name`
// +kubebuilder:printcolumn:name="Timestamp",type=string,JSONPath=`.spec.timestamp`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// PointInTimeRestore brings the primary storage of a suspended CamundaCluster
// with relational secondary storage to the same point in time as its
// database. It reads the exporter position of every partition from that
// database. Then it deletes and creates the broker data volumes again. Then
// it runs the Camunda restore application with the requested point, one
// time for each broker.
//
// The database reaches that point in one of two ways. A DatabaseServerConfig
// that declares pitr.recovery: operator is rolled back by its publisher. The
// restore writes the request on the contract and waits for the answer. A
// contract that declares external, the default, is rolled back before you
// create the restore. The restore reads the database as it is.
//
// The restore prepares the cluster itself. It suspends the cluster and waits
// for its brokers to stop. When it completes, it removes the suspension, but
// only a suspension that it applied itself. A failed restore leaves the
// cluster suspended. A restore that somebody deletes while it runs also
// leaves the cluster suspended. Empty or half-written broker volumes cause
// more damage under running brokers.
//
// It writes no version. This kind restores the primary storage of the
// cluster from the continuous backups of the same cluster. Thus no backup
// names a version other than the version that the cluster runs.
type PointInTimeRestore struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec names the cluster to restore and the point that its database
	// holds. It is immutable. A restore runs one time. To try again, create
	// a new resource.
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
