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
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/restore"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
)

// day is how long the retention period of a database server counts one day.
const day = 24 * time.Hour

// chain is what a phase of the restore resolves from the cluster: the cluster
// itself, its storage contract, the logical database behind it, the server
// that holds the database, and the facts of the live broker StatefulSet.
type chain struct {
	cluster  *v1.CamundaCluster
	storage  *v1.SecondaryStorageConfig
	dbConfig *v1.DatabaseConfig
	server   *v1.DatabaseServerConfig
	target   *restore.Target
}

// errClusterReplaced reports that the cluster of the restore is not the
// cluster that the restore pinned. A cluster that was deleted and created
// again under one name is another cluster with other primary storage, so the
// restore ends instead of writing into it.
var errClusterReplaced = errors.New("the CamundaCluster was replaced")

// admit runs every rule that must hold before the operator reads the
// database, in the documented order. A rule that does not hold keeps the
// restore in Pending, where it touches nothing and recovers on its own once
// the cause is gone.
//
// Admission ends by reading the database in the same reconcile. The read is
// the first call that leaves the cluster, but it changes nothing, and a
// restore that the database holds back must report Pending. The phase is
// staged first, so the one status write of this reconcile records whichever
// of the two outcomes the read produced.
func (r *Reconciler) admit(
	ctx context.Context,
	pitr *v1.PointInTimeRestore,
) (restore.Outcome, error) {
	resolved, failure, err := r.resolve(ctx, pitr)
	if err != nil {
		return r.resolveFailed(pitr, err)
	}
	if failure != nil {
		return r.waiting(pitr, failure), nil
	}

	if failure := pitrAvailable(resolved.server, pitr.Spec.Timestamp.Time); failure != nil {
		return r.waiting(pitr, failure), nil
	}

	// The database-state check compares the wall clock of the broker with
	// spec.timestamp, so a cluster whose brokers run in another zone never
	// reaches that check.
	failure, err = brokerClockComparable(
		ctx, r.APIReader, resolved.cluster.Namespace, resolved.target.Broker,
	)
	if err != nil {
		return restore.Outcome{}, err
	}
	if failure != nil {
		return r.waiting(pitr, failure), nil
	}

	failure, err = r.dedicatedServer(ctx, resolved.server)
	if err != nil {
		return restore.Outcome{}, err
	}
	if failure != nil {
		return r.waiting(pitr, failure), nil
	}

	// Every rule of this restore holds, so the cluster becomes this restore's
	// alone. The claim point is the same for all three restore kinds, and it
	// comes before every phase that touches storage. Two restores of one
	// cluster therefore never both pass validation.
	claimed, err := restore.Take(
		ctx, r.Client, r.APIReader, pitr.Namespace, pitr.Spec.ClusterRef.Name, claimant(pitr),
	)
	if err != nil {
		return restore.Outcome{}, err
	}
	if claimed.Failure != nil {
		return r.waiting(pitr, claimed.Failure), nil
	}

	// Everything that this restore is allowed to act on is now known: the
	// chain, the rules of the server, and the clock of the brokers. The record
	// goes in before the database is read, so every later look is measured
	// against the chain that the read used.
	pitr.Status.Storage = pinnedChain(resolved.storage, resolved.dbConfig, resolved.server)
	pitr.Status.Phase = v1.PointInTimeRestoreValidatingDatabaseState

	return r.validateDatabaseState(ctx, pitr, resolved)
}

