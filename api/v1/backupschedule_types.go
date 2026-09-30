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

// BackupScheduleSpec is the backup policy of one cluster: when the schedule
// creates a backup, and how many backups it keeps.
type BackupScheduleSpec struct {
	// ClusterRef references the CamundaCluster to back up. At each trigger,
	// the operator creates the backup kind for the storage type of the
	// cluster: LogicalBackupElasticsearch or LogicalBackupRDBMS.
	// +required
	ClusterRef ClusterRef `json:"clusterRef"`
	// Schedule is when the backups run: a five-field cron expression
	// (minute, hour, day of month, month, day of week), evaluated in UTC.
	// +kubebuilder:validation:Pattern=`^\s*([0-9A-Za-z*?,/-]+\s+){4}[0-9A-Za-z*?,/-]+\s*$`
	// +required
	Schedule string `json:"schedule"`
	// Retained limits how many backups of this schedule stay. The schedule
	// counts and deletes the backups in its namespace whose
	// camunda.io/backup-schedule label names this schedule. A backup that you
	// create with that label counts too, and the schedule can delete it. The
	// schedule never deletes a backup that is not in a final phase.
	// +kubebuilder:default={}
	// +optional
	Retained *RetainedBackups `json:"retained,omitempty"`
}

// RetainedBackups limits the backups that a schedule keeps, for each final
// phase. When the count of a phase is more than its limit, the operator
// deletes the oldest backups of that phase, down to the limit. The deletion
// also removes their stored backup data.
type RetainedBackups struct {
	// Completed is how many completed backups the schedule keeps.
	// +kubebuilder:default=7
	// +kubebuilder:validation:Minimum=1
	// +optional
	Completed *int32 `json:"completed,omitempty"`
	// Failed is how many failed backups the schedule keeps. With zero, the
	// operator deletes a failed backup soon after it fails.
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=0
	// +optional
	Failed *int32 `json:"failed,omitempty"`
}

// BackupScheduleStatus is the observed state of the schedule.
type BackupScheduleStatus struct {
	// LastScheduleTime is the last trigger that the schedule used, also when
	// it skipped the trigger. The schedule never tries a skipped trigger
	// again.
	// +optional
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`
	// LastBackupName is the backup that the schedule created most recently.
	// +optional
	LastBackupName string `json:"lastBackupName,omitempty"`
	// ObservedGeneration is the last generation that the operator processed.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the current state. The Ready condition has the
	// reason Healthy while the schedule can run its backups, and
	// InvalidReference while a reference does not resolve.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Schedule",type=string,JSONPath=`.spec.schedule`
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=`.spec.clusterRef.name`,priority=1
// +kubebuilder:printcolumn:name="Last schedule",type=date,JSONPath=`.status.lastScheduleTime`,priority=1
// +kubebuilder:printcolumn:name="Last backup",type=string,JSONPath=`.status.lastBackupName`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// BackupSchedule creates logical backups of one CamundaCluster on a cron
// schedule, and deletes old backups that have its label. At each trigger, the
// operator creates the backup kind for the storage type of the cluster. The
// backup has the name <schedule>-<unix-timestamp> and the labels
// camunda.io/cluster and camunda.io/backup-schedule. If a name is too long
// for a resource name or a label value, the operator shortens it. Then it
// adds a hash of the full name, so two long names stay different.
//
// The backups have no owner reference to the schedule, so a deletion of the
// schedule never deletes its backups. The schedule skips a trigger while the
// cluster is suspended, or while a backup of this schedule is not in a final
// phase. It records an event for each skipped trigger.
type BackupSchedule struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of BackupSchedule
	// +required
	Spec BackupScheduleSpec `json:"spec"`

	// status defines the observed state of BackupSchedule
	// +optional
	Status BackupScheduleStatus `json:"status,omitzero"`
}

// GetStatusConditions returns a pointer to the status conditions. The
// component framework stages conditions on the resource through it.
func (in *BackupSchedule) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// GetKind returns the CRD kind. The component framework uses it for event and
// metric recording.
func (in *BackupSchedule) GetKind() string { return "BackupSchedule" }

// SetObservedGeneration records the last reconciled generation in status.
func (in *BackupSchedule) SetObservedGeneration(generation int64) {
	in.Status.ObservedGeneration = generation
}

// +kubebuilder:object:root=true

// BackupScheduleList contains a list of BackupSchedule
type BackupScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []BackupSchedule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BackupSchedule{}, &BackupScheduleList{})
}
