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

func TestParseMinIOListing(t *testing.T) {
	const file = `{"status":"success","type":"file","size":42,` +
		`"key":"camunda/ns/cluster/17/uid/camunda.dump","etag":"abc"}`
	const emptyFile = `{"status":"success","type":"file","size":0,"key":"camunda/ns/cluster/17/marker"}`
	const folder = `{"status":"success","type":"folder","size":0,"key":"camunda/ns/cluster/"}`
	const failed = `{"status":"error","error":{"message":"Access Denied."}}`

	tests := []struct {
		name    string
		out     string
		want    []MinIOObject
		wantErr string
	}{
		{
			name: "an empty listing holds no object",
			out:  "",
		},
		{
			name: "blank lines hold no object",
			out:  "\n  \n\n",
		},
		{
			name: "a file entry becomes an object with its key and size",
			out:  file,
			want: []MinIOObject{{Key: "camunda/ns/cluster/17/uid/camunda.dump", Size: 42}},
		},
		{
			name: "an empty object keeps its zero size",
			out:  emptyFile,
			want: []MinIOObject{{Key: "camunda/ns/cluster/17/marker", Size: 0}},
		},
		{
			name: "a prefix entry is not an object",
			out:  folder + "\n" + file,
			want: []MinIOObject{{Key: "camunda/ns/cluster/17/uid/camunda.dump", Size: 42}},
		},
		{
			name: "every file entry of a listing becomes an object",
			out:  file + "\n" + emptyFile + "\n",
			want: []MinIOObject{
				{Key: "camunda/ns/cluster/17/uid/camunda.dump", Size: 42},
				{Key: "camunda/ns/cluster/17/marker", Size: 0},
			},
		},
		{
			name:    "a failed entry is an error, not an empty listing",
			out:     failed,
			wantErr: "the mc listing reports",
		},
		{
			name:    "a line that is not JSON is an error that names the line",
			out:     "mc: <ERROR> Unable to list.",
			wantErr: `decoding the mc listing line "mc: <ERROR> Unable to list."`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMinIOListing(tt.out)

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
