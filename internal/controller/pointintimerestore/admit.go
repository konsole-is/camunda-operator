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
	databasecomponents "github.com/konsole-is/camunda-operator/pkg/components/database"
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

// chainPin says how a look compares the storage chain it resolved against the
// chain the restore pinned. The caller chooses it, because only the caller
// knows whether the restore itself asked for the server to change.
type chainPin int

const (
	// pinExact binds every field of the pinned chain. It also requires that
	// the contract publishes an identity, and that the record which published
	// it describes the endpoint and the admin user that the spec names now.
	pinExact chainPin = iota
	// pinAcrossRecovery binds every field except the endpoint. That is the
	// one a rollback of the server replaces, and the phase that asked for the
	// rollback records the new one itself. The system identifier still binds:
	// a physical recovery restores the pg_control of the base backup, so the
	// instance behind the new endpoint reports the identity the restore
	// pinned, and one that reports another identity is another server. The
	// contracts, their identities, and the logical database bind as well: one
	// that was deleted and created again under its name is another contract,
	// mid-rollback as much as before it.
	pinAcrossRecovery
)

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
// It ends by claiming the cluster and suspending it. The claim comes first,
// because it is what serialises the operations on a cluster, and admission is
// about to write to that cluster's spec.
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
	resolved, failure, err := r.resolve(ctx, pitr, pinExact)
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

	failure, err = r.dedicatedServer(ctx, resolved.server, resolved.dbConfig)
	if err != nil {
		return restore.Outcome{}, err
	}
	if failure != nil {
		return r.waiting(pitr, failure), nil
	}

	// Every rule of this restore holds, so the cluster becomes this restore's
	// alone. The claim point is the same for all three restore kinds, and it
	// comes before every phase that touches storage, and before the restore
	// writes anything on the cluster spec. Two restores of one cluster
	// therefore never both pass validation.
	claimed, err := restore.Take(
		ctx, r.Client, r.APIReader, pitr, pitr.Spec.ClusterRef.Name,
	)
	if err != nil {
		return restore.Outcome{}, err
	}
	if claimed.Failure != nil {
		return r.waiting(pitr, claimed.Failure), nil
	}

	// The restore suspends the cluster and waits until its brokers stopped.
	// It writes no version: this kind restores the primary storage of the
	// cluster from the backups of that same cluster, so no backup records a
	// version that the cluster is not already running.
	prepared, err := restore.Prepare(
		ctx, r.Client, &pitr.Status.RestoreProgress, restore.PrepareInput{
			Owner:   pitr,
			Cluster: resolved.cluster,
			Target:  resolved.target,
			Poll:    r.opts.PollInterval,
		},
	)
	if err != nil {
		return restore.Outcome{}, err
	}
	if prepared.Failure != nil {
		return r.waiting(pitr, prepared.Failure), nil
	}
	if !prepared.Done {
		// The restore stays in Pending while it prepares its cluster. It has
		// erased nothing, and it recovers on its own once the cluster
		// converges.
		pitr.Status.Phase = v1.PointInTimeRestorePending

		return prepared, nil
	}

	// Everything that this restore is allowed to act on is now known: the
	// chain, the rules of the server, and the clock of the brokers. The record
	// goes in before the database is read, so every later look is measured
	// against the chain that the read used.
	pitr.Status.Storage = pinnedChain(resolved.storage, resolved.dbConfig, resolved.server)

	// A contract that rolls its own server back is asked to do so first. A
	// contract that declares external is rolled back before the restore was
	// created, and the database is read as it stands.
	if resolved.server.OperatorRecovers() {
		pitr.Status.Phase = v1.PointInTimeRestoreRestoringDatabase
		r.progressing(pitr, fmt.Sprintf(
			"DatabaseServerConfig %s rolls its own server back. The restore asks it for %s",
			client.ObjectKeyFromObject(resolved.server),
			pitr.Spec.Timestamp.UTC().Format(time.RFC3339),
		))

		return restore.Outcome{Wait: restore.Shortly}, nil
	}

	pitr.Status.Phase = v1.PointInTimeRestoreValidatingDatabaseState

	return r.validateDatabaseState(ctx, pitr, resolved)
}

