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
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serveStatuses answers each request with the next status of statuses and
// the last one for every request after that. A 200 carries body.
func serveStatuses(t *testing.T, body string, statuses ...int) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		i := int(hits.Add(1)) - 1
		status := statuses[min(i, len(statuses)-1)]
		w.WriteHeader(status)
		if status == http.StatusOK {
			_, _ = w.Write([]byte(body))
		}
	}))
	t.Cleanup(server.Close)

	return server, &hits
}

func TestDownload(t *testing.T) {
	tests := []struct {
		name     string
		statuses []int
		want     string
		wantErr  string
		wantHits int32
	}{
		{
			name:     "a server error clears on the third try",
			statuses: []int{http.StatusInternalServerError, http.StatusInternalServerError, http.StatusOK},
			want:     "kind: List\n",
			wantHits: 3,
		},
		{
			name:     "a not found fails at once",
			statuses: []int{http.StatusNotFound},
			wantErr:  "HTTP 404",
			wantHits: 1,
		},
		{
			name:     "a server error that lasts fails after the last try",
			statuses: []int{http.StatusBadGateway},
			wantErr:  "HTTP 502",
			wantHits: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, hits := serveStatuses(t, "kind: List\n", tt.statuses...)

			body, err := download(server.URL, 4, time.Millisecond)

			assert.Equal(t, tt.wantHits, hits.Load())
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(body))
		})
	}
}

func TestDownloadRetriesAConnectionError(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()

	start := time.Now()
	_, err := download(url, 3, 10*time.Millisecond)

	require.ErrorContains(t, err, "downloading")
	assert.GreaterOrEqual(t, time.Since(start), 30*time.Millisecond, "two backoffs of 10ms and 20ms")
}

func TestRetryMakesOneTryWhenAttemptsIsNotPositive(t *testing.T) {
	server, hits := serveStatuses(t, "", http.StatusInternalServerError)

	_, err := download(server.URL, 0, time.Millisecond)

	require.ErrorContains(t, err, "HTTP 500")
	assert.Equal(t, int32(1), hits.Load())
}
