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
	"github.com/konsole-is/camunda-operator/pkg/restore"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
)

// day is how long the retention period of a database server counts one day.
const day = 24 * time.Hour

// chain is what a phase of the restore resolves from the cluster: the cluster
// itself, the logical database behind its storage contract, the server that
// holds the database, and the facts of the live broker StatefulSet.
type chain struct {
	cluster  *v1.CamundaCluster
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
func (r *Reconciler) admit(ctx context.Context, pitr *v1.PointInTimeRestore) (hold, error) {
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

	failure, err = r.dedicatedServer(ctx, resolved.server)
	if err != nil {
		return settle, err
	}
	if failure != nil {
		return r.waiting(pitr, failure), nil
	}

	pitr.Status.ClusterUID = resolved.cluster.UID
	pitr.Status.Phase = v1.PointInTimeRestoreValidatingDatabaseState

	return r.validateDatabaseState(ctx, pitr, resolved)
}

// resolve reads the storage chain of the restore and the facts of the live
// broker StatefulSet. Every reference that does not resolve, and every state
// that a restore must not run against, comes back as a
// *conditions.PreCheckFailure whose message names what is wrong. The phase
// decides how long such a failure may hold the restore.
//
// The reads are live. A stale suspend flag or a stale storage reference would
// let the restore delete the volumes of a cluster that moved on.
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
			return nil, invalidReference("CamundaCluster %s does not exist", key), nil
		}

		return nil, nil, fmt.Errorf("reading CamundaCluster %s: %w", key, err)
	}
	if pitr.Status.ClusterUID != "" && cluster.UID != pitr.Status.ClusterUID {
		return nil, nil, fmt.Errorf(
			"%w: CamundaCluster %s admitted the restore with UID %s and now has UID %s",
			errClusterReplaced, key, pitr.Status.ClusterUID, cluster.UID,
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
		return nil, invalidReference(
			"CamundaCluster %s has no spec.backupStorageRef, so Zeebe holds no primary storage "+
				"backup that the restore could read", key,
		), nil
	}

	if cluster.Spec.StorageRef == "" {
		return nil, invalidReference("CamundaCluster %s has no spec.storageRef", key), nil
	}

	var storage v1.SecondaryStorageConfig
	storageKey := types.NamespacedName{Namespace: namespace, Name: cluster.Spec.StorageRef}
	if err := r.APIReader.Get(ctx, storageKey, &storage); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, invalidReference("SecondaryStorageConfig %s does not exist", storageKey), nil
		}

		return nil, nil, fmt.Errorf("reading SecondaryStorageConfig %s: %w", storageKey, err)
	}
	if storage.Spec.Type != v1.SecondaryStorageTypeRDBMS || storage.Spec.RDBMS == nil {
		return nil, invalidReference(
			"SecondaryStorageConfig %s stores the data of CamundaCluster %s in %s. A point-in-time "+
				"restore exists only for a relational database, because an Elasticsearch cluster has "+
				"no point-in-time recovery. Use a LogicalRestore instead",
			storageKey, key, storage.Spec.Type,
		), nil
	}

	var dbConfig v1.DatabaseConfig
	dbKey := types.NamespacedName{Namespace: namespace, Name: storage.Spec.RDBMS.DatabaseConfigRef}
	if err := r.APIReader.Get(ctx, dbKey, &dbConfig); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, invalidReference("DatabaseConfig %s does not exist", dbKey), nil
		}

		return nil, nil, fmt.Errorf("reading DatabaseConfig %s: %w", dbKey, err)
	}

	var server v1.DatabaseServerConfig
	serverKey := types.NamespacedName{Name: dbConfig.Spec.ServerRef}
	if err := r.APIReader.Get(ctx, serverKey, &server); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, invalidReference("DatabaseServerConfig %q does not exist", serverKey.Name), nil
		}

		return nil, nil, fmt.Errorf("reading DatabaseServerConfig %q: %w", serverKey.Name, err)
	}

	target, err := restore.ReadTarget(ctx, r.APIReader, &cluster)
	if err != nil {
		var failure *conditions.PreCheckFailure
		if errors.As(err, &failure) {
			return nil, failure, nil
		}

		return nil, nil, err
	}
	pitr.Status.Brokers = target.Brokers

	return &chain{
		cluster:  &cluster,
		dbConfig: &dbConfig,
		server:   &server,
		target:   target,
	}, nil, nil
}

// resolveFailed maps an error of resolve onto the outcome of the phase. A
// cluster that was replaced ends the restore, because nothing of it pairs with
// the replacement. Every other error is a transient read that the caller
// retries.
func (r *Reconciler) resolveFailed(pitr *v1.PointInTimeRestore, err error) (hold, error) {
	if errors.Is(err, errClusterReplaced) {
		r.fail(pitr, err.Error())

		return settle, nil
	}

	return settle, err
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
	var databases v1.DatabaseList
	if err := r.List(
		ctx, &databases, client.MatchingFields{databaseServerRefField: server.Name},
	); err != nil {
		return nil, fmt.Errorf("listing the databases of DatabaseServerConfig %q: %w", server.Name, err)
	}

	if len(databases.Items) < 2 {
		return nil, nil
	}

	names := make([]string, 0, len(databases.Items))
	for i := range databases.Items {
		names = append(names, databases.Items[i].Namespace+"/"+databases.Items[i].Name)
	}
	slices.Sort(names)

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonSharedServer,
		Message: fmt.Sprintf(
			"DatabaseServerConfig %q holds the databases %s. Point-in-time recovery rolls back the "+
				"whole server, so it would roll back a database that another cluster uses. Move the "+
				"cluster to a server of its own",
			server.Name, strings.Join(names, ", "),
		),
	}, nil
}

// invalidReference builds the failure of a reference that does not resolve.
func invalidReference(format string, args ...any) *conditions.PreCheckFailure {
	return &conditions.PreCheckFailure{
		Reason:  v1.ReasonInvalidReference,
		Message: fmt.Sprintf(format, args...),
	}
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
