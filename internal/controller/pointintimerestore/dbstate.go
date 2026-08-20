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

package pointintimerestore

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/pgbootstrap"
)

// clockSlack is how far the exporter clock of the database may run ahead of
// spec.timestamp before the restore refuses to start. The database writes
// LAST_UPDATED with its own clock, and the timestamp of the caller comes from
// another one. One minute bounds an ordinary difference without hiding a
// database that nobody rolled back.
const clockSlack = time.Minute

// exporterPositionQuery reads the exporter position of every partition of the
// Camunda schema.
//
// The identifiers are unquoted on purpose. Camunda creates the table through
// Liquibase without a quoting strategy, and its own mapper reads it unquoted
// too, so PostgreSQL folds every name to lower case. A quoted upper-case name
// would find no table at all. The query takes no identifier from a variable,
// so it needs no quoting helper.
const exporterPositionQuery = `SELECT partition_id, last_updated FROM exporter_position`

// undefinedTable is the SQLSTATE that PostgreSQL reports for a table that does
// not exist.
const undefinedTable = "42P01"

// errNoExporterTable reports that the database carries no exporter position
// table. An empty database is exactly the state that the check must catch, so
// it is a hold and not a transient error.
var errNoExporterTable = errors.New("the database has no exporter position table")

// enterDatabaseState resolves the storage chain again and reads the database.
// The restore already left Pending, so a dependency that stopped resolving
// bounds it: the restore holds through the mid-run grace and then fails.
func (r *Reconciler) enterDatabaseState(
	ctx context.Context,
	pitr *v1.PointInTimeRestore,
) (hold, error) {
	resolved, failure, err := r.resolve(ctx, pitr)
	if err != nil {
		return r.resolveFailed(pitr, err)
	}
	if failure != nil {
		return r.holdStarted(pitr, failure), nil
	}
	recovered(pitr)

	return r.validateDatabaseState(ctx, pitr, resolved)
}

// validateDatabaseState reads the exporter position of every partition from
// the restored database and records what it read in status.observedPositions.
// It runs before the operator touches a volume, and it is the reason the
// common error of this flow costs nothing: a database that nobody restored
// holds the restore in Pending instead of losing the primary storage of the
// cluster.
//
// A database that is ahead of spec.timestamp sends the restore back to
// Pending, where it waits without a bound and recovers on its own. A database
// that the operator cannot reach holds the restore in this phase, where the
// mid-run grace bounds it.
func (r *Reconciler) validateDatabaseState(
	ctx context.Context,
	pitr *v1.PointInTimeRestore,
	resolved *chain,
) (hold, error) {
	positions, failure, err := r.readState(ctx, resolved)
	if err != nil {
		return settle, err
	}
	if failure != nil {
		if failure.Reason == v1.ReasonDatabaseNotRestored {
			return r.waiting(pitr, failure), nil
		}

		return r.holdStarted(pitr, failure), nil
	}

	pitr.Status.ObservedPositions = positions

	if failure := decide(
		positions, resolved.target.Partitions, pitr.Spec.Timestamp.Time, clockSlack,
	); failure != nil {
		return r.waiting(pitr, failure), nil
	}
	recovered(pitr)

	pitr.Status.Phase = v1.PointInTimeRestoreRestoringPrimaryStorage
	r.progressing(pitr, fmt.Sprintf(
		"the database holds no state after %s; the broker volumes are next",
		pitr.Spec.Timestamp.UTC().Format(time.RFC3339),
	))

	return shortly, nil
}

