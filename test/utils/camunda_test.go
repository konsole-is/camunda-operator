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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCheckpointTime(t *testing.T) {
	want := time.Date(2026, 2, 25, 6, 44, 29, 309000000, time.UTC)

	tests := []struct {
		name  string
		value string
		want  time.Time
	}{
		{
			// The layout of the documented example.
			name:  "zulu",
			value: "2026-02-25T06:44:29.309Z",
			want:  want,
		},
		{
			// The layout that a running broker writes. It is not RFC 3339:
			// the offset carries no colon.
			name:  "numeric offset",
			value: "2026-03-18T13:20:13.001+0000",
			want:  time.Date(2026, 3, 18, 13, 20, 13, 1000000, time.UTC),
		},
		{
			name:  "offset that is not UTC",
			value: "2026-02-25T07:44:29.309+0100",
			want:  want,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCheckpointTime(tt.value)
			require.NoError(t, err)
			assert.True(t, got.Equal(tt.want), "got %s, want %s", got, tt.want)
		})
	}

	_, err := ParseCheckpointTime("25/02/2026 06:44:29")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "25/02/2026 06:44:29")
}
