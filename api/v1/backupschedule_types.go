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

// BackupScheduleSpec is the backup policy of one cluster: when a backup is
// created and how many the schedule keeps.
type BackupScheduleSpec struct {
	// ClusterRef references the CamundaCluster to back up. At each trigger
	// the operator creates the backup kind that matches the storage type of
	// the cluster: LogicalBackupElasticsearch or LogicalBackupRDBMS.
	// +required
	ClusterRef ClusterRef `json:"clusterRef"`
	// Schedule is when the backups run: a five-field cron expression
	// (minute, hour, day of month, month, day of week), evaluated in UTC.
	// +kubebuilder:validation:Pattern=`^\s*([0-9A-Za-z*?,/-]+\s+){4}[0-9A-Za-z*?,/-]+\s*$`
	// +required
	Schedule string `json:"schedule"`
	// Retained bounds how many backups of this schedule are kept. The
	// schedule prunes only the backups it created, matched by the
	// camunda.io/backup-schedule label. It never touches a backup that a
	// human created, and it never touches a backup that has not reached a
	// terminal phase.
	// +kubebuilder:default={}
	// +optional
	Retained *RetainedBackups `json:"retained,omitempty"`
}

// RetainedBackups bounds the backups that a schedule keeps, by terminal
// phase. When the count of a phase exceeds its bound, the oldest backups
// beyond it are deleted through the backup finalizer, which removes the
// stored artifacts too.
type RetainedBackups struct {
	// Completed is how many completed backups the schedule keeps.
	// +kubebuilder:default=7
	// +kubebuilder:validation:Minimum=1
	// +optional
	Completed *int32 `json:"completed,omitempty"`
	// Failed is how many failed backups the schedule keeps. Zero deletes a
	// failed backup at the first look after it fails.
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=0
	// +optional
	Failed *int32 `json:"failed,omitempty"`
}

// BackupScheduleStatus is the observed state of the schedule.
type BackupScheduleStatus struct {
	// LastScheduleTime is the most recent trigger that the schedule
	// consumed, whether it created a backup or skipped. A skipped trigger is
	// never retried.
	// +optional
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`
	// LastBackupName is the backup that the schedule created most recently.
	// +optional
	LastBackupName string `json:"lastBackupName,omitempty"`
	// ObservedGeneration is the last generation reconciled by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the current state. The Ready condition is Healthy
	// while the schedule can run its backups, and InvalidReference while a
	// reference does not resolve.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=`.spec.clusterRef.name`
// +kubebuilder:printcolumn:name="Schedule",type=string,JSONPath=`.spec.schedule`
// +kubebuilder:printcolumn:name="Last schedule",type=date,JSONPath=`.status.lastScheduleTime`
// +kubebuilder:printcolumn:name="Last backup",type=string,JSONPath=`.status.lastBackupName`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// BackupSchedule creates logical backups of one CamundaCluster on a cron
// schedule and prunes the backups it created. At each trigger the operator
// creates the backup kind that matches the storage type of the cluster,
// named <schedule>-<unix-timestamp> and labeled camunda.io/cluster and
// camunda.io/backup-schedule. The backups carry no owner reference to the
// schedule, so deleting a schedule never deletes its backups. A trigger is
// skipped, with an event, while the cluster is suspended or while a backup
// of this schedule has not reached a terminal phase.
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