// readState opens the logical database of the cluster with the application
// credentials and reads the exporter positions. A database that carries no
// exporter position table is a DatabaseNotRestored hold: the caller restored
// nothing into it, or restored another database.
func (r *Reconciler) readState(
	ctx context.Context,
	resolved *chain,
) ([]v1.PartitionPosition, *conditions.PreCheckFailure, error) {
	user, password, failure, err := r.credentials(ctx, resolved.dbConfig)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	positions, err := r.opts.ReadPositions(ctx, pgbootstrap.Connection{
		Host:     resolved.server.Spec.Host,
		Port:     resolved.server.Spec.Port,
		User:     user,
		Password: password,
	}, resolved.dbConfig.Spec.DatabaseName)
	switch {
	case errors.Is(err, errNoExporterTable):
		return nil, &conditions.PreCheckFailure{
			Reason: v1.ReasonDatabaseNotRestored,
			Message: fmt.Sprintf(
				"the database %q carries no exporter position, so it holds no Camunda state: %s",
				resolved.dbConfig.Spec.DatabaseName, err,
			),
		}, nil
	case err != nil:
		return nil, &conditions.PreCheckFailure{
			Reason: v1.ReasonConnectionFailed,
			Message: fmt.Sprintf(
				"the operator cannot read the exporter position of the database %q on %s: %s",
				resolved.dbConfig.Spec.DatabaseName, resolved.server.Spec.Host, err,
			),
		}, nil
	}

	return positions, nil, nil
}

// decide reports why the database is not ready for the requested point, or nil
// when it is. The database is ready when it reports a position for every
// partition of the cluster and no position lies later than want plus slack.
//
// A position that lies before want is safe: Zeebe re-exports the difference
// after the restore. A position for a partition that the cluster does not run
// says nothing about this cluster, so it holds nothing back.
func decide(
	positions []v1.PartitionPosition,
	partitions int32,
	want time.Time,
	slack time.Duration,
) *conditions.PreCheckFailure {
	if len(positions) == 0 {
		return &conditions.PreCheckFailure{
			Reason: v1.ReasonDatabaseNotRestored,
			Message: "the database reports no exporter position at all, so it holds no restored " +
				"Camunda state",
		}
	}

	latest := want.Add(slack)
	missing := make([]string, 0, partitions)
	for partition := int32(1); partition <= partitions; partition++ {
		at := slices.IndexFunc(positions, func(p v1.PartitionPosition) bool {
			return p.PartitionID == partition
		})
		if at < 0 {
			missing = append(missing, "partition "+strconv.FormatInt(int64(partition), 10))

			continue
		}

		if position := positions[at].LastUpdated.Time; position.After(latest) {
			return &conditions.PreCheckFailure{
				Reason: v1.ReasonDatabaseNotRestored,
				Message: fmt.Sprintf(
					"the database is ahead of the requested point: partition %d last exported at %s, "+
						"which is later than spec.timestamp %s. Restore the database to the requested "+
						"point, and the restore continues on its own",
					partition,
					position.UTC().Format(time.RFC3339),
					want.UTC().Format(time.RFC3339),
				),
			}
		}
	}

	if len(missing) > 0 {
		return &conditions.PreCheckFailure{
			Reason: v1.ReasonDatabaseNotRestored,
			Message: fmt.Sprintf(
				"the database reports no exporter position for %s of the %d partitions of the "+
					"cluster, so it does not hold the state of the requested point",
				strings.Join(missing, ", "), partitions,
			),
		}
	}

	return nil
}

// readPositions is the production reader: it opens the logical database with
// the credentials of the caller and reads one row per partition. It is the
// default of Options.ReadPositions.
func readPositions(
	ctx context.Context,
	conn pgbootstrap.Connection,
	database string,
) ([]v1.PartitionPosition, error) {
	db, err := pgbootstrap.Open(ctx, conn, database)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close(ctx) }()

	rows, err := db.Query(ctx, exporterPositionQuery)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == undefinedTable {
			return nil, fmt.Errorf("%w: %s", errNoExporterTable, pgErr.Message)
		}

		return nil, fmt.Errorf("reading the exporter positions of %q: %w", database, err)
	}

	positions, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (v1.PartitionPosition, error) {
		var (
			partition int32
			updated   time.Time
		)
		if err := row.Scan(&partition, &updated); err != nil {
			return v1.PartitionPosition{}, err
		}

		return v1.PartitionPosition{
			PartitionID: partition, LastUpdated: metav1.NewTime(updated),
		}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading the exporter positions of %q: %w", database, err)
	}

	slices.SortFunc(positions, func(a, b v1.PartitionPosition) int {
		return int(a.PartitionID - b.PartitionID)
	})

	return positions, nil
}
