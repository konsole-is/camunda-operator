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
	// again while the restore ran. A restore rewrites primary storage, so it
	// holds until the cluster is suspended again, and it fails after the
	// mid-run grace.
	//
	// Admission never reports the reason. A restore suspends its own cluster
	// there, and it unsuspends the cluster again when it completes.
	ReasonClusterNotSuspended = "ClusterNotSuspended"
	// ReasonClusterClaimed means that another backup or another restore holds
	// the cluster. The restore waits in Pending until that holder reaches a
	// terminal phase. Nothing bounds the wait, and the reason names no kind,
	// because the holder can be either.
	ReasonClusterClaimed = "ClusterClaimed"
	// ReasonIncompatibleTarget means that the target cluster cannot hold the
	// backup: the target is not the cluster the backup was taken from, the
	// secondary storage types differ, the backup bucket differs, or the
	// Camunda versions break the version rule. Only the Elasticsearch kind
	// also compares the partition counts, because a relational backup records
	// none. Only a logical restore reports the reason.
	ReasonIncompatibleTarget = "IncompatibleTarget"
)

// LogicalBackupRef references a completed logical backup in the namespace of
// the restore. The reference never crosses a namespace. The kind of the
// restore says which backup kind it reads, so the reference carries a name
// alone.
type LogicalBackupRef struct {
	// Name of the backup, in the namespace of this restore.
	// +required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// LogicalRestorePhase tracks a one-shot logical restore. Completed and Failed
// are terminal. A retry is a new resource. Both logical restore kinds use it,
// because their phase values are the same.
// +kubebuilder:validation:Enum=Pending;ValidatingCompatibility;RestoringSecondaryStorage;RestoringPrimaryStorage;Completed;Failed
type LogicalRestorePhase string

// The phases of a logical restore, in order.
const (
	// LogicalRestorePending means that the restore did not start real work.
	// A pre-check still holds it: the target cluster runs, another operation
	// holds the cluster, or the backup is not completed.
	LogicalRestorePending LogicalRestorePhase = "Pending"
	// LogicalRestoreValidatingCompatibility means that the operator compares
	// the backup against the target: the storage type, the backup bucket, the
	// Camunda version, and, on the Elasticsearch kind, the partition count.
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

// RestoreProgress is the part of a restore status that every restore kind
// has. It is embedded with json:",inline", so each status keeps the field
// names it had before and the CRD schema does not change. controller-gen
// flattens an inline embedded struct the same way encoding/json does.
//
// pkg/restore reads and writes this struct in place, through the driver
// calls that every restore kind makes. TargetClusterUID is the exception:
// each controller pins it during its own admission, before the driver first
// runs, so that every rule it checks is measured against one cluster. A
// PointInTimeRestore pins Brokers in its admission too, because its rules
// read the live broker StatefulSet there. For the other kinds the driver
// pins Brokers on its first primary-storage pass. Each kind owns its own
// phase and the fields of its own procedure.
type RestoreProgress struct {
	// TargetClusterUID pins the identity of the target cluster. A cluster
	// that is deleted and created again under the same name is another
	// cluster, and this restore is not its restore.
	// +optional
	TargetClusterUID types.UID `json:"targetClusterUID,omitempty"`
	// Brokers is the broker count read from the live broker StatefulSet and
	// recorded before the restore deletes a volume. It fixes how many volumes
	// are recreated and how many Jobs run.
	// +optional
	Brokers int32 `json:"brokers,omitempty"`
	// PrimaryJobNames are the per-broker restore-application Jobs, in broker
	// order. The operator records them before it applies the Jobs, so the
	// record covers every Job that the next look finds.
	// +optional
	PrimaryJobNames []string `json:"primaryJobNames,omitempty"`
	// RecreatedClaims names the broker data claims that the restore deleted
	// and created again. A reconcile that re-enters does not delete a claim
	// twice.
	// +optional
	RecreatedClaims []string `json:"recreatedClaims,omitempty"`
	// FirstFailedAt is when a dependency of the running restore first stopped
	// resolving. The operator measures the mid-run grace from it. It stays
	// set once the restore starts, because a dependency that flaps must not
	// reset the grace.
	// +optional
	FirstFailedAt *metav1.Time `json:"firstFailedAt,omitempty"`
	// ClusterSuspended records that this restore suspended its target
	// cluster. The restore withdraws that suspension when it reaches
	// Completed. A cluster that its owner suspended carries no such record,
	// so it stays suspended, and so does the cluster of a failed restore.
	// +optional
	ClusterSuspended bool `json:"clusterSuspended,omitempty"`
	// TerminalReason is the Ready reason recorded at the terminal transition.
	// The operator stages the terminal condition again from this field, so a
	// write conflict cannot replace the reason with a weaker one.
	// +optional
	TerminalReason string `json:"terminalReason,omitempty"`
	// FailureMessage names the failing phase and its error. The Ready
	// condition carries the same message.
	// +optional
	FailureMessage string `json:"failureMessage,omitempty"`
	// CompletionTime is when the restore reached a terminal phase.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
	// ObservedGeneration is the last generation reconciled by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the current state of the restore.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
