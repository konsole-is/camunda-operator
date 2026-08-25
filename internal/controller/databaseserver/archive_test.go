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
	locationA = "s3://bucket-a/clusters/databaseserver/ns/camunda (region eu-west-1)"
	locationB = "s3://bucket-b/clusters/databaseserver/ns/camunda (region eu-west-1)"
	locationC = "s3://bucket-c/clusters/databaseserver/ns/camunda (region eu-west-1)"
)

// archiveOpenedAt is when every record of these fixtures opens.
var archiveOpenedAt = metav1.NewTime(time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC))

// archiveRecord is one record of the current cluster at locationA. The cases
// vary where the server archives to now, never where a record was written.
func archiveRecord(to *metav1.Time) v1.ArchiveRecord {
	return v1.ArchiveRecord{
		ServerName: "camunda", ObjectStorageRef: "bucket", Location: locationA,
		From: archiveOpenedAt, To: to,
	}
}

// legacyArchiveRecord is a record from before the location was recorded. It
// carries the bucket contract that named it and nothing else.
func legacyArchiveRecord(ref string, to *metav1.Time) v1.ArchiveRecord {
	return v1.ArchiveRecord{
		ServerName: "camunda", ObjectStorageRef: ref, From: archiveOpenedAt, To: to,
	}
}

// archivingServerWith returns a server whose archive status holds history and
// boundary.
func archivingServerWith(
	history []v1.ArchiveRecord,
	boundary *v1.ArchiveBoundary,
) *v1.DatabaseServer {
	return &v1.DatabaseServer{
		ObjectMeta: metav1.ObjectMeta{Name: "camunda", Namespace: "ns"},
		Status: v1.DatabaseServerStatus{
			Cluster: "camunda",
			Archive: &v1.DatabaseServerArchiveStatus{History: history, Boundary: boundary},
		},
	}
}

// A move is what closes an interval and opens the next one. The location
// decides it, and a record from before the location was recorded is placed by
// the bucket contract that named it, which is all such a record carries.
func TestArchiveMoved(t *testing.T) {
	t.Parallel()

	closedAt := metav1.NewTime(archiveOpenedAt.Add(time.Hour))
	movedAt := metav1.NewTime(archiveOpenedAt.Add(90 * time.Minute))

	tests := []struct {
		name     string
		history  []v1.ArchiveRecord
		boundary *v1.ArchiveBoundary
		ref      string
		location string
		want     bool
	}{
		{
			name:     "no archive written yet",
			ref:      "bucket",
			location: locationA,
		},
		{
			name:     "the archive of the location the spec names is open",
			history:  []v1.ArchiveRecord{archiveRecord(nil)},
			ref:      "bucket",
			location: locationA,
		},
		{
			name:     "the spec moved the archive, and the record is still open",
			history:  []v1.ArchiveRecord{archiveRecord(nil)},
			ref:      "bucket",
			location: locationB,
			want:     true,
		},
		{
			name:     "the archive was re-enabled elsewhere, with every record closed",
			history:  []v1.ArchiveRecord{archiveRecord(&closedAt)},
			ref:      "bucket",
			location: locationB,
			want:     true,
		},
		{
			name:     "the recorded move still names the location the spec does",
			history:  []v1.ArchiveRecord{archiveRecord(&closedAt)},
			boundary: &v1.ArchiveBoundary{At: movedAt, Location: locationB, ObjectStorageRef: "bucket"},
			ref:      "bucket",
			location: locationB,
		},
		{
			name:     "the archive moved again before a record opened",
			history:  []v1.ArchiveRecord{archiveRecord(&closedAt)},
			boundary: &v1.ArchiveBoundary{At: movedAt, Location: locationB, ObjectStorageRef: "bucket"},
			ref:      "bucket",
			location: locationC,
			want:     true,
		},
		{
			name:     "the archive moved back before a record opened",
			history:  []v1.ArchiveRecord{archiveRecord(&closedAt)},
			boundary: &v1.ArchiveBoundary{At: movedAt, Location: locationB, ObjectStorageRef: "bucket"},
			ref:      "bucket",
			location: locationA,
			want:     true,
		},
		{
			name:    "the bucket does not resolve",
			history: []v1.ArchiveRecord{archiveRecord(nil)},
			ref:     "bucket",
		},
		// The location says nothing about these two, so the contract that
		// named the record is all there is to place it by.
		{
			name:     "a record from before the location was recorded, under its own contract",
			history:  []v1.ArchiveRecord{legacyArchiveRecord("bucket", nil)},
			ref:      "bucket",
			location: locationB,
		},
		{
			name:     "a record from before the location was recorded, under another contract",
			history:  []v1.ArchiveRecord{legacyArchiveRecord("bucket-before", &closedAt)},
			ref:      "bucket",
			location: locationB,
			want:     true,
		},
		{
			name:     "a record with neither a location nor a contract",
			history:  []v1.ArchiveRecord{legacyArchiveRecord("", &closedAt)},
			ref:      "bucket",
			location: locationB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := archivingServerWith(tt.history, tt.boundary)

			assert.Equal(t, tt.want, archiveMoved(server, tt.ref, tt.location))
		})
	}
}

