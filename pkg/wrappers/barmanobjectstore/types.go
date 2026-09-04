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

package barmanobjectstore

// This file mirrors the upstream ObjectStore API of the CloudNativePG
// barman-cloud plugin, in the order upstream declares it. The file order rule
// of how-we-write-go does not apply to it.

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is the group and version of the ObjectStore custom
	// resource.
	GroupVersion = schema.GroupVersion{Group: "barmancloud.cnpg.io", Version: "v1"}

	// SchemeBuilder registers the ObjectStore types. A manager and a test
	// suite that read or write the kind add them to their scheme through
	// AddToScheme.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme adds the ObjectStore types to a scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func addKnownTypes(s *runtime.Scheme) error {
	s.AddKnownTypes(GroupVersion, &ObjectStore{}, &ObjectStoreList{})
	metav1.AddToGroupVersion(s, GroupVersion)

	return nil
}

// CompressionType is a compression algorithm that barman-cloud applies to an
// uploaded file.
type CompressionType string

// The compression algorithms that barman-cloud accepts. WAL files accept
// every value; a base backup accepts every value except xz and zstd.
const (
	CompressionTypeGzip   CompressionType = "gzip"
	CompressionTypeBzip2  CompressionType = "bzip2"
	CompressionTypeLz4    CompressionType = "lz4"
	CompressionTypeSnappy CompressionType = "snappy"
	CompressionTypeXz     CompressionType = "xz"
	CompressionTypeZstd   CompressionType = "zstd"
)

// EncryptionType is the server-side encryption that the object storage
// applies to an uploaded file.
type EncryptionType string

// The server-side encryption values that barman-cloud accepts.
const (
	EncryptionTypeAES256 EncryptionType = "AES256"
	EncryptionTypeAWSKMS EncryptionType = "aws:kms"
)

// LogLevel is the verbosity of the sidecar that the plugin adds to every
// instance of a Cluster.
type LogLevel string

// The verbosity values that the plugin sidecar accepts.
const (
	LogLevelError   LogLevel = "error"
	LogLevelWarning LogLevel = "warning"
	LogLevelInfo    LogLevel = "info"
	LogLevelDebug   LogLevel = "debug"
	LogLevelTrace   LogLevel = "trace"
)

// +kubebuilder:object:root=true

// ObjectStore is one object storage location that the Barman Cloud plugin
// writes the WAL archive and the base backups of a Cluster to.
// +kubebuilder:object:generate=true
type ObjectStore struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ObjectStoreSpec `json:"spec"`
	// +optional
	Status ObjectStoreStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ObjectStoreList is a list of ObjectStore objects.
// +kubebuilder:object:generate=true
type ObjectStoreList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ObjectStore `json:"items"`
}

// ObjectStoreSpec is the desired state of an ObjectStore.
// +kubebuilder:object:generate=true
type ObjectStoreSpec struct {
	// Configuration addresses the object storage and says how barman-cloud
	// uploads to it.
	Configuration BarmanObjectStoreConfiguration `json:"configuration"`
	// RetentionPolicy is how long a base backup and the WAL files that
	// follow it are kept, as a barman retention policy such as "30d". An
	// empty policy keeps every backup.
	// +optional
	RetentionPolicy string `json:"retentionPolicy,omitempty"`
	// InstanceSidecarConfiguration shapes the sidecar container that the
	// plugin adds to every instance of a Cluster that archives here.
	// +optional
	InstanceSidecarConfiguration *InstanceSidecarConfiguration `json:"instanceSidecarConfiguration,omitempty"`
}

