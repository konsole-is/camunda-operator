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

// ObjectStorageType identifies the storage API of a bucket.
// +kubebuilder:validation:Enum=S3;GCS;AzureBlob
type ObjectStorageType string

// ObjectStorageTypeS3, ObjectStorageTypeGCS, and ObjectStorageTypeAzureBlob
// are the supported storage APIs.
const (
	ObjectStorageTypeS3        ObjectStorageType = "S3"
	ObjectStorageTypeGCS       ObjectStorageType = "GCS"
	ObjectStorageTypeAzureBlob ObjectStorageType = "AzureBlob"
)

// ObjectStorageAuthType selects how consumers authenticate against a bucket.
// +kubebuilder:validation:Enum=workloadIdentity;credentials
type ObjectStorageAuthType string

// ObjectStorageAuthTypeWorkloadIdentity and ObjectStorageAuthTypeCredentials
// are the supported authentication choices. Workload identity binds a cloud
// principal to the consumer's ServiceAccount; credentials are static keys in
// a Secret.
const (
	ObjectStorageAuthTypeWorkloadIdentity ObjectStorageAuthType = "workloadIdentity"
	ObjectStorageAuthTypeCredentials      ObjectStorageAuthType = "credentials"
)

// S3WorkloadIdentity names the AWS principal that the bucket trusts. An empty
// block means the consumer's ServiceAccount chain already carries the
// identity (EKS Pod Identity), so the operator adds nothing.
type S3WorkloadIdentity struct {
	// RoleARN is the IAM role that consumers assume. When set, the operator
	// puts it in the eks.amazonaws.com/role-arn annotation of the consumer's
	// ServiceAccount (IRSA).
	// +optional
	RoleARN string `json:"roleArn,omitempty"`
}

// S3CredentialsSecretRef references an access-key pair stored in a Secret.
// Namespace is required so that references stay uniform and explicit across
// all contract kinds.
type S3CredentialsSecretRef struct {
	// Name of the Secret holding the keys.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Namespace of the Secret.
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace"`
	// AccessKeyIDKey is the key in the Secret holding the access key ID.
	// +kubebuilder:validation:MinLength=1
	AccessKeyIDKey string `json:"accessKeyIdKey"`
	// SecretAccessKeyKey is the key in the Secret holding the secret access
	// key.
	// +kubebuilder:validation:MinLength=1
	SecretAccessKeyKey string `json:"secretAccessKeyKey"`
}

// S3Credentials holds the static keys of an S3 bucket.
type S3Credentials struct {
	// SecretRef names the Secret keys that hold the access-key pair.
	SecretRef S3CredentialsSecretRef `json:"secretRef"`
}

// S3StorageAuth selects how consumers authenticate against an S3 bucket.
// +kubebuilder:validation:XValidation:rule="(self.type == 'credentials') == has(self.credentials)",message="credentials is required when auth.type is credentials and must not be set otherwise"
// +kubebuilder:validation:XValidation:rule="!has(self.workloadIdentity) || self.type == 'workloadIdentity'",message="workloadIdentity is only valid when auth.type is workloadIdentity"
type S3StorageAuth struct {
	// Type is the authentication choice. Defaults to workloadIdentity.
	// +kubebuilder:default=workloadIdentity
	// +optional
	Type ObjectStorageAuthType `json:"type,omitempty"`
	// WorkloadIdentity names the trusted principal. Only valid with type
	// workloadIdentity; an empty or absent block means "trust the
	// ServiceAccount chain, add nothing".
	// +optional
	WorkloadIdentity *S3WorkloadIdentity `json:"workloadIdentity,omitempty"`
	// Credentials are static keys. Required with type credentials, forbidden
	// otherwise.
	// +optional
	Credentials *S3Credentials `json:"credentials,omitempty"`
}

// S3Storage describes an S3 or S3-compatible bucket.
// +kubebuilder:validation:XValidation:rule="(has(self.region) && self.region != '') || (has(self.endpoint) && self.endpoint != '')",message="region is required unless endpoint is set"
type S3Storage struct {
	// BucketName is the bucket name as used by storage client SDKs.
	// +kubebuilder:validation:MinLength=1
	BucketName string `json:"bucketName"`
	// BasePath is the key prefix under which consumers write objects. Empty
	// means the bucket root.
	// +optional
	BasePath string `json:"basePath,omitempty"`
	// Region of the bucket. Required unless endpoint is set.
	// +optional
	Region string `json:"region,omitempty"`
	// Endpoint is the URL of an S3-compatible store (MinIO, Ceph, and more).
	// Empty means AWS S3, addressed through region.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https')",message="endpoint must be a valid http or https URL"
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
	// ForcePathStyle forces path-style bucket addressing. Set it for
	// S3-compatible stores that do not serve virtual-hosted-style requests.
	// +optional
	ForcePathStyle bool `json:"forcePathStyle,omitempty"`
	// Auth selects how consumers authenticate. An absent block means
	// workload identity through the ServiceAccount chain.
	// +kubebuilder:default={type: workloadIdentity}
	// +optional
	Auth S3StorageAuth `json:"auth,omitempty"`
}

