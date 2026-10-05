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
	"context"
	"fmt"
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
)

// archiveMoved reports whether the archive the server writes now is not the
// one it wrote before.
//
// The location decides it. A record written before the location was recorded
// is placed by the bucket contract that named it, which is all such a record
// carries: the same contract reads as the same archive, and another one as a
// move. A server that has written no archive has not moved.
func archiveMoved(server *v1.DatabaseServer, ref, location string) bool {
	previousRef, previousLocation := previousArchive(server)

	switch {
	case previousRef == "" && previousLocation == "":
		return false
	case previousLocation == "":
		return ref != "" && previousRef != ref
	default:
		return location != "" && previousLocation != location
	}
}

// previousArchive returns the bucket contract and the location of the archive
// the server wrote before this reconcile: the ones an unspent boundary marks,
// the ones the open record names, or the ones of the last record it wrote.
func previousArchive(server *v1.DatabaseServer) (ref, location string) {
	status := server.Status.Archive
	if status == nil {
		return "", ""
	}
	if status.Boundary != nil {
		return status.Boundary.ObjectStorageRef, status.Boundary.Location
	}
	if open := openArchiveRecord(server); open != nil {
		return open.ObjectStorageRef, open.Location
	}
	if last := len(status.History) - 1; last >= 0 {
		return status.History[last].ObjectStorageRef, status.History[last].Location
	}

	return "", ""
}

// openArchiveRecord returns the archive the server writes now, or nil when it
// writes none. At most one record is open: closeArchiveRecords closes every
// one of them, and reconcileArchiveHistory closes the open record before it
// appends.
func openArchiveRecord(server *v1.DatabaseServer) *v1.ArchiveRecord {
	if server.Status.Archive == nil {
		return nil
	}

	for i := range server.Status.Archive.History {
		if server.Status.Archive.History[i].To == nil {
			return &server.Status.Archive.History[i]
		}
	}

	return nil
}

// archiveBoundary returns the point after which a base backup belongs to the
// archive the server writes now, or nil when nothing bounds it. It is read on
// a reconcile that finds no move; one that finds a move reads no backups.
//
// Two things bound the current archive, and the later of them wins: a record
// the server closed before, and a move it recorded on an earlier reconcile.
// The second is what covers a server with no interval open, which is where an
// archive that was disabled and re-enabled elsewhere leaves it.
func archiveBoundary(server *v1.DatabaseServer, merged v1.DatabaseServerSpec) *metav1.Time {
	closed := closedArchiveEnd(server)
	if merged.Archive == nil {
		return closed
	}

	if recorded := archiveBoundaryOf(server); recorded != nil {
		return laterTime(closed, &recorded.At)
	}

	return closed
}

// closedArchiveEnd returns when the archive of the current cluster last
// closed, or nil when the server has never closed one for it. A closed
// interval means the server stopped archiving and started again, so everything
// the new archive holds comes after that point.
func closedArchiveEnd(server *v1.DatabaseServer) *metav1.Time {
	if server.Status.Archive == nil {
		return nil
	}

	var end *metav1.Time
	for i := range server.Status.Archive.History {
		record := &server.Status.Archive.History[i]
		if record.ServerName != components.ClusterName(server) || record.To == nil {
			continue
		}
		if end == nil || record.To.After(end.Time) {
			end = record.To
		}
	}

	return end
}

// archiveBoundaryOf returns the move of the archive that no record holds yet,
// or nil when the server has none.
func archiveBoundaryOf(server *v1.DatabaseServer) *v1.ArchiveBoundary {
	if server.Status.Archive == nil {
		return nil
	}

	return server.Status.Archive.Boundary
}

// laterTime returns the later of a and b, or the one that is set when the
// other is nil.
func laterTime(a, b *metav1.Time) *metav1.Time {
	if a == nil {
		return b
	}
	if b == nil || a.After(b.Time) {
		return a
	}

	return b
}

