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

func TestRestoreOwners(t *testing.T) {
	assert.Equal(t, Owner{Key: LogicalRestoreRDBMSKey, Name: "r"}, LogicalRestoreRDBMS("r"))
	assert.Equal(t, "camunda.io/logical-restore-rdbms", LogicalRestoreRDBMSKey)

	assert.Equal(t, Owner{Key: PointInTimeRestoreKey, Name: "p"}, PointInTimeRestore("p"))
	assert.Equal(t, "camunda.io/point-in-time-restore", PointInTimeRestoreKey)
}

func TestManagementClusterOwner(t *testing.T) {
	assert.Equal(t, Owner{Key: ManagementClusterKey, Name: "m"}, ManagementCluster("m"))
	assert.Equal(t, "camunda.io/management-cluster", ManagementClusterKey)
}

func TestBoundedNameKeepsANameThatFits(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "my-cluster", BoundedName("my-cluster", 63))
	assert.Equal(t, "abcde", BoundedName("abcde", 5), "a name at the limit is kept")
}

func TestBoundedNameTruncatesAndSuffixesALongName(t *testing.T) {
	t.Parallel()

	name := strings.Repeat("a", 80)
	bounded := BoundedName(name, 63)

	assert.Len(t, bounded, 63)
	assert.Equal(t, strings.Repeat("a", 52)+"-", bounded[:53], "the head is kept")
	assert.Regexp(t, "^a+-[0-9a-f]{10}$", bounded, "a hash of the whole name ends it")
	assert.Equal(t, bounded, BoundedName(name, 63), "the result is deterministic")
}

func TestBoundedNameSeparatesTwoLongNamesThatShareTheirHead(t *testing.T) {
	t.Parallel()

	head := strings.Repeat("a", 80)

	assert.NotEqual(t, BoundedName(head+"-one", 63), BoundedName(head+"-two", 63))
}

// A custom resource name reaches 253 characters, but a label value stops at
// 63. The owner label is part of every selector, so an owner name that does
// not fit would make the API server reject the whole selector. The
// constructor of each kind applies the bound, so no call site can forget it.
func TestALongOwnerNameFitsALabelValue(t *testing.T) {
	t.Parallel()

	owners := []Owner{
		Cluster(strings.Repeat("c", 253)),
		ElasticsearchCluster(strings.Repeat("e", 253)),
		Database(strings.Repeat("d", 253)),
		LogicalBackupElasticsearch(strings.Repeat("l", 253)),
		LogicalBackupRDBMS(strings.Repeat("r", 253)),
		LogicalRestoreElasticsearch(strings.Repeat("x", 253)),
		LogicalRestoreRDBMS(strings.Repeat("y", 253)),
		BackupSchedule(strings.Repeat("s", 253)),
		PointInTimeRestore(strings.Repeat("p", 253)),
		ManagementCluster(strings.Repeat("m", 253)),
	}

	for _, owner := range owners {
		for key, value := range Managed(owner, "component") {
			assert.Empty(t, validation.IsValidLabelValue(value), "%s=%s", key, value)
		}
		for key, value := range Discovery(owner, "component") {
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

// A limit with no room for a head and a hash must not index the name
// negatively. Nothing passes one today, but a panic here would take the whole
// manager down rather than fail one reconcile.
func TestBoundedNameHandlesALimitSmallerThanTheHash(t *testing.T) {
	t.Parallel()

	for _, limit := range []int{0, 1, 5, nameHashLength + 1} {
		assert.Len(t, BoundedName(strings.Repeat("a", 253), limit), limit, limit)
	}

	assert.NotEqual(
		t,
		BoundedName(strings.Repeat("a", 253), 5),
		BoundedName(strings.Repeat("b", 253), 5),
		"a short bound still separates two names",
	)
}