// GCSWorkloadIdentity names the Google principal that the bucket trusts. An
// empty block means the consumer's ServiceAccount chain already carries the
// identity (Workload Identity Federation for GKE), so the operator adds
// nothing.
type GCSWorkloadIdentity struct {
	// ServiceAccountEmail is the Google service account that consumers
	// impersonate. When set, the operator puts it in the
	// iam.gke.io/gcp-service-account annotation of the consumer's
	// ServiceAccount.
	// +optional
	ServiceAccountEmail string `json:"serviceAccountEmail,omitempty"`
}

// GCSCredentials holds the static key of a GCS bucket.
type GCSCredentials struct {
	// SecretRef names the Secret key that holds the service-account JSON
	// key.
	SecretRef SecretKeyRef `json:"secretRef"`
}

// GCSStorageAuth selects how consumers authenticate against a GCS bucket.
// +kubebuilder:validation:XValidation:rule="(self.type == 'credentials') == has(self.credentials)",message="credentials is required when auth.type is credentials and must not be set otherwise"
// +kubebuilder:validation:XValidation:rule="!has(self.workloadIdentity) || self.type == 'workloadIdentity'",message="workloadIdentity is only valid when auth.type is workloadIdentity"
type GCSStorageAuth struct {
	// Type is the authentication choice. Defaults to workloadIdentity.
	// +kubebuilder:default=workloadIdentity
	// +optional
	Type ObjectStorageAuthType `json:"type,omitempty"`
	// WorkloadIdentity names the trusted principal. Only valid with type
	// workloadIdentity; an empty or absent block means "trust the
	// ServiceAccount chain, add nothing".
	// +optional
	WorkloadIdentity *GCSWorkloadIdentity `json:"workloadIdentity,omitempty"`
	// Credentials is a static service-account key. Required with type
	// credentials, forbidden otherwise.
	// +optional
	Credentials *GCSCredentials `json:"credentials,omitempty"`
}

// GCSStorage describes a Google Cloud Storage bucket.
type GCSStorage struct {
	// BucketName is the bucket name as used by storage client SDKs.
	// +kubebuilder:validation:MinLength=1
	BucketName string `json:"bucketName"`
	// BasePath is the key prefix under which consumers write objects. Empty
	// means the bucket root.
	// +optional
	BasePath string `json:"basePath,omitempty"`
	// Auth selects how consumers authenticate. An absent block means
	// workload identity through the ServiceAccount chain.
	// +kubebuilder:default={type: workloadIdentity}
	// +optional
	Auth GCSStorageAuth `json:"auth,omitempty"`
}

// AzureBlobWorkloadIdentity names the Azure principal that the container
// trusts. An empty block means the consumer's ServiceAccount chain already
// carries the identity, so the operator adds nothing.
type AzureBlobWorkloadIdentity struct {
	// ClientID is the managed identity that consumers use. When set, the
	// operator puts it in the azure.workload.identity/client-id annotation
	// of the consumer's ServiceAccount.
	// +optional
	ClientID string `json:"clientId,omitempty"`
}

// AzureBlobCredentials holds the static key of an Azure storage account.
type AzureBlobCredentials struct {
	// SecretRef names the Secret key that holds the storage account key.
	SecretRef SecretKeyRef `json:"secretRef"`
}

// AzureBlobStorageAuth selects how consumers authenticate against an Azure
// Blob container.
// +kubebuilder:validation:XValidation:rule="(self.type == 'credentials') == has(self.credentials)",message="credentials is required when auth.type is credentials and must not be set otherwise"
// +kubebuilder:validation:XValidation:rule="!has(self.workloadIdentity) || self.type == 'workloadIdentity'",message="workloadIdentity is only valid when auth.type is workloadIdentity"
type AzureBlobStorageAuth struct {
	// Type is the authentication choice. Defaults to workloadIdentity.
	// +kubebuilder:default=workloadIdentity
	// +optional
	Type ObjectStorageAuthType `json:"type,omitempty"`
	// WorkloadIdentity names the trusted principal. Only valid with type
	// workloadIdentity; an empty or absent block means "trust the
	// ServiceAccount chain, add nothing".
	// +optional
	WorkloadIdentity *AzureBlobWorkloadIdentity `json:"workloadIdentity,omitempty"`
	// Credentials is a static storage account key. Required with type
	// credentials, forbidden otherwise.
	// +optional
	Credentials *AzureBlobCredentials `json:"credentials,omitempty"`
}