// resolve reads the storage chain of the restore and the facts of the live
// broker StatefulSet. Every reference that does not resolve, and every state
// that a restore must not run against, comes back as a
// *conditions.PreCheckFailure whose message names what is wrong. The phase
// decides how long such a failure holds the restore.
//
// pin says how the chain of this look is compared against the chain the
// restore pinned. Only the phase that asked the server to roll itself back
// passes pinAcrossRecovery. Every other caller binds every field.
//
// The reads are live. A stale suspend flag or a stale storage reference lets
// the restore delete the volumes of a cluster that moved on. The suspension
// itself is not a rule of this function: admission writes it, and the phases
// after admission read it through notSuspended.
func (r *Reconciler) resolve(
	ctx context.Context,
	pitr *v1.PointInTimeRestore,
	pin chainPin,
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
	serverKey := types.NamespacedName{Namespace: namespace, Name: dbConfig.Spec.ServerRef}
	if err := r.APIReader.Get(ctx, serverKey, &server); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference("DatabaseServerConfig %s does not exist", serverKey), nil
		}

		return nil, nil, fmt.Errorf("reading DatabaseServerConfig %s: %w", serverKey, err)
	}
	if pin == pinExact && server.Status.SystemIdentifier == "" {
		return nil, logicalbackup.InvalidReference(
			"DatabaseServerConfig %s has not published its system identifier, so the operator "+
				"cannot tell which PostgreSQL instance the endpoint reaches. Point-in-time "+
				"recovery rolls back the whole instance, and the rule that protects every "+
				"other database on it needs that identity. Wait until the DatabaseServerConfig "+
				"reports Ready",
			serverKey,
		), nil
	}
	// The identity of an endpoint the contract no longer names is the identity
	// of another server. Every rule below reads the instance from it, and the
	// restore reaches the endpoint of the spec, so a record that belongs to the
	// server before the change validates one instance and rolls back another.
	if pin == pinExact && !server.ProbedForCurrentSpec() {
		return nil, logicalbackup.InvalidReference(
			"DatabaseServerConfig %s has not reached the server that its spec names now, so its "+
				"system identifier belongs to the server before that change: the record was "+
				"probed at %s with Secret %q and keys %s, and the spec names %s with Secret %q "+
				"and keys %s. Point-in-time recovery rolls back the whole instance behind the "+
				"endpoint. Wait until the contract is probed again for the endpoint and the "+
				"credentials it names now",
			serverKey,
			server.Status.ProbedEndpoint,
			server.Status.ProbedSecretName,
			server.Status.ProbedSecretKeys,
			fmt.Sprintf("%s:%d", server.Spec.Host, server.Spec.Port),
			server.Spec.AdminCredentialsSecretRef.Name,
			server.Spec.AdminCredentialsSecretRef.UsernameKey+"/"+server.Spec.AdminCredentialsSecretRef.PasswordKey,
		), nil
	}

	if err := pinnedChainCurrent(pitr, &storage, &dbConfig, &server, pin); err != nil {
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

// notSuspended reports the cluster that started running again. Suspension is
// a standing condition of a restore, not a gate that admission passes once.
// The primary-storage phase erases the data volumes of the brokers, and a
// cluster that is unsuspended mid-run starts its brokers again, so the
// restore would erase under them.
//
// Only a phase after admission reports it. Admission suspends the cluster
// itself, so a cluster that is not suspended there is one that the restore is
// about to suspend.
func notSuspended(cluster *v1.CamundaCluster) *conditions.PreCheckFailure {
	if cluster.Spec.Suspend {
		return nil
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonClusterNotSuspended,
		Message: fmt.Sprintf(
			"CamundaCluster %s/%s started running again while the restore ran. A restore rewrites "+
				"its primary storage, so it runs only while spec.suspend is true",
			cluster.Namespace, cluster.Name,
		),
	}
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
		SystemIdentifier:          server.Status.SystemIdentifier,
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
	pin chainPin,
) error {
	pinned := pitr.Status.Storage
	if pinned == nil {
		return nil
	}

	current := pinnedChain(storage, dbConfig, server)
	if chainsAgree(pinned, current, pin) {
		return nil
	}

	return fmt.Errorf(
		"%w: the restore validated %s, and the cluster now uses %s. Create a new restore for the "+
			"database the cluster uses now",
		errChainChanged, describeChain(pinned), describeChain(current),
	)
}

// chainsAgree reports whether current is the chain that pinned records, under
// the given pin. The volatile fields are cleared on both copies rather than
// skipped, so a difference in any other field still answers false, and the
// failure message still names the real values.
func chainsAgree(pinned, current *v1.PointInTimeRestoreStorage, pin chainPin) bool {
	a, b := *pinned, *current
	if pin == pinAcrossRecovery {
		a.Endpoint, b.Endpoint = "", ""
		// A contract clears its identity when its endpoint moves and
		// publishes it again once it reached the server. One it has not
		// published yet states nothing, so it cannot disagree with the pin.
		if b.SystemIdentifier == "" {
			a.SystemIdentifier = ""
		}
	}

	return a == b
}

// describeChain names one storage chain in a failure message. It carries the
// system identifier, because an endpoint that is repointed at another
// PostgreSQL instance reads the same in every other field.
func describeChain(chain *v1.PointInTimeRestoreStorage) string {
	return fmt.Sprintf(
		"%s of DatabaseConfig %s on %s at %s, system identifier %s",
		chain.SecondaryStorageConfig,
		chain.DatabaseConfig,
		chain.DatabaseName,
		chain.Endpoint,
		chain.SystemIdentifier,
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
				"DatabaseServerConfig %s declares no point-in-time recovery. Set spec.pitr.enabled "+
					"to true once the server archives its write-ahead log", client.ObjectKeyFromObject(server),
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
				"spec.timestamp %s is older than the retention period of DatabaseServerConfig %s, "+
					"which is %d days. The server archives nothing of that point any more",
				want.UTC().Format(time.RFC3339), client.ObjectKeyFromObject(server), days,
			),
		}
	}

	return nil
}

