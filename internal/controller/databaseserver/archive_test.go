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
	components "github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
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
	reconcileArchiveHistory(server, merged, nil, &started, locationB, true, true, appliedAt)

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

// The ObjectStore of the new location is what moves the archive. Until that
// apply lands, the objects still go where they went, so the record stays open
// and no boundary is written. The move is found again on the next reconcile.
func TestReconcileArchiveHistoryHoldsAMoveTheApplyDidNotReach(t *testing.T) {
	t.Parallel()

	now := metav1.NewTime(archiveOpenedAt.Add(2 * time.Hour))
	started := metav1.NewTime(archiveOpenedAt.Add(time.Hour))
	merged := v1.DatabaseServerSpec{
		Archive: &v1.DatabaseServerArchiveSpec{ObjectStorageRef: "bucket", RetentionPeriodDays: 30},
	}

	server := archivingServerWith([]v1.ArchiveRecord{archiveRecord(nil)}, nil)

	reconcileArchiveHistory(server, merged, nil, &started, locationB, true, false, now)

	history := server.Status.Archive.History
	require.Len(t, history, 1)
	assert.Nil(t, history[0].To, "the archive is where it was, so its record is still open")
	assert.Nil(t, server.Status.Archive.Boundary)
}

// ArchiveReady reports the uploads of the archive the server writes now. A
// suspended server writes no write-ahead log, and a server that asks for no
// archive writes none either, so what CloudNativePG left on its cluster
// describes neither of them. An outage inside the grace period is reported by
// nobody and looked at again.
func TestReportedAndPendingArchiveOutage(t *testing.T) {
	t.Parallel()

	archiving := v1.DatabaseServerSpec{
		Archive: &v1.DatabaseServerArchiveSpec{ObjectStorageRef: "bucket", RetentionPeriodDays: 30},
	}
	suspended := v1.DatabaseServerSpec{Archive: archiving.Archive, Suspend: true}
	confirmed := &components.ArchiveOutage{Reason: "ContinuousArchivingFailing", Confirmed: true}
	held := &components.ArchiveOutage{Reason: "ContinuousArchivingFailing"}

	assert.Same(t, confirmed, reportedArchiveOutage(confirmed, archiving))
	assert.Nil(t, reportedArchiveOutage(held, archiving))
	assert.Nil(t, reportedArchiveOutage(confirmed, suspended))
	assert.Nil(t, reportedArchiveOutage(confirmed, v1.DatabaseServerSpec{}))
	assert.Nil(t, reportedArchiveOutage(nil, archiving))

	assert.True(t, pendingArchiveOutage(held, archiving))
	assert.False(t, pendingArchiveOutage(confirmed, archiving))
	assert.False(t, pendingArchiveOutage(held, suspended))
	assert.False(t, pendingArchiveOutage(held, v1.DatabaseServerSpec{}))
	assert.False(t, pendingArchiveOutage(nil, archiving))
}

// unverifiedFrom says what the archive the server writes now is missing. The
// plugin fills the gap once the uploads run again, so a record that closed
// while they were failing must not keep the mark and turn it into history. The
// closers of a record run before this one in the same reconcile.
func TestMarkArchiveOutageClearsTheMarkOffClosedRecords(t *testing.T) {
	t.Parallel()

	closedAt := metav1.NewTime(archiveOpenedAt.Add(time.Hour))
	stoppedAt := metav1.NewTime(archiveOpenedAt.Add(30 * time.Minute))
	outage := &components.ArchiveOutage{Since: stoppedAt, Confirmed: true}

	closed := archiveRecord(&closedAt)
	closed.UnverifiedFrom = &stoppedAt
	server := archivingServerWith([]v1.ArchiveRecord{closed}, nil)

	// The drop of spec.archive closes the record and reports on no outage.
	markArchiveOutage(server, nil)
	assert.Nil(t, server.Status.Archive.History[0].UnverifiedFrom)

	// A record that closes while the uploads are still failing loses it too.
	// The outage belongs to the archive the server writes now, and there is
	// none.
	closed = archiveRecord(&closedAt)
	closed.UnverifiedFrom = &stoppedAt
	server = archivingServerWith([]v1.ArchiveRecord{closed}, nil)

	markArchiveOutage(server, outage)
	assert.Nil(t, server.Status.Archive.History[0].UnverifiedFrom)

	// The open record still takes it, and the closed one beside it does not.
	server = archivingServerWith(
		[]v1.ArchiveRecord{archiveRecord(&closedAt), archiveRecord(nil)}, nil,
	)
	server.Status.Archive.History[0].UnverifiedFrom = &stoppedAt

	markArchiveOutage(server, outage)
	assert.Nil(t, server.Status.Archive.History[0].UnverifiedFrom)
	require.NotNil(t, server.Status.Archive.History[1].UnverifiedFrom)
	assert.True(t, server.Status.Archive.History[1].UnverifiedFrom.Equal(&stoppedAt))
}

