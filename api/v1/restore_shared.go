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

// The condition vocabulary that every restore kind reports. A reason that only
// one restore kind reports is declared next to that kind, in its types file.
const (
	// ReasonClusterNotSuspended means that the target cluster started running
	// again while the restore ran. A restore writes primary storage, so it
	// waits until the cluster is suspended again.
	//
	// A restore suspends its cluster itself before it starts, and removes
	// that suspension when it completes. Thus this reason does not appear
	// before the restore starts.
	ReasonClusterNotSuspended = "ClusterNotSuspended"
	// ReasonClusterClaimed means that another backup or another restore holds
	// the cluster. The restore waits in Pending until that holder reaches a
	// final phase. The wait has no time limit. The reason does not name the
	// kind of the holder, because it can be a backup or a restore.
	ReasonClusterClaimed = "ClusterClaimed"
	// ReasonIncompatibleTarget means that the target cluster cannot hold the
	// backup. The target is not the cluster of the backup, the secondary
	// storage types differ, the backup bucket differs, or the Camunda
	// versions break the version rule. Only the Elasticsearch kind also
	// compares the partition counts, because a relational backup records
	// none. Only a logical restore reports this reason.
	ReasonIncompatibleTarget = "IncompatibleTarget"
)

// LogicalBackupRef references a completed logical backup in the namespace of
// the restore. The reference never crosses a namespace. The kind of the
// restore sets the kind of the backup, so the reference holds only a name.
type LogicalBackupRef struct {
	// Name is the name of the backup, in the namespace of this restore.
	// +required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// LogicalRestorePhase is the phase of a logical restore that runs one time.
// Completed and Failed are final. To try again, create a new resource. Both
// logical restore kinds use it, because their phases are the same.
// +kubebuilder:validation:Enum=Pending;ValidatingCompatibility;RestoringSecondaryStorage;RestoringPrimaryStorage;Completed;Failed
type LogicalRestorePhase string

// The phases of a logical restore, in order.
const (
	// LogicalRestorePending means that the restore did not start its work. A
	// pre-check stops it: the target cluster runs, another operation holds
	// the cluster, or the backup is not completed.
	LogicalRestorePending LogicalRestorePhase = "Pending"
	// LogicalRestoreValidatingCompatibility means that the operator compares
	// the backup with the target. It compares the storage type, the backup
	// bucket, the Camunda version, and, on the Elasticsearch kind, the
	// partition count.
	LogicalRestoreValidatingCompatibility LogicalRestorePhase = "ValidatingCompatibility"
	// LogicalRestoreRestoringSecondaryStorage means that the operator writes
	// the backup into the secondary storage of the target.
	LogicalRestoreRestoringSecondaryStorage LogicalRestorePhase = "RestoringSecondaryStorage"
	// LogicalRestoreRestoringPrimaryStorage means that the operator created
	// the broker data volumes again and runs the restore application on them.
	LogicalRestoreRestoringPrimaryStorage LogicalRestorePhase = "RestoringPrimaryStorage"
	// LogicalRestoreCompleted means that the restore finished. The operator
	// removes the Jobs of the brokers, so their pods release the broker data
	// volumes. It removes the suspension that it applied, so the target runs
	// again, unless its owner suspended it.
	LogicalRestoreCompleted LogicalRestorePhase = "Completed"
	// LogicalRestoreFailed means that the restore failed. The Ready condition
	// names the failed phase. The operator keeps the Jobs of the brokers,
	// because their logs show the cause. Thus a restore that reached
	// RestoringPrimaryStorage holds the broker data volumes until you delete
	// it. A restore that failed in an earlier phase records no Job in
	// PrimaryJobNames and holds nothing.
	LogicalRestoreFailed LogicalRestorePhase = "Failed"
)

// pkg/restore reads and writes this struct in place, through the driver
// calls that every restore kind makes. TargetClusterUID is the exception.
// Each controller records it during its own admission, before the driver
// first runs, so that every rule it checks uses one cluster. A
// PointInTimeRestore records Brokers in its admission too, because its rules
// read the live broker StatefulSet there. For the other kinds, the driver
// records Brokers on its first primary-storage pass. Each kind owns its own
// phase and the fields of its own procedure.

// RestoreProgress is the part of a restore status that every restore kind
// has. Each restore status embeds it inline, so its fields appear directly
// in the status.
type RestoreProgress struct {
	// TargetClusterUID records the identity of the target cluster. A cluster
	// that is deleted and created again with the same name is another
	// cluster, and this restore does not apply to it.
	// +optional
	TargetClusterUID types.UID `json:"targetClusterUID,omitempty"`
	// Brokers is the broker count of the broker StatefulSet. The operator
	// records it before the restore deletes a volume. It sets how many
	// volumes the restore creates again and how many Jobs run.
	// +optional
	Brokers int32 `json:"brokers,omitempty"`
	// PrimaryJobNames are the restore application Jobs, one for each broker,
	// in broker order. The operator records them before it creates the Jobs.
	// A completed restore removes these Jobs. The logs of these Jobs explain
	// a failed restore.
	// +optional
	PrimaryJobNames []string `json:"primaryJobNames,omitempty"`
	// RecreatedClaims names the broker data claims that the restore deleted
	// and created again. Thus the restore does not delete a claim two times.
	// +optional
	RecreatedClaims []string `json:"recreatedClaims,omitempty"`
	// FirstFailedAt is when a dependency of the running restore first stopped
	// resolving. The grace period starts at this time. The field can clear if
	// the dependency recovers before the restore deletes target indices or
	// records a recreated broker volume or a broker Job. After that point, the
	// field stays set, so a dependency that fails and recovers again and
	// again does not reset the grace period.
	// +optional
	FirstFailedAt *metav1.Time `json:"firstFailedAt,omitempty"`
	// ClusterSuspended records that this restore suspended its target
	// cluster. The restore removes that suspension when it reaches
	// Completed. A cluster that its owner suspended has no such record, so
	// it stays suspended. The cluster of a failed restore also stays
	// suspended.
	// +optional
	ClusterSuspended bool `json:"clusterSuspended,omitempty"`
	// TerminalReason is the Ready reason that the operator recorded when the
	// restore reached its final phase.
	// +optional
	TerminalReason string `json:"terminalReason,omitempty"`
	// FailureMessage names the failed phase and its error. The Ready
	// condition has the same message.
	// +optional
	FailureMessage string `json:"failureMessage,omitempty"`
	// CompletionTime is when the restore reached a final phase.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
	// ObservedGeneration is the last generation that the operator processed.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the current state of the restore.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
