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
// account and names it after the server, so only its metadata is
// configurable. The operator adds the workload-identity annotations of the
// archive bucket on its own; an annotation set here wins over the derived one
// on the same key.
type DatabaseServerServiceAccountSpec struct {
	// Annotations to set on the ServiceAccount, typically workload-identity
	// annotations (IRSA, GCP Workload Identity, and more) that grant the
	// instance pods access to cloud resources.
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
// continuous archive: the write-ahead log of every instance, plus the base
// backups a recovery starts from. A server with an archive publishes
// pitr.enabled true on its contract; a server without one publishes false and
// no point-in-time restore can reach it.
//
// The archive is not the backup model of the operator. BackupSchedule and
// LogicalBackupRDBMS take logical dumps and never see these base backups.
type DatabaseServerArchiveSpec struct {
	// ObjectStorageRef names a cluster-scoped ObjectStorageConfig. The
	// operator writes the archive under a prefix of that bucket that holds
	// this server alone.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=253
	ObjectStorageRef string `json:"objectStorageRef"`
	// RetentionPeriodDays is how far into the past a restore can reach. It is
	// what the operator enforces on the bucket and what the contract of this
	// server publishes, so the declared value and the enforced value are one.
	// +kubebuilder:validation:Minimum=1
	RetentionPeriodDays int32 `json:"retentionPeriodDays"`
	// BaseBackupSchedule is when a base backup is taken, as the six-field
	// cron of CloudNativePG (seconds first, in UTC). The first base backup
	// runs as soon as the server is up, whatever the schedule says, because
	// the archive can be recovered from only after it completes.
	// +kubebuilder:default="0 0 2 * * *"
	// +optional
	BaseBackupSchedule string `json:"baseBackupSchedule,omitempty"`
}

// DatabaseServerSpec defines the desired state of DatabaseServer.
//
// The type doubles as the configuration baseline of a DatabaseServerPreset,
// so the field that is required on a DatabaseServer — databaseServerConfig —
// is optional at the schema level here and enforced on the DatabaseServer
// usage instead.
type DatabaseServerSpec struct {
	// PresetRef names a cluster-scoped DatabaseServerPreset used as the
	// configuration baseline; fields set inline override the preset's value
	// for that field wholesale.
	// +optional
	PresetRef string `json:"presetRef,omitempty"`
	// PlatformConfigRef names a cluster-scoped CamundaPlatformConfig. Only
	// its image settings are read: spec.imageRegistry and
	// spec.images.postgres decide where the PostgreSQL image is pulled from,
	// which is what an air-gapped cluster needs. Empty leaves the image at
	// its default repository.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=253
	// +optional
	PlatformConfigRef string `json:"platformConfigRef,omitempty"`
	// Version is the PostgreSQL major version to run, as a bare number such
	// as "17". It selects the image tag. Camunda 8.9 supports PostgreSQL 14
	// and later; the floor is enforced by the controller on the
	// preset-merged result. Required unless the resolved preset provides it.
	//
	// The major of a running server cannot change. A value that names another
	// major, higher or lower, is refused on the Ready condition with reason
	// VersionChangeRefused, and the server keeps running the major it has. A
	// preset can raise it the same way and is refused the same way. To run
	// another major, create a server on it and move the data over.
	// +kubebuilder:validation:Pattern=`^\d+$`
	// +optional
	Version string `json:"version,omitempty"`
	// Instances is the number of PostgreSQL instances. One instance has no
	// failover: a node that goes away takes the server with it until the
	// volume is reattached. Defaults to 1.
	// +kubebuilder:validation:Minimum=1
	// +optional
	Instances *int32 `json:"instances,omitempty"`
	// Resources are the CPU and memory of each instance.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// StorageSize is the size of the data volume of each instance. It cannot
	// shrink, because PostgreSQL data volumes cannot be reduced in place.
	// Admission rejects a lower inline value on a DatabaseServer through a
	// CEL transition rule. That rule does not bind this shared field, so a
	// preset baseline can be lowered: a server that already applied a larger
	// size keeps it and records a StorageShrinkIgnored event. Required unless
	// the resolved preset provides it.
	// +optional
	StorageSize *resource.Quantity `json:"storageSize,omitempty"`
	// StorageClassName is the StorageClass of the data volumes. Defaults to
	// the cluster's default StorageClass.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
	// WALStorageSize puts the write-ahead log on a volume of its own, of this
	// size. Unset keeps the log on the data volume. It cannot shrink, for the
	// same reason storageSize cannot, and a lowered preset baseline is
	// ignored the same way.
	//
	// It can be added to a running server, and it cannot be taken away again:
	// CloudNativePG refuses a cluster that gives up the volume. A cleared
	// field, inline or from a preset, keeps the volume at the size it has and
	// records a WALStorageKept event.
	// +optional
	WALStorageSize *resource.Quantity `json:"walStorageSize,omitempty"`
	// ServiceAccount configures the ServiceAccount of the instance pods.
	// +optional
	ServiceAccount *DatabaseServerServiceAccountSpec `json:"serviceAccount,omitempty"`
	// Scheduling constraints for the instance pods; when set, it replaces the
	// preset's scheduling block entirely (no merge).
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
	// DatabaseServerConfig names the DatabaseServerConfig the operator
	// publishes in this CR's own namespace with the endpoint, the admin
	// credentials, and the point-in-time-recovery capability of the server.
	// Required on a DatabaseServer, forbidden in a preset.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=253
	// +optional
	DatabaseServerConfig string `json:"databaseServerConfig,omitempty"`
	// Archive points the server at the bucket that holds its continuous
	// archive. Without it the server takes no part in point-in-time restore.
	// +optional
	Archive *DatabaseServerArchiveSpec `json:"archive,omitempty"`
	// Suspend stops the PostgreSQL instances and keeps their data volumes.
	// The operator hibernates the CloudNativePG cluster: the instance pods
	// are removed, the volumes stay, and setting the field back to false
	// brings the instances back on the same volumes. Defaults to false.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// The per-component conditions of a DatabaseServer. Ready is derived from
// ClusterReady, ContractReady, and, on a server that asks for an archive,
// ArchiveReady. MonitoringReady always stands on its own, and so does
// ArchiveReady on a server with no archive.
const (
	// ConditionClusterReady reports whether the CloudNativePG cluster that the
	// published contract points at is healthy.
	ConditionClusterReady = "ClusterReady"
	// ConditionArchiveReady reports whether the archive the server writes now
	// can be recovered from, which it can once its first base backup
	// completed. It reads Disabled on a server with no spec.archive.
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
// started. The operator decides at start whether it watches CloudNativePG
// resources, so it must restart after CloudNativePG is installed.
const ReasonCNPGNotInstalled = "CNPGNotInstalled"

// ReasonBarmanPluginNotInstalled is the Ready reason of a DatabaseServer that
// asks for an archive on a cluster that did not serve the ObjectStore kind of
// the Barman Cloud plugin when the operator started. A server without an
// archive block is unaffected.
const ReasonBarmanPluginNotInstalled = "BarmanPluginNotInstalled"

// ReasonVersionChangeRefused is the Ready reason of a DatabaseServer whose
// merged version names a PostgreSQL major other than the one its data
// directory runs. The server keeps running the major it has: everything the
// operator renders takes that major, so a rollback in flight finishes and the
// contract and the archive stay maintained. A major change needs a new server,
// and there is no annotation that lets it through.
const ReasonVersionChangeRefused = "VersionChangeRefused"

// ArchiveRecord is one continuous archive that a server has written. A
// recovery replays it from a base backup up to the requested point, so only a
// point inside the interval of a record can be reached.
type ArchiveRecord struct {
	// ServerName is the archive directory in the bucket, equal to the name of
	// the CloudNativePG cluster that wrote it.
	ServerName string `json:"serverName"`
	// ObjectStorageRef is the cluster-scoped ObjectStorageConfig that holds
	// this archive. A server that is pointed at another bucket closes this
	// record and opens one of its own, so every interval names the bucket a
	// restore of that interval has to read.
	// +optional
	ObjectStorageRef string `json:"objectStorageRef,omitempty"`
	// From is the earliest point this archive can be recovered to: when its
	// first base backup completed.
	From metav1.Time `json:"from"`
	// To is the latest point this archive can be recovered to. It is unset
	// while the archive is the one the server writes to now.
	// +optional
	To *metav1.Time `json:"to,omitempty"`
}

// DatabaseServerArchiveStatus is the observed state of the archive of a
// server.
type DatabaseServerArchiveStatus struct {
	// History lists every archive the server has written, oldest first. A
	// recovery picks the record whose interval holds the requested point.
	// Records are never removed, so a restore can reach back across an
	// earlier recovery for as long as the bucket keeps the objects. Removing
	// spec.archive closes the record that is open and keeps the list, and
	// asking for an archive again opens a record of its own. Pointing
	// spec.archive at another bucket does the same. The window with no
	// archive therefore lies inside no interval, and no restore can reach a
	// point in it.
	// +optional
	History []ArchiveRecord `json:"history,omitempty"`
}

// RecoveryArchiveRef names the archive that a recovery reads: the directory in
// the bucket, and the bucket contract that holds it.
type RecoveryArchiveRef struct {
	// ServerName is the archive directory, equal to the name of the
	// CloudNativePG cluster that wrote it.
	ServerName string `json:"serverName"`
	// ObjectStorageRef is the cluster-scoped ObjectStorageConfig that the
	// archive lives in.
	ObjectStorageRef string `json:"objectStorageRef"`
}

// DatabaseServerRecoveryStatus is the recovery request that the server works
// on now, or the last one it answered. It is what makes a recovery resumable:
// the steps read it to tell which cluster they build and whether the request
// still needs an answer.
//
// It holds the whole answer, not a reference to it. The answer is published on
// a contract that somebody can delete and create again, and the server has to
// be able to publish it a second time from what it remembers.
type DatabaseServerRecoveryStatus struct {
	// RequestID is the requestID of the request, as the contract carries it.
	RequestID string `json:"requestID"`
	// Contract is the DatabaseServerConfig that carried the request. The
	// server answers on that contract, and it does not change the contract it
	// publishes while the request is unanswered.
	Contract string `json:"contract"`
	// RequestedBy is the requestedBy of the request, as the contract carries
	// it.
	RequestedBy string `json:"requestedBy"`
	// TargetTime is the targetTime of the request, as the contract carries it.
	TargetTime string `json:"targetTime"`
	// Cluster is the CloudNativePG cluster that the recovery builds. It is
	// empty for a request that the server refused, whether or not it had built
	// one by then.
	// +optional
	Cluster string `json:"cluster,omitempty"`
	// PreviousCluster is the cluster that the contract pointed at before the
	// recovery moved it. A recovery that fails after the move puts the
	// contract back on it, because that cluster still holds the data.
	// +optional
	PreviousCluster string `json:"previousCluster,omitempty"`
	// Archive is the archive that the recovery reads. It is recorded before
	// the recovery builds anything and read on every look after that, so a
	// spec that names another bucket in the middle does not move a recovery
	// that is already running.
	// +optional
	Archive *RecoveryArchiveRef `json:"archive,omitempty"`
	// Result is the result the server published for the request. It is unset
	// while the recovery runs.
	// +optional
	Result RecoveryResult `json:"result,omitempty"`
	// Message is the message the server published with Result.
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
	// Cluster is the CloudNativePG cluster that the published contract points
	// at. It is the name of the server until a recovery replaces it.
	// +optional
	Cluster string `json:"cluster,omitempty"`
	// SystemIdentifier is the identity of the PostgreSQL instance that runs
	// behind the contract, as CloudNativePG reports it. A recovery restores
	// the pg_control of the base backup it reads, so the recovered instance
	// reports the identity it recovered from and this value stays. The
	// endpoint of the contract is what a recovery replaces.
	// +optional
	SystemIdentifier string `json:"systemIdentifier,omitempty"`
	// Archive is the observed state of the continuous archive of the server.
	// It is unset until the server has written one. Removing spec.archive
	// does not clear it: the bucket still holds what the server wrote, and a
	// server that archives again can recover from it.
	// +optional
	Archive *DatabaseServerArchiveStatus `json:"archive,omitempty"`
	// Recovery is the recovery request that the server works on now, or the
	// last one it answered. The answer itself is published on the contract,
	// in spec.pitr.lastRecovery.
	// +optional
	Recovery *DatabaseServerRecoveryStatus `json:"recovery,omitempty"`
	// Volumes lists the bound PersistentVolumeClaims of the current cluster
	// and the capacity that each one reports, sorted by name. A server with a
	// write-ahead log volume reports that claim here too.
	// +listType=map
	// +listMapKey=name
	// +optional
	Volumes []VolumeStatus `json:"volumes,omitempty"`
	// Conditions represent the current state. Ready carries a pre-check
	// reason (InvalidReference, MissingSecret, CNPGNotInstalled,
	// BarmanPluginNotInstalled), or it is derived from the cluster, the
	// contract, and the archive of a server that asks for one. The
	// per-component conditions (ClusterReady, ArchiveReady, ContractReady,
	// MonitoringReady) also appear here. MonitoringReady, and ArchiveReady
	// without spec.archive, are reported on their own and never on Ready. A
	// PodMonitor observes the server rather than runs it, so a broken one
	// never makes the server not ready.
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
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.version`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="oldSelf.hasValue() || (self.metadata.name.matches('^[a-z]([-a-z0-9]*[a-z0-9])?$') && self.metadata.name.size() <= 46)",message="metadata.name must be a DNS-1035 label of at most 46 characters, because it names the CloudNativePG cluster of the server",optionalOldSelf=true

// DatabaseServer runs a PostgreSQL server for one orchestration cluster
// through the external CloudNativePG operator, archives it continuously to an
// object storage bucket, and publishes the connection details as a
// DatabaseServerConfig that a Database and a PointInTimeRestore consume.
//
// The name of the CR names the CloudNativePG cluster, so admission holds it to
// a DNS-1035 label of at most 46 characters: the 50 that CloudNativePG accepts
// for a cluster name, less the four of the "-r99" that a rollback appends. A
// name inside that bound reaches the cluster of every one of the first
// ninety-nine rollbacks whole. A rollback after those shortens it to a head
// and a hash.
//
// The rule runs on create only. A name never changes on update, so re-checking
// it would only reject an edit of some other field on an object that predates
// the rule.
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
