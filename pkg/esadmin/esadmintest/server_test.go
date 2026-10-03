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
	"encoding/json"
	"net/http"
	"strings"
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
		method   string
		path     string
		body     string
		filtered string
	}{
		{
			name:     "index resolution",
			path:     "/_resolve/index/camunda-*?" + tolerant + "&expand_wildcards=open,closed",
			filtered: "&filter_path=indices.name,aliases.indices",
		},
		{
			name:     "recovery",
			path:     "/camunda-*/_recovery?active_only=true&" + tolerant,
			filtered: "&filter_path=*.shards.stage",
		},
		{
			name: "shard health",
			path: "/_cluster/health/camunda-*?level=shards&timeout=0s",
			filtered: "&filter_path=indices.*.shards.*.initializing_shards," +
				"indices.*.shards.*.unassigned_primary_shards",
		},
		{
			name:   "allocation explanation",
			method: http.MethodPost,
			path:   "/_cluster/allocation/explain",
			body:   `{"index":"camunda-1","shard":0,"primary":true}`,
			filtered: "?filter_path=current_state,unassigned_info.reason," +
				"unassigned_info.last_allocation_status",
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

			assert.Equal(t, http.StatusBadRequest, send(t, tt.method, server.URL()+tt.path, tt.body))
			assert.NotEqual(t, http.StatusBadRequest, send(t, tt.method, server.URL()+tt.path+tt.filtered, tt.body))
		})
	}
}

// The fake prunes a filtered answer the way Elasticsearch does, so a client
// that decodes a field that its filter does not name reads nothing in a test
// too.
func TestServerAnswersOnlyTheFilteredFields(t *testing.T) {
	server := esadmintest.New()
	t.Cleanup(server.Close)
	server.SetIndices("camunda-1")

	request, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		server.URL()+"/camunda-1/_recovery?filter_path=*.shards.stage",
		nil,
	)
	require.NoError(t, err)
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	defer func() { _ = response.Body.Close() }()

	var body any
	require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
	want := map[string]any{
		"camunda-1": map[string]any{"shards": []any{map[string]any{"stage": "DONE"}}},
	}
	assert.Equal(t, want, body)
}

// send makes one request and returns its status. An empty method is GET.
func send(t *testing.T, method, url, body string) int {
	t.Helper()

	if method == "" {
		method = http.MethodGet
	}
	request, err := http.NewRequestWithContext(t.Context(), method, url, strings.NewReader(body))
	require.NoError(t, err)
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())

	return response.StatusCode
}