// archiveStart returns when the earliest base backup of the archive the server
// writes now completed, or nil when none has. Only a backup that the archive
// plugin took counts. It is what tells the archive component that the archive
// can be recovered from, and what opens the interval of that archive in
// status.
//
// after is the boundary of the current archive, from archiveBoundary. Only the
// backups that began after it count: the backups of an archive the server
// wrote before reach no point in the one it writes now, so treating one of
// them as the start declares a window that no restore can reach. A backup that
// recorded no start is skipped there. The first archive of a server has no
// boundary, and every completed backup counts by its end. A server that asks
// for no archive takes no base backups, so it reads none.
func (r *DatabaseServerReconciler) archiveStart(
	ctx context.Context,
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
	after *metav1.Time,
) (*metav1.Time, error) {
	if merged.Archive == nil {
		return nil, nil
	}

	var backups cnpgv1.BackupList
	if err := r.List(
		ctx, &backups,
		client.InNamespace(server.Namespace),
		client.MatchingLabels{components.CNPGClusterNameLabel: components.ClusterName(server)},
	); err != nil {
		return nil, fmt.Errorf("listing base backups of %q: %w", components.ClusterName(server), err)
	}

	var earliest *metav1.Time
	for i := range backups.Items {
		backup := &backups.Items[i]
		// The label scopes the read. The cluster the backup names is what
		// decides, because this operator writes neither.
		if backup.Spec.Cluster.Name != components.ClusterName(server) ||
			backup.Status.Phase != cnpgv1.BackupPhaseCompleted ||
			backup.Status.StoppedAt == nil {
			continue
		}
		// A backup that another method or another plugin took writes nothing
		// to this archive, so it reaches no point a restore of it can use.
		if backup.Spec.Method != cnpgv1.BackupMethodPlugin ||
			backup.Spec.PluginConfiguration == nil ||
			backup.Spec.PluginConfiguration.Name != components.BarmanPluginName {
			continue
		}
		// The start decides, not the end. A backup that was already running
		// when the interval closed writes its base backup to the bucket the
		// server left: the plugin gave it that destination when it started,
		// and the one ObjectStore is rewritten in place. Its end falls after
		// the close, so an end-only test opens the new interval on an object
		// that the new bucket does not hold. A backup with no recorded start
		// sits on neither side of the boundary and goes the same way.
		//
		// That skip is a guard rather than a path a supported stack takes.
		// The Barman Cloud plugin reports the start of every backup it
		// completes, and CloudNativePG copies it into status.startedAt.
		if after != nil && (backup.Status.StartedAt == nil || !backup.Status.StartedAt.After(after.Time)) {
			continue
		}
		if earliest == nil || backup.Status.StoppedAt.Before(earliest) {
			earliest = backup.Status.StoppedAt
		}
	}

	return earliest, nil
}

// reportedArchiveOutage returns the stop in the write-ahead log uploads that
// the server reports on ArchiveReady and on its open archive record, or nil
// when it reports on none.
func reportedArchiveOutage(
	outage *components.ArchiveOutage,
	merged v1.DatabaseServerSpec,
) *components.ArchiveOutage {
	// An unconfirmed outage is none of them. CloudNativePG raises its condition
	// on one failed upload, and the plugin uploads the segment again, so a
	// reported outage waits for the grace period of
	// components.ArchiveOutageGracePeriod.
	//
	// A suspended server reports on none either. Its instances are gone, so it
	// writes no write-ahead log to lose, and what CloudNativePG left on the
	// condition describes the server that ran before the suspension. A server
	// that asks for no archive is the same case: the cluster carries no archive
	// plugin, and what stands on the condition is what the server archived
	// before.
	if outage == nil || !outage.Confirmed || merged.Suspend || !components.Archiving(merged) {
		return nil
	}

	return outage
}