// BarmanObjectStoreConfiguration is the barman-cloud configuration of one
// object storage location.
//
// It carries no server name. The CRD forbids the serverName field of the
// upstream barman-cloud configuration, and the Cluster names its archive
// directory through plugins[].parameters.serverName instead.
// +kubebuilder:object:generate=true
type BarmanObjectStoreConfiguration struct {
	// GoogleCredentials authenticate against Google Cloud Storage.
	// +optional
	GoogleCredentials *GoogleCredentials `json:"googleCredentials,omitempty"`
	// S3Credentials authenticate against S3 and every S3-compatible store.
	// +optional
	S3Credentials *S3Credentials `json:"s3Credentials,omitempty"`
	// AzureCredentials authenticate against Azure Blob Storage.
	// +optional
	AzureCredentials *AzureCredentials `json:"azureCredentials,omitempty"`
	// EndpointURL addresses an S3-compatible store. An empty URL uses the
	// endpoint of the provider that the credentials name.
	// +optional
	EndpointURL string `json:"endpointURL,omitempty"`
	// EndpointCA names a Secret key that holds the certificate authority of
	// the endpoint.
	// +optional
	EndpointCA *SecretKeySelector `json:"endpointCA,omitempty"`
	// DestinationPath is the bucket URL that holds the archive, such as
	// "s3://bucket/path".
	DestinationPath string `json:"destinationPath"`
	// Wal shapes the upload of the WAL files.
	// +optional
	Wal *WalBackupConfiguration `json:"wal,omitempty"`
	// Data shapes the upload of the base backups.
	// +optional
	Data *DataBackupConfiguration `json:"data,omitempty"`
	// Tags are passed to barman-cloud for every uploaded file.
	// +optional
	Tags map[string]string `json:"tags,omitempty"`
	// HistoryTags are passed to barman-cloud for every uploaded history
	// file.
	// +optional
	HistoryTags map[string]string `json:"historyTags,omitempty"`
}

// S3Credentials authenticates against S3. Either AccessKeyID together with
// SecretAccessKey, or InheritFromIAMRole, names the identity.
// +kubebuilder:object:generate=true
type S3Credentials struct {
	// AccessKeyID names a Secret key that holds the access key.
	// +optional
	AccessKeyID *SecretKeySelector `json:"accessKeyId,omitempty"`
	// SecretAccessKey names a Secret key that holds the secret access key.
	// +optional
	SecretAccessKey *SecretKeySelector `json:"secretAccessKey,omitempty"`
	// Region names a Secret key that holds the region of the bucket.
	// +optional
	Region *SecretKeySelector `json:"region,omitempty"`
	// SessionToken names a Secret key that holds a session token.
	// +optional
	SessionToken *SecretKeySelector `json:"sessionToken,omitempty"`
	// InheritFromIAMRole takes the credentials from the environment of the
	// pod instead of from a Secret.
	// +optional
	InheritFromIAMRole bool `json:"inheritFromIAMRole,omitempty"`
}

// AzureCredentials authenticates against Azure Blob Storage. ConnectionString
// carries every needed value on its own. Without it, exactly one of
// StorageKey, StorageSasToken, InheritFromAzureAD, and
// UseDefaultAzureCredentials names the identity.
// +kubebuilder:object:generate=true
type AzureCredentials struct {
	// ConnectionString names a Secret key that holds a connection string.
	// +optional
	ConnectionString *SecretKeySelector `json:"connectionString,omitempty"`
	// StorageAccount names a Secret key that holds the storage account.
	// +optional
	StorageAccount *SecretKeySelector `json:"storageAccount,omitempty"`
	// StorageKey names a Secret key that holds the storage account key.
	// +optional
	StorageKey *SecretKeySelector `json:"storageKey,omitempty"`
	// StorageSasToken names a Secret key that holds a shared access
	// signature token.
	// +optional
	StorageSasToken *SecretKeySelector `json:"storageSasToken,omitempty"`
	// InheritFromAzureAD takes the credentials from the environment of the
	// pod instead of from a Secret.
	// +optional
	InheritFromAzureAD bool `json:"inheritFromAzureAD,omitempty"`
	// UseDefaultAzureCredentials runs the default Azure authentication flow.
	// +optional
	UseDefaultAzureCredentials bool `json:"useDefaultAzureCredentials,omitempty"`
}

// GoogleCredentials authenticates against Google Cloud Storage. Either
// ApplicationCredentials or GKEEnvironment names the identity.
// +kubebuilder:object:generate=true
type GoogleCredentials struct {
	// ApplicationCredentials names a Secret key that holds a service account
	// key in JSON.
	// +optional
	ApplicationCredentials *SecretKeySelector `json:"applicationCredentials,omitempty"`
	// GKEEnvironment takes the credentials from the environment of the pod
	// instead of from a Secret.
	// +optional
	GKEEnvironment bool `json:"gkeEnvironment,omitempty"`
}

