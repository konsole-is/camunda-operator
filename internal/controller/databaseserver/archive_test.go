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

const (
	locationA = "s3://bucket-a/clusters/databaseserver/ns/camunda"
	locationB = "s3://bucket-b/clusters/databaseserver/ns/camunda"
	locationC = "s3://bucket-c/clusters/databaseserver/ns/camunda"
)

// The boundary decides which base backups belong to the archive the server
// writes now. A move of the archive has to move it in the same reconcile that
// applies the move: the archive component reads it before the history writes
// anything, so a boundary that waits for the recorded close lets a base backup
// of the location the server leaves report the new archive ready.
//
// A move with no interval open closes no record, so a boundary read from the
// records alone is not moved at all. status.archive.boundary carries that one.
func TestArchiveBoundary(t *testing.T) {
	t.Parallel()

	now := metav1.NewTime(time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC))
	closedAt := metav1.NewTime(now.Add(-time.Hour))
	movedAt := metav1.NewTime(now.Add(-30 * time.Minute))
	openedAt := metav1.NewTime(now.Add(-2 * time.Hour))

	record := func(location string, to *metav1.Time) v1.ArchiveRecord {
		return v1.ArchiveRecord{
			ServerName: "camunda", ObjectStorageRef: "bucket", Location: location,
			From: openedAt, To: to,
		}
	}

	tests := []struct {
		name     string
		history  []v1.ArchiveRecord
		boundary *v1.ArchiveBoundary
		location string
		noSpec   bool
		want     *metav1.Time
	}{
		{
			name:     "no archive written yet",
			location: locationA,
		},
		{
			name:     "the archive of the location the spec names is open",
			history:  []v1.ArchiveRecord{record(locationA, nil)},
			location: locationA,
		},
		{
			name:     "an archive closed before, and none open",
			history:  []v1.ArchiveRecord{record(locationA, &closedAt)},
			location: locationA,
			want:     &closedAt,
		},
		{
			name:     "the spec moved the archive, and the record is still open",
			history:  []v1.ArchiveRecord{record(locationA, nil)},
			location: locationB,
			want:     &now,
		},
		{
			name:     "an open record from before the location was recorded",
			history:  []v1.ArchiveRecord{record("", nil)},
			location: locationB,
		},
		{
			name:     "the server asks for no archive",
			history:  []v1.ArchiveRecord{record(locationA, &closedAt)},
			location: locationA,
			noSpec:   true,
			want:     &closedAt,
		},
		{
			name:     "the bucket does not resolve",
			history:  []v1.ArchiveRecord{record(locationA, nil)},
			location: "",
		},
		// The archive was disabled, which closed the record, and re-enabled on
		// another location. Nothing is open to close, so the move lives on the
		// boundary or it is lost.
		{
			name:     "re-enabled on another location, with every record closed",
			history:  []v1.ArchiveRecord{record(locationA, &closedAt)},
			location: locationB,
			want:     &now,
		},
		{
			name:     "the recorded move still names the location the spec does",
			history:  []v1.ArchiveRecord{record(locationA, &closedAt)},
			boundary: &v1.ArchiveBoundary{At: movedAt, Location: locationB},
			location: locationB,
			want:     &movedAt,
		},
		{
			name:     "the archive moved again before a record opened",
			history:  []v1.ArchiveRecord{record(locationA, &closedAt)},
			boundary: &v1.ArchiveBoundary{At: movedAt, Location: locationB},
			location: locationC,
			want:     &now,
		},
		{
			name:     "the archive moved back before a record opened",
			history:  []v1.ArchiveRecord{record(locationA, &closedAt)},
			boundary: &v1.ArchiveBoundary{At: movedAt, Location: locationB},
			location: locationA,
			want:     &now,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := &v1.DatabaseServer{
				ObjectMeta: metav1.ObjectMeta{Name: "camunda", Namespace: "ns"},
				Status: v1.DatabaseServerStatus{
					Cluster: "camunda",
					Archive: &v1.DatabaseServerArchiveStatus{
						History: tt.history, Boundary: tt.boundary,
					},
				},
			}
			merged := v1.DatabaseServerSpec{}
			if !tt.noSpec {
				merged.Archive = &v1.DatabaseServerArchiveSpec{
					ObjectStorageRef: "bucket", RetentionPeriodDays: 30,
				}
			}

			got := archiveBoundary(server, merged, tt.location, now)

			if tt.want == nil {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got)
			assert.True(t, got.Equal(tt.want), got)
		})
	}
}
