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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/validation"
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

// A custom resource name reaches 253 characters, but a label value stops at
// 63. The owner label is part of every selector, so an owner name that does
// not fit would make the API server reject the whole selector.
func TestALongOwnerNameFitsALabelValue(t *testing.T) {
	t.Parallel()

	owners := []Owner{
		Cluster(strings.Repeat("c", 253)),
		ElasticsearchCluster(strings.Repeat("e", 253)),
		Database(strings.Repeat("d", 253)),
		LogicalBackupElasticsearch(strings.Repeat("l", 253)),
		LogicalBackupRDBMS(strings.Repeat("r", 253)),
		BackupSchedule(strings.Repeat("s", 253)),
	}

	for _, owner := range owners {
		for key, value := range Managed(owner, "component") {
			assert.Empty(t, validation.IsValidLabelValue(value), "%s=%s", key, value)
		}
	}

	assert.NotEqual(
		t,
		Cluster(strings.Repeat("c", 80)+"one").Name,
		Cluster(strings.Repeat("c", 80)+"two").Name,
		"two long owners keep separate label values",
	)
}