// resolve reads the storage chain of the restore and the facts of the live
// broker StatefulSet. Every reference that does not resolve, and every state
// that a restore must not run against, comes back as a
// *conditions.PreCheckFailure whose message names what is wrong. The phase
// decides how long such a failure holds the restore.
//
// The reads are live. A stale suspend flag or a stale storage reference lets
// the restore delete the volumes of a cluster that moved on.
func (r *Reconciler) resolve(
	ctx context.Context,
	pitr *v1.PointInTimeRestore,
) (*chain, *conditions.PreCheckFailure, error) {
	namespace := pitr.Namespace
	name := pitr.Spec.ClusterRef.Name

	var cluster v1.CamundaCluster
	key := types.NamespacedName{Namespace: namespace, Name: name}
	if err := r.APIReader.Get(ctx, key, &cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference("CamundaCluster %s does not exist", key), nil
		}

		return nil, nil, fmt.Errorf("reading CamundaCluster %s: %w", key, err)
	}
	// The identity of the cluster is pinned at the first look, before any rule
	// runs. A restore that waits for a rule keeps waiting for the cluster it
	// read, and a cluster that was deleted and created again under one name is
	// another cluster with other primary storage.
	if pitr.Status.TargetClusterUID == "" {
		pitr.Status.TargetClusterUID = cluster.UID
	}
	if cluster.UID != pitr.Status.TargetClusterUID {
		return nil, nil, fmt.Errorf(
			"%w: CamundaCluster %s was replaced. It admitted the restore with UID %s and now has "+
				"UID %s, so its primary storage is not the storage this restore validated",
			errClusterReplaced, key, pitr.Status.TargetClusterUID, cluster.UID,
		)
	}

	// The operator only reads the suspend state. Whoever owns the cluster
	// suspends it before the restore and unsuspends it after. Never write
	// spec.suspend here: this operator does not own the cluster spec.
	if !cluster.Spec.Suspend {
		return nil, &conditions.PreCheckFailure{
			Reason: v1.ReasonClusterNotSuspended,
			Message: fmt.Sprintf(
				"CamundaCluster %s is not suspended. Set spec.suspend to true, so that no workload "+
					"writes while the restore runs", key,
			),
		}, nil
	}

	if cluster.Spec.BackupStorageRef == "" {
		return nil, logicalbackup.InvalidReference(
			"CamundaCluster %s has no spec.backupStorageRef, so Zeebe holds no primary storage "+
				"backup for the restore to read", key,
		), nil
	}

	if cluster.Spec.StorageRef == "" {
		return nil, logicalbackup.InvalidReference("CamundaCluster %s has no spec.storageRef", key), nil
	}

	var storage v1.SecondaryStorageConfig
	storageKey := types.NamespacedName{Namespace: namespace, Name: cluster.Spec.StorageRef}
	if err := r.APIReader.Get(ctx, storageKey, &storage); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference("SecondaryStorageConfig %s does not exist", storageKey), nil
		}

		return nil, nil, fmt.Errorf("reading SecondaryStorageConfig %s: %w", storageKey, err)
	}
	if storage.Spec.Type != v1.SecondaryStorageTypeRDBMS || storage.Spec.RDBMS == nil {
		return nil, logicalbackup.InvalidReference(
			"SecondaryStorageConfig %s stores the data of CamundaCluster %s in %s. A point-in-time "+
				"restore exists only for a relational database, because an Elasticsearch cluster has "+
				"no point-in-time recovery. Use a LogicalRestoreElasticsearch or a "+
				"LogicalRestoreRDBMS instead",
			storageKey, key, storage.Spec.Type,
		), nil
	}

	var dbConfig v1.DatabaseConfig
	dbKey := types.NamespacedName{Namespace: namespace, Name: storage.Spec.RDBMS.DatabaseConfigRef}
	if err := r.APIReader.Get(ctx, dbKey, &dbConfig); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference("DatabaseConfig %s does not exist", dbKey), nil
		}

		return nil, nil, fmt.Errorf("reading DatabaseConfig %s: %w", dbKey, err)
	}

	var server v1.DatabaseServerConfig
	serverKey := types.NamespacedName{Name: dbConfig.Spec.ServerRef}
	if err := r.APIReader.Get(ctx, serverKey, &server); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference("DatabaseServerConfig %q does not exist", serverKey.Name), nil
		}

		return nil, nil, fmt.Errorf("reading DatabaseServerConfig %q: %w", serverKey.Name, err)
	}

	if err := pinnedChainCurrent(pitr, &storage, &dbConfig, &server); err != nil {
		return nil, nil, err
	}

	target, failure, err := restore.ResolveTarget(ctx, r.APIReader, &cluster)
	if err != nil || failure != nil {
		return nil, failure, err
	}
	// The broker count is pinned at the first look, not followed. A restore
	// recreates the volumes of the brokers it read and runs a Job for each of
	// them, and a cluster that is scaled while it runs changes neither.
	if pitr.Status.Brokers == 0 {
		pitr.Status.Brokers = target.Brokers
	}

	return &chain{
		cluster:  &cluster,
		storage:  &storage,
		dbConfig: &dbConfig,
		server:   &server,
		target:   target,
	}, nil, nil
}

