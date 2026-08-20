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
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/pgbootstrap"
)

// clockSlack is how far the exporter clock of the database can run ahead of
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
// finds no table at all. The query takes no identifier from a variable, so it
// needs no quoting helper.
const exporterPositionQuery = `SELECT partition_id, last_updated FROM exporter_position`

// undefinedTable is the SQLSTATE that PostgreSQL reports for a table that does
// not exist.
const undefinedTable = "42P01"

// readTimeout bounds one read of the exporter positions, the connection and
// the query together. The reconcile context carries no deadline, and
// connect_timeout bounds only the handshake. A database that accepts the
// connection and then stalls must not hold the worker, because one stalled
// database then blocks every other restore.
const readTimeout = 30 * time.Second

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

	// The clock of the grace runs until the phase itself makes progress. A
	// chain that resolves is not progress here: the database read is the work
	// of this phase, and it is the part that a broken dependency stops.
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
		"the database holds no state after %s. The broker volumes are next",
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

	// The whole read ends within readTimeout, the injected reader included.
	// The deadline belongs to the caller, not to one implementation of the
	// seam.
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	positions, err := r.opts.ReadPositions(
		ctx, pgbootstrap.Connection{
			Host:     resolved.server.Spec.Host,
			Port:     resolved.server.Spec.Port,
			User:     user,
			Password: password,
		}, resolved.dbConfig.Spec.DatabaseName,
	)
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
	var ahead *conditions.PreCheckFailure

	for partition := int32(1); partition <= partitions; partition++ {
		at := slices.IndexFunc(positions, func(p v1.PartitionPosition) bool {
			return p.PartitionID == partition
		})
		if at < 0 {
			missing = append(missing, "partition "+strconv.FormatInt(int64(partition), 10))

			continue
		}

		if position := positions[at].LastUpdated.Time; position.After(latest) && ahead == nil {
			ahead = &conditions.PreCheckFailure{
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

	// A missing row outranks a row that is ahead. Both mean that the database
	// must be restored again, and a message that names one cause at a time
	// sends the reader back a second time.
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

	return ahead
}

// zoneVars are the environment variables that set the time zone of the broker.
// TZ sets the clock of the container. The others reach the JVM through the
// launcher of the distribution, which runs
// `exec "$JAVACMD" $JAVA_OPTS @EXTRA_JVM_ARGUMENTS@ $EXTRA_JVM_OPTS`
// (Camunda 8.9, dist/src/main/scripts/unixBinTemplate), plus the two variables
// that the JVM itself reads.
var zoneVars = []string{
	"TZ",
	"JAVA_TOOL_OPTIONS",
	"JDK_JAVA_OPTIONS",
	"_JAVA_OPTIONS",
	"JAVA_OPTS",
	"EXTRA_JVM_OPTS",
}

// utcZones are the zone names that make the wall clock of the broker the same
// clock as spec.timestamp. The comparison ignores case, as the zone database
// does.
var utcZones = []string{
	"UTC", "UTC0", "UCT", "ZULU", "UNIVERSAL", "GREENWICH", "GMT", "GMT0", "GMT+0", "GMT-0",
	"ETC/UTC", "ETC/UCT", "ETC/ZULU", "ETC/UNIVERSAL", "ETC/GREENWICH",
	"ETC/GMT", "ETC/GMT0", "ETC/GMT+0", "ETC/GMT-0",
}

// brokerClockComparable reports why the operator cannot compare the exporter
// position of the database with spec.timestamp, or nil when it can.
//
// The RDBMS exporter writes LAST_UPDATED with LocalDateTime.now() of the
// broker JVM, into a column that carries no zone (Camunda 8.9,
// RdbmsExporter.updatePositionInRdbms). The value is therefore the wall clock
// of the broker, while spec.timestamp is an instant in UTC. The two are the
// same clock only while the brokers run in UTC, which a container does by
// default and which the operator never changes.
//
// A cluster can change it through spec.zeebe.extraEnv and
// spec.zeebe.extraEnvFrom, so both are read here. A broker west of UTC writes
// a position that reads earlier than it is, a database that is ahead of the
// requested point passes the check, and the cluster loses its volumes. So a
// broker that names another zone, or a zone that the operator cannot read,
// holds the restore instead.
//
// The reader must be uncached, and it reads in the namespace of the cluster,
// which is where the kubelet resolves these sources too.
func brokerClockComparable(
	ctx context.Context,
	reader client.Reader,
	namespace string,
	broker *corev1.Container,
) (*conditions.PreCheckFailure, error) {
	// The kubelet builds the environment of a container in this order: every
	// envFrom source first, in order, then every env entry, which overrides a
	// name that a source already carried. The check judges the result, not the
	// layers. A cluster that carries TZ in a ConfigMap and sets TZ to UTC in
	// extraEnv runs in UTC.
	effective := map[string]string{}

	for _, source := range broker.EnvFrom {
		failure, err := collectSource(ctx, reader, namespace, source, effective)
		if err != nil || failure != nil {
			return failure, err
		}
	}

	for _, env := range broker.Env {
		if !slices.Contains(zoneVars, env.Name) {
			continue
		}
		if env.ValueFrom != nil {
			// This entry wins over whatever a source carried, and only the
			// kubelet resolves it, so the effective zone is unknowable here.
			return clockUnreadable(fmt.Sprintf(
				"the broker container takes %s from a reference, which only the kubelet resolves",
				env.Name,
			)), nil
		}
		effective[env.Name] = env.Value
	}

	// The names are sorted, so one environment always reports the same
	// variable.
	for _, name := range slices.Sorted(maps.Keys(effective)) {
		if failure := zoneFailure(name, effective[name]); failure != nil {
			return failure, nil
		}
	}

	return nil, nil
}

// collectSource resolves one envFrom source of the broker and records the
// variables of it that can set a zone, under the name the kubelet gives them.
// A source that the cluster marks optional and that does not exist carries no
// variable. Any other source that the operator cannot read holds the restore:
// an unread source can carry the one variable that makes this check wrong.
func collectSource(
	ctx context.Context,
	reader client.Reader,
	namespace string,
	source corev1.EnvFromSource,
	effective map[string]string,
) (*conditions.PreCheckFailure, error) {
	switch {
	case source.ConfigMapRef != nil:
		var config corev1.ConfigMap
		key := types.NamespacedName{Namespace: namespace, Name: source.ConfigMapRef.Name}
		if err := reader.Get(ctx, key, &config); err != nil {
			return unreadableSource(err, "ConfigMap", key, source.ConfigMapRef.Optional)
		}
		collectZoneVars(source.Prefix, config.Data, effective)
	case source.SecretRef != nil:
		var secret corev1.Secret
		key := types.NamespacedName{Namespace: namespace, Name: source.SecretRef.Name}
		if err := reader.Get(ctx, key, &secret); err != nil {
			return unreadableSource(err, "Secret", key, source.SecretRef.Optional)
		}

		data := make(map[string]string, len(secret.Data))
		for name, value := range secret.Data {
			data[name] = string(value)
		}
		collectZoneVars(source.Prefix, data, effective)
	}

	return nil, nil
}

// collectZoneVars records every entry of one source that can set a zone, under
// the name the kubelet gives it.
func collectZoneVars(prefix string, data, effective map[string]string) {
	for name, value := range data {
		if slices.Contains(zoneVars, prefix+name) {
			effective[prefix+name] = value
		}
	}
}

// unreadableSource maps the read error of an envFrom source. A missing source
// that the cluster marks optional carries nothing. A missing source that it
// does not mark optional keeps the brokers from starting at all, and the
// operator cannot read the zone either way. Any other error is transient and
// goes back to the caller.
func unreadableSource(
	err error,
	kind string,
	key types.NamespacedName,
	optional *bool,
) (*conditions.PreCheckFailure, error) {
	if !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("reading the %s %s of the broker environment: %w", kind, key, err)
	}
	if optional != nil && *optional {
		return nil, nil
	}

	return clockUnreadable(fmt.Sprintf(
		"the broker container takes its environment from the %s %s, which does not exist", kind, key,
	)), nil
}

// zoneFailure reports why the variable name with the given value makes the
// wall clock of the broker something other than UTC, or nil when it does not.
// A variable that carries no zone at all is not this check's business.
func zoneFailure(name, value string) *conditions.PreCheckFailure {
	if !slices.Contains(zoneVars, name) {
		return nil
	}

	if name == "TZ" {
		if utc(value) {
			return nil
		}

		return clockUnreadable(fmt.Sprintf("the broker container runs with TZ %q", value))
	}

	zone, set := userTimezone(value)
	if !set || utc(zone) {
		return nil
	}

	return clockUnreadable(fmt.Sprintf(
		"the broker container runs with -Duser.timezone=%s in %s", zone, name,
	))
}

// clockUnreadable builds the hold of a broker whose clock the operator cannot
// compare with spec.timestamp.
func clockUnreadable(what string) *conditions.PreCheckFailure {
	return &conditions.PreCheckFailure{
		Reason: v1.ReasonPitrUnavailable,
		Message: fmt.Sprintf(
			"%s. The database records the exporter position with the wall clock of the broker and no "+
				"zone, so the operator can compare it with spec.timestamp only while the brokers run "+
				"in UTC. Run the brokers in UTC, then create the restore again",
			what,
		),
	}
}

// userTimezone returns the value of the last -Duser.timezone option in a Java
// options string, and whether it names one at all.
func userTimezone(options string) (string, bool) {
	const flag = "-Duser.timezone="

	zone := ""
	set := false
	for option := range strings.FieldsSeq(options) {
		if after, found := strings.CutPrefix(option, flag); found {
			zone, set = after, true
		}
	}

	return zone, set
}

// utc reports whether zone names UTC. An empty value names no zone, and the
// container then runs in UTC.
func utc(zone string) bool {
	return zone == "" || slices.Contains(utcZones, strings.ToUpper(zone))
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
