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

package databaseserver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The boundary decides which base backups belong to the archive the server
// writes now. A bucket change has to move it in the same reconcile that
// applies the change: the archive component reads it before the history writes
// anything, so a boundary that waits for the recorded close lets a base backup
// of the bucket the server leaves report the new archive ready.
func TestArchiveBoundary(t *testing.T) {
	t.Parallel()

	now := metav1.NewTime(time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC))
	closedAt := metav1.NewTime(now.Add(-time.Hour))
	openedAt := metav1.NewTime(now.Add(-2 * time.Hour))

	record := func(bucket string, to *metav1.Time) v1.ArchiveRecord {
		return v1.ArchiveRecord{
			ServerName: "camunda", ObjectStorageRef: bucket, From: openedAt, To: to,
		}
	}

	tests := []struct {
		name    string
		history []v1.ArchiveRecord
		bucket  string
		want    *metav1.Time
	}{
		{
			name:   "no archive written yet",
			bucket: "bucket-a",
		},
		{
			name:    "the archive of the bucket the spec names is open",
			history: []v1.ArchiveRecord{record("bucket-a", nil)},
			bucket:  "bucket-a",
		},
		{
			name:    "an archive closed before, and none open",
			history: []v1.ArchiveRecord{record("bucket-a", &closedAt)},
			bucket:  "bucket-a",
			want:    &closedAt,
		},
		{
			name:    "the spec moved the bucket, and the record is still open",
			history: []v1.ArchiveRecord{record("bucket-a", nil)},
			bucket:  "bucket-b",
			want:    &now,
		},
		{
			name:    "an open record from before the bucket field existed",
			history: []v1.ArchiveRecord{record("", nil)},
			bucket:  "bucket-b",
		},
		{
			name:    "the server asks for no archive",
			history: []v1.ArchiveRecord{record("bucket-a", &closedAt)},
			want:    &closedAt,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := &v1.DatabaseServer{
				ObjectMeta: metav1.ObjectMeta{Name: "camunda", Namespace: "ns"},
				Status: v1.DatabaseServerStatus{
					Cluster: "camunda",
					Archive: &v1.DatabaseServerArchiveStatus{History: tt.history},
				},
			}
			merged := v1.DatabaseServerSpec{}
			if tt.bucket != "" {
				merged.Archive = &v1.DatabaseServerArchiveSpec{
					ObjectStorageRef: tt.bucket, RetentionPeriodDays: 30,
				}
			}

			got := archiveBoundary(server, merged, now)

			if tt.want == nil {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got)
			assert.True(t, got.Equal(tt.want), got)
		})
	}
}
