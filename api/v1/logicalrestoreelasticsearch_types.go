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

// LogicalRestoreElasticsearchSpec names the backup to restore and the cluster
// to restore into. The whole spec is immutable. A restore runs one time. To
// try again, create a new resource.
type LogicalRestoreElasticsearchSpec struct {
	// BackupRef references the completed LogicalBackupElasticsearch to
	// restore from, in the namespace of this restore.
	// +required
	BackupRef LogicalBackupRef `json:"backupRef"`
	// TargetClusterRef references the CamundaCluster to restore into. It
	// must name the cluster of the backup, because the restore application
	// reads the primary-storage backup under the prefix of that cluster. The
	// cluster must stay suspended for the whole restore.
	// +required
	TargetClusterRef ClusterRef `json:"targetClusterRef"`
}

// LogicalRestoreElasticsearchStatus is the progress of the restore to a
// final phase.
type LogicalRestoreElasticsearchStatus struct {
	// Phase is the phase of the restore. After an interruption, the restore
	// continues at this phase.
	// +optional
	Phase LogicalRestorePhase `json:"phase,omitempty"`
	// BackupID is the backup that the restore reads. The operator records it
	// when the restore starts. A backup that is deleted and created again
	// with the same name has another ID, and this restore does not read it.
	// Later phases of the restore still read the backup resource, so keep it
	// until the restore completes.
	// +optional
	BackupID int64 `json:"backupId,omitempty"`
	// Backend is the Elasticsearch that the restore writes, as the scheme,
	// the host, and the port. The operator records it when the restore
	// starts. From then until the final phase, and after it while
	// recoveryHeld is true, no other CamundaCluster starts on this backend.
	// The restore waits while its target does not hold the backend.
	// +optional
	Backend string `json:"backend,omitempty"`
	// RecoveryHeld is true while a failed or deleted restore keeps the
	// backend, because Elasticsearch can still recover snapshots that the
	// restore asked for. While it is true, no other CamundaCluster starts on
	// the backend, and the target stays suspended. No other backup or restore
	// of the target starts. A deleted restore stays while it is true. It is
	// unset on a restore that was never held, and false when the hold ends.
	// +optional
	RecoveryHeld *bool `json:"recoveryHeld,omitempty"`
	// RecoveryUnknownSince is the time since when a held restore cannot read
	// the recovery from Elasticsearch. If the recovery stays unknown for ten
	// minutes, the restore releases the backend.
	// +optional
	RecoveryUnknownSince *metav1.Time `json:"recoveryUnknownSince,omitempty"`
	// Repository is the Elasticsearch snapshot repository that the restore
	// reads from, on the Elasticsearch of the target.
	// +optional
	Repository string `json:"repository,omitempty"`
	// RestoredSnapshots names every snapshot that the restore asked
	// Elasticsearch to restore. When the restore continues after an
	// interruption, it does not delete the indices again.
	// +optional
	RestoredSnapshots []string `json:"restoredSnapshots,omitempty"`
	// RestoreProgress is the part of the status that every restore kind has.
	RestoreProgress `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=logicalrestoreelasticsearches,shortName=lres
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Backup",type=string,JSONPath=`.spec.backupRef.name`
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.targetClusterRef.name`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LogicalRestoreElasticsearch restores one completed
// LogicalBackupElasticsearch into one suspended CamundaCluster. It deletes
// the Camunda indices of the target and restores every snapshot of the backup
// into its Elasticsearch. Then it gives the brokers empty data volumes and
// runs the Camunda restore application one time for each broker.
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
// removes a field only for the manager that set it. This applies also to a
// target that gets its version from a preset. The value of the restore wins
// over the preset until somebody removes the field.
type LogicalRestoreElasticsearch struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec names the backup to restore and the cluster to restore into. It
	// is immutable. A restore runs one time. To try again, create a new
	// resource.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable: a restore is one-shot, retried by creating a new resource"
	Spec LogicalRestoreElasticsearchSpec `json:"spec"`

	// status defines the observed state of the restore
	// +optional
	Status LogicalRestoreElasticsearchStatus `json:"status,omitzero"`
}

// GetStatusConditions returns a pointer to the status conditions. The
// component framework stages conditions on the resource through it.
func (in *LogicalRestoreElasticsearch) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// GetKind returns the CRD kind. The component framework uses it for event and
// metric recording.
func (in *LogicalRestoreElasticsearch) GetKind() string { return "LogicalRestoreElasticsearch" }

// SetObservedGeneration records the last reconciled generation in status.
func (in *LogicalRestoreElasticsearch) SetObservedGeneration(generation int64) {
	in.Status.ObservedGeneration = generation
}

// Terminal reports whether the restore reached a phase it never leaves.
func (in *LogicalRestoreElasticsearch) Terminal() bool {
	return in.Status.Phase == LogicalRestoreCompleted ||
		in.Status.Phase == LogicalRestoreFailed
}

// +kubebuilder:object:root=true

// LogicalRestoreElasticsearchList contains a list of LogicalRestoreElasticsearch
type LogicalRestoreElasticsearchList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []LogicalRestoreElasticsearch `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LogicalRestoreElasticsearch{}, &LogicalRestoreElasticsearchList{})
}
