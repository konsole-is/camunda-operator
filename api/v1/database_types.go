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

// CredentialsSpec names the Secret a controller writes generated credentials
// to (keys username and password).
type CredentialsSpec struct {
	// SecretName is the name of the credentials Secret. Each controller
	// documents its kind-specific default derived from the CR name.
	// +optional
	SecretName string `json:"secretName,omitempty"`
	// SecretNamespace is the namespace for the credentials Secret. Defaults
	// to the CR's target namespace.
	// +optional
	SecretNamespace string `json:"secretNamespace,omitempty"`
}

// BackupCredentialsSpec configures the backup credentials Secret, which is
// created unless disabled.
type BackupCredentialsSpec struct {
	// Disabled skips creating the backup user and Secret. Defaults to false.
	// +optional
	Disabled bool `json:"disabled,omitempty"`

	CredentialsSpec `json:",inline"`
}

// DatabaseSpec defines the desired state of Database.
type DatabaseSpec struct {
	// ServerRef names the cluster-scoped DatabaseServerConfig describing the
	// server to create the database in.
	// +kubebuilder:validation:MinLength=1
	ServerRef string `json:"serverRef"`
	// DatabaseName is the name of the logical database to create, a valid
	// PostgreSQL identifier. It must be unique per server: the controller
	// rejects a Database whose serverRef and databaseName collide with an
	// existing one.
	// +kubebuilder:validation:Pattern=`^[a-z_][a-z0-9_]{0,62}$`
	DatabaseName string `json:"databaseName"`
	// TargetNamespace is the namespace where the created DatabaseConfig,
	// SecondaryStorageConfig, and credential Secrets are placed (each
	// Secret's namespace can be overridden per Secret). Set it to the
	// consuming cluster's namespace, since consumers resolve the bindings by
	// name in their own namespace; for that reason the field is required with
	// no default.
	// +kubebuilder:validation:MinLength=1
	TargetNamespace string `json:"targetNamespace"`
	// ApplicationCredentials configures the application credentials Secret,
	// always created. The Secret name defaults to <CR name>-credentials.
	// +optional
	ApplicationCredentials *CredentialsSpec `json:"applicationCredentials,omitempty"`
	// BackupCredentials configures the backup credentials Secret, created
	// unless disabled. The Secret name defaults to
	// <CR name>-backup-credentials.
	// +optional
	BackupCredentials *BackupCredentialsSpec `json:"backupCredentials,omitempty"`
	// DatabaseConfig names the DatabaseConfig the operator creates in
	// targetNamespace. Defaults to the CR name.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=253
	// +optional
	DatabaseConfig string `json:"databaseConfig,omitempty"`
	// SecondaryStorageConfig, when set, makes the operator also create a
	// SecondaryStorageConfig of type rdbms with this name in targetNamespace,
	// wired to the DatabaseConfig. Omit it for databases not used as Camunda
	// secondary storage (Keycloak, Identity, Web Modeler).
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=253
	// +optional
	SecondaryStorageConfig string `json:"secondaryStorageConfig,omitempty"`
}

// DatabaseStatus is the observed state of a Database.
type DatabaseStatus struct {
	// ObservedGeneration is the last generation reconciled by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the current state. The Ready condition carries
	// reasons Healthy, Progressing, InvalidReference, MissingSecret, or
	// ConnectionFailed, and the operator's per-component conditions
	// (BindingsReady) also appear here.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// Database bootstraps a logical database and its users on an existing
// PostgreSQL server over plain SQL and publishes the result as a
// DatabaseConfig — and optionally a SecondaryStorageConfig — in its target
// namespace. Deletion garbage-collects the published bindings and Secrets
// through owner references but never drops the logical database or the SQL
// users.
type Database struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Database
	// +required
	Spec DatabaseSpec `json:"spec"`

	// status defines the observed state of Database
	// +optional
	Status DatabaseStatus `json:"status,omitzero"`
}

// GetConditions returns the resource's status conditions.
func (in *Database) GetConditions() []metav1.Condition { return in.Status.Conditions }

// GetObservedGeneration returns the last reconciled generation recorded in status.
func (in *Database) GetObservedGeneration() int64 { return in.Status.ObservedGeneration }

// +kubebuilder:object:root=true

// DatabaseList contains a list of Database
type DatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Database `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Database{}, &DatabaseList{})
}