// The boundary is read on a reconcile that finds no move. It is the latest of
// the records the server closed and a move it recorded on an earlier reconcile
// and no record holds yet. The second is what covers a server with no interval
// open, which is where an archive that was disabled and re-enabled elsewhere
// leaves it.
func TestArchiveBoundary(t *testing.T) {
	t.Parallel()

	closedAt := metav1.NewTime(archiveOpenedAt.Add(time.Hour))
	movedAt := metav1.NewTime(archiveOpenedAt.Add(90 * time.Minute))

	tests := []struct {
		name     string
		history  []v1.ArchiveRecord
		boundary *v1.ArchiveBoundary
		noSpec   bool
		want     *metav1.Time
	}{
		{
			name: "no archive written yet",
		},
		{
			name:    "the archive the server writes is open",
			history: []v1.ArchiveRecord{archiveRecord(nil)},
		},
		{
			name:    "an archive closed before, and none open",
			history: []v1.ArchiveRecord{archiveRecord(&closedAt)},
			want:    &closedAt,
		},
		{
			name:     "a move recorded before, later than every close",
			history:  []v1.ArchiveRecord{archiveRecord(&closedAt)},
			boundary: &v1.ArchiveBoundary{At: movedAt, Location: locationB},
			want:     &movedAt,
		},
		{
			name:     "a close later than the recorded move",
			history:  []v1.ArchiveRecord{archiveRecord(&movedAt)},
			boundary: &v1.ArchiveBoundary{At: closedAt, Location: locationB},
			want:     &movedAt,
		},
		{
			name:    "the server asks for no archive",
			history: []v1.ArchiveRecord{archiveRecord(&closedAt)},
			noSpec:  true,
			want:    &closedAt,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := archivingServerWith(tt.history, tt.boundary)
			merged := v1.DatabaseServerSpec{}
			if !tt.noSpec {
				merged.Archive = &v1.DatabaseServerArchiveSpec{
					ObjectStorageRef: "bucket", RetentionPeriodDays: 30,
				}
			}

			got := archiveBoundary(server, merged)

			if tt.want == nil {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got)
			assert.True(t, got.Equal(tt.want), got)
		})
	}
}

// The boundary of a move has to be no earlier than the moment the ObjectStore
// of the new location was applied, or a base backup that started while the old
// one still stood counts as one of the new archive. Reconcile reads the clock
// after the components apply and hands that instant here, so what this pins is
// that the instant is used as given: nothing here reads a clock of its own, and
// archiveBoundary takes no clock at all.
//
// The gap itself cannot be reproduced in envtest. It is one apply wide, and
// status timestamps carry whole seconds, so a base backup cannot be placed
// inside it.
func TestReconcileArchiveHistoryRecordsAMoveAtTheGivenInstant(t *testing.T) {
	t.Parallel()

	appliedAt := metav1.NewTime(archiveOpenedAt.Add(2 * time.Hour))
	started := metav1.NewTime(archiveOpenedAt.Add(time.Hour))
	merged := v1.DatabaseServerSpec{
		Archive: &v1.DatabaseServerArchiveSpec{ObjectStorageRef: "bucket", RetentionPeriodDays: 30},
	}

	server := archivingServerWith([]v1.ArchiveRecord{archiveRecord(nil)}, nil)

	// A base backup that completed before the ObjectStore moved. The move wins
	// over it: the component is never consulted, which a nil one shows.
	reconcileArchiveHistory(server, merged, nil, &started, locationB, true, appliedAt)

	history := server.Status.Archive.History
	require.Len(t, history, 1, "a move opens no record")
	require.NotNil(t, history[0].To)
	assert.True(t, history[0].To.Equal(&appliedAt), history[0].To)

	boundary := server.Status.Archive.Boundary
	require.NotNil(t, boundary)
	assert.True(t, boundary.At.Equal(&appliedAt), boundary.At)
	assert.Equal(t, locationB, boundary.Location)
	assert.Equal(t, "bucket", boundary.ObjectStorageRef)
}
