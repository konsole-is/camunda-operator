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

// DatabaseConfigSpec describes one logical database: its server, name, and
// application credentials.
type DatabaseConfigSpec struct {
	// ServerRef names the DatabaseServerConfig describing the server hosting
	// this database.
	// +kubebuilder:validation:MinLength=1
	ServerRef string `json:"serverRef"`
	// DatabaseName is the name of the logical database on the server.
	// +kubebuilder:validation:MinLength=1
	DatabaseName string `json:"databaseName"`
	// CredentialsSecretRef names an application user with read/write access to
	// the database.
	CredentialsSecretRef CredentialsSecretRef `json:"credentialsSecretRef"`
	// BackupCredentialsSecretRef names a separate user with dump/restore
	// privileges, used by the backup and restore controllers.
	// +optional
	BackupCredentialsSecretRef *CredentialsSecretRef `json:"backupCredentialsSecretRef,omitempty"`
}

// DatabaseConfigStatus is the observed validation state of the contract.
type DatabaseConfigStatus struct {
	// ObservedGeneration is the last generation reconciled by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the current validation state; the Ready condition
	// carries reasons Healthy, InvalidReference, or MissingSecret.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DatabaseConfig is the namespaced contract CRD that describes one logical
// database — its server, name, and application credentials — for the
// controllers and components that connect to it. Consumers resolve references
// to it by name in their own namespace.
type DatabaseConfig struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of DatabaseConfig
	// +required
	Spec DatabaseConfigSpec `json:"spec"`

	// status defines the observed state of DatabaseConfig
	// +optional
	Status DatabaseConfigStatus `json:"status,omitzero"`
}

// GetConditions returns the resource's status conditions.
func (in *DatabaseConfig) GetConditions() []metav1.Condition { return in.Status.Conditions }

// GetObservedGeneration returns the last reconciled generation recorded in status.
func (in *DatabaseConfig) GetObservedGeneration() int64 { return in.Status.ObservedGeneration }

// +kubebuilder:object:root=true

// DatabaseConfigList contains a list of DatabaseConfig
type DatabaseConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []DatabaseConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DatabaseConfig{}, &DatabaseConfigList{})
}
