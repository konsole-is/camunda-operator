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

// ObjectStorageProvider identifies the cloud provider hosting a bucket.
// +kubebuilder:validation:Enum=aws;gcp;azure
type ObjectStorageProvider string

// ObjectStorageProviderAWS, ObjectStorageProviderGCP, and
// ObjectStorageProviderAzure are the supported bucket providers.
const (
	ObjectStorageProviderAWS   ObjectStorageProvider = "aws"
	ObjectStorageProviderGCP   ObjectStorageProvider = "gcp"
	ObjectStorageProviderAzure ObjectStorageProvider = "azure"
)

// ObjectStorageType identifies the storage API of a bucket.
// +kubebuilder:validation:Enum=S3;GCS;AzureBlob
type ObjectStorageType string

// ObjectStorageTypeS3, ObjectStorageTypeGCS, and ObjectStorageTypeAzureBlob
// are the supported storage APIs; each pairs with exactly one provider.
const (
	ObjectStorageTypeS3        ObjectStorageType = "S3"
	ObjectStorageTypeGCS       ObjectStorageType = "GCS"
	ObjectStorageTypeAzureBlob ObjectStorageType = "AzureBlob"
)

// ObjectStorageConfigSpec describes a cloud bucket and the workload identity
// trusted to access it. Access is granted through workload identity, so the
// contract references no Secrets.
// +kubebuilder:validation:XValidation:rule="(self.provider == 'aws' && self.type == 'S3') || (self.provider == 'gcp' && self.type == 'GCS') || (self.provider == 'azure' && self.type == 'AzureBlob')",message="spec.type must match spec.provider: aws pairs with S3, gcp with GCS, azure with AzureBlob"
type ObjectStorageConfigSpec struct {
	// Provider is the cloud provider hosting the bucket; it determines the
	// workload-identity mechanism.
	Provider ObjectStorageProvider `json:"provider"`
	// Type is the storage API of the bucket; it must match the provider.
	Type ObjectStorageType `json:"type"`
	// BucketID is the provider-specific unique identifier of the bucket, for
	// example an ARN on AWS.
	// +kubebuilder:validation:MinLength=1
	BucketID string `json:"bucketId"`
	// BucketName is the bucket name as used by storage client SDKs.
	// +kubebuilder:validation:MinLength=1
	BucketName string `json:"bucketName"`
	// BasePath is the key prefix under which consumers write objects. Empty
	// means the bucket root.
	// +optional
	BasePath string `json:"basePath,omitempty"`
	// AccountID is the workload identity the bucket trusts: an IAM role ARN
	// (aws), a service account email (gcp), or a managed identity client ID
	// (azure).
	// +kubebuilder:validation:MinLength=1
	AccountID string `json:"accountId"`
}

// ObjectStorageConfigStatus is the observed validation state of the contract.
type ObjectStorageConfigStatus struct {
	// ObservedGeneration is the last generation reconciled by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the current validation state; the Ready condition
	// carries the reason Healthy.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// ObjectStorageConfig is the contract CRD that describes a cloud bucket — for
// backups or document storage — and the workload identity trusted to access
// it.
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

// GetConditions returns the resource's status conditions.
func (in *ObjectStorageConfig) GetConditions() []metav1.Condition { return in.Status.Conditions }

// GetObservedGeneration returns the last reconciled generation recorded in status.
func (in *ObjectStorageConfig) GetObservedGeneration() int64 { return in.Status.ObservedGeneration }

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
