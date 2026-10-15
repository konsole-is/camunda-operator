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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// Sections returns the core PluginSections, in the order of the create flow.
func Sections() []pluginsv1alpha1.PluginSection {
	cluster := []pluginsv1alpha1.ClaimKind{pluginsv1alpha1.ClaimKindCamundaClusterClaim}
	plane := []pluginsv1alpha1.ClaimKind{pluginsv1alpha1.ClaimKindCamundaManagementClusterClaim}
	both := []pluginsv1alpha1.ClaimKind{
		pluginsv1alpha1.ClaimKindCamundaClusterClaim,
		pluginsv1alpha1.ClaimKindCamundaManagementClusterClaim,
	}

	// Order steps by 10, so that a plugin section can go between two core
	// sections.
	return []pluginsv1alpha1.PluginSection{
		section(SectionCloudIdentity, pluginsv1alpha1.PluginSectionSpec{
			Label:            "Cloud identity",
			Description:      "The cloud identity that the pods of the cluster run as.",
			Order:            10,
			EmptyOptionLabel: "No cloud identity",
			Slot:             pluginsv1alpha1.SlotWorkloadIdentity,
			AppliesTo:        cluster,
			Outputs: []pluginsv1alpha1.OutputDecl{
				{Key: OutputPrincipal, Description: "The cloud identity, for example the ARN of an AWS IAM role."},
				{
					Key:         OutputServiceAccountAnnotations,
					Description: "The annotations, as a JSON object, that bind the ServiceAccount of the cluster to the cloud identity.",
				},
			},
		}),
		section(SectionEncryptionKey, pluginsv1alpha1.PluginSectionSpec{
			Label:            "Encryption key",
			Description:      "The key that other sections use to encrypt volumes and buckets.",
			Order:            20,
			EmptyOptionLabel: "No key",
			AppliesTo:        cluster,
			Outputs: []pluginsv1alpha1.OutputDecl{
				{Key: OutputKeyRef, Description: "The reference of the key, for example the ARN of an AWS KMS key."},
				{Key: OutputFormat, Description: "The format of keyRef, for example aws-kms-arn."},
			},
		}),
		section(SectionVolumeEncryption, pluginsv1alpha1.PluginSectionSpec{
			Label:            "Volume encryption",
			Description:      "How the volumes of the cluster are encrypted.",
			Order:            30,
			EmptyOptionLabel: "Unencrypted volumes",
			Slot:             pluginsv1alpha1.SlotVolumeEncryption,
			AppliesTo:        cluster,
			Outputs: []pluginsv1alpha1.OutputDecl{
				{Key: OutputStorageClassName, Description: "The StorageClass of the Zeebe volumes."},
			},
		}),
		section(SectionBackupStorage, pluginsv1alpha1.PluginSectionSpec{
			Label:            "Backup storage",
			Description:      "Where the cluster keeps its backups.",
			Order:            40,
			EmptyOptionLabel: "No backups",
			Slot:             pluginsv1alpha1.SlotBackupStorage,
			AppliesTo:        cluster,
			Outputs: []pluginsv1alpha1.OutputDecl{
				{
					Key:         OutputObjectStorageConfig,
					Description: "The name of the ObjectStorageConfig that the cluster writes its backups to.",
				},
			},
		}),
		section(SectionSecondaryStorage, pluginsv1alpha1.PluginSectionSpec{
			Label:       "Secondary storage",
			Description: "The Elasticsearch or PostgreSQL storage of the cluster.",
			Order:       50,
			Slot:        pluginsv1alpha1.SlotSecondaryStorage,
			AppliesTo:   cluster,
			Outputs: []pluginsv1alpha1.OutputDecl{
				{
					Key:         OutputSecondaryStorageConfig,
					Description: "The name of the SecondaryStorageConfig that the cluster reads.",
				},
			},
		}),
		section(SectionAccess, pluginsv1alpha1.PluginSectionSpec{
			Label:            "Access",
			Description:      "How users and clients reach the cluster or the management plane from outside.",
			Order:            60,
			EmptyOptionLabel: "No access from outside",
			Slot:             pluginsv1alpha1.SlotExternalAccess,
			AppliesTo:        both,
			Outputs: []pluginsv1alpha1.OutputDecl{
				{Key: OutputExternalURL, Description: "The URL of the web applications and the REST API of a cluster."},
				{Key: OutputGRPCHost, Description: "The host of the gRPC API of a cluster."},
				{Key: OutputOptimizeURL, Description: "The URL of Optimize of a cluster."},
				{Key: OutputManagementHost, Description: "The host of a management plane."},
			},
		}),
		section(SectionConnectorsPermissions, pluginsv1alpha1.PluginSectionSpec{
			Label:            "Connectors permissions",
			Description:      "The cloud permissions of the Connectors, apart from the cloud identity of the cluster.",
			Order:            70,
			EmptyOptionLabel: "No cloud permissions",
			Slot:             pluginsv1alpha1.SlotConnectorsIdentity,
			AppliesTo:        cluster,
			Outputs: []pluginsv1alpha1.OutputDecl{
				{Key: OutputPrincipal, Description: "The cloud identity of the Connectors."},
				{
					Key:         OutputServiceAccountAnnotations,
					Description: "The annotations, as a JSON object, that bind the ServiceAccount of the Connectors to their cloud identity.",
				},
			},
		}),
		section(SectionManagementDatabase, pluginsv1alpha1.PluginSectionSpec{
			Label:       "Databases",
			Description: "The databases of Keycloak, Management Identity, and Web Modeler.",
			Order:       80,
			Slot:        pluginsv1alpha1.SlotManagementDatabases,
			AppliesTo:   plane,
			Outputs: []pluginsv1alpha1.OutputDecl{
				{Key: OutputKeycloakDatabaseConfig, Description: "The name of the DatabaseConfig of Keycloak."},
				{
					Key:         OutputIdentityDatabaseConfig,
					Description: "The name of the DatabaseConfig of Management Identity.",
				},
				{Key: OutputWebModelerDatabaseConfig, Description: "The name of the DatabaseConfig of Web Modeler."},
			},
		}),
	}
}

func section(name string, spec pluginsv1alpha1.PluginSectionSpec) pluginsv1alpha1.PluginSection {
	spec.Selection = pluginsv1alpha1.SelectionOne

	return pluginsv1alpha1.PluginSection{
		TypeMeta: metav1.TypeMeta{
			APIVersion: pluginsv1alpha1.GroupVersion.String(),
			Kind:       "PluginSection",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       spec,
	}
}
