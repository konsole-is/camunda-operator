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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRustFSListing(t *testing.T) {
	tests := []struct {
		name    string
		out     string
		want    []RustFSObject
		wantErr string
	}{
		{
			name: "null is an empty bucket",
			out:  "null\n",
		},
		{
			name: "each entry becomes an object with its key and size",
			out: `[
    {"key": "camunda/ns/cluster/17/uid/camunda.dump", "size": 42},
    {"key": "camunda/ns/cluster/17/marker", "size": 0}
]
`,
			want: []RustFSObject{
				{Key: "camunda/ns/cluster/17/uid/camunda.dump", Size: 42},
				{Key: "camunda/ns/cluster/17/marker", Size: 0},
			},
		},
		{
			name:    "no output is an error, not an empty bucket",
			out:     "",
			wantErr: "decoding the bucket listing",
		},
		{
			name:    "output that is not JSON is an error that names it",
			out:     "aws: [ERROR]: An error occurred (NoSuchBucket)",
			wantErr: `decoding the bucket listing "aws: [ERROR]: An error occurred (NoSuchBucket)"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRustFSListing(tt.out)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
