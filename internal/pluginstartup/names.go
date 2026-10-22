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

package pluginstartup

// FieldManager is the server-side apply field manager of the core sections
// and the built-in registrations.
const FieldManager = "camunda-operator-builtins"

// ControllerName is the controller name of every built-in registration.
const ControllerName = "camunda-operator"

// The names of the core PluginSections.
const (
	SectionCloudIdentity         = "cloud-identity"
	SectionEncryptionKey         = "encryption-key"
	SectionVolumeEncryption      = "volume-encryption"
	SectionBackupStorage         = "backup-storage"
	SectionSecondaryStorage      = "secondary-storage"
	SectionAccess                = "access"
	SectionConnectorsPermissions = "connectors-permissions"
	SectionManagementDatabase    = "management-database"
)

// The names of the built-in PluginRegistrations.
const (
	RegistrationElasticsearch            = "elasticsearch"
	RegistrationDatabaseServer           = "database-server"
	RegistrationManagementDatabaseServer = "management-database-server"
	RegistrationExistingSecondaryStorage = "existing-secondary-storage"
	RegistrationExistingObjectStorage    = "existing-object-storage"
	RegistrationExistingDatabaseConfigs  = "existing-database-configs"
)

// The suffixes that a built-in registration adds to the claim name to name
// its backing resource (CL4).
const (
	BackingSuffixElasticsearch = "-es"
)

// The output keys of the core sections. A registration that serves a
// section writes the outputs of the section.
const (
	OutputPrincipal                 = "principal"
	OutputServiceAccountAnnotations = "serviceAccountAnnotations"
	OutputKeyRef                    = "keyRef"
	OutputFormat                    = "format"
	OutputStorageClassName          = "storageClassName"
	OutputObjectStorageConfig       = "objectStorageConfig"
	OutputSecondaryStorageConfig    = "secondaryStorageConfig"
	OutputExternalURL               = "externalUrl"
	OutputGRPCHost                  = "grpcHost"
	OutputOptimizeURL               = "optimizeUrl"
	OutputManagementHost            = "managementHost"
	OutputKeycloakDatabaseConfig    = "keycloakDatabaseConfig"
	OutputIdentityDatabaseConfig    = "identityDatabaseConfig"
	OutputWebModelerDatabaseConfig  = "webModelerDatabaseConfig"
)
