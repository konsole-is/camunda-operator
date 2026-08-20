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

package logicalrestoreelasticsearch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// compatiblePair returns a pair that matches in every fact. Each case changes
// the one fact it is about. The two cluster names are one name, because a
// backup restores into the cluster it was taken from alone.
func compatiblePair() compatibility {
	return compatibility{
		TargetStorageType: v1.SecondaryStorageTypeElasticsearch,
		BackupCluster:     "my-cluster",
		TargetCluster:     "my-cluster",
		BackupPartitions:  3,
		TargetPartitions:  3,
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
		// want lists the fragments that the message must carry. An empty list
		// means that the pair is compatible.
		want []string
	}{
		{
			name: "a pair that matches in every fact",
			in:   compatiblePair(),
		},
		{
			name: "a relational target",
			in: func() compatibility {
				in := compatiblePair()
				in.TargetStorageType = v1.SecondaryStorageTypeRDBMS

				return in
			}(),
			want: []string{"holds elasticsearch data", "stores its data in rdbms"},
		},
		{
			name: "a target that is another cluster",
			in: func() compatibility {
				in := compatiblePair()
				in.TargetCluster = "my-clone"

				return in
			}(),
			want: []string{
				`CamundaCluster "my-cluster"`,
				`the target cluster is "my-clone"`,
				"A restore into another cluster is not supported",
				`The prefix of "my-clone" holds no backup of "my-cluster"`,
			},
		},
		{
			name: "partition counts that differ",
			in: func() compatibility {
				in := compatiblePair()
				in.TargetPartitions = 6

				return in
			}(),
			want: []string{"3 partitions", "runs 6"},
		},
		{
			name: "a backup that recorded no partition count",
			in: func() compatibility {
				in := compatiblePair()
				in.BackupPartitions = 0

				return in
			}(),
			want: []string{"0 partitions", "runs 3"},
		},
		{
			name: "a target that backs up through another bucket",
			in: func() compatibility {
				in := compatiblePair()
				in.TargetBucket = "other-backups"

				return in
			}(),
			want: []string{`"backups"`, `"other-backups"`},
		},
		{
			name: "a backup that recorded no Camunda version",
			in: func() compatibility {
				in := compatiblePair()
				in.BackupVersion = ""

				return in
			}(),
			want: []string{"did not record the Camunda version"},
		},
		{
			name: "a target one patch release newer",
			in: func() compatibility {
				in := compatiblePair()
				in.TargetVersion = "8.9.10"

				return in
			}(),
			want: []string{"8.9.9", "8.9.10", "exact version"},
		},
		{
			name: "a target one minor release newer",
			in: func() compatibility {
				in := compatiblePair()
				in.TargetVersion = "8.10.0"

				return in
			}(),
			want: []string{"exact version"},
		},
		{
			name: "a backup version that names no version",
			in: func() compatibility {
				in := compatiblePair()
				in.BackupVersion = "8.9"

				return in
			}(),
			want: []string{`the backup recorded the Camunda version "8.9"`, "x.y.z"},
		},
		{
			name: "a target version that names no version",
			in: func() compatibility {
				in := compatiblePair()
				in.TargetVersion = "latest"

				return in
			}(),
			want: []string{`the target cluster runs the Camunda version "latest"`, "x.y.z"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			failure := check(test.in)
			if len(test.want) == 0 {
				assert.Nil(t, failure)

				return
			}

			require.NotNil(t, failure)
			assert.Equal(t, v1.ReasonIncompatibleTarget, failure.Reason)
			for _, fragment := range test.want {
				assert.Contains(t, failure.Message, fragment)
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value   string
		want    [3]int
		wantErr bool
	}{
		{value: "8.9.9", want: [3]int{8, 9, 9}},
		{value: "8.10.0", want: [3]int{8, 10, 0}},
		{value: "8.9", wantErr: true},
		{value: "8.9.9.1", wantErr: true},
		{value: "8.9.x", wantErr: true},
		{value: "8.-1.0", wantErr: true},
		{value: "", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			t.Parallel()

			got, err := parseVersion(test.value)
			if test.wantErr {
				require.ErrorIs(t, err, errNotVersion)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}
