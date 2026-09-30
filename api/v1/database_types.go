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

// CredentialsSpec names the Secret to which the operator writes generated
// credentials, with the keys username and password. The Secret is in the
// namespace of the CR.
type CredentialsSpec struct {
	// SecretName is the name of the credentials Secret. The default comes
	// from the CR name, as the field that uses this type states.
	// +optional
	SecretName string `json:"secretName,omitempty"`
}

// BackupCredentialsSpec configures the backup credentials Secret. The
// operator creates it unless it is disabled.
type BackupCredentialsSpec struct {
	// Disabled stops the operator from creating the backup user and Secret.
	// Defaults to false.
	// +optional
	Disabled bool `json:"disabled,omitempty"`

	CredentialsSpec `json:",inline"`
}

// DatabaseSpec defines the desired state of Database.
type DatabaseSpec struct {
	// ServerRef names the DatabaseServerConfig, in this namespace, of the
	// server on which the operator creates the database.
	// +kubebuilder:validation:MinLength=1
	ServerRef string `json:"serverRef"`
	// DatabaseName is the name of the logical database to create, as a valid
	// PostgreSQL identifier. It must be unique on each PostgreSQL instance
	// that a contract reaches, not only on each contract. The operator
	// refuses a Database with the same databaseName as a Database of any
	// namespace on the same instance.
	// +kubebuilder:validation:Pattern=`^[a-z_][a-z0-9_]{0,62}$`
	DatabaseName string `json:"databaseName"`
	// ApplicationCredentials configures the application credentials Secret.
	// The operator always creates it. The Secret name defaults to
	// <CR name>-credentials.
	// +optional
	ApplicationCredentials *CredentialsSpec `json:"applicationCredentials,omitempty"`
	// BackupCredentials configures the backup credentials Secret. The
	// operator creates it unless it is disabled. The Secret name defaults to
	// <CR name>-backup-credentials.
	// +optional
	BackupCredentials *BackupCredentialsSpec `json:"backupCredentials,omitempty"`
	// DatabaseConfig names the DatabaseConfig that the operator creates in
	// the namespace of this Database. Defaults to the CR name.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=253
	// +optional
	DatabaseConfig string `json:"databaseConfig,omitempty"`
	// SecondaryStorageConfig, when set, makes the operator also create a
	// SecondaryStorageConfig of type rdbms with this name in the namespace of
	// this Database. It references the DatabaseConfig. Omit it for a database
	// that is not Camunda secondary storage (Keycloak, Identity, Web
	// Modeler).
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
	// CollisionKey is the logical database that this Database last resolved:
	// the system identifier of the server and the database name. Every
	// Database records it, also a Database that loses the name to another.
	// Thus the field shows what a Database asked for, not what it owns. The
	// operator sets it only after it reaches the server. If the spec names a
	// missing server, or a server that is not probed for the current spec,
	// the old key stays until that server answers.
	//
	// A Database whose Ready condition reports InvalidReference and names
	// another Database does not own the name that it shows here. The
	// operator never clears the field. Thus an owner whose server or
	// contract is gone keeps the logical database. Delete that Database to
	// release the name.
	// +optional
	CollisionKey string `json:"collisionKey,omitempty"`
	// Conditions represent the current state. Ready holds the reason of a
	// failed pre-check (InvalidReference, MissingSecret,
	// ServerIdentityUnknown, ConnectionFailed). Otherwise it takes the status
	// and the reason of the BindingsReady condition, which also appears here.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ReasonServerIdentityUnknown means that the DatabaseServerConfig of the
// Database did not publish status.systemIdentifier yet. Until then, the
// operator does not know which PostgreSQL instance the contract reaches. Thus
// it cannot apply the uniqueness rule of the logical database name.
const ReasonServerIdentityUnknown = "ServerIdentityUnknown"

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Database",type=string,JSONPath=`.spec.databaseName`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Database creates a logical database and its users on an existing
// PostgreSQL server with SQL. It publishes the result as a DatabaseConfig in
// its own namespace, and, when set, as a SecondaryStorageConfig. When you
// delete a Database, Kubernetes deletes the published contracts and Secrets.
// The logical database and the SQL users stay on the server.
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
