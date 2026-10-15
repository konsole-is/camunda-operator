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

// backupStateBody is the documented example answer of the backup runtime
// state endpoint, cut to what BackupPositions reads and the fields around it.
const backupStateBody = `{
  "checkpointStates": [
    {"checkpointId": 1772001869309, "checkpointType": "SCHEDULED_BACKUP", "partitionId": 1,
     "checkpointPosition": 580, "checkpointTimestamp": "2026-02-25T06:44:29.309Z"},
    {"checkpointId": 1772001899000, "checkpointType": "MARKER", "partitionId": 2,
     "checkpointPosition": 190, "checkpointTimestamp": "2026-02-25T06:44:59.000Z"}
  ],
  "backupStates": [
    {"checkpointId": 1772001869309, "checkpointType": "SCHEDULED_BACKUP", "partitionId": 1,
     "checkpointPosition": 580, "firstLogPosition": 1, "checkpointTimestamp": "2026-02-25T06:44:29.391Z"},
    {"checkpointId": 1772001869309, "checkpointType": "SCHEDULED_BACKUP", "partitionId": 2,
     "checkpointPosition": 135, "firstLogPosition": 1, "checkpointTimestamp": "2026-02-25T06:44:29.394Z"}
  ],
  "ranges": []
}`

func TestBackupPositions(t *testing.T) {
	t.Run("reads the backup of each partition and not its newer marker", func(t *testing.T) {
		got, err := BackupPositions([]byte(backupStateBody))
		require.NoError(t, err)
		assert.Equal(t, map[int]int64{1: 580, 2: 135}, got)
	})

	t.Run("keeps the newest of two backups of one partition", func(t *testing.T) {
		got, err := BackupPositions([]byte(`{"backupStates": [
			{"partitionId": 1, "checkpointPosition": 186},
			{"partitionId": 1, "checkpointPosition": 90}
		]}`))
		require.NoError(t, err)
		assert.Equal(t, map[int]int64{1: 186}, got)
	})

	t.Run("leaves out a partition without a backup", func(t *testing.T) {
		got, err := BackupPositions([]byte(`{"checkpointStates": [], "backupStates": []}`))
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("reports a body that is not JSON", func(t *testing.T) {
		_, err := BackupPositions([]byte("curl: (7) Failed to connect"))
		require.Error(t, err)
	})
}

func TestExporterPositions(t *testing.T) {
	t.Run("reads one row per partition", func(t *testing.T) {
		got, err := ExporterPositions("1|187\n2|42\n")
		require.NoError(t, err)
		assert.Equal(t, map[int]int64{1: 187, 2: 42}, got)
	})

	t.Run("reads empty output as no partition", func(t *testing.T) {
		got, err := ExporterPositions("\n")
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	for name, out := range map[string]string{
		"a row without a separator":        "1 187",
		"a partition that is not a number": "one|187",
		"a position that is not a number":  "1|-",
	} {
		t.Run("reports "+name, func(t *testing.T) {
			_, err := ExporterPositions(out)
			require.Error(t, err)
			assert.Contains(t, err.Error(), out)
		})
	}
}

func TestUncoveredPartitions(t *testing.T) {
	tests := []struct {
		name     string
		exported map[int]int64
		backups  map[int]int64
		want     []int
	}{
		{
			name:     "a checkpoint past the exported position covers it",
			exported: map[int]int64{1: 180},
			backups:  map[int]int64{1: 186},
		},
		{
			name:     "a checkpoint at the exported position covers it",
			exported: map[int]int64{1: 186},
			backups:  map[int]int64{1: 186},
		},
		{
			// The numbers of a restore that the restore application refused:
			// the exporter had exported the record after the checkpoint.
			name:     "a checkpoint one record behind the exported position does not cover it",
			exported: map[int]int64{1: 187},
			backups:  map[int]int64{1: 186},
			want:     []int{1},
		},
		{
			name:     "a partition without a backup is uncovered",
			exported: map[int]int64{1: 10, 2: 10},
			backups:  map[int]int64{1: 20},
			want:     []int{2},
		},
		{
			name:     "uncovered partitions come in ascending order",
			exported: map[int]int64{3: 50, 1: 50, 2: 5},
			backups:  map[int]int64{1: 40, 2: 40, 3: 40},
			want:     []int{1, 3},
		},
		{
			name:    "no exported position leaves nothing uncovered",
			backups: map[int]int64{1: 40},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, UncoveredPartitions(tt.exported, tt.backups))
		})
	}
}
