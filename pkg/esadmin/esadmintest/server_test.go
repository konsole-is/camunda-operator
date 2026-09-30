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

package esadmintest_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/konsole-is/camunda-operator/pkg/esadmin/esadmintest"
)

// Each read whose answer grows with the size of the cluster must carry a
// filter_path. The fake refuses the read without one, so a client test that
// passes proves that the client filters.
func TestServerRefusesAReadWithoutItsFilter(t *testing.T) {
	const tolerant = "ignore_unavailable=true&allow_no_indices=true"

	tests := []struct {
		name     string
		path     string
		filtered string
	}{
		{
			name:     "index resolution",
			path:     "/camunda-*?" + tolerant + "&expand_wildcards=open,closed",
			filtered: "&filter_path=*.settings.index.uuid",
		},
		{
			name:     "recovery",
			path:     "/camunda-*/_recovery?active_only=true&" + tolerant,
			filtered: "&filter_path=*.shards.stage",
		},
		{
			name: "routing table",
			path: "/_cluster/state/routing_table/camunda-*?" + tolerant,
			filtered: "&filter_path=routing_table.indices.*.shards.*.state," +
				"routing_table.indices.*.shards.*.primary," +
				"routing_table.indices.*.shards.*.recovery_source.type," +
				"routing_table.indices.*.shards.*.unassigned_info.reason," +
				"routing_table.indices.*.shards.*.unassigned_info.allocation_status",
		},
		{
			name:     "snapshot status",
			path:     "/_snapshot/repo/snap",
			filtered: "?filter_path=snapshots.state,snapshots.metadata",
		},
		{
			name:     "node statistics",
			path:     "/_nodes/stats/fs",
			filtered: "?filter_path=nodes.*.fs.total.total_in_bytes,nodes.*.fs.total.available_in_bytes",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := esadmintest.New()
			t.Cleanup(server.Close)
			server.SetIndices("camunda-1")

			assert.Equal(t, http.StatusBadRequest, get(t, server.URL()+tt.path))
			assert.NotEqual(t, http.StatusBadRequest, get(t, server.URL()+tt.path+tt.filtered))
		})
	}
}

func get(t *testing.T, url string) int {
	t.Helper()

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	require.NoError(t, err)
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())

	return response.StatusCode
}
