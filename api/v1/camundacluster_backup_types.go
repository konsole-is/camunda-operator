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

// ClusterBackupSpec configures the backups of one orchestration cluster.
// spec.backupStorageRef sets where the backups go. This block is the policy,
// so a preset can hold it for many clusters.
type ClusterBackupSpec struct {
	// PrimaryStorage configures the backups that Zeebe takes of its own
	// primary storage. A point-in-time restore uses them.
	// +optional
	PrimaryStorage *PrimaryStorageBackupSpec `json:"primaryStorage,omitempty"`
	// Dump configures the Job that writes the logical database to the backup
	// bucket. A LogicalBackupRDBMS can replace this block as a whole.
	// +optional
	Dump *BackupDumpSpec `json:"dump,omitempty"`
}

// PrimaryStorageBackupSpec configures the backup scheduler of Zeebe. On a
// relational cluster, Camunda takes these backups itself, without a call
// from the operator. They pair with the database dump. A restore reads the
// exporter position from the restored database and selects the
// primary-storage backups that match it.
type PrimaryStorageBackupSpec struct {
	// Continuous keeps every log segment until it is backed up, so that a
	// restore finds a range without gaps. Defaults to true on a relational
	// cluster with a backupStorageRef. A preset can turn it on for many
	// clusters, and one cluster can still turn it off.
	// +optional
	Continuous *bool `json:"continuous,omitempty"`
	// Schedule is the interval at which Zeebe takes a primary-storage
	// backup: an ISO 8601 duration, a CRON expression, or "none". Defaults
	// to PT1H. Always use a schedule with continuous. Otherwise the log grows
	// without limit.
	// +kubebuilder:validation:MinLength=1
	// +optional
	Schedule string `json:"schedule,omitempty"`
	// CheckpointInterval is the interval at which Zeebe writes marker
	// checkpoints into the log stream, as an ISO 8601 duration of days and
	// time (P2DT3H, PT15M). Camunda parses no week, month, or year units.
	// It is the granularity of a point-in-time restore. Defaults to PT15M.
	// +kubebuilder:validation:Pattern=`^P([0-9]+D(T(([0-9]+H)([0-9]+M)?([0-9]+([.][0-9]+)?S)?|([0-9]+M)([0-9]+([.][0-9]+)?S)?|[0-9]+([.][0-9]+)?S))?|T(([0-9]+H)([0-9]+M)?([0-9]+([.][0-9]+)?S)?|([0-9]+M)([0-9]+([.][0-9]+)?S)?|[0-9]+([.][0-9]+)?S))$`
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
	// ISO 8601 duration of days and time (P7D, PT12H). Camunda parses no
	// week, month, or year units. Defaults to P7D. It limits the restore
	// window, so set it at least as long as the recovery point that the
	// cluster needs. A BackupSchedule can keep database dumps for longer than
	// this window. A restore of such an old dump is not possible.
	// +kubebuilder:validation:Pattern=`^P([0-9]+D(T(([0-9]+H)([0-9]+M)?([0-9]+([.][0-9]+)?S)?|([0-9]+M)([0-9]+([.][0-9]+)?S)?|[0-9]+([.][0-9]+)?S))?|T(([0-9]+H)([0-9]+M)?([0-9]+([.][0-9]+)?S)?|([0-9]+M)([0-9]+([.][0-9]+)?S)?|[0-9]+([.][0-9]+)?S))$`
	// +optional
	Window string `json:"window,omitempty"`
	// CleanupSchedule is the interval at which Zeebe looks for backups
	// outside the window: an ISO 8601 duration, a CRON expression, or
	// "none". Defaults to PT1H.
	// +kubebuilder:validation:MinLength=1
	// +optional
	CleanupSchedule string `json:"cleanupSchedule,omitempty"`
}

