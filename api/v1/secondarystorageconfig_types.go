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

// SecondaryStorageType identifies which secondary storage backend a contract
// describes.
// +kubebuilder:validation:Enum=elasticsearch;rdbms
type SecondaryStorageType string

const (
	// SecondaryStorageTypeElasticsearch selects an Elasticsearch backend.
	SecondaryStorageTypeElasticsearch SecondaryStorageType = "elasticsearch"
	// SecondaryStorageTypeRDBMS selects a relational database backend.
	SecondaryStorageTypeRDBMS SecondaryStorageType = "rdbms"
)

// ElasticsearchStorage holds Elasticsearch connection details.
// +kubebuilder:validation:XValidation:rule="!has(self.caSecretRef) || url(self.endpoint).getScheme() == 'https'",message="caSecretRef requires an https endpoint"
type ElasticsearchStorage struct {
	// Endpoint is the HTTP(S) endpoint of the Elasticsearch cluster.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https')",message="endpoint must be a valid http or https URL"
	Endpoint string `json:"endpoint"`
	// CredentialsSecretRef names a basic-auth user with read/write access to
	// the Camunda indices.
	CredentialsSecretRef CredentialsSecretRef `json:"credentialsSecretRef"`
	// CASecretRef names the CA bundle consumers use to verify the endpoint's
	// TLS certificate. Set it when the endpoint serves a certificate not
	// signed by a well-known CA, such as the self-signed certificate of an
	// ECK-managed cluster. Omit it for publicly trusted endpoints; it is only
	// valid with an https endpoint.
	// +optional
	CASecretRef *SecretKeyRef `json:"caSecretRef,omitempty"`
}

// RDBMSStorage holds relational database backend details.
type RDBMSStorage struct {
	// DatabaseConfigRef names the DatabaseConfig, in this contract's own
	// namespace, describing the logical database to use.
	// +kubebuilder:validation:MinLength=1
	DatabaseConfigRef string `json:"databaseConfigRef"`
}

// SecondaryStorageConfigSpec tells an orchestration cluster where its
// secondary storage lives and how to authenticate against it.
// +kubebuilder:validation:XValidation:rule="(self.type == 'elasticsearch') == has(self.elasticsearch) && (self.type == 'rdbms') == has(self.rdbms)",message="exactly the block matching spec.type must be set"
type SecondaryStorageConfigSpec struct {
	// Type selects which secondary storage backend this contract describes.
	Type SecondaryStorageType `json:"type"`
	// Elasticsearch connection details. Required when type is elasticsearch,
	// forbidden otherwise.
	// +optional
	Elasticsearch *ElasticsearchStorage `json:"elasticsearch,omitempty"`
	// RDBMS backend details. Required when type is rdbms, forbidden otherwise.
	// +optional
	RDBMS *RDBMSStorage `json:"rdbms,omitempty"`
}

// SecondaryStorageConfigStatus is the observed validation state of the contract.
type SecondaryStorageConfigStatus struct {
	// ObservedGeneration is the last generation reconciled by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the current validation state; the Ready condition
	// carries reasons Healthy, MissingSecret, or InvalidReference.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// SecondaryStorageConfig is the namespaced contract CRD that tells an
// orchestration cluster where its secondary storage lives — an Elasticsearch
// cluster or a relational database — and how to authenticate against it.
// Consumers resolve references to it by name in their own namespace.
type SecondaryStorageConfig struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of SecondaryStorageConfig
	// +required
	Spec SecondaryStorageConfigSpec `json:"spec"`

	// status defines the observed state of SecondaryStorageConfig
	// +optional
	Status SecondaryStorageConfigStatus `json:"status,omitzero"`
}

// GetConditions returns the resource's status conditions.
func (in *SecondaryStorageConfig) GetConditions() []metav1.Condition { return in.Status.Conditions }

// GetObservedGeneration returns the last reconciled generation recorded in status.
func (in *SecondaryStorageConfig) GetObservedGeneration() int64 { return in.Status.ObservedGeneration }

// +kubebuilder:object:root=true

// SecondaryStorageConfigList contains a list of SecondaryStorageConfig
type SecondaryStorageConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []SecondaryStorageConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SecondaryStorageConfig{}, &SecondaryStorageConfigList{})
}
