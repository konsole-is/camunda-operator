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
// to (keys username and password). The Secret lives in the namespace of the
// CR.
type CredentialsSpec struct {
	// SecretName is the name of the credentials Secret. Each controller
	// documents its kind-specific default derived from the CR name.
	// +optional
	SecretName string `json:"secretName,omitempty"`
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
	// ServerRef names the DatabaseServerConfig of this namespace describing
	// the server to create the database in.
	// +kubebuilder:validation:MinLength=1
	ServerRef string `json:"serverRef"`
	// DatabaseName is the name of the logical database to create, a valid
	// PostgreSQL identifier. It must be unique per server, and the server is
	// the PostgreSQL instance that the contract reaches, not the contract:
	// the controller rejects a Database whose databaseName collides with a
	// Database of any namespace on the same instance.
	// +kubebuilder:validation:Pattern=`^[a-z_][a-z0-9_]{0,62}$`
	DatabaseName string `json:"databaseName"`
	// ApplicationCredentials configures the application credentials Secret,
	// always created. The Secret name defaults to <CR name>-credentials.
	// +optional
	ApplicationCredentials *CredentialsSpec `json:"applicationCredentials,omitempty"`
	// BackupCredentials configures the backup credentials Secret, created
	// unless disabled. The Secret name defaults to
	// <CR name>-backup-credentials.
	// +optional
	BackupCredentials *BackupCredentialsSpec `json:"backupCredentials,omitempty"`
	// DatabaseConfig names the DatabaseConfig the operator creates in the
	// namespace of this Database. Defaults to the CR name.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=253
	// +optional
	DatabaseConfig string `json:"databaseConfig,omitempty"`
	// SecondaryStorageConfig, when set, makes the operator also create a
	// SecondaryStorageConfig of type rdbms with this name in the namespace of
	// this Database, wired to the DatabaseConfig. Omit it for databases not
	// used as Camunda secondary storage (Keycloak, Identity, Web Modeler).
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
	// CollisionKey is the logical database that this Database names: the
	// system identifier of the server and the database name. Every claimant
	// records it, so the field says what a Database asks for and not what it
	// owns. One logical database on one server has one owner across all
	// namespaces, and the first Database to claim it owns it. The Ready
	// condition says whether this Database is that owner. The operator never
	// clears the field, so an owner whose server or contract is gone keeps
	// the logical database. Delete that Database to release the name.
	// +optional
	CollisionKey string `json:"collisionKey,omitempty"`
	// Conditions represent the current state. Ready carries a pre-check
	// reason (InvalidReference, MissingSecret, ServerIdentityUnknown,
	// ConnectionFailed), or it takes the status and the reason of the
	// BindingsReady component condition, which also appears here.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ReasonServerIdentityUnknown means that the DatabaseServerConfig of the
// Database has not published status.systemIdentifier yet. Until it does, the
// operator cannot tell which PostgreSQL instance the contract reaches, so it
// cannot apply the uniqueness rule of the logical database name.
const ReasonServerIdentityUnknown = "ServerIdentityUnknown"

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Database bootstraps a logical database and its users on an existing
// PostgreSQL server over plain SQL and publishes the result as a
// DatabaseConfig — and optionally a SecondaryStorageConfig — in its own
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

// GetStatusConditions returns a pointer to the status conditions. The
// component framework stages per-component conditions on the resource through
// it.
func (in *Database) GetStatusConditions() *[]metav1.Condition { return &in.Status.Conditions }

// GetKind returns the CRD kind. The component framework derives its
// per-component SSA field managers (Database/<component>) from it.
func (in *Database) GetKind() string { return "Database" }

// SetObservedGeneration records the last reconciled generation in status.
func (in *Database) SetObservedGeneration(generation int64) {
	in.Status.ObservedGeneration = generation
}

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
