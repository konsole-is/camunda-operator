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

package logicalrestore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// oneMinorNewer is the version one minor release after the version of the
// fixture pair. A relational backup restores with it, an Elasticsearch backup
// does not.
const oneMinorNewer = "8.10.0"

// elasticsearchPair returns a compatible Elasticsearch pair. Each test changes
// the one fact it is about.
func elasticsearchPair() compatibility {
	return compatibility{
		BackupStorageType: v1.SecondaryStorageTypeElasticsearch,
		TargetStorageType: v1.SecondaryStorageTypeElasticsearch,
		BackupPartitions:  3,
		TargetPartitions:  3,
		BackupBucket:      "backups",
		TargetBucket:      "backups",
		BackupVersion:     "8.9.9",
		TargetVersion:     "8.9.9",
	}
}

// relationalPair returns a compatible relational pair. A relational backup
// records no partition count, so both counts are the target's.
func relationalPair() compatibility {
	in := elasticsearchPair()
	in.BackupStorageType = v1.SecondaryStorageTypeRDBMS
	in.TargetStorageType = v1.SecondaryStorageTypeRDBMS
	in.BackupPartitions = 0

	return in
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
			name: "elasticsearch pair that matches in every fact",
			in:   elasticsearchPair(),
		},
		{
			name: "elasticsearch backup into a relational target",
			in: func() compatibility {
				in := elasticsearchPair()
				in.TargetStorageType = v1.SecondaryStorageTypeRDBMS

				return in
			}(),
			want: []string{"elasticsearch", "rdbms"},
		},
		{
			name: "relational backup into an elasticsearch target",
			in: func() compatibility {
				in := relationalPair()
				in.TargetStorageType = v1.SecondaryStorageTypeElasticsearch

				return in
			}(),
			want: []string{"rdbms", "elasticsearch"},
		},
		{
			name: "partition counts differ",
			in: func() compatibility {
				in := elasticsearchPair()
				in.TargetPartitions = 5

				return in
			}(),
			want: []string{"3", "5"},
		},
		{
			name: "the target backs up through another bucket",
			in: func() compatibility {
				in := elasticsearchPair()
				in.TargetBucket = "other-backups"

				return in
			}(),
			want: []string{"backups", "other-backups"},
		},
		{
			name: "elasticsearch, one patch level newer",
			in: func() compatibility {
				in := elasticsearchPair()
				in.TargetVersion = "8.9.10"

				return in
			}(),
			want: []string{"8.9.9", "8.9.10", "exact"},
		},
		{
			name: "elasticsearch, one minor newer",
			in: func() compatibility {
				in := elasticsearchPair()
				in.TargetVersion = oneMinorNewer

				return in
			}(),
			want: []string{"8.9.9", oneMinorNewer},
		},
		{
			name: "relational, the same version",
			in:   relationalPair(),
		},
		{
			name: "relational, a newer patch level",
			in: func() compatibility {
				in := relationalPair()
				in.TargetVersion = "8.9.12"

				return in
			}(),
		},
		{
			name: "relational, one minor newer",
			in: func() compatibility {
				in := relationalPair()
				in.TargetVersion = oneMinorNewer

				return in
			}(),
		},
		{
			name: "relational, two minors newer",
			in: func() compatibility {
				in := relationalPair()
				in.TargetVersion = "8.11.0"

				return in
			}(),
			want: []string{"8.9.9", "8.11.0"},
		},
		{
			name: "relational, the target is older than the backup",
			in: func() compatibility {
				in := relationalPair()
				in.BackupVersion = oneMinorNewer
				in.TargetVersion = "8.9.9"

				return in
			}(),
			want: []string{oneMinorNewer, "8.9.9"},
		},
		{
			name: "the target version is not a version of the form x.y.z",
			in: func() compatibility {
				in := elasticsearchPair()
				in.TargetVersion = "latest"

				return in
			}(),
			want: []string{"latest"},
		},
		{
			name: "the backup version is not a version of the form x.y.z",
			in: func() compatibility {
				in := elasticsearchPair()
				in.BackupVersion = "8.9"

				return in
			}(),
			want: []string{"8.9"},
		},
		{
			name: "the backup recorded no version",
			in: func() compatibility {
				in := elasticsearchPair()
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