// AzureBlobStorage describes an Azure Blob Storage container.
type AzureBlobStorage struct {
	// AccountName is the storage account that holds the container.
	// +kubebuilder:validation:MinLength=1
	AccountName string `json:"accountName"`
	// Container is the blob container that consumers write to.
	// +kubebuilder:validation:MinLength=1
	Container string `json:"container"`
	// BasePath is the blob prefix under which consumers write objects. Empty
	// means the container root.
	// +optional
	BasePath string `json:"basePath,omitempty"`
	// Endpoint is the URL of the blob service. Empty means the public Azure
	// endpoint of the account. Set it for Azurite and sovereign clouds.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https')",message="endpoint must be a valid http or https URL"
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
	// Auth selects how consumers authenticate. An absent block means
	// workload identity through the ServiceAccount chain.
	// +kubebuilder:default={type: workloadIdentity}
	// +optional
	Auth AzureBlobStorageAuth `json:"auth,omitempty"`
}

// ObjectStorageConfigSpec describes a bucket and how consumers authenticate
// against it. It is a discriminated union: type selects exactly one of the
// s3, gcs, and azureBlob blocks.
// +kubebuilder:validation:XValidation:rule="(self.type == 'S3') == has(self.s3) && (self.type == 'GCS') == has(self.gcs) && (self.type == 'AzureBlob') == has(self.azureBlob)",message="exactly the block matching spec.type must be set"
type ObjectStorageConfigSpec struct {
	// Type selects the storage API of the bucket.
	Type ObjectStorageType `json:"type"`
	// S3 describes an S3 or S3-compatible bucket. Required when type is S3,
	// forbidden otherwise.
	// +optional
	S3 *S3Storage `json:"s3,omitempty"`
	// GCS describes a Google Cloud Storage bucket. Required when type is
	// GCS, forbidden otherwise.
	// +optional
	GCS *GCSStorage `json:"gcs,omitempty"`
	// AzureBlob describes an Azure Blob Storage container. Required when
	// type is AzureBlob, forbidden otherwise.
	// +optional
	AzureBlob *AzureBlobStorage `json:"azureBlob,omitempty"`
}

// ObjectStorageConfigStatus is the observed validation state of the contract.
type ObjectStorageConfigStatus struct {
	// ObservedGeneration is the last generation reconciled by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the current validation state; the Ready condition
	// carries the reasons Healthy and MissingSecret.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// ObjectStorageConfig is the contract CRD that describes a bucket — for
// backups or document storage — and how consumers authenticate against it:
// workload identity on the consumer's ServiceAccount, or static credentials
// in a Secret.
type ObjectStorageConfig struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ObjectStorageConfig
	// +required
	Spec ObjectStorageConfigSpec `json:"spec"`

	// status defines the observed state of ObjectStorageConfig
	// +optional
	Status ObjectStorageConfigStatus `json:"status,omitzero"`
}

// GetStatusConditions returns a pointer to the status conditions. The
// component framework stages conditions on the resource through it.
func (in *ObjectStorageConfig) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// GetKind returns the CRD kind. The component framework uses it for event and
// metric recording.
func (in *ObjectStorageConfig) GetKind() string { return "ObjectStorageConfig" }

// SetObservedGeneration records the last reconciled generation in status.
func (in *ObjectStorageConfig) SetObservedGeneration(generation int64) {
	in.Status.ObservedGeneration = generation
}

// CredentialsSecret returns the name, namespace, and keys of the static
// credentials Secret of the active storage block, or nil when the contract
// uses workload identity. The returned keys are the Secret keys that must
// exist, in a stable order.
func (in *ObjectStorageConfig) CredentialsSecret() *ObjectStorageCredentialsSecret {
	switch {
	case in.Spec.S3 != nil && in.Spec.S3.Auth.Credentials != nil:
		ref := in.Spec.S3.Auth.Credentials.SecretRef
		return &ObjectStorageCredentialsSecret{
			Name:      ref.Name,
			Namespace: ref.Namespace,
			Keys:      []string{ref.AccessKeyIDKey, ref.SecretAccessKeyKey},
		}
	case in.Spec.GCS != nil && in.Spec.GCS.Auth.Credentials != nil:
		ref := in.Spec.GCS.Auth.Credentials.SecretRef
		return &ObjectStorageCredentialsSecret{
			Name:      ref.Name,
			Namespace: ref.Namespace,
			Keys:      []string{ref.Key},
		}
	case in.Spec.AzureBlob != nil && in.Spec.AzureBlob.Auth.Credentials != nil:
		ref := in.Spec.AzureBlob.Auth.Credentials.SecretRef
		return &ObjectStorageCredentialsSecret{
			Name:      ref.Name,
			Namespace: ref.Namespace,
			Keys:      []string{ref.Key},
		}
	}

	return nil
}

// ObjectStorageCredentialsSecret is the resolved location of a contract's
// static credentials: the Secret and the keys that must exist in it.
type ObjectStorageCredentialsSecret struct {
	// Name of the Secret.
	Name string
	// Namespace of the Secret.
	Namespace string
	// Keys that must exist in the Secret.
	Keys []string
}

// +kubebuilder:object:root=true

// ObjectStorageConfigList contains a list of ObjectStorageConfig
type ObjectStorageConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ObjectStorageConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ObjectStorageConfig{}, &ObjectStorageConfigList{})
}