// errChainChanged reports that the storage chain of the cluster is no longer
// the chain that the restore validated.
var errChainChanged = errors.New("the storage chain of the cluster changed")

// pinnedChain records what the restore validated. The rules of the server and
// the state of the database are checked once, and every link of the chain is
// mutable, so the record is what every later look is measured against.
func pinnedChain(
	storage *v1.SecondaryStorageConfig,
	dbConfig *v1.DatabaseConfig,
	server *v1.DatabaseServerConfig,
) *v1.PointInTimeRestoreStorage {
	return &v1.PointInTimeRestoreStorage{
		SecondaryStorageConfig:    storage.Name,
		SecondaryStorageConfigUID: storage.UID,
		DatabaseConfig:            dbConfig.Name,
		DatabaseConfigUID:         dbConfig.UID,
		DatabaseServerConfig:      server.Name,
		DatabaseServerConfigUID:   server.UID,
		DatabaseName:              dbConfig.Spec.DatabaseName,
		Endpoint:                  fmt.Sprintf("%s:%d", server.Spec.Host, server.Spec.Port),
	}
}

// pinnedChainCurrent reports whether the chain of this look is the chain that
// the restore validated. A restore that has pinned nothing yet passes: it is
// on its first look, and admission pins the chain before it reads the
// database.
//
// The check is what keeps the destructive phase honest. The rules of the
// server, the dedicated-server rule, the clock of the brokers, and the
// exporter position are all read once, against one chain. A cluster that is
// repointed afterwards carries another database, which nothing validated.
func pinnedChainCurrent(
	pitr *v1.PointInTimeRestore,
	storage *v1.SecondaryStorageConfig,
	dbConfig *v1.DatabaseConfig,
	server *v1.DatabaseServerConfig,
) error {
	pinned := pitr.Status.Storage
	if pinned == nil {
		return nil
	}

	current := pinnedChain(storage, dbConfig, server)
	if *current == *pinned {
		return nil
	}

	return fmt.Errorf(
		"%w: the restore validated %s of DatabaseConfig %s on %s at %s, and the cluster now uses %s "+
			"of DatabaseConfig %s on %s at %s. Create a new restore for the database the cluster "+
			"uses now",
		errChainChanged,
		pinned.SecondaryStorageConfig, pinned.DatabaseConfig, pinned.DatabaseName, pinned.Endpoint,
		current.SecondaryStorageConfig, current.DatabaseConfig, current.DatabaseName, current.Endpoint,
	)
}

// resolveFailed maps an error of resolve onto the outcome of the phase. A
// cluster that was replaced, and a storage chain that moved, both end the
// restore: nothing of what it validated pairs with what the cluster carries
// now. Every other error is a transient read that the caller retries.
func (r *Reconciler) resolveFailed(
	pitr *v1.PointInTimeRestore,
	err error,
) (restore.Outcome, error) {
	if errors.Is(err, errClusterReplaced) || errors.Is(err, errChainChanged) {
		r.fail(pitr, v1.ReasonFailed, err.Error())

		return restore.Outcome{}, nil
	}

	return restore.Outcome{}, err
}

// pitrAvailable reports why the server cannot serve the requested point, or
// nil when it can. The DatabaseServerConfig is the capability declaration of
// the server. This operator never restores the server itself, and it never
// uses the admin credentials of the contract.
func pitrAvailable(server *v1.DatabaseServerConfig, want time.Time) *conditions.PreCheckFailure {
	pitr := server.Spec.PITR
	if pitr == nil || !pitr.Enabled {
		return &conditions.PreCheckFailure{
			Reason: v1.ReasonPitrUnavailable,
			Message: fmt.Sprintf(
				"DatabaseServerConfig %q declares no point-in-time recovery. Set spec.pitr.enabled "+
					"to true once the server archives its write-ahead log", server.Name,
			),
		}
	}

	// The clock is the reason this rule cannot be a CEL rule on the schema.
	if want.After(time.Now()) {
		return &conditions.PreCheckFailure{
			Reason: v1.ReasonPitrUnavailable,
			Message: fmt.Sprintf(
				"spec.timestamp %s lies in the future. A database holds no state of a point that "+
					"did not happen yet", want.UTC().Format(time.RFC3339),
			),
		}
	}

	days := int32(0)
	if pitr.RetentionPeriodDays != nil {
		days = *pitr.RetentionPeriodDays
	}
	if oldest := time.Now().Add(-time.Duration(days) * day); want.Before(oldest) {
		return &conditions.PreCheckFailure{
			Reason: v1.ReasonPitrUnavailable,
			Message: fmt.Sprintf(
				"spec.timestamp %s is older than the retention period of DatabaseServerConfig %q, "+
					"which is %d days. The server archives nothing of that point any more",
				want.UTC().Format(time.RFC3339), server.Name, days,
			),
		}
	}

	return nil
}

