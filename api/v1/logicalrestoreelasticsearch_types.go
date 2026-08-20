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
// to restore into. The whole spec is immutable: a restore is one operation,
// retried by creating a new resource.
type LogicalRestoreElasticsearchSpec struct {
	// backupRef references the completed LogicalBackupElasticsearch to
	// restore from. It lives in the namespace of this restore.
	// +required
	BackupRef LogicalBackupRef `json:"backupRef"`
	// targetClusterRef references the CamundaCluster to restore into. It can
	// differ from the cluster the backup was taken from. The cluster must
	// stay suspended for the whole restore.
	// +required
	TargetClusterRef ClusterRef `json:"targetClusterRef"`
}

// LogicalRestoreElasticsearchStatus tracks the restore to a terminal phase.
type LogicalRestoreElasticsearchStatus struct {
	// phase of the restore. It is the resume marker: a reconcile that
	// re-enters after a crash continues at the recorded phase.
	// +optional
	Phase LogicalRestorePhase `json:"phase,omitempty"`
	// backupId is the backup that the restore reads, pinned when the restore
	// starts. The backup resource can be deleted afterwards without moving
	// the restore to another set of artifacts.
	// +optional
	BackupID int64 `json:"backupId,omitempty"`
	// repository is the Elasticsearch snapshot repository that the restore
	// reads from, on the Elasticsearch of the target.
	// +optional
	Repository string `json:"repository,omitempty"`
	// restoredSnapshots names every snapshot that the restore asked
	// Elasticsearch to restore. It is also the resume marker of the
	// secondary-storage phase: a look that finds it deletes no index again.
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
// the Camunda indices of the target, restores every snapshot of the backup
// into its Elasticsearch, gives the brokers empty data volumes, and runs the
// Camunda restore application once per broker. The operator only reads
// spec.suspend of the target. Whoever owns the cluster suspends it before the
// restore and unsuspends it after.
type LogicalRestoreElasticsearch struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec names the backup to restore and the cluster to restore into. It is
	// immutable: a restore is one-shot, retried by creating a new resource.
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
