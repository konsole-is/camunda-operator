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

package labels

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManagedCarriesOwnerComponentAndManager(t *testing.T) {
	t.Parallel()

	assert.Equal(
		t, map[string]string{
			"camunda.io/elasticsearch-cluster": "my-cluster",
			"camunda.io/component":             "elasticsearch",
			"app.kubernetes.io/managed-by":     "camunda-operator",
		}, Managed(ElasticsearchCluster("my-cluster"), "elasticsearch"),
	)
}

func TestDiscoveryOmitsTheManager(t *testing.T) {
	t.Parallel()

	assert.Equal(
		t, map[string]string{
			"camunda.io/database":  "my-db",
			"camunda.io/component": "database",
		}, Discovery(Database("my-db"), "database"),
	)
}

func TestMergeLetsOperatorLabelsWin(t *testing.T) {
	t.Parallel()

	user := map[string]string{"camunda.io/elasticsearch-cluster": "someone-else", "team": "platform"}
	merged := Merge(user, Managed(ElasticsearchCluster("my-cluster"), "elasticsearch"))

	assert.Equal(t, "my-cluster", merged["camunda.io/elasticsearch-cluster"])
	assert.Equal(t, "platform", merged["team"])
	assert.Equal(t, "someone-else", user["camunda.io/elasticsearch-cluster"], "the input is not mutated")
}

func TestRestoreOwners(t *testing.T) {
	assert.Equal(t, Owner{Key: LogicalRestoreRDBMSKey, Name: "r"}, LogicalRestoreRDBMS("r"))
	assert.Equal(t, "camunda.io/logical-restore-rdbms", LogicalRestoreRDBMSKey)

	assert.Equal(t, Owner{Key: PointInTimeRestoreKey, Name: "p"}, PointInTimeRestore("p"))
	assert.Equal(t, "camunda.io/point-in-time-restore", PointInTimeRestoreKey)
}