// advanceArchiveFloor raises status.archive.reachableFrom to the point the
// retention period prunes the bucket to at now. A server that writes no
// archive prunes nothing, so its floor stands still.
func advanceArchiveFloor(server *v1.DatabaseServer, merged v1.DatabaseServerSpec, now metav1.Time) {
	if merged.Archive == nil {
		return
	}

	floor := metav1.NewTime(now.Add(-time.Duration(merged.Archive.RetentionPeriodDays) * day))
	if server.Status.Archive == nil {
		server.Status.Archive = &v1.DatabaseServerArchiveStatus{}
	}

	// The plugin prunes by the retention period that is in force when it runs,
	// and what it removes is gone. A raised period therefore lowers nothing.
	// The floor stays at the highest point any past period pruned to, and the
	// window widens to the new period only as the archive writes past it.
	if reached := server.Status.Archive.ReachableFrom; reached != nil && !floor.After(reached.Time) {
		return
	}

	server.Status.Archive.ReachableFrom = &floor
}

// reconcileArchiveHistory keeps status.archive.history in step with the spec.
//
// While the server archives, the interval of the current archive opens once
// the archive component reports ready. A recovery reaches only a point inside
// a recorded interval, so the record must exist before the first restore, and
// its start is when the first base backup of that archive completed: the
// archive cannot be recovered to any point before that.
//
// When the spec drops the archive, the open record closes at now and no record
// is written again. The closed records stay: the bucket still holds those
// archives, and a server that archives again can recover from them. A server
// that archives again opens a record of its own, so the window with no archive
// stays outside every interval and no restore can ask for a point in it.
//
// A spec that moves the archive to another location closes the open record the
// same way, at the moment the archive arrives there. Each record therefore
// names one location, and a restore of that interval knows where to read it.
// With no record open, the move is recorded as status.archive.boundary
// instead, and the next record opens after it.
//
// applied says that the ObjectStore of the new location reached the API
// server. Until it does, the plugin still writes to the location the server
// came from, so the record stays open and no boundary is written: a base
// backup that completes in that window belongs to the archive that is still
// being written. The move is found again on every reconcile until one of them
// applies it.
//
// A recovery closes the record of the cluster it replaces itself, at the
// moment the contract moves. What is left here is the record of an archive
// that another cluster of this server wrote and never closed, which only an
// operator of an older version can leave behind.
func reconcileArchiveHistory(
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
	archiveComp *component.Component,
	archiveStart *metav1.Time,
	location string,
	moved bool,
	applied bool,
	now metav1.Time,
) {
	if merged.Archive == nil {
		closeArchiveRecords(server, now)
		return
	}

	open := openArchiveRecord(server)
	if open != nil {
		// A record from before these fields existed names neither the bucket
		// nor the location. Nothing else can place it, so it takes what the
		// server writes to now.
		if open.ObjectStorageRef == "" {
			open.ObjectStorageRef = merged.Archive.ObjectStorageRef
		}
		// The location is adopted only into a record of the contract the
		// server writes through now. A record of another contract moved since,
		// and labelling it with the current location would call the archive it
		// holds the one the server writes.
		if open.Location == "" && open.ObjectStorageRef == merged.Archive.ObjectStorageRef {
			open.Location = location
		}
	}

	if moved {
		if !applied {
			return
		}

		// The interval of the location the server leaves ends here, at the
		// same now that archiveBoundary already gave the guard. The record of
		// the new location opens on a later look, once a base backup of it has
		// completed after this point. The boundary carries that point until
		// then, because a closed record alone cannot: an archive that was
		// disabled and re-enabled elsewhere closes no record at the move.
		if open != nil {
			open.To = &now
		}
		markArchiveBoundary(server, now, location, merged.Archive.ObjectStorageRef)

		return
	}

	if archiveStart == nil || archiveComp.GetCondition(server).Status != metav1.ConditionTrue {
		return
	}

	if open != nil {
		if open.ServerName == components.ClusterName(server) {
			return
		}
		// The server writes one archive at a time, so an archive of another
		// cluster that is still open ended where this one starts.
		open.To = archiveStart
	}

	if server.Status.Archive == nil {
		server.Status.Archive = &v1.DatabaseServerArchiveStatus{}
	}
	// The record holds the move from here on, so the boundary is spent.
	server.Status.Archive.Boundary = nil
	server.Status.Archive.History = append(server.Status.Archive.History, v1.ArchiveRecord{
		ServerName:       components.ClusterName(server),
		ObjectStorageRef: merged.Archive.ObjectStorageRef,
		Location:         location,
		From:             *archiveStart,
	})
}

