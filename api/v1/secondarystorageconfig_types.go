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

// SecondaryStorageType identifies the secondary storage backend that a
// contract describes.
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
	// CredentialsSecretRef names a basic-auth user with read and write
	// access to the Camunda indices.
	CredentialsSecretRef LocalCredentialsSecretRef `json:"credentialsSecretRef"`
	// CASecretRef names the CA bundle that consumers use to verify the TLS
	// certificate of the endpoint. Set it when a well-known CA did not sign
	// the certificate of the endpoint. An example is the self-signed
	// certificate of a cluster that ECK runs. Omit it for an endpoint with a
	// publicly trusted certificate. It is valid only with an https endpoint.
	// +optional
	CASecretRef *LocalSecretKeyRef `json:"caSecretRef,omitempty"`
	// SnapshotRepository names the snapshot repository in this Elasticsearch
	// cluster that backups write to. An ElasticsearchCluster with a
	// snapshotStorageRef registers the repository and sets this field in the
	// contract that it publishes. For an Elasticsearch cluster that this
	// operator does not run, register the repository yourself and then set
	// this field. A cluster that takes backups needs it. Without it, the
	// backup components have no place to write. The name is part of a URL
	// path of the Elasticsearch API, so it permits only a small set of
	// characters.
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`
	// +kubebuilder:validation:MaxLength=253
	// +optional
	SnapshotRepository string `json:"snapshotRepository,omitempty"`
	// NodeCount is the number of data nodes of the Elasticsearch cluster. An
	// ElasticsearchCluster sets it in the contract that it publishes. For an
	// Elasticsearch cluster that this operator does not run, set it yourself.
	// A consumer that sets no index replica count of its own gets 0 replicas
	// on one node and 1 replica on two or more nodes. Without a node count,
	// the consumer keeps the default of the Camunda application.
	// +kubebuilder:validation:Minimum=1
	// +optional
	NodeCount *int32 `json:"nodeCount,omitempty"`
}

// IndexReplicas returns the replica count of each index that a consumer
// creates on this backend. requested is the setting of the consumer, and it
// wins when it is set. The result is nil when neither requested nor NodeCount
// is set.
func (s *ElasticsearchStorage) IndexReplicas(requested *int32) *int32 {
	if requested != nil {
		return new(*requested)
	}
	if s.NodeCount == nil {
		return nil
	}
	return new(min(1, *s.NodeCount-1))
}

// ReplicasExceedNodes reports whether NodeCount is too small to place
// replicas. It is false when NodeCount is not set.
func (s *ElasticsearchStorage) ReplicasExceedNodes(replicas int32) bool {
	// Elasticsearch never puts a replica on the node of its primary.
	return s.NodeCount != nil && replicas > *s.NodeCount-1
}

// RDBMSStorage holds relational database backend details.
type RDBMSStorage struct {
	// DatabaseConfigRef names the DatabaseConfig of the logical database to
	// use, in the namespace of this contract.
	// +kubebuilder:validation:MinLength=1
	DatabaseConfigRef string `json:"databaseConfigRef"`
}

// SecondaryStorageConfigSpec tells an orchestration cluster where its
// secondary storage is and how to authenticate against it.
// +kubebuilder:validation:XValidation:rule="(self.type == 'elasticsearch') == has(self.elasticsearch) && (self.type == 'rdbms') == has(self.rdbms)",message="exactly the block matching spec.type must be set"
type SecondaryStorageConfigSpec struct {
	// Type selects the secondary storage backend that this contract
	// describes.
	Type SecondaryStorageType `json:"type"`
	// Elasticsearch holds the Elasticsearch connection details. Required
	// when type is elasticsearch. Forbidden with other types.
	// +optional
	Elasticsearch *ElasticsearchStorage `json:"elasticsearch,omitempty"`
	// RDBMS holds the relational database details. Required when type is
	// rdbms. Forbidden with other types.
	// +optional
	RDBMS *RDBMSStorage `json:"rdbms,omitempty"`
}

// SecondaryStorageConfigStatus is the observed validation state of the contract.
type SecondaryStorageConfigStatus struct {
	// ObservedGeneration is the last generation that the operator processed.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the current validation state. The Ready condition
	// has the reasons Healthy, MissingSecret, or InvalidReference.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// SecondaryStorageConfig is the namespaced contract CRD that tells an
// orchestration cluster where its secondary storage is and how to
// authenticate against it. The storage is an Elasticsearch cluster or a
// relational database. Consumers find it by name in their own namespace.
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

// GetStatusConditions returns a pointer to the status conditions. The
// component framework stages conditions on the resource through it.
func (in *SecondaryStorageConfig) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// GetKind returns the CRD kind. The component framework uses it for event and
// metric recording.
func (in *SecondaryStorageConfig) GetKind() string { return "SecondaryStorageConfig" }

// SetObservedGeneration records the last reconciled generation in status.
func (in *SecondaryStorageConfig) SetObservedGeneration(generation int64) {
	in.Status.ObservedGeneration = generation
}

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
