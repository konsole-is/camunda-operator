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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DatabaseServerServiceAccountSpec configures the ServiceAccount that
// CloudNativePG creates for the instance pods. CloudNativePG owns that
// account and gives it the name of the server, so you can set only its
// metadata. The operator adds the workload-identity annotations of the
// archive bucket itself. An annotation set here wins over the operator
// annotation with the same key.
type DatabaseServerServiceAccountSpec struct {
	// Annotations to set on the ServiceAccount. Usually these are
	// workload-identity annotations (IRSA, GCP Workload Identity, and more)
	// that give the instance pods access to cloud resources.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// PodMonitorSpec configures the Prometheus PodMonitor of a resource whose
// pods serve their own metrics endpoint. A DatabaseServer scrapes the
// metrics port of every CloudNativePG instance. The PodMonitor is created
// only when the Kubernetes cluster serves the kind.
type PodMonitorSpec struct {
	// Enabled creates the PodMonitor when true. Defaults to false.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// Labels are extra labels applied to the PodMonitor.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations are extra annotations applied to the PodMonitor.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// Interval is how often Prometheus scrapes the pods, as a Prometheus
	// duration such as 30s. Empty leaves the interval to the Prometheus
	// configuration.
	// +kubebuilder:validation:Pattern=`^(0|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$`
	// +optional
	Interval string `json:"interval,omitempty"`
}

// DatabaseServerMonitoringSpec groups the Prometheus scraping integration of
// a database server.
type DatabaseServerMonitoringSpec struct {
	// PodMonitor configures the Prometheus PodMonitor over the instance pods.
	// +optional
	PodMonitor *PodMonitorSpec `json:"podMonitor,omitempty"`
}

// DatabaseServerArchiveSpec points a server at the bucket that holds its
// continuous archive: the write-ahead log of every instance, and the base
// backups that a recovery starts from. A server with an archive publishes
// pitr.enabled true on its contract. A server without one publishes false,
// and no point-in-time restore can reach it.
//
// The archive is not the backup model of the operator. BackupSchedule and
// LogicalBackupRDBMS take logical dumps and never use these base backups.
type DatabaseServerArchiveSpec struct {
	// ObjectStorageRef names an ObjectStorageConfig in the namespace of this
	// server. The operator writes the archive under a prefix of that bucket
	// that only this server uses.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=253
	ObjectStorageRef string `json:"objectStorageRef"`
	// RetentionPeriodDays is how far into the past a restore can reach. The
	// operator enforces this value on the bucket, and the contract of this
	// server publishes the same value.
	//
	// The maximum is 36500 days, which is a hundred years. The operator counts
	// the reachable window in nanoseconds. A longer period overflows that
	// count, so no restore request can reach a point.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=36500
	RetentionPeriodDays int32 `json:"retentionPeriodDays"`
	// BaseBackupSchedule is when the server takes a base backup. It is the
	// six-field cron of CloudNativePG (seconds first, in UTC), or one of the
	// descriptors @yearly, @annually, @monthly, @weekly, @daily, @midnight,
	// @hourly and @every. The first base backup runs as soon as the server is
	// up, whatever the schedule says. A recovery from the archive is possible
	// only after that backup completes.
	//
	// The API server refuses the five-field cron of a Kubernetes CronJob.
	// CloudNativePG reads the first field as seconds, so a five-field value
	// runs at a different time than intended.
	//
	// Each field has the bounds that CloudNativePG accepts: 0-59 for seconds
	// and minutes, 0-23 for hours, 1-31 for the day of the month, 1-12 or
	// JAN-DEC for the month, and 0-6 or SUN-SAT for the day of the week. The
	// schema cannot find a range whose first value is above its second, such
	// as FRI-MON. The operator refuses such a range with Ready reason
	// InvalidReference, before it applies anything.
	//
	// A step has at most three digits. An @every number has at most six
	// digits on each side of the point. These limits are stricter than the
	// parser of CloudNativePG. That parser accepts a longer number, then
	// overflows and stops taking base backups.
	// +kubebuilder:validation:Pattern=`^(\s*([*?]|[0-5]?\d(-[0-5]?\d)?)(/[1-9]\d{0,2})?(,([*?]|[0-5]?\d(-[0-5]?\d)?)(/[1-9]\d{0,2})?)*\s+([*?]|[0-5]?\d(-[0-5]?\d)?)(/[1-9]\d{0,2})?(,([*?]|[0-5]?\d(-[0-5]?\d)?)(/[1-9]\d{0,2})?)*\s+([*?]|([01]?\d|2[0-3])(-([01]?\d|2[0-3]))?)(/[1-9]\d{0,2})?(,([*?]|([01]?\d|2[0-3])(-([01]?\d|2[0-3]))?)(/[1-9]\d{0,2})?)*\s+([*?]|([1-9]|[12]\d|3[01])(-([1-9]|[12]\d|3[01]))?)(/[1-9]\d{0,2})?(,([*?]|([1-9]|[12]\d|3[01])(-([1-9]|[12]\d|3[01]))?)(/[1-9]\d{0,2})?)*\s+([*?]|([1-9]|1[0-2]|[Jj]([Aa][Nn]|[Uu][LlNn])|[Ff][Ee][Bb]|[Mm][Aa][RrYy]|[Aa]([Pp][Rr]|[Uu][Gg])|[Ss][Ee][Pp]|[Oo][Cc][Tt]|[Nn][Oo][Vv]|[Dd][Ee][Cc])(-([1-9]|1[0-2]|[Jj]([Aa][Nn]|[Uu][LlNn])|[Ff][Ee][Bb]|[Mm][Aa][RrYy]|[Aa]([Pp][Rr]|[Uu][Gg])|[Ss][Ee][Pp]|[Oo][Cc][Tt]|[Nn][Oo][Vv]|[Dd][Ee][Cc]))?)(/[1-9]\d{0,2})?(,([*?]|([1-9]|1[0-2]|[Jj]([Aa][Nn]|[Uu][LlNn])|[Ff][Ee][Bb]|[Mm][Aa][RrYy]|[Aa]([Pp][Rr]|[Uu][Gg])|[Ss][Ee][Pp]|[Oo][Cc][Tt]|[Nn][Oo][Vv]|[Dd][Ee][Cc])(-([1-9]|1[0-2]|[Jj]([Aa][Nn]|[Uu][LlNn])|[Ff][Ee][Bb]|[Mm][Aa][RrYy]|[Aa]([Pp][Rr]|[Uu][Gg])|[Ss][Ee][Pp]|[Oo][Cc][Tt]|[Nn][Oo][Vv]|[Dd][Ee][Cc]))?)(/[1-9]\d{0,2})?)*\s+([*?]|([0-6]|[Ss]([Uu][Nn]|[Aa][Tt])|[Mm][Oo][Nn]|[Tt]([Uu][Ee]|[Hh][Uu])|[Ww][Ee][Dd]|[Ff][Rr][Ii])(-([0-6]|[Ss]([Uu][Nn]|[Aa][Tt])|[Mm][Oo][Nn]|[Tt]([Uu][Ee]|[Hh][Uu])|[Ww][Ee][Dd]|[Ff][Rr][Ii]))?)(/[1-9]\d{0,2})?(,([*?]|([0-6]|[Ss]([Uu][Nn]|[Aa][Tt])|[Mm][Oo][Nn]|[Tt]([Uu][Ee]|[Hh][Uu])|[Ww][Ee][Dd]|[Ff][Rr][Ii])(-([0-6]|[Ss]([Uu][Nn]|[Aa][Tt])|[Mm][Oo][Nn]|[Tt]([Uu][Ee]|[Hh][Uu])|[Ww][Ee][Dd]|[Ff][Rr][Ii]))?)(/[1-9]\d{0,2})?)*\s*|@((year|annual|month|week|dai|hour)ly|midnight|every (\d{1,6}(\.\d{1,6})?[hms])+))$`
	// +kubebuilder:default="0 0 2 * * *"
	// +optional
	BaseBackupSchedule string `json:"baseBackupSchedule,omitempty"`
}

// DatabaseServerSpec defines the desired state of DatabaseServer.
//
// A DatabaseServerPreset uses the same type as its baseline. For this reason,
// the schema marks databaseServerConfig as optional, and the DatabaseServer
// requires it. A preset refuses the fields that belong to one server only.
// A preset also refuses the version, which belongs to a CamundaRelease.
type DatabaseServerSpec struct {
	// PresetRef names a cluster-scoped DatabaseServerPreset to use as the
	// configuration baseline. A field set on the server replaces the value of
	// the preset for that field completely.
	// +optional
	PresetRef string `json:"presetRef,omitempty"`
	// ReleaseRef names a cluster-scoped CamundaRelease that provides the
	// PostgreSQL major version. It merges over the preset and under this spec.
	// Forbidden in a preset.
	// +optional
	ReleaseRef string `json:"releaseRef,omitempty"`
	// PlatformConfigRef names a cluster-scoped CamundaPlatformConfig. The
	// server reads only its image settings. spec.images.postgres sets where
	// the server pulls the PostgreSQL image from, for example in an air-gapped
	// cluster. When empty, the server uses the default image repository.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=253
	// +optional
	PlatformConfigRef string `json:"platformConfigRef,omitempty"`
	// Version is the PostgreSQL major version to run, as a number such as
	// "17". It selects the image tag. Camunda 8.9 supports PostgreSQL 14 and
	// later, and the operator refuses a lower version. Required unless the
	// resolved release provides it. Forbidden in a preset.
	//
	// The major version of a running server cannot change. The operator
	// refuses another major version, higher or lower, with Ready reason
	// VersionChangeRefused. The server then keeps the major version it has.
	// The same applies to a new major version from a release. To run another
	// major version, create a new server and move the data to it.
	// +kubebuilder:validation:Pattern=`^\d+$`
	// +optional
	Version string `json:"version,omitempty"`
	// Instances is the number of PostgreSQL instances. Defaults to 1. One
	// instance has no failover. If its node goes away, the server is down
	// until the volume is attached again.
	// +kubebuilder:validation:Minimum=1
	// +optional
	Instances *int32 `json:"instances,omitempty"`
	// Resources are the CPU and memory of each instance.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// StorageSize is the size of the data volume of each instance. Required
	// unless the resolved preset provides it. It cannot shrink, because a
	// PostgreSQL data volume cannot become smaller in place. The API server
	// refuses a smaller value on a DatabaseServer. A preset can set a smaller
	// value. A server that has a larger size then keeps it and records a
	// StorageShrinkIgnored event.
	// +optional
	StorageSize *resource.Quantity `json:"storageSize,omitempty"`
	// StorageClassName is the StorageClass of the data volumes. Defaults to
	// the default StorageClass of the Kubernetes cluster.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
	// WALStorageSize puts the write-ahead log on a separate volume of this
	// size. When unset, the log stays on the data volume. It cannot shrink,
	// as storageSize cannot. The server ignores a smaller value from a preset
	// in the same way.
	//
	// You can add it to a running server, but you cannot remove it again.
	// CloudNativePG refuses a cluster that removes the volume. If the field
	// is cleared, on the server or in a preset, the volume keeps its size and
	// the server records a WALStorageKept event.
	// +optional
	WALStorageSize *resource.Quantity `json:"walStorageSize,omitempty"`
	// ServiceAccount configures the ServiceAccount of the instance pods.
	// +optional
	ServiceAccount *DatabaseServerServiceAccountSpec `json:"serviceAccount,omitempty"`
	// Scheduling holds the scheduling constraints of the instance pods. When
	// set, it replaces the scheduling block of the preset, with no merge.
	// +optional
	Scheduling *SchedulingSpec `json:"scheduling,omitempty"`
	// PodLabels are extra labels applied to the instance pods.
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`
	// PodAnnotations are extra annotations applied to the instance pods.
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`
	// Monitoring configures the Prometheus scraping integration.
	// +optional
	Monitoring *DatabaseServerMonitoringSpec `json:"monitoring,omitempty"`
	// DatabaseServerConfig names the DatabaseServerConfig that the operator
	// publishes in the namespace of this server. It holds the endpoint, the
	// admin credentials, and the point-in-time recovery capability of the
	// server. Required on a DatabaseServer, forbidden in a preset.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=253
	// +optional
	DatabaseServerConfig string `json:"databaseServerConfig,omitempty"`
	// Archive points the server at the bucket that holds its continuous
	// archive. Without it the server takes no part in point-in-time restore.
	// +optional
	Archive *DatabaseServerArchiveSpec `json:"archive,omitempty"`
	// Suspend stops the PostgreSQL instances and keeps their data volumes.
	// Defaults to false. The operator hibernates the CloudNativePG cluster:
	// it removes the instance pods and keeps the volumes. When you set the
	// field back to false, the instances start again on the same volumes.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// The per-component conditions of a DatabaseServer. Ready follows
// ClusterReady, ContractReady, and, on a server with an archive,
// ArchiveReady. MonitoringReady never affects Ready. ArchiveReady on a server
// with no archive does not affect Ready either.
const (
	// ConditionClusterReady reports whether the CloudNativePG cluster that the
	// published contract points at is healthy.
	ConditionClusterReady = "ClusterReady"
	// ConditionArchiveReady reports whether a recovery from the current
	// archive of the server is possible. This is true when the first base
	// backup completed and the write-ahead log of the server still reaches
	// the bucket. It reads Disabled on a server with no spec.archive.
	ConditionArchiveReady = "ArchiveReady"
	// ConditionContractReady reports whether the DatabaseServerConfig of the
	// server is published and the superuser Secret behind it exists.
	ConditionContractReady = "ContractReady"
	// ConditionMonitoringReady reports whether the PodMonitor over the
	// instance pods is applied. It reads Disabled when scraping is off.
	ConditionMonitoringReady = "MonitoringReady"
)

// ReasonCNPGNotInstalled is the Ready reason of a DatabaseServer on a cluster
// that did not serve the CloudNativePG Cluster kind when the operator
// started. Restart the operator after you install CloudNativePG.
const ReasonCNPGNotInstalled = "CNPGNotInstalled"

// ReasonBarmanPluginNotInstalled is the Ready reason of a DatabaseServer that
// asks for an archive on a cluster that did not serve the ObjectStore kind of
// the Barman Cloud plugin when the operator started. A server without an
// archive block is unaffected.
const ReasonBarmanPluginNotInstalled = "BarmanPluginNotInstalled"

// ReasonContractTaken is the ContractReady reason of a DatabaseServer whose
// merged spec.databaseServerConfig names a DatabaseServerConfig that already
// exists and that this server did not publish. The server does not write to
// it, so its consumers keep the endpoint and the credentials that they read.
// The server also refuses a contract with no owner. Such a contract is the
// bring-your-own-server API, for a server that the operator does not run.
// The message names the owner, or says that no owner controls it. Give this
// server a name of its own, or remove that contract.
const ReasonContractTaken = "ContractTaken"

// ReasonClusterTaken is the ClusterReady reason of a DatabaseServer that
// derives the name of an existing CloudNativePG cluster that it does not
// control. The server does not write to that cluster and runs nothing
// itself, so the database of the other owner stays unchanged. The server
// publishes no contract and takes no base backup of that cluster. The
// archive that it wrote before and its history stay. The message names the
// holder, or says that no owner controls it. Remove that cluster, or give
// this server a name of its own.
const ReasonClusterTaken = "ClusterTaken"

// ReasonArchiveTaken is the ArchiveReady reason of a DatabaseServer that
// derives the name of a Barman Cloud ObjectStore that another owner controls.
// The cluster archives its write-ahead log through the ObjectStore of that
// name, so the server removes the archive from the cluster. The server then
// archives no write-ahead log and takes no base backup. It publishes no
// point-in-time recovery capability on its contract, and it refuses a
// rollback. The archive that it wrote before and its history stay. The
// message names the holder. Remove that ObjectStore, or give this server a
// name of its own.
const ReasonArchiveTaken = "ArchiveTaken"

// ReasonArchiveFailing is the ArchiveReady reason of a DatabaseServer whose
// write-ahead log stopped reaching the bucket. CloudNativePG reports the
// failed uploads on its cluster. The server waits a grace period before it
// reports them, so a segment that the plugin retries does not change the
// condition. The archive holds every point up to the last segment that
// arrived. The open record of status.archive.history sets unverifiedFrom to
// that point. The message gives the CloudNativePG report. Repair the bucket
// or the credentials of the bucket.
const ReasonArchiveFailing = "ArchiveFailing"

// ReasonVersionChangeRefused is the Ready reason of a DatabaseServer whose
// merged version names a PostgreSQL major version other than the one of its
// data directory. The server keeps its current major version. A rollback in
// progress finishes, and the operator keeps the contract and the archive up
// to date. A change of the major version needs a new server. No annotation
// permits it.
const ReasonVersionChangeRefused = "VersionChangeRefused"

// ArchiveRecord is one continuous archive that a server wrote. A recovery
// replays it from a base backup up to the requested point. A restore can
// reach only a point inside the interval of a record.
type ArchiveRecord struct {
	// ServerName is the archive directory in the bucket, equal to the name of
	// the CloudNativePG cluster that wrote it.
	ServerName string `json:"serverName"`
	// ObjectStorageRef is the ObjectStorageConfig, in the namespace of this
	// server, that holds this archive. When the server moves to another
	// bucket, it closes this record and opens a new one. Thus every interval
	// names the bucket that a restore of that interval reads.
	// +optional
	ObjectStorageRef string `json:"objectStorageRef,omitempty"`
	// Location is where in object storage the server wrote this archive: the
	// provider, the bucket, and the path, as one URL. Two intervals hold the
	// same archive only when their locations are equal. The name of an
	// ObjectStorageConfig is not sufficient, because you can edit the object
	// or create it again with the same name. A record without a location gets
	// the current location of the server.
	// +optional
	Location string `json:"location,omitempty"`
	// From is the earliest point that a recovery from this archive can reach:
	// the time when its first base backup completed.
	From metav1.Time `json:"from"`
	// To is the latest point that a recovery from this archive can reach. It
	// is unset while the server still writes to this archive.
	// +optional
	To *metav1.Time `json:"to,omitempty"`
	// UnverifiedFrom is the point after which this archive can miss parts of
	// the write-ahead log. It is set while CloudNativePG reports that the
	// uploads of the server fail. A restore to a point after it fails. When
	// the uploads work again, the plugin uploads the segments that it held
	// back. The field is then cleared, and a restore can reach the whole
	// interval again. Only the record that the server writes to now has it.
	// +optional
	UnverifiedFrom *metav1.Time `json:"unverifiedFrom,omitempty"`
}

// ArchiveBoundary is the moment when a server moved its archive to another
// location while no interval was open. Only a base backup that began after
// this moment belongs to the current archive. A base backup that began
// before it wrote to the old location.
type ArchiveBoundary struct {
	// At is when the server moved.
	At metav1.Time `json:"at"`
	// Location is the new location, in the form of ArchiveRecord.Location.
	// The boundary ends when the archive of that location opens a record. It
	// moves when the location moves again.
	Location string `json:"location"`
	// ObjectStorageRef is the ObjectStorageConfig of that location. It is
	// information for the reader only.
	// +optional
	ObjectStorageRef string `json:"objectStorageRef,omitempty"`
}

// DatabaseServerArchiveStatus is the observed state of the archive of a
// server.
type DatabaseServerArchiveStatus struct {
	// History lists every archive that the server wrote, oldest first. A
	// recovery uses the record whose interval holds the requested point. The
	// operator never removes a record. Thus a restore can reach back across
	// an earlier recovery for as long as the bucket keeps the objects. When
	// you remove spec.archive, the open record closes and the list stays.
	// When you add an archive again, a new record opens. A move of
	// spec.archive to another bucket does the same. No restore can reach a
	// point in a time without an archive.
	// +optional
	History []ArchiveRecord `json:"history,omitempty"`
	// Boundary is the last move of the archive that no record holds yet. The
	// move happens when spec.archive is enabled again on another location,
	// or when the location moves again before a base backup opens a record.
	// It prevents a base backup of the old location from opening the
	// interval of the new location. It is cleared when that interval opens.
	// +optional
	Boundary *ArchiveBoundary `json:"boundary,omitempty"`
	// ReachableFrom is the oldest point that the objects in the bucket still
	// cover. The retention period removes old objects from the bucket. A
	// longer retention period does not bring back what a shorter one already
	// removed. The window grows to the new retention period only as the
	// archive writes past this point. The server refuses a rollback to a
	// point before it. When it is unset, only the retention period limits
	// the window.
	// +optional
	ReachableFrom *metav1.Time `json:"reachableFrom,omitempty"`
}

// RecoveryArchiveRef names the archive that a recovery reads: the directory in
// the bucket, the location of that bucket, and the bucket contract that
// names it. It also holds the archive settings of the server at that moment.
// Until the rollback has an answer, the operator keeps the archive as it
// was. It ignores each edit of spec.archive in that time: a moved bucket, a
// changed retention or schedule, and a removal.
type RecoveryArchiveRef struct {
	// ServerName is the archive directory, equal to the name of the
	// CloudNativePG cluster that wrote it.
	ServerName string `json:"serverName"`
	// ObjectStorageRef is the ObjectStorageConfig, in the namespace of this
	// server, that the archive lives in.
	ObjectStorageRef string `json:"objectStorageRef"`
	// Location is where in object storage the archive is, in the form of
	// ArchiveRecord.Location. A running recovery reads the location that it
	// recorded, so an edit of the ObjectStorageConfig does not move it.
	// +optional
	Location string `json:"location,omitempty"`
	// RetentionPeriodDays is spec.archive.retentionPeriodDays as it stood
	// when the rollback started.
	// +optional
	RetentionPeriodDays int32 `json:"retentionPeriodDays,omitempty"`
	// BaseBackupSchedule is spec.archive.baseBackupSchedule as it stood when
	// the rollback started.
	// +optional
	BaseBackupSchedule string `json:"baseBackupSchedule,omitempty"`
	// Identity is the workload identity of that bucket when the rollback
	// started. The pods keep this identity while they read the bucket. It is
	// unset for a bucket with static credentials, and for a bucket that
	// names no identity.
	// +optional
	Identity *RecoveryArchiveIdentity `json:"identity,omitempty"`
}

// RecoveryArchiveIdentity is the workload identity of a bucket: what the pods
// of a consumer use to read the objects in it. A bucket with static
// credentials has none.
type RecoveryArchiveIdentity struct {
	// Annotations are the annotations of the ServiceAccount of the pods.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// PodLabels are the labels that the pods need. Only Azure needs one.
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`
}

// DatabaseServerRecoveryStatus is the recovery request that the server works
// on now, or the last one that it answered. With it, a recovery can
// continue after an interruption.
//
// It holds the whole answer, not a reference to it. The server publishes the
// answer on a contract. If somebody deletes the contract and creates it
// again, the server publishes the answer again from this status.
type DatabaseServerRecoveryStatus struct {
	// RequestID is the requestID of the request, as the contract carries it.
	RequestID string `json:"requestID"`
	// Contract is the DatabaseServerConfig that carried the request. The
	// server answers on that contract. Until it answers, it does not change
	// the contract that it publishes.
	Contract string `json:"contract"`
	// RequestedBy is the requestedBy of the request, as the contract carries
	// it.
	RequestedBy string `json:"requestedBy"`
	// TargetTime is the targetTime of the request, as the contract carries it.
	TargetTime string `json:"targetTime"`
	// Cluster is the CloudNativePG cluster that the recovery builds. It is
	// empty for a request that the server refused, also when the server had
	// built a cluster before.
	// +optional
	Cluster string `json:"cluster,omitempty"`
	// PreviousCluster is the cluster that the contract pointed at before the
	// recovery moved it. If a recovery fails after the move, the contract
	// points at this cluster again, because it still holds the data.
	// +optional
	PreviousCluster string `json:"previousCluster,omitempty"`
	// Archive is the archive that the recovery reads. The server records it
	// before the recovery builds anything. A spec change to another bucket
	// does not move a recovery that is already running.
	// +optional
	Archive *RecoveryArchiveRef `json:"archive,omitempty"`
	// Result is the result that the server published for the request. It is
	// unset while the recovery runs.
	// +optional
	Result RecoveryResult `json:"result,omitempty"`
	// Message is the message that the server published with Result.
	// +optional
	Message string `json:"message,omitempty"`
	// CompletedAt is when the server answered the request. It is unset while
	// the recovery runs.
	// +optional
	CompletedAt *metav1.Time `json:"completedAt,omitempty"`
}

// DatabaseServerStatus is the observed state of a DatabaseServer.
type DatabaseServerStatus struct {
	// ObservedGeneration is the last generation reconciled by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Version is the PostgreSQL major version that the server runs, as a
	// number such as "17". It comes from the merged spec, so it is correct
	// when the release, the preset, or the server gives the version. While
	// the operator refuses a version change, it shows the major version of the
	// data directory. It is empty until the operator resolves the references
	// of the server for the first time.
	// +optional
	Version string `json:"version,omitempty"`
	// Cluster is the CloudNativePG cluster that the published contract points
	// at. It is the name of the server until a recovery replaces it.
	// +optional
	Cluster string `json:"cluster,omitempty"`
	// SystemIdentifier is the identity of the PostgreSQL instance behind the
	// contract, as CloudNativePG reports it. A recovery restores the
	// pg_control of its base backup, so the recovered instance keeps the
	// identity and this value does not change. A recovery replaces the
	// endpoint of the contract.
	// +optional
	SystemIdentifier string `json:"systemIdentifier,omitempty"`
	// Archive is the observed state of the continuous archive of the server.
	// It is unset until the server writes an archive. A removal of
	// spec.archive does not clear it. The bucket still holds what the server
	// wrote, and a server that archives again can recover from it.
	// +optional
	Archive *DatabaseServerArchiveStatus `json:"archive,omitempty"`
	// Recovery is the recovery request that the server works on now, or the
	// last one it answered. The answer itself is published on the contract,
	// in spec.pitr.lastRecovery.
	// +optional
	Recovery *DatabaseServerRecoveryStatus `json:"recovery,omitempty"`
	// Volumes lists the bound PersistentVolumeClaims of the current cluster
	// and the capacity that each one reports, sorted by name. A server with a
	// write-ahead log volume also shows that claim. A server that does not own
	// the cluster of its derived name shows no claims, because they belong
	// to the other cluster.
	// +listType=map
	// +listMapKey=name
	// +optional
	Volumes []VolumeStatus `json:"volumes,omitempty"`
	// Conditions represent the current state. Ready holds the reason of a
	// failed pre-check (InvalidReference, MissingSecret, CNPGNotInstalled,
	// BarmanPluginNotInstalled). Otherwise it follows the cluster, the
	// contract, and, when the server has an archive, the archive. The
	// per-component conditions (ClusterReady, ArchiveReady, ContractReady,
	// MonitoringReady) also appear here. MonitoringReady never affects Ready,
	// so a broken PodMonitor does not make the server not ready. ArchiveReady
	// without spec.archive does not affect Ready either.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.status.version`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="oldSelf.hasValue() || (self.metadata.name.matches('^[a-z]([-a-z0-9]*[a-z0-9])?$') && self.metadata.name.size() <= 46)",message="metadata.name must be a DNS-1035 label of at most 46 characters, because it names the CloudNativePG cluster of the server",optionalOldSelf=true

// DatabaseServer runs one PostgreSQL instance through the external
// CloudNativePG operator. It archives the instance continuously to an object
// storage bucket. It publishes the connection details as a
// DatabaseServerConfig, which a Database and a PointInTimeRestore use. One or
// more orchestration clusters use the instance, each through its own
// Database.
//
// A PointInTimeRestore rolls the whole instance back. It needs a server that
// holds the database of its cluster and nothing else.
//
// The name of the CR is also the name of the CloudNativePG cluster. On
// create, the name must be a DNS-1035 label of at most 46 characters.
// CloudNativePG accepts 50 characters, and a rollback appends up to four
// ("-r99"). While the recovery index is below 100, the cluster of a rollback
// gets the full name. Above that, the operator shortens the name to a head
// and a hash. Each archive record adds one to the index: a rollback, an
// archive that is enabled again, and a change of bucket.
type DatabaseServer struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of DatabaseServer
	// +kubebuilder:validation:XValidation:rule="has(self.databaseServerConfig)",message="databaseServerConfig is required"
	// +kubebuilder:validation:XValidation:rule="!has(oldSelf.storageSize) || !has(self.storageSize) || !quantity(string(self.storageSize)).isLessThan(quantity(string(oldSelf.storageSize)))",message="storageSize may not be shrunk"
	// +kubebuilder:validation:XValidation:rule="!has(oldSelf.walStorageSize) || !has(self.walStorageSize) || !quantity(string(self.walStorageSize)).isLessThan(quantity(string(oldSelf.walStorageSize)))",message="walStorageSize may not be shrunk"
	// +required
	Spec DatabaseServerSpec `json:"spec"`

	// status defines the observed state of DatabaseServer
	// +optional
	Status DatabaseServerStatus `json:"status,omitzero"`
}

// GetStatusConditions returns a pointer to the status conditions. The
// component framework stages per-component conditions on the resource through
// it.
func (in *DatabaseServer) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// GetKind returns the CRD kind. The component framework uses it for event and
// metric recording and derives its per-component SSA field managers
// (DatabaseServer/<component>) from it.
func (in *DatabaseServer) GetKind() string { return "DatabaseServer" }

// SetObservedGeneration records the last reconciled generation in status.
func (in *DatabaseServer) SetObservedGeneration(generation int64) {
	in.Status.ObservedGeneration = generation
}

// +kubebuilder:object:root=true

// DatabaseServerList contains a list of DatabaseServer
type DatabaseServerList struct {
	metav1.TypeMeta `                 json:",inline"`
	metav1.ListMeta `                 json:"metadata,omitzero"`
	Items           []DatabaseServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DatabaseServer{}, &DatabaseServerList{})
}