// dedicatedServer reports that more than one Database lives on the server, or
// nil when exactly one does. Point-in-time recovery on the engine rolls back
// the whole server, not one logical database, so a shared server rolls back
// the database of another cluster too.
//
// A DatabaseServerConfig is cluster-scoped, so the rule counts the Database
// resources of every namespace.
func (r *Reconciler) dedicatedServer(
	ctx context.Context,
	server *v1.DatabaseServerConfig,
) (*conditions.PreCheckFailure, error) {
	// The read is live and unindexed, like every other read of this
	// controller. A cached list can miss the Database that a sibling cluster
	// created a moment ago, and the rule exists to protect that sibling. The
	// filter runs here because a cluster holds few Database resources, and an
	// index of this field repeats what the Database controller already
	// indexes, which one manager rejects.
	var databases v1.DatabaseList
	if err := r.APIReader.List(ctx, &databases); err != nil {
		return nil, fmt.Errorf("listing the databases of DatabaseServerConfig %q: %w", server.Name, err)
	}

	names := make([]string, 0, 2)
	for i := range databases.Items {
		if databases.Items[i].Spec.ServerRef != server.Name {
			continue
		}
		names = append(names, databases.Items[i].Namespace+"/"+databases.Items[i].Name)
	}

	// Zero is not one. A server that no Database resource names carries no
	// evidence at all: the operator cannot tell whether it holds one database
	// or ten, and point-in-time recovery rolls back all of them. On a path
	// that deletes volumes, the absence of evidence holds the restore.
	if len(names) == 0 {
		return logicalbackup.InvalidReference(
			"no Database resource names DatabaseServerConfig %q, so the operator cannot tell which "+
				"databases the server holds. Point-in-time recovery rolls back the whole server. "+
				"Declare the database of the cluster as a Database resource on a server of its own",
			server.Name,
		), nil
	}

	if len(names) == 1 {
		return nil, nil
	}
	slices.Sort(names)

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonSharedServer,
		Message: fmt.Sprintf(
			"DatabaseServerConfig %q holds the databases %s. Point-in-time recovery rolls back the "+
				"whole server, so it also rolls back a database that another cluster uses. Move the "+
				"cluster to a server of its own",
			server.Name, strings.Join(names, ", "),
		),
	}, nil
}

// credentials reads the application credentials of the logical database. They
// are the credentials that the brokers use, resolved through the storage chain
// exactly as the restore application resolves them.
func (r *Reconciler) credentials(
	ctx context.Context,
	dbConfig *v1.DatabaseConfig,
) (user, password string, failure *conditions.PreCheckFailure, err error) {
	ref := dbConfig.Spec.CredentialsSecretRef
	key := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}

	secret, message, err := secretref.Get(ctx, r.APIReader, key, ref.UsernameKey, ref.PasswordKey)
	if err != nil {
		return "", "", nil, fmt.Errorf(
			"reading the credentials of DatabaseConfig %s: %w",
			client.ObjectKeyFromObject(dbConfig), err,
		)
	}
	if message != "" {
		return "", "", &conditions.PreCheckFailure{
			Reason: v1.ReasonMissingSecret,
			Message: fmt.Sprintf(
				"the application credentials of DatabaseConfig %s are not readable: %s",
				client.ObjectKeyFromObject(dbConfig), message,
			),
		}, nil
	}

	return string(secret.Data[ref.UsernameKey]), string(secret.Data[ref.PasswordKey]), nil, nil
}
