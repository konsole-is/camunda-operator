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

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// Registrations returns the built-in PluginRegistrations. Each one names
// operatorVersion as the version of its controller and serviceAccount as the
// ServiceAccount of the operator. None sets operatorVersions, because the
// operator serves them itself.
func Registrations(
	operatorVersion string,
	serviceAccount pluginsv1alpha1.ServiceAccountRef,
) []pluginsv1alpha1.PluginRegistration {
	storageInputs := []pluginsv1alpha1.InputDecl{
		{Name: OutputStorageClassName, From: SectionVolumeEncryption + "/" + OutputStorageClassName},
		{Name: OutputObjectStorageConfig, From: SectionBackupStorage + "/" + OutputObjectStorageConfig},
		{Name: OutputServiceAccountAnnotations, From: SectionCloudIdentity + "/" + OutputServiceAccountAnnotations},
	}
	secondaryStorageOutputs := []pluginsv1alpha1.OutputDecl{
		{
			Key:         OutputSecondaryStorageConfig,
			Description: "The name of the SecondaryStorageConfig that the cluster reads.",
		},
	}
	databaseOutputs := []pluginsv1alpha1.OutputDecl{
		{Key: OutputKeycloakDatabaseConfig, Description: "The name of the DatabaseConfig of Keycloak."},
		{Key: OutputIdentityDatabaseConfig, Description: "The name of the DatabaseConfig of Management Identity."},
		{Key: OutputWebModelerDatabaseConfig, Description: "The name of the DatabaseConfig of Web Modeler."},
	}

	specs := []struct {
		name string
		spec pluginsv1alpha1.PluginRegistrationSpec
	}{
		{RegistrationElasticsearch, pluginsv1alpha1.PluginRegistrationSpec{
			Sections:      []string{SectionSecondaryStorage},
			Label:         "New Elasticsearch cluster",
			Description:   "Runs an Elasticsearch cluster for the cluster, from an Elasticsearch cluster preset.",
			Mode:          pluginsv1alpha1.ModeDefault,
			Parameters:    schema(elasticsearchParameters),
			ClaimEditable: []string{"overrides"},
			Inputs:        storageInputs,
			Outputs:       secondaryStorageOutputs,
		}},
		{RegistrationDatabaseServer, pluginsv1alpha1.PluginRegistrationSpec{
			Sections:      []string{SectionSecondaryStorage},
			Label:         "New PostgreSQL server",
			Description:   "Runs a PostgreSQL server and a database for the cluster, from a database server preset.",
			Mode:          pluginsv1alpha1.ModeOptional,
			Parameters:    schema(databaseServerParameters),
			ClaimEditable: []string{"overrides"},
			Inputs:        storageInputs,
			Outputs:       secondaryStorageOutputs,
		}},
		{RegistrationExistingSecondaryStorage, pluginsv1alpha1.PluginRegistrationSpec{
			Sections:    []string{SectionSecondaryStorage},
			Label:       "Existing secondary storage",
			Description: "Uses a SecondaryStorageConfig that is already in the namespace.",
			Mode:        pluginsv1alpha1.ModeOptional,
			Parameters:  schema(existingSecondaryStorageParameters),
			Outputs:     secondaryStorageOutputs,
		}},
		{RegistrationExistingObjectStorage, pluginsv1alpha1.PluginRegistrationSpec{
			Sections:    []string{SectionBackupStorage},
			Label:       "Existing bucket",
			Description: "Uses an ObjectStorageConfig that is already in the namespace.",
			Mode:        pluginsv1alpha1.ModeOptional,
			Parameters:  schema(existingObjectStorageParameters),
			Outputs: []pluginsv1alpha1.OutputDecl{
				{
					Key:         OutputObjectStorageConfig,
					Description: "The name of the ObjectStorageConfig that the cluster writes its backups to.",
				},
			},
		}},
		{RegistrationManagementDatabaseServer, pluginsv1alpha1.PluginRegistrationSpec{
			Sections:    []string{SectionManagementDatabase},
			Label:       "New PostgreSQL server",
			Description: "Runs a PostgreSQL server with one database for each component of the management plane.",
			Mode:        pluginsv1alpha1.ModeDefault,
			Parameters:  schema(managementDatabaseServerParameters),
			Inputs: []pluginsv1alpha1.InputDecl{
				{Name: OutputStorageClassName, From: SectionVolumeEncryption + "/" + OutputStorageClassName},
			},
			Outputs: databaseOutputs,
		}},
		{RegistrationExistingDatabaseConfigs, pluginsv1alpha1.PluginRegistrationSpec{
			Sections:    []string{SectionManagementDatabase},
			Label:       "Existing databases",
			Description: "Uses DatabaseConfigs that are already in the namespace, one for each component.",
			Mode:        pluginsv1alpha1.ModeOptional,
			Parameters:  schema(existingDatabaseConfigsParameters),
			Outputs:     databaseOutputs,
		}},
	}

	registrations := make([]pluginsv1alpha1.PluginRegistration, 0, len(specs))
	for _, s := range specs {
		spec := s.spec
		spec.Controller = pluginsv1alpha1.ControllerInfo{Name: ControllerName, Version: operatorVersion}
		spec.ServiceAccount = serviceAccount
		spec.DeletionPolicies = []pluginsv1alpha1.DeletionPolicy{
			pluginsv1alpha1.DeletionPolicyDelete,
			pluginsv1alpha1.DeletionPolicyOrphan,
		}

		registrations = append(registrations, pluginsv1alpha1.PluginRegistration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: pluginsv1alpha1.GroupVersion.String(),
				Kind:       "PluginRegistration",
			},
			ObjectMeta: metav1.ObjectMeta{Name: s.name},
			Spec:       spec,
		})
	}

	return registrations
}

func schema(raw string) *apiextensionsv1.JSON {
	return &apiextensionsv1.JSON{Raw: []byte(raw)}
}