// DumpPodSpec configures the pod of the Job that dumps the logical database
// of a relational cluster and uploads it to the backup bucket. It holds all
// the settings that a backup can set for one run. The pod runs the dump and
// then the upload, so one resource block sizes both. It never names the
// image. BackupDumpSpec gives the reason.
type DumpPodSpec struct {
	// Resources are the CPU and memory of the dump pod.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// ExtraEnv are extra environment variables of the dump pod.
	//
	// In the spec.dump of a LogicalBackupRDBMS, every name that starts with
	// PG or UPLOAD_ is reserved, and the API server refuses it. A PG* name is
	// connection policy, because libpq reads PGHOSTADDR, PGSERVICE,
	// PGOPTIONS, and more. UPLOAD_* is the upload contract. The variables of
	// a backup reach only the dump container, never the upload container.
	// Cloud SDKs read endpoint, proxy, and configuration variables from the
	// environment, and the author of a backup must not change where the dump
	// goes.
	//
	// In the spec.backup.dump of the cluster, no name is reserved, and the
	// variables reach every container. The cluster owner sets the connection
	// policy, PGSSLMODE included.
	//
	// Unlike the extraEnv of a workload, this list is atomic. One field
	// manager owns the whole list, because no extension adds entries to a
	// dump pod.
	// +optional
	ExtraEnv []corev1.EnvVar `json:"extraEnv,omitempty"`
	// ExtraEnvFrom are extra environment sources of the dump pod. In the
	// spec.dump of a LogicalBackupRDBMS, every source needs a prefix that
	// cannot make a PG* or UPLOAD_* name. The writer of the referenced object
	// chooses its keys. As with ExtraEnv, the sources of a
	// backup reach only the dump container. The block of the cluster needs no
	// prefix, and its sources reach every container.
	// +optional
	ExtraEnvFrom []corev1.EnvFromSource `json:"extraEnvFrom,omitempty"`
	// PodLabels are extra labels of the dump pod.
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`
	// PodAnnotations are extra annotations of the dump pod. A service mesh
	// sidecar keeps running after the dump finishes, and the Job does not
	// complete. Set the injection annotation of the mesh to false here.
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`
	// Scheduling holds the scheduling constraints of the dump pod. When set,
	// it replaces the block of a preset, with no merge.
	// +optional
	Scheduling *SchedulingSpec `json:"scheduling,omitempty"`
	// ScratchVolume is where the pod writes the dump before the upload. When
	// set, it replaces the block of a preset, with no merge. The dump pod
	// runs with fsGroup 999, the postgres group. Thus pg_dump can write to a
	// volume that a storage class gives with the root owner.
	// +optional
	ScratchVolume *ScratchVolumeSpec `json:"scratchVolume,omitempty"`
	// ActiveDeadlineSeconds is the number of seconds that the dump Job can
	// run before it fails, counted from its start. When the dump block that
	// applies does not set it, the Job gets 86400 (24 hours). In the block of
	// the cluster, an unset value takes the value of the preset. The
	// spec.dump of a LogicalBackupRDBMS replaces the block of the cluster, so
	// an unset value there gives 86400, not the value of the cluster.
	//
	// The default is large, so that a very large dump can complete. The Job
	// always has a deadline. A pod that cannot start uses no retry, so
	// without a deadline a broken Job stays active for the life of the
	// backup. A lower value fails a stuck dump sooner. A higher value gives a
	// long dump the time that it needs.
	// +kubebuilder:validation:Minimum=1
	// +optional
	ActiveDeadlineSeconds *int64 `json:"activeDeadlineSeconds,omitempty"`
}

// BackupDumpSpec is the dump configuration of the cluster. It holds the pod
// settings that a backup can also set for one run, and the image that runs
// the dump. Only the cluster sets the image. The Job runs under the
// ServiceAccount of the cluster, with the cloud identity and the database
// credentials of the cluster. Thus the cluster owner chooses the program
// that the Job runs, never the person who creates a backup. A
// LogicalBackupRDBMS replaces all the pod settings and always uses the image
// of the cluster.
type BackupDumpSpec struct {
	DumpPodSpec `json:",inline"`
	// PostgresImage is the full image reference of the dump container. It
	// replaces the default postgres:<major> of the upstream registry. Set it
	// in an air-gapped installation, where the default image is not
	// available.
	// +optional
	PostgresImage string `json:"postgresImage,omitempty"`
}

// ScratchVolumeSpec sizes the volume that holds a database dump until the
// upload. An unset block is an emptyDir with the limits of the node. A large
// database can fill it.
// +kubebuilder:validation:XValidation:rule="!has(self.storageClassName) || has(self.sizeLimit)",message="a scratch volume with a storage class needs a sizeLimit to request"
type ScratchVolumeSpec struct {
	// SizeLimit is the size of the emptyDir that holds the dump. Set it to
	// the size the dump needs, with room to spare.
	// +optional
	SizeLimit *resource.Quantity `json:"sizeLimit,omitempty"`
	// StorageClassName makes the scratch volume a PersistentVolumeClaim of
	// this class, not an emptyDir. Use it when the dump is larger than the
	// ephemeral storage of a node.
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
	// operator publishes none. The value basic is for a user who adds a
	// Spring Security configuration to the port, and for a later Camunda
	// version that secures it.
	Method ManagementAuthMethod `json:"method"`
	// CredentialsSecretRef names the username and password of the management
	// port. It is set only when Method is basic.
	// +optional
	CredentialsSecretRef *LocalCredentialsSecretRef `json:"credentialsSecretRef,omitempty"`
}

// ManagementBinding is the published address of the management API of one
// cluster. Extensions that operate the cluster, such as the backup kinds,
// read it. They do not derive the Service name, the port, or the
// authentication themselves. It is empty while the cluster is suspended,
// because the management API is then not available.
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
	// backups to. It is set only on a cluster with Elasticsearch storage and a
	// backupStorageRef. It comes from the SecondaryStorageConfig.
	// +optional
	BackupRepository string `json:"backupRepository,omitempty"`
}
