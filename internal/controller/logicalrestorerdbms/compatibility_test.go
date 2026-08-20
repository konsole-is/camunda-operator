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

package logicalrestorerdbms

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// oneMinorNewer is the version one minor release after the version of the
// fixture pair. A relational backup restores with it.
const oneMinorNewer = "8.10.0"

// pair returns a compatible relational pair. Each test changes the one fact
// it is about.
func pair() compatibility {
	return compatibility{
		TargetStorageType: v1.SecondaryStorageTypeRDBMS,
		BackupBucket:      "backups",
		TargetBucket:      "backups",
		BackupVersion:     "8.9.9",
		TargetVersion:     "8.9.9",
	}
}

func TestCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   compatibility
		// want lists the fragments that the message must carry. An empty
		// list means that the pair is compatible.
		want []string
	}{
		{
			name: "a pair that matches in every fact",
			in:   pair(),
		},
		{
			name: "the target stores its data in elasticsearch",
			in: func() compatibility {
				in := pair()
				in.TargetStorageType = v1.SecondaryStorageTypeElasticsearch

				return in
			}(),
			want: []string{"rdbms", "elasticsearch"},
		},
		{
			name: "the target backs up through another bucket",
			in: func() compatibility {
				in := pair()
				in.TargetBucket = "other-backups"

				return in
			}(),
			want: []string{`"backups"`, `"other-backups"`},
		},
		{
			name: "a newer patch level",
			in: func() compatibility {
				in := pair()
				in.TargetVersion = "8.9.12"

				return in
			}(),
		},
		{
			name: "one minor newer",
			in: func() compatibility {
				in := pair()
				in.TargetVersion = oneMinorNewer

				return in
			}(),
		},
		{
			name: "two minors newer",
			in: func() compatibility {
				in := pair()
				in.TargetVersion = "8.11.0"

				return in
			}(),
			want: []string{"8.9.9", "8.11.0", "one minor version newer"},
		},
		{
			name: "the target is older than the backup",
			in: func() compatibility {
				in := pair()
				in.BackupVersion = oneMinorNewer

				return in
			}(),
			want: []string{oneMinorNewer, "8.9.9", "one minor version newer"},
		},
		{
			name: "another major version",
			in: func() compatibility {
				in := pair()
				in.TargetVersion = "9.0.0"

				return in
			}(),
			want: []string{"one minor version newer"},
		},
		{
			name: "the target version is not a version of the form x.y.z",
			in: func() compatibility {
				in := pair()
				in.TargetVersion = "latest"

				return in
			}(),
			want: []string{`"latest"`, "x.y.z"},
		},
		{
			name: "the backup version is not a version of the form x.y.z",
			in: func() compatibility {
				in := pair()
				in.BackupVersion = "8.9"

				return in
			}(),
			want: []string{`"8.9"`, "x.y.z"},
		},
		{
			name: "the backup recorded no version",
			in: func() compatibility {
				in := pair()
				in.BackupVersion = ""

				return in
			}(),
			want: []string{"did not record"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			failure := check(tt.in)

			if len(tt.want) == 0 {
				assert.Nil(t, failure)

				return
			}

			require.NotNil(t, failure)
			assert.Equal(t, v1.ReasonIncompatibleTarget, failure.Reason)
			for _, fragment := range tt.want {
				assert.Contains(t, failure.Message, fragment)
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		want  version
		valid bool
	}{
		{value: "8.9.9", want: version{major: 8, minor: 9, patch: 9}, valid: true},
		{value: "8.10.12", want: version{major: 8, minor: 10, patch: 12}, valid: true},
		{value: "8.9", valid: false},
		{value: "8.9.9.1", valid: false},
		{value: "8.9.x", valid: false},
		{value: "8.-1.0", valid: false},
		{value: "", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Parallel()

			got, err := parseVersion(tt.value)
			if !tt.valid {
				require.ErrorIs(t, err, errNotVersion)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