// dedicatedServer reports that the server holds any logical database other
// than the one of this cluster, or nil when it holds that one alone.
// Point-in-time recovery on the engine rolls back the whole PostgreSQL
// instance, not one logical database, so anything else on it is rolled back
// too.
//
// The server is the instance, not the contract that describes it. The rule
// therefore reads the Database resources of every namespace whose contract
// reports the system identifier of this one. Two contracts that name one
// instance under different hosts are one server here, and a Database whose
// contract reports no identity for the spec it has now cannot be placed on
// either side.
//
// The one Database that remains must hold the logical database of dbConfig. A
// hand-written DatabaseConfig can name a database that no Database resource
// declares, and the single claimant on the instance is then somebody else's.
// contractIdentity is what one listed DatabaseServerConfig says about the
// PostgreSQL instance behind it.
type contractIdentity struct {
	// systemIdentifier is the instance that the last probe reached. It is
	// empty until the contract reaches its server.
	systemIdentifier string
	// probedForSpec is false while the record describes an endpoint or an
	// admin user that the spec of the contract no longer names. The identity
	// then belongs to the server before that change, and so does the claim
	// that a Database recorded against it.
	probedForSpec bool
}

func (r *Reconciler) dedicatedServer(
	ctx context.Context,
	server *v1.DatabaseServerConfig,
	dbConfig *v1.DatabaseConfig,
) (*conditions.PreCheckFailure, error) {
	key := client.ObjectKeyFromObject(server)

	// Both reads are live and unindexed, like every other read of this
	// controller. A cached list can miss the Database that a sibling cluster
	// created a moment ago, and the rule exists to protect that sibling. The
	// contracts are read as one list rather than one Get per Database, so the
	// cost of the rule does not grow with the number of claimants.
	var databases v1.DatabaseList
	if err := r.APIReader.List(ctx, &databases); err != nil {
		return nil, fmt.Errorf("listing the databases of DatabaseServerConfig %s: %w", key, err)
	}

	var contracts v1.DatabaseServerConfigList
	if err := r.APIReader.List(ctx, &contracts); err != nil {
		return nil, fmt.Errorf("listing the database server contracts: %w", err)
	}

	identities := make(map[types.NamespacedName]contractIdentity, len(contracts.Items))
	for i := range contracts.Items {
		contract := &contracts.Items[i]
		identities[client.ObjectKeyFromObject(contract)] = contractIdentity{
			systemIdentifier: contract.Status.SystemIdentifier,
			probedForSpec:    contract.ProbedForCurrentSpec(),
		}
	}

	var placed []*v1.Database
	var unplaced []string
	for i := range databases.Items {
		database := &databases.Items[i]
		switch databaseIdentity(database, identities) {
		case server.Status.SystemIdentifier:
			placed = append(placed, database)
		case "":
			unplaced = append(unplaced, database.Namespace+"/"+database.Name)
		}
	}
	slices.SortFunc(placed, func(a, b *v1.Database) int {
		return strings.Compare(a.Namespace+"/"+a.Name, b.Namespace+"/"+b.Name)
	})
	slices.Sort(unplaced)

	if len(placed) > 1 {
		names := make([]string, 0, len(placed))
		for _, database := range placed {
			names = append(names, database.Namespace+"/"+database.Name)
		}

		return &conditions.PreCheckFailure{
			Reason: v1.ReasonSharedServer,
			Message: fmt.Sprintf(
				"the server that DatabaseServerConfig %s describes holds the databases %s. "+
					"Point-in-time recovery rolls back the whole server, so it also rolls back a "+
					"database that another cluster uses. Move the cluster to a server of its own",
				key, strings.Join(names, ", "),
			),
		}, nil
	}

	// A Database whose contract publishes no identity for the spec it has now
	// can be on this server or on another one, and nothing here can tell
	// which. Recovery rolls back the whole server, so a database that cannot
	// be ruled out is a database this restore can destroy.
	if len(unplaced) > 0 {
		return logicalbackup.InvalidReference(
			"the DatabaseServerConfig of the Database resources %s publishes no system identifier "+
				"that the operator can trust for the endpoint and the credentials its spec names "+
				"now, so the operator cannot tell whether they live on the server that %s "+
				"describes. Point-in-time recovery rolls back the whole server. Wait until every "+
				"DatabaseServerConfig is probed for the spec it has now, or remove the Database "+
				"resources whose server no longer exists",
			strings.Join(unplaced, ", "), key,
		), nil
	}

	// Zero is not one. A server that no Database resource names carries no
	// evidence at all: the operator cannot tell whether it holds one database
	// or ten, and point-in-time recovery rolls back all of them. On a path
	// that deletes volumes, the absence of evidence holds the restore.
	if len(placed) == 0 {
		return logicalbackup.InvalidReference(
			"no Database resource resolves to the server that DatabaseServerConfig %s describes, "+
				"so the operator cannot tell which databases the server holds. Point-in-time "+
				"recovery rolls back the whole server. Declare the database of the cluster as a "+
				"Database resource on a server of its own",
			key,
		), nil
	}

	// One Database on the instance is not enough: it has to be the one of this
	// cluster. A hand-written DatabaseConfig names a logical database directly,
	// and nothing so far has tied that name to the single claimant.
	if only := placed[0]; only.Spec.DatabaseName != dbConfig.Spec.DatabaseName {
		return logicalbackup.InvalidReference(
			"the only Database resource on the server that DatabaseServerConfig %s describes is "+
				"%s, which holds the logical database %q, and this cluster stores its records in "+
				"%q. Point-in-time recovery rolls back the whole server, so it would roll back a "+
				"database that this restore never validated. Declare the database of the cluster "+
				"as a Database resource on a server of its own",
			key,
			only.Namespace+"/"+only.Name,
			only.Spec.DatabaseName,
			dbConfig.Spec.DatabaseName,
		), nil
	}

	return nil, nil
}

