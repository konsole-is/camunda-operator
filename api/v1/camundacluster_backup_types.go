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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// ClusterBackupSpec configures how backups of one orchestration cluster
// behave. Where they are written is spec.backupStorageRef; this block is the
// policy, so a preset can hold it for a fleet of clusters.
type ClusterBackupSpec struct {
	// PrimaryStorage configures the backups that Zeebe takes of its own
	// primary storage. They are the feed of a point-in-time restore.
	// +optional
	PrimaryStorage *PrimaryStorageBackupSpec `json:"primaryStorage,omitempty"`
	// Dump configures the Job that writes the logical database to the backup
	// bucket. A LogicalBackupRDBMS can replace this block as a whole.
	// +optional
	Dump *BackupDumpSpec `json:"dump,omitempty"`
}

// PrimaryStorageBackupSpec configures the backup scheduler of Zeebe. Camunda
// takes these backups without an API call from this operator, on a relational
// cluster. They pair with the database dump: a restore reads the exporter
// position from the restored database and picks the primary-storage backups
// that match it.
type PrimaryStorageBackupSpec struct {
	// Continuous keeps every log segment until it is backed up, so that a
	// restore finds an unbroken range. Defaults to true on a relational
	// cluster with a backupStorageRef. It is a pointer, so a preset can turn
	// it on for a fleet and one cluster can still turn it off.
	// +optional
	Continuous *bool `json:"continuous,omitempty"`
	// Schedule is the interval at which Zeebe takes a primary-storage
	// backup: an ISO 8601 duration, a CRON expression, or "none". Defaults
	// to PT1H. Always pair a schedule with continuous, or the log grows
	// without bound.
	// +kubebuilder:validation:MinLength=1
	// +optional
	Schedule string `json:"schedule,omitempty"`
	// CheckpointInterval is the interval at which Zeebe writes marker
	// checkpoints into the log stream, as an ISO 8601 duration. It is the
	// granularity of a point-in-time restore. Defaults to PT15M.
	// +kubebuilder:validation:Pattern=`^P([0-9]+[YMWD])*(T([0-9]+(\.[0-9]+)?[HMS])+)?$`
	// +kubebuilder:validation:MinLength=2
	// +optional
	CheckpointInterval string `json:"checkpointInterval,omitempty"`
	// Retention bounds how long Zeebe keeps its primary-storage backups.
	// +optional
	Retention *PrimaryStorageRetentionSpec `json:"retention,omitempty"`
}

// PrimaryStorageRetentionSpec bounds the primary-storage backups that Zeebe
// keeps. Zeebe always keeps at least one backup, even outside the window.
type PrimaryStorageRetentionSpec struct {
	// Window is how far back the backups stay available for a restore, as an
	// ISO 8601 duration. Defaults to P7D. It bounds the restore window, so
	// set it at least as long as the recovery point the cluster needs. A
	// BackupSchedule that keeps database dumps for longer than this window
	// keeps dumps that can no longer be restored.
	// +kubebuilder:validation:Pattern=`^P([0-9]+[YMWD])*(T([0-9]+(\.[0-9]+)?[HMS])+)?$`
	// +kubebuilder:validation:MinLength=2
	// +optional
	Window string `json:"window,omitempty"`
	// CleanupSchedule is the interval at which Zeebe looks for backups
	// outside the window: an ISO 8601 duration, a CRON expression, or
	// "none". Defaults to PT1H.
	// +kubebuilder:validation:MinLength=1
	// +optional
	CleanupSchedule string `json:"cleanupSchedule,omitempty"`
}

// BackupDumpSpec configures the pod of the Job that dumps the logical
// database of a relational cluster and uploads it to the backup bucket. The
// pod runs the dump and the upload in turn, so one resource block sizes both.
type BackupDumpSpec struct {
	// Resources are the CPU and memory of the dump pod.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// ExtraEnv are extra environment variables of the dump pod.
	// +optional
	ExtraEnv []corev1.EnvVar `json:"extraEnv,omitempty"`
	// ExtraEnvFrom are extra environment sources of the dump pod.
	// +optional
	ExtraEnvFrom []corev1.EnvFromSource `json:"extraEnvFrom,omitempty"`
	// PodLabels are extra labels of the dump pod.
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`
	// PodAnnotations are extra annotations of the dump pod. Set the
	// injection annotation of a service mesh to false here: a sidecar that
	// keeps running after the dump finishes stops the Job from completing.
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`
	// Scheduling constraints of the dump pod. When set, it replaces the
	// block of a preset entirely (no merge).
	// +optional
	Scheduling *SchedulingSpec `json:"scheduling,omitempty"`
	// ScratchVolume is where the dump is written before it is uploaded. When
	// set, it replaces the block of a preset entirely (no merge).
	// +optional
	ScratchVolume *ScratchVolumeSpec `json:"scratchVolume,omitempty"`
}

// ScratchVolumeSpec sizes the volume that holds a database dump until it is
// uploaded. An unset block is an emptyDir that the node bounds, which a large
// database can exhaust.
type ScratchVolumeSpec struct {
	// SizeLimit is the size of the emptyDir that holds the dump. Set it to
	// the size the dump needs, with room to spare.
	// +optional
	SizeLimit *resource.Quantity `json:"sizeLimit,omitempty"`
	// StorageClassName makes the scratch volume a PersistentVolumeClaim of
	// this class instead of an emptyDir. Use it when the dump is larger than
	// the ephemeral storage of a node.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
}

// ManagementAuthMethod is how a consumer authenticates against the management
// API of a cluster.
// +kubebuilder:validation:Enum=none;basic
type ManagementAuthMethod string

// ManagementAuthMethodNone and ManagementAuthMethodBasic are the supported
// methods of the management API.
const (
	ManagementAuthMethodNone  ManagementAuthMethod = "none"
	ManagementAuthMethodBasic ManagementAuthMethod = "basic"
)

// ManagementAuth is how a consumer of the management binding authenticates.
type ManagementAuth struct {
	// Method is the authentication method of the management port. Camunda
	// 8.9 serves the actuator endpoints without authentication, so the
	// operator publishes none. The field carries basic for a user who puts
	// their own Spring Security configuration in front of the port, and for
	// a later Camunda version that secures it.
	Method ManagementAuthMethod `json:"method"`
	// CredentialsSecretRef names the username and password of the management
	// port. It is set only when Method is basic.
	// +optional
	CredentialsSecretRef *CredentialsSecretRef `json:"credentialsSecretRef,omitempty"`
}

// ManagementBinding is the published address of the management API of one
// cluster. Extensions that drive the cluster — the backup kinds first — read
// it and never rebuild the Service name, the port, or the authentication from
// the internals of the CamundaCluster controller. It is empty while the
// cluster is suspended, because the management API is then unreachable.
type ManagementBinding struct {
	// Endpoint is the base URL of the management API, for example
	// http://my-cluster-zeebe.my-namespace.svc:9600.
	Endpoint string `json:"endpoint"`
	// Auth is how a consumer authenticates against the endpoint.
	Auth ManagementAuth `json:"auth"`
	// Version is the Camunda version of the cluster, for example 8.9.9. A
	// consumer selects its endpoint set from the minor version.
	Version string `json:"version"`
	// Partitions is the partition count of the cluster. A restore must match
	// it, so a backup records it.
	Partitions int32 `json:"partitions"`
	// BackupRepository is the snapshot repository that the components write
	// backups to. It is set only on an Elasticsearch-backed cluster with a
	// backupStorageRef, and comes from the SecondaryStorageConfig.
	// +optional
	BackupRepository string `json:"backupRepository,omitempty"`
}
