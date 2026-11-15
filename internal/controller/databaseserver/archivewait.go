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
	"strings"
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
	"github.com/konsole-is/camunda-operator/pkg/pgbootstrap"
)

// defaultArchiveWait is how long a recovery waits for the archive to hold the
// write-ahead log of the requested point, counted from when the server
// recorded the request.
const defaultArchiveWait = 5 * time.Minute

// walTimeout bounds one look at the primary instance: the Secret read, the
// connection, and the query together.
const walTimeout = 10 * time.Second

// WriteAheadLog closes and reads the write-ahead log of the PostgreSQL server
// that a connection reaches.
type WriteAheadLog interface {
	// CloseSegment commits a transaction, ends the write-ahead log segment
	// that holds its commit record, and returns the name of that segment.
	// The server must be a primary.
	CloseSegment(ctx context.Context, conn pgbootstrap.Connection) (string, error)
	// LastArchived returns the name of the last file that the archiver of
	// the server reports as archived, or the empty string when it archived
	// none yet. The name is not always a segment: see archivedThrough.
	LastArchived(ctx context.Context, conn pgbootstrap.Connection) (string, error)
}

// awaitArchivedTarget reports whether the recovery that status.recovery
// records still waits for the archive it reads to hold the requested point.
// It sets startedAt and closedSegment of status.recovery when they are
// missing.
//
// The wait ends at once for an archive that the server no longer writes, for
// one whose uploads the server reports as failing, and when the server no
// longer owns the cluster it runs from. It ends at the latest when the wait
// passes, also when the archive has not shown that it holds the point: the
// recovery then decides, and CloudNativePG fails a recovery whose target the
// archive does not reach.
func (r *DatabaseServerReconciler) awaitArchivedTarget(
	ctx context.Context,
	server *v1.DatabaseServer,
	source v1.ArchiveRecord,
) bool {
	// An archive that the server stopped writing takes no more write-ahead
	// log, and one whose uploads fail takes none until they are repaired.
	// Waiting for either changes nothing, and the archive can still hold an
	// older point.
	if !readsLiveArchive(server, source) || reportsArchiveOutage(server) {
		return false
	}

	recovery := server.Status.Recovery
	now := time.Now()

	// A record that an older operator wrote has no start.
	if recovery.StartedAt == nil {
		recovery.StartedAt = new(metav1.NewTime(now))
	}
	log := logf.FromContext(ctx).WithValues("cluster", recovery.Cluster, "targetTime", recovery.TargetTime)

	if !now.Before(recovery.StartedAt.Add(r.archiveWait())) {
		log.Info(
			"Started a recovery before the archive showed that it holds the requested point",
			"closedSegment", recovery.ClosedSegment,
		)

		return false
	}

	ctx, cancel := context.WithTimeout(ctx, walTimeout)
	defer cancel()

	// The Secret and the primary of a cluster that another owner took hold
	// the database of that owner.
	owned, err := r.ownsRunningCluster(ctx, server)
	if err != nil {
		return unanswered(log, err)
	}
	if !owned {
		return false
	}

	conn, err := r.primaryConnection(ctx, server)
	if err != nil {
		return unanswered(log, err)
	}

	// Record then act: the segment goes into status before anything reads
	// the archive for it. A status write that is lost only closes one more
	// segment on the next look.
	if recovery.ClosedSegment == "" {
		segment, err := r.wal().CloseSegment(ctx, conn)
		if err != nil {
			return unanswered(log, err)
		}
		recovery.ClosedSegment = segment

		return true
	}

	last, err := r.wal().LastArchived(ctx, conn)
	if err != nil {
		return unanswered(log, err)
	}

	return !archivedThrough(last, recovery.ClosedSegment)
}

// readsLiveArchive reports whether source is the archive that the server
// writes now, the open record of its archive history.
func readsLiveArchive(server *v1.DatabaseServer, source v1.ArchiveRecord) bool {
	open := openArchiveRecord(server)

	return open != nil && open.ServerName == source.ServerName && open.Location == source.Location
}

// reportsArchiveOutage reports whether ArchiveReady, as the last reconcile left
// it, says that the uploads of the write-ahead log of server fail.
func reportsArchiveOutage(server *v1.DatabaseServer) bool {
	archive := meta.FindStatusCondition(server.Status.Conditions, v1.ConditionArchiveReady)

	return archive != nil && archive.Reason == v1.ReasonArchiveFailing
}

// archiveWait returns how long a recovery waits for its archive.
func (r *DatabaseServerReconciler) archiveWait() time.Duration {
	if r.ArchiveWait > 0 {
		return r.ArchiveWait
	}

	return defaultArchiveWait
}

