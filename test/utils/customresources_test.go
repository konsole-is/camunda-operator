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

package utils

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var testCRDs = []CRD{
	{Name: "camundaclusters.core.camunda.io", Group: "core.camunda.io", Kind: "CamundaCluster"},
	{Name: "camundaplatformconfigs.core.camunda.io", Group: "core.camunda.io", Kind: "CamundaPlatformConfig"},
	{Name: "environments.fleet.camunda.io", Group: "fleet.camunda.io", Kind: "Environment"},
}

func TestParseCRDs(t *testing.T) {
	out := `{"items": [
		{"metadata": {"name": "camundaclusters.core.camunda.io"},
		 "spec": {"group": "core.camunda.io", "names": {"kind": "CamundaCluster", "plural": "camundaclusters"}}},
		{"metadata": {"name": "elasticsearches.elasticsearch.k8s.elastic.co"},
		 "spec": {"group": "elasticsearch.k8s.elastic.co", "names": {"kind": "Elasticsearch"}}},
		{"metadata": {"name": "environments.fleet.camunda.io"},
		 "spec": {"group": "fleet.camunda.io", "names": {"kind": "Environment"}}}
	]}`

	t.Run("keeps only the CRDs of the groups", func(t *testing.T) {
		crds, err := parseCRDs(out, []string{"core.camunda.io", "fleet.camunda.io"})
		require.NoError(t, err)
		assert.Equal(t, []CRD{testCRDs[0], testCRDs[2]}, crds)
	})

	t.Run("no group matches no CRD", func(t *testing.T) {
		crds, err := parseCRDs(out, nil)
		require.NoError(t, err)
		assert.Empty(t, crds)
	})

	t.Run("output that is not JSON is an error", func(t *testing.T) {
		_, err := parseCRDs("error: the server is down", []string{"core.camunda.io"})
		require.Error(t, err)
	})
}

func TestListCustomResourcesWithoutCRDs(t *testing.T) {
	resources, err := ListCustomResources(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, resources)
}

func TestCRDNames(t *testing.T) {
	assert.Equal(
		t,
		"camundaclusters.core.camunda.io,camundaplatformconfigs.core.camunda.io,environments.fleet.camunda.io",
		CRDNames(testCRDs),
	)
}

func TestParseCustomResources(t *testing.T) {
	deleted := metav1.NewTime(time.Date(2026, 11, 15, 14, 39, 53, 0, time.UTC))

	t.Run("maps each item to its CRD with the fields of its deletion", func(t *testing.T) {
		out := `{"apiVersion": "v1", "kind": "List", "items": [
			{"apiVersion": "core.camunda.io/v1", "kind": "CamundaCluster",
			 "metadata": {"name": "camunda", "namespace": "camunda-e2e",
			  "finalizers": ["core.camunda.io/storage-claim"], "deletionTimestamp": "2026-11-15T14:39:53Z"}},
			{"apiVersion": "core.camunda.io/v1", "kind": "CamundaPlatformConfig",
			 "metadata": {"name": "camunda-e2e"}}
		]}`

		resources, err := parseCustomResources(out, testCRDs)
		require.NoError(t, err)
		require.Len(t, resources, 2)

		assert.Equal(t, "camundaclusters.core.camunda.io", resources[0].Resource)
		assert.Equal(t, "CamundaCluster", resources[0].Kind)
		assert.Equal(t, "camunda-e2e", resources[0].Namespace)
		assert.Equal(t, "camunda", resources[0].Name)
		assert.Equal(t, []string{"core.camunda.io/storage-claim"}, resources[0].Finalizers)
		require.NotNil(t, resources[0].DeletionTimestamp)
		assert.True(t, deleted.Equal(resources[0].DeletionTimestamp))

		assert.Equal(t, "camundaplatformconfigs.core.camunda.io", resources[1].Resource)
		assert.Empty(t, resources[1].Namespace)
		assert.Empty(t, resources[1].Finalizers)
		assert.Nil(t, resources[1].DeletionTimestamp)
	})

	t.Run("an empty list is no resources", func(t *testing.T) {
		resources, err := parseCustomResources(`{"apiVersion": "v1", "kind": "List", "items": []}`, testCRDs)
		require.NoError(t, err)
		assert.Empty(t, resources)
	})

	t.Run("a kind of the same name in another group is an error", func(t *testing.T) {
		out := `{"items": [{"apiVersion": "other.io/v1", "kind": "CamundaCluster", "metadata": {"name": "x"}}]}`
		_, err := parseCustomResources(out, testCRDs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "other.io/v1")
	})

	t.Run("output that is not JSON is an error", func(t *testing.T) {
		_, err := parseCustomResources("error: the server is down", testCRDs)
		require.Error(t, err)
	})
}

func TestFormatCustomResources(t *testing.T) {
	deleted := metav1.NewTime(time.Date(2026, 11, 15, 14, 39, 53, 0, time.FixedZone("CET", 3600)))

	t.Run("one line for each resource", func(t *testing.T) {
		got := FormatCustomResources([]CustomResource{
			{
				Kind:              "LogicalBackupElasticsearch",
				Namespace:         "camunda-e2e",
				Name:              "camunda-es-restore-source",
				Finalizers:        []string{"core.camunda.io/backup-artifacts", "core.camunda.io/suspension-hold"},
				DeletionTimestamp: &deleted,
			},
			{Kind: "CamundaPlatformConfig", Name: "camunda-e2e"},
		})

		assert.Equal(
			t,
			"LogicalBackupElasticsearch camunda-e2e/camunda-es-restore-source "+
				"finalizers=[core.camunda.io/backup-artifacts,core.camunda.io/suspension-hold] "+
				"deletionTimestamp=2026-11-15T13:39:53Z\n"+
				"CamundaPlatformConfig camunda-e2e finalizers=[] deletionTimestamp=none\n",
			got,
		)
	})

	t.Run("no resources is no lines", func(t *testing.T) {
		assert.Empty(t, FormatCustomResources(nil))
	})
}