// The floor is the point the highest retention period ever in force pruned
// the bucket to. A raise leaves it where it is, and the clock moves it again
// only once now minus the retention period passes it.
func TestAdvanceArchiveFloor(t *testing.T) {
	t.Parallel()

	floorAt := func(d time.Duration) *metav1.Time {
		point := metav1.NewTime(archiveOpenedAt.Add(d))

		return &point
	}

	tests := []struct {
		name   string
		spec   *v1.DatabaseServerArchiveSpec
		status *v1.DatabaseServerArchiveStatus
		now    metav1.Time
		want   *metav1.Time
	}{
		{
			name: "records the floor of an archive that carries none",
			spec: &v1.DatabaseServerArchiveSpec{ObjectStorageRef: "bucket", RetentionPeriodDays: 7},
			now:  archiveOpenedAt,
			want: floorAt(-7 * day),
		},
		{
			name:   "moves the floor with the clock",
			spec:   &v1.DatabaseServerArchiveSpec{ObjectStorageRef: "bucket", RetentionPeriodDays: 7},
			status: &v1.DatabaseServerArchiveStatus{ReachableFrom: floorAt(-7 * day)},
			now:    metav1.NewTime(archiveOpenedAt.Add(day)),
			want:   floorAt(-6 * day),
		},
		{
			name:   "keeps the floor when the retention period is raised",
			spec:   &v1.DatabaseServerArchiveSpec{ObjectStorageRef: "bucket", RetentionPeriodDays: 30},
			status: &v1.DatabaseServerArchiveStatus{ReachableFrom: floorAt(-7 * day)},
			now:    archiveOpenedAt,
			want:   floorAt(-7 * day),
		},
		{
			name:   "moves the floor up when the retention period is lowered",
			spec:   &v1.DatabaseServerArchiveSpec{ObjectStorageRef: "bucket", RetentionPeriodDays: 7},
			status: &v1.DatabaseServerArchiveStatus{ReachableFrom: floorAt(-30 * day)},
			now:    archiveOpenedAt,
			want:   floorAt(-7 * day),
		},
		{
			name:   "leaves the floor of a server that archives nothing",
			status: &v1.DatabaseServerArchiveStatus{ReachableFrom: floorAt(-7 * day)},
			now:    metav1.NewTime(archiveOpenedAt.Add(30 * day)),
			want:   floorAt(-7 * day),
		},
		{
			name: "writes no floor for a server that archives nothing",
			now:  archiveOpenedAt,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := &v1.DatabaseServer{Status: v1.DatabaseServerStatus{Archive: tt.status}}

			advanceArchiveFloor(server, v1.DatabaseServerSpec{Archive: tt.spec}, tt.now)

			if tt.want == nil {
				assert.Nil(t, server.Status.Archive)

				return
			}

			require.NotNil(t, server.Status.Archive)
			require.NotNil(t, server.Status.Archive.ReachableFrom)
			assert.True(
				t, server.Status.Archive.ReachableFrom.Equal(tt.want),
				server.Status.Archive.ReachableFrom,
			)
		})
	}
}