// closeArchiveRecords ends the interval of every archive the server still has
// open. Only the drop of spec.archive calls it. The server then writes no
// archive at all, so every open interval is over, including one that another
// cluster of this server left open.
func closeArchiveRecords(server *v1.DatabaseServer, at metav1.Time) {
	if server.Status.Archive == nil {
		return
	}

	for i := range server.Status.Archive.History {
		if server.Status.Archive.History[i].To == nil {
			server.Status.Archive.History[i].To = &at
		}
	}
}

// markArchiveBoundary records that the archive of the server moved to location
// at now, with no interval open to hold the move.
func markArchiveBoundary(server *v1.DatabaseServer, now metav1.Time, location, bucket string) {
	if server.Status.Archive == nil {
		server.Status.Archive = &v1.DatabaseServerArchiveStatus{}
	}

	server.Status.Archive.Boundary = &v1.ArchiveBoundary{
		At:               now,
		Location:         location,
		ObjectStorageRef: bucket,
	}
}

// markArchiveOutage records on the open archive record the point from which
// the archive can be missing write-ahead log, and clears the point once the
// uploads run again. The plugin uploads the segments it held back then, so
// every point of the interval can be reached again.
//
// The mark states what the archive the server writes now is missing, and it is
// never history. A record that was closed while the uploads were failing loses
// it here, whichever of the closers ended it.
func markArchiveOutage(server *v1.DatabaseServer, outage *components.ArchiveOutage) {
	if server.Status.Archive == nil {
		return
	}

	for i := range server.Status.Archive.History {
		record := &server.Status.Archive.History[i]
		if record.To != nil {
			record.UnverifiedFrom = nil
		}
	}

	open := openArchiveRecord(server)
	if open == nil {
		return
	}

	if outage == nil {
		open.UnverifiedFrom = nil

		return
	}

	open.UnverifiedFrom = outage.Since.DeepCopy()
}

// stageArchiveOutage reports on ArchiveReady that the write-ahead log of the
// server stopped reaching the bucket. The guard on the archive blocks on the
// same outage, and ocf reports that as Blocked, which reads the same as the
// wait for the first base backup. This names what CloudNativePG reports and
// what the archive still holds.
func stageArchiveOutage(server *v1.DatabaseServer, outage *components.ArchiveOutage) {
	if outage == nil {
		return
	}

	stageFailure(
		server, v1.ConditionArchiveReady, v1.ReasonArchiveFailing,
		components.ArchiveFailingMessage(outage),
	)
}

// pendingArchiveOutageWait returns what is left of the grace period of a stop
// in the write-ahead log uploads that the server does not report on yet, or
// zero when it has none to wait out.
func pendingArchiveOutageWait(
	outage *components.ArchiveOutage,
	merged v1.DatabaseServerSpec,
	now time.Time,
) time.Duration {
	if outage == nil || outage.Confirmed || merged.Suspend || !components.Archiving(merged) {
		return 0
	}

	// The deadline can pass between the read of the cluster, which decided
	// Confirmed, and this call. The look is then due at once, and a zero here
	// would mean no look at all.
	return max(outage.Since.Add(components.ArchiveOutageGracePeriod).Sub(now), time.Millisecond)
}

// closeArchiveRecord ends the interval of the archive that serverName has
// open, and leaves every other record alone. A recovery closes the archive of
// the cluster it replaces, and the cluster it built can already have opened one
// of its own by then: its first base backup completes whenever CloudNativePG
// takes it, before or after the contract moves.
func closeArchiveRecord(server *v1.DatabaseServer, serverName string, at metav1.Time) {
	if server.Status.Archive == nil {
		return
	}

	for i := range server.Status.Archive.History {
		record := &server.Status.Archive.History[i]
		if record.ServerName == serverName && record.To == nil {
			record.To = &at
		}
	}
}
