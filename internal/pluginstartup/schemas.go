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
	"fmt"
	"strconv"
	"strings"

	"github.com/konsole-is/camunda-operator/pkg/planedatabases"
)

// The parameter schemas of the built-in registrations, in OpenAPI v3.
// resources and scheduling keep unknown fields, because the adapter applies
// them to a kind whose own schema validates them.
const (
	elasticsearchParameters = `{
  "type": "object",
  "required": ["preset", "releaseRef"],
  "properties": {
    "preset": {"type": "string", "minLength": 1, "description": "The name of the ElasticsearchClusterPreset that the Elasticsearch cluster starts from."},
    "releaseRef": {"type": "string", "minLength": 1, "description": "The name of the CamundaRelease that gives the Elasticsearch version. The claim controller sets it to the release of the claim."},
    "overrides": {
      "type": "object",
      "description": "The values that replace the values of the preset.",
      "properties": {
        "replicas": {"type": "integer", "format": "int32", "minimum": 1, "description": "The number of Elasticsearch nodes."},
        "resources": {"type": "object", "x-kubernetes-preserve-unknown-fields": true, "description": "The CPU and memory of each node, as spec.resources of an ElasticsearchCluster."},
        "storageSize": {"type": "string", "description": "The volume size of each node, for example 50Gi. An override never lowers the live storage size."},
        "scheduling": {"type": "object", "x-kubernetes-preserve-unknown-fields": true, "description": "Where the nodes run, as spec.scheduling of an ElasticsearchCluster."},
        "podLabels": {"type": "object", "additionalProperties": {"type": "string"}, "description": "The labels to add to the Elasticsearch pods."}
      }
    }
  }
}`

	databaseServerParameters = `{
  "type": "object",
  "required": ["preset", "releaseRef"],
  "properties": {
    "preset": {"type": "string", "minLength": 1, "description": "The name of the DatabaseServerPreset that the PostgreSQL server starts from."},
    "releaseRef": {"type": "string", "minLength": 1, "description": "The name of the CamundaRelease that gives the PostgreSQL major version. The claim controller sets it to the release of the claim."},
    "overrides": {
      "type": "object",
      "description": "The values that replace the values of the preset.",
      "properties": {
        "instances": {"type": "integer", "format": "int32", "minimum": 1, "description": "The number of PostgreSQL instances."},
        "resources": {"type": "object", "x-kubernetes-preserve-unknown-fields": true, "description": "The CPU and memory of each instance, as spec.resources of a DatabaseServer."},
        "storageSize": {"type": "string", "description": "The volume size of each instance, for example 20Gi. The size only grows."},
        "scheduling": {"type": "object", "x-kubernetes-preserve-unknown-fields": true, "description": "Where the instances run, as spec.scheduling of a DatabaseServer."},
        "podLabels": {"type": "object", "additionalProperties": {"type": "string"}, "description": "The labels to add to the PostgreSQL pods."}
      }
    }
  }
}`

	existingSecondaryStorageParameters = `{
  "type": "object",
  "required": ["name"],
  "properties": {
    "name": {"type": "string", "minLength": 1, "description": "The name of a SecondaryStorageConfig in the namespace of the claim."}
  }
}`

	existingObjectStorageParameters = `{
  "type": "object",
  "required": ["name"],
  "properties": {
    "name": {"type": "string", "minLength": 1, "description": "The name of an ObjectStorageConfig in the namespace of the claim."}
  }
}`
)

// The parameter schemas of the management database registrations. They
// list the components of planedatabases.All.
var (
	managementDatabaseServerParameters = `{
  "type": "object",
  "required": ["preset"],
  "properties": {
    "preset": {"type": "string", "minLength": 1, "description": "The name of the DatabaseServerPreset that the PostgreSQL server starts from."},
    "version": {"type": "string", "pattern": "^\\d+$", "default": "` + ManagementDatabaseServerVersion + `", "description": "The PostgreSQL major version of the server, for example 17."},
    "databases": {
      "type": "array",
      "description": "The components that get a database. The management plane claim sets it.",
      "items": {"type": "string", "enum": ` + managementDatabaseEnum() + `}
    }
  }
}`

	// The adapter refuses two equal names. The schema cannot say it.
	existingDatabaseConfigsParameters = `{
  "type": "object",
  "properties": {
` + existingDatabaseConfigsProperties() + `
  }
}`
)

// managementDatabaseEnum returns the components of planedatabases.All as a
// JSON array.
func managementDatabaseEnum() string {
	quoted := make([]string, 0, len(planedatabases.All))
	for _, c := range planedatabases.Components() {
		quoted = append(quoted, strconv.Quote(string(c)))
	}

	return "[" + strings.Join(quoted, ", ") + "]"
}

// existingDatabaseConfigsProperties returns one JSON property for each
// component of planedatabases.All, which names its DatabaseConfig.
func existingDatabaseConfigsProperties() string {
	properties := make([]string, 0, len(planedatabases.All))
	for _, d := range planedatabases.All {
		properties = append(properties, fmt.Sprintf(
			`    %q: {"type": "string", "minLength": 1, "description": %q}`,
			string(d.Component),
			d.OutputDescription(),
		))
	}

	return strings.Join(properties, ",\n")
}
