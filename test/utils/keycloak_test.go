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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestURLExists(t *testing.T) {
	tests := []struct {
		name     string
		statuses []int
		want     bool
		wantErr  string
		wantHits int32
	}{
		{
			name:     "a server error clears on the second try",
			statuses: []int{http.StatusServiceUnavailable, http.StatusOK},
			want:     true,
			wantHits: 2,
		},
		{
			name:     "a not found is an absent file at once",
			statuses: []int{http.StatusNotFound},
			want:     false,
			wantHits: 1,
		},
		{
			name:     "a forbidden fails at once",
			statuses: []int{http.StatusForbidden},
			wantErr:  "HTTP 403",
			wantHits: 1,
		},
		{
			name:     "a server error that lasts fails after the last try",
			statuses: []int{http.StatusInternalServerError},
			wantErr:  "HTTP 500",
			wantHits: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, hits := serveStatuses(t, "", tt.statuses...)

			exists, err := urlExists(server.URL, 3, time.Millisecond)

			assert.Equal(t, tt.wantHits, hits.Load())
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, exists)
		})
	}
}
