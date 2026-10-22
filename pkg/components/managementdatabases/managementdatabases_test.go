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

package managementdatabases

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
)

func TestDatabaseNameAndLogicalName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		component     Component
		name, logical string
	}{
		{Keycloak, "ops-keycloak", "keycloak"},
		{Identity, "ops-identity", "identity"},
		{WebModeler, "ops-web-modeler", "web_modeler"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.name, DatabaseName("ops", tt.component))
		assert.Equal(t, tt.logical, LogicalName(tt.component))
	}
}

func TestServerSetsThePresetTheVersionTheContractAndTheStorageClass(t *testing.T) {
	t.Parallel()

	v := goldenValues(Identity)
	v.StorageClassName = "encrypted"

	server := Server(v)

	assert.Equal(t, "ops-db", server.Name)
	assert.Equal(t, "ops-ns", server.Namespace)
	assert.Equal(t, v.Labels, server.Labels)
	assert.Equal(t, "small", server.Spec.PresetRef)
	assert.Equal(t, "17", server.Spec.Version)
	assert.Equal(t, "ops-db", server.Spec.DatabaseServerConfig)
	require.NotNil(t, server.Spec.StorageClassName)
	assert.Equal(t, "encrypted", *server.Spec.StorageClassName)
}

func TestServerWithoutStorageClassKeepsTheDefault(t *testing.T) {
	t.Parallel()

	assert.Nil(t, Server(goldenValues(Identity)).Spec.StorageClassName)
}

// The DatabaseServer controller refuses a merged spec that ValidateMerged
// refuses. A preset gives the storage size, never the version or the
// contract, so the server must carry those itself.
func TestServerPassesTheMergedValidationOverAPresetWithAStorageSize(t *testing.T) {
	t.Parallel()

	server := Server(goldenValues(Identity))
	server.Spec.StorageSize = new(resource.MustParse("10Gi"))

	assert.NoError(t, databaseserver.ValidateMerged(server.Spec))
}

func TestDatabasePublishesTheDatabaseConfigOfItsName(t *testing.T) {
	t.Parallel()

	db := Database(goldenValues(WebModeler), WebModeler)

	assert.Equal(t, "ops-web-modeler", db.Name)
	assert.Equal(t, "ops-ns", db.Namespace)
	assert.Equal(t, "ops-db", db.Spec.ServerRef)
	assert.Equal(t, "web_modeler", db.Spec.DatabaseName)
	assert.Equal(t, "ops-web-modeler", db.Spec.DatabaseConfig)
	assert.Empty(t, db.Spec.SecondaryStorageConfig)
}

// goldenValues returns the values of the plane ops with the listed
// components.
func goldenValues(databases ...Component) Values {
	return Values{
		Namespace: "ops-ns",
		Plane:     "ops",
		Server:    "ops-db",
		Preset:    "small",
		Version:   "17",
		Databases: databases,
		Labels: map[string]string{
			"plugins.camunda.io/registration": "management-database-server",
			"plugins.camunda.io/request":      "ops-management-database",
			"app.kubernetes.io/managed-by":    "camunda-operator",
		},
	}
}