// WalBackupConfiguration shapes the upload of the WAL files.
// +kubebuilder:object:generate=true
type WalBackupConfiguration struct {
	// Compression is the algorithm that compresses a WAL file before it is
	// uploaded. An empty value uploads the file uncompressed.
	// +optional
	Compression CompressionType `json:"compression,omitempty"`
	// Encryption is the server-side encryption that the object storage
	// applies. An empty value uses the encryption of the bucket.
	// +optional
	Encryption EncryptionType `json:"encryption,omitempty"`
	// MaxParallel is how many WAL files barman-cloud uploads at once.
	// +optional
	MaxParallel int `json:"maxParallel,omitempty"`
	// ArchiveAdditionalCommandArgs are passed to barman-cloud-wal-archive.
	// +optional
	ArchiveAdditionalCommandArgs []string `json:"archiveAdditionalCommandArgs,omitempty"`
	// RestoreAdditionalCommandArgs are passed to barman-cloud-wal-restore.
	// +optional
	RestoreAdditionalCommandArgs []string `json:"restoreAdditionalCommandArgs,omitempty"`
}

// DataBackupConfiguration shapes the upload of the base backups.
// +kubebuilder:object:generate=true
type DataBackupConfiguration struct {
	// Compression is the algorithm that compresses the base backup before it
	// is uploaded. An empty value uploads it uncompressed. The plugin
	// rejects xz and zstd here.
	// +optional
	Compression CompressionType `json:"compression,omitempty"`
	// Encryption is the server-side encryption that the object storage
	// applies. An empty value uses the encryption of the bucket.
	// +optional
	Encryption EncryptionType `json:"encryption,omitempty"`
	// Jobs is how many upload jobs barman-cloud runs at once.
	// +optional
	Jobs *int32 `json:"jobs,omitempty"`
	// ImmediateCheckpoint asks PostgreSQL for a checkpoint at once instead
	// of spreading the write over the checkpoint interval. The backup starts
	// sooner and the server takes more I/O while it runs.
	// +optional
	ImmediateCheckpoint bool `json:"immediateCheckpoint,omitempty"`
	// AdditionalCommandArgs are passed to barman-cloud-backup.
	// +optional
	AdditionalCommandArgs []string `json:"additionalCommandArgs,omitempty"`
	// RestoreAdditionalCommandArgs are passed to barman-cloud-restore.
	// +optional
	RestoreAdditionalCommandArgs []string `json:"restoreAdditionalCommandArgs,omitempty"`
}

// InstanceSidecarConfiguration shapes the sidecar container that the plugin
// adds to every instance of a Cluster that archives to this ObjectStore.
// +kubebuilder:object:generate=true
type InstanceSidecarConfiguration struct {
	// Env are the environment variables of the sidecar.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
	// Resources are the requests and limits of the sidecar.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// RetentionPolicyIntervalSeconds is how often the sidecar applies
	// RetentionPolicy. The plugin defaults it to 1800.
	// +optional
	RetentionPolicyIntervalSeconds *int32 `json:"retentionPolicyIntervalSeconds,omitempty"`
	// LogLevel is the verbosity of the sidecar. The plugin defaults it to
	// info.
	// +optional
	LogLevel LogLevel `json:"logLevel,omitempty"`
	// AdditionalContainerArgs are passed to the sidecar command.
	// +optional
	AdditionalContainerArgs []string `json:"additionalContainerArgs,omitempty"`
}

// ObjectStoreStatus is the observed state of an ObjectStore.
// +kubebuilder:object:generate=true
type ObjectStoreStatus struct {
	// ServerRecoveryWindow maps the server name of every archive in this
	// store to the window a recovery of it can reach.
	// +optional
	ServerRecoveryWindow map[string]RecoveryWindow `json:"serverRecoveryWindow,omitempty"`
}

// RecoveryWindow is the span between the first recoverability point and the
// last successful base backup of one archived server. A recovery can reach
// any point in it.
// +kubebuilder:object:generate=true
type RecoveryWindow struct {
	// FirstRecoverabilityPoint is the earliest point a recovery can reach.
	// +optional
	FirstRecoverabilityPoint *metav1.Time `json:"firstRecoverabilityPoint,omitempty"`
	// LastSuccessfulBackupTime is when the last base backup completed.
	// +optional
	LastSuccessfulBackupTime *metav1.Time `json:"lastSuccessfulBackupTime,omitempty"`
	// LastFailedBackupTime is when the last base backup failed.
	// +optional
	LastFailedBackupTime *metav1.Time `json:"lastFailedBackupTime,omitempty"`
}

// SecretKeySelector names one key of one Secret in the namespace of the
// ObjectStore.
// +kubebuilder:object:generate=true
type SecretKeySelector struct {
	// Name is the name of the Secret.
	Name string `json:"name"`
	// Key is the key inside the Secret.
	Key string `json:"key"`
}