// databaseIdentity returns the PostgreSQL instance that database resolves to,
// or the empty string when nothing places it. A listed contract answers, and
// only while its record describes the spec it has now: a record of the server
// before a move places the database on that server, and the claim in
// status.collisionKey was recorded against the same record. A contract that
// no longer exists falls back to that claim, which the Database controller
// recorded the last time it resolved the server and never clears.
func databaseIdentity(database *v1.Database, identities map[types.NamespacedName]contractIdentity) string {
	ref := types.NamespacedName{Namespace: database.Namespace, Name: database.Spec.ServerRef}
	contract, listed := identities[ref]
	if !listed {
		return databasecomponents.CollisionIdentity(database.Status.CollisionKey)
	}
	if !contract.probedForSpec {
		return ""
	}

	return contract.systemIdentifier
}

// credentials reads the application credentials of the logical database. They
// are the credentials that the brokers use, resolved through the storage chain
// exactly as the restore application resolves them.
func (r *Reconciler) credentials(
	ctx context.Context,
	dbConfig *v1.DatabaseConfig,
) (user, password string, failure *conditions.PreCheckFailure, err error) {
	ref := dbConfig.Spec.CredentialsSecretRef
	key := types.NamespacedName{Namespace: dbConfig.Namespace, Name: ref.Name}

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