// ownsRunningCluster reports whether the server owns the CloudNativePG
// cluster that it runs from. A cluster that is gone is not owned.
func (r *DatabaseServerReconciler) ownsRunningCluster(ctx context.Context, server *v1.DatabaseServer) (bool, error) {
	key := types.NamespacedName{Namespace: server.Namespace, Name: components.ClusterName(server)}

	// Live: another owner can take the name at any moment.
	var cluster cnpgv1.Cluster
	if err := r.APIReader.Get(ctx, key, &cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, fmt.Errorf("reading the cluster %s: %w", key, err)
	}

	return ownedByServer(server, &cluster), nil
}

// primaryConnection returns the connection to the primary instance of the
// cluster that the server runs now, as the superuser that CloudNativePG
// writes.
func (r *DatabaseServerReconciler) primaryConnection(
	ctx context.Context,
	server *v1.DatabaseServer,
) (pgbootstrap.Connection, error) {
	key := types.NamespacedName{Namespace: server.Namespace, Name: components.SuperuserSecretName(server)}

	// Live: the cache keeps the metadata of a Secret only.
	var secret corev1.Secret
	if err := r.APIReader.Get(ctx, key, &secret); err != nil {
		return pgbootstrap.Connection{}, fmt.Errorf("reading the superuser Secret %s: %w", key, err)
	}

	return pgbootstrap.Connection{
		Host:     components.ReadWriteHost(server),
		Port:     components.PostgresPort,
		User:     string(secret.Data[components.SuperuserUsernameKey]),
		Password: string(secret.Data[components.SuperuserPasswordKey]),
	}, nil
}

// unanswered is the answer of a look that could not reach the primary
// instance or its archiver: the recovery looks again.
func unanswered(log logr.Logger, err error) bool {
	log.V(1).Info("Could not read the write-ahead log of the primary instance", "error", err.Error())

	return true
}

// wal returns the write-ahead log operations of the reconciler.
func (r *DatabaseServerReconciler) wal() WriteAheadLog {
	if r.WAL != nil {
		return r.WAL
	}

	return primaryWAL{}
}

// archivedThrough reports whether last, the name that pg_stat_archiver gives
// for the last archived file, is segment or a later segment of its timeline.
//
// A name counts only when it is a segment or a .backup file, whose first 24
// characters name its segment. A .partial file, a .history file, a segment of
// another timeline, and the empty name count as nothing archived.
func archivedThrough(last, segment string) bool {
	archived, ok := segmentOf(last)
	if !ok {
		return false
	}

	wanted, ok := segmentOf(segment)

	// The first 8 characters are the timeline.
	return ok && archived[:8] == wanted[:8] && archived >= wanted
}

// segmentOf returns the segment that name is, or that a .backup file names,
// in upper case, and whether name is one of the two.
func segmentOf(name string) (string, bool) {
	const segmentLength = 24

	if len(name) < segmentLength || (len(name) > segmentLength && !strings.HasSuffix(name, ".backup")) {
		return "", false
	}

	prefix := strings.ToUpper(name[:segmentLength])
	if strings.Trim(prefix, "0123456789ABCDEF") != "" {
		return "", false
	}

	return prefix, true
}

// maintenanceDatabase is the database that the write-ahead log operations
// connect to. Both act on the whole server.
const maintenanceDatabase = "postgres"

// primaryWAL is the production WriteAheadLog over pgx.
type primaryWAL struct{}

// CloseSegment implements WriteAheadLog.
func (primaryWAL) CloseSegment(ctx context.Context, conn pgbootstrap.Connection) (string, error) {
	db, err := pgbootstrap.Open(ctx, conn, maintenanceDatabase)
	if err != nil {
		return "", err
	}
	defer func() { _ = db.Close(context.Background()) }()

	// A recovery to a time stops at the first commit or abort record after
	// that time. Without one, PostgreSQL ends the recovery before the target
	// and fails it. This transaction commits now, in autocommit, so the
	// segment that closes below holds such a record.
	if _, err := db.Exec(ctx, "SELECT pg_current_xact_id()"); err != nil {
		return "", fmt.Errorf("committing a transaction on %s: %w", conn.Host, err)
	}

	// pg_switch_wal returns the end of the segment it closed, and
	// pg_walfile_name names the segment that ends there.
	var segment string
	if err := db.QueryRow(ctx, "SELECT pg_walfile_name(pg_switch_wal())").Scan(&segment); err != nil {
		return "", fmt.Errorf("closing the write-ahead log segment on %s: %w", conn.Host, err)
	}

	return segment, nil
}

// LastArchived implements WriteAheadLog.
func (primaryWAL) LastArchived(ctx context.Context, conn pgbootstrap.Connection) (string, error) {
	db, err := pgbootstrap.Open(ctx, conn, maintenanceDatabase)
	if err != nil {
		return "", err
	}
	defer func() { _ = db.Close(context.Background()) }()

	var last *string
	if err := db.QueryRow(ctx, "SELECT last_archived_wal FROM pg_stat_archiver").Scan(&last); err != nil {
		return "", fmt.Errorf("reading the archiver of %s: %w", conn.Host, err)
	}
	if last == nil {
		return "", nil
	}

	return *last, nil
}