func TestHeldArchive(t *testing.T) {
	recorded := &v1.RecoveryArchiveRef{
		ServerName:          "camunda",
		ObjectStorageRef:    "backups",
		Location:            locationA,
		RetentionPeriodDays: 30,
		BaseBackupSchedule:  "0 0 2 * * *",
	}

	tests := []struct {
		name     string
		recorded *v1.RecoveryArchiveRef
		spec     *v1.DatabaseServerArchiveSpec
		want     v1.DatabaseServerArchiveSpec
	}{
		{
			name:     "takes every setting from the record when the spec keeps an archive",
			recorded: recorded,
			spec: &v1.DatabaseServerArchiveSpec{
				ObjectStorageRef:    "other",
				RetentionPeriodDays: 1,
				BaseBackupSchedule:  "0 0 5 * * *",
			},
			want: v1.DatabaseServerArchiveSpec{
				ObjectStorageRef:    "backups",
				RetentionPeriodDays: 30,
				BaseBackupSchedule:  "0 0 2 * * *",
			},
		},
		{
			name:     "takes every setting from the record when the spec removed the archive",
			recorded: recorded,
			spec:     nil,
			want: v1.DatabaseServerArchiveSpec{
				ObjectStorageRef:    "backups",
				RetentionPeriodDays: 30,
				BaseBackupSchedule:  "0 0 2 * * *",
			},
		},
		// A record written before the settings were recorded carries neither.
		{
			name: "falls back to the spec for the settings an older record lacks",
			recorded: &v1.RecoveryArchiveRef{
				ServerName:       "camunda",
				ObjectStorageRef: "backups",
				Location:         locationA,
			},
			spec: &v1.DatabaseServerArchiveSpec{
				ObjectStorageRef:    "other",
				RetentionPeriodDays: 7,
				BaseBackupSchedule:  "0 0 5 * * *",
			},
			want: v1.DatabaseServerArchiveSpec{
				ObjectStorageRef:    "backups",
				RetentionPeriodDays: 7,
				BaseBackupSchedule:  "0 0 5 * * *",
			},
		},
		{
			name: "renders no retention when neither the older record nor the spec holds one",
			recorded: &v1.RecoveryArchiveRef{
				ServerName:       "camunda",
				ObjectStorageRef: "backups",
				Location:         locationA,
			},
			spec: nil,
			want: v1.DatabaseServerArchiveSpec{ObjectStorageRef: "backups"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := heldArchive(tt.recorded, tt.spec)

			require.NotNil(t, got)
			assert.Equal(t, tt.want, *got)
		})
	}
}

// A rollback that a spec removed the archive under is held only while the
// record can describe that archive. An older record that carries no retention
// cannot: rendering it would put "0d" on the ObjectStore, which the CRD
// refuses, and publish a retention that the contract refuses. The removal
// applies instead, and the rollback is refused.
func TestRecoveryHoldsSpecOnArchiveRemoval(t *testing.T) {
	server := func(retentionDays int32) *v1.DatabaseServer {
		return &v1.DatabaseServer{
			Status: v1.DatabaseServerStatus{
				Recovery: &v1.DatabaseServerRecoveryStatus{
					Contract:    "camunda",
					RequestedBy: "ns/pitr-1",
					Archive: &v1.RecoveryArchiveRef{
						ServerName:          "camunda",
						ObjectStorageRef:    "backups",
						RetentionPeriodDays: retentionDays,
					},
				},
			},
		}
	}
	merged := v1.DatabaseServerSpec{DatabaseServerConfig: "camunda"}

	hold := recoveryHoldsSpec(server(30), merged)
	require.NotNil(t, hold)
	assert.Equal(t, v1.ReasonInvalidReference, hold.Reason)
	assert.Contains(t, hold.Message, "Put spec.archive back")

	assert.Nil(t, recoveryHoldsSpec(server(0), merged))
}
