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

package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/database"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/leaseclaim"
)

// ClaimFinalizer holds a Database that can own a claim Lease. The reconcile
// adds it before the first claim attempt, which is before it knows whether
// there is a claim to take, so every Database carries it. The deletion
// releases the Leases under it. A Database that took no claim releases
// nothing and goes away at once.
const ClaimFinalizer = "core.camunda.io/database-claim"

// databaseCollisionField indexes Database CRs by the claim they recorded in
// status.collisionKey. The collision rule can then list every claimant of one
// logical database on one PostgreSQL instance, across all namespaces, with
// one field-indexed query.
const databaseCollisionField = "database.status.collisionKey"

// claimRetryInterval is the wait before a Database that lost the claim to its
// logical database, or that waits its turn for one, looks again. Nothing
// watches the claim Lease, and a holder that moves to another logical
// database does not always wake the Databases that wait for the one it left.
const claimRetryInterval = 60 * time.Second

// errClaimLost reports that another Database holds the claim to the logical
// database of this one. It is the single pre-check failure that withdraws the
// bindings: every other one leaves a server unreachable or a reference
// unresolved, and the published bindings stay valid. A lost claim does not.
// The holder owns the logical database and rotates the role passwords, so the
// credentials of the Database that lost it open nothing.
var errClaimLost = errors.New("another Database holds the claim")

// errClaimNotFirst reports that another Database goes first for a logical
// database that nobody holds yet. It is an order, not a loss. The reconcile
// answers it with a wait: the bindings of this Database stay, and so does
// every claim it holds. A Database that withdrew on this would give up a
// logical database it still runs on because a claimant of another name is
// older.
var errClaimNotFirst = errors.New("another Database goes first for the claim")

// claim takes the claim on the logical database that key names. The Lease
// decides it, and the cached rule of checkCollision never overrules the
// Lease. A Lease that exists names the holder, and the rule answers from an
// index that lags behind: it would name the oldest claimant, which is a
// Database that lost the race as often as it is the holder. The rule
// therefore runs only while no Lease exists, where it decides which claimant
// tries first.
//
// A claim that another Database holds returns an error that wraps
// errClaimLost, so the reconcile withdraws the bindings and reports the
// holder. A Lease that carries no holder annotations is not one of ours: it
// blocks without a takeover, and the failure names it.
func (r *DatabaseReconciler) claim(ctx context.Context, database *v1.Database, key string) error {
	blocker, err := r.claims().TakeUnclaimed(ctx, database, key, func(ctx context.Context) error {
		return r.checkCollision(ctx, database, key)
	})
	if err != nil || blocker == nil {
		return err
	}

	if blocker.Foreign() {
		return fmt.Errorf("%w: %w", errClaimLost, &conditions.PreCheckFailure{
			Reason: v1.ReasonInvalidReference,
			Message: fmt.Sprintf(
				"Lease %s claims database %q on the same server and names no Database. "+
					"Delete it if nothing else uses it",
				blocker.Lease, database.Spec.DatabaseName,
			),
		})
	}

	return fmt.Errorf("%w: %w", errClaimLost, &conditions.PreCheckFailure{
		Reason: v1.ReasonInvalidReference,
		Message: fmt.Sprintf(
			"Database %s already claims database %q on the same server",
			blocker.Holder.NamespacedName, database.Spec.DatabaseName,
		),
	})
}

// claims is the claim protocol on the logical databases, over the Leases of
// the namespace of the operator.
func (r *DatabaseReconciler) claims() *leaseclaim.Claim[*v1.Database] {
	return components.ClaimSchema().NewClaim(r.Client, r.APIReader, r.ClaimNamespace)
}

// checkCollision orders the claimants of the logical database that key names
// by first creation. The claim belongs to the PostgreSQL instance, so the
// list covers every namespace: an older Database that reaches the same
// instance through another contract goes first, and the failure names it. A
// rejection wraps errClaimNotFirst, which the reconcile answers with a wait.
// Nothing holds the logical database at this point, so this is an order and
// not a loss, and the Database keeps its bindings and every claim it holds.
//
// The list comes from the cache, and a claimant reaches it only after the
// status flush that records its key. The rule therefore decides who tries the
// claim first, never who holds it. Its caller runs it only while no Lease
// exists, so it never names a claimant as the holder of a claim that another
// Database took.
func (r *DatabaseReconciler) checkCollision(
	ctx context.Context, database *v1.Database, key string,
) error {
	var list v1.DatabaseList
	if err := r.List(ctx, &list, client.MatchingFields{databaseCollisionField: key}); err != nil {
		return fmt.Errorf("listing databases claiming %q: %w", key, err)
	}

	first := components.PreferredClaimant(withSelf(list.Items, database))
	if first == nil ||
		(first.Namespace == database.Namespace && first.Name == database.Name) {
		return nil
	}

	return fmt.Errorf("%w: %w", errClaimNotFirst, &conditions.PreCheckFailure{
		Reason: v1.ReasonInvalidReference,
		Message: fmt.Sprintf(
			"Database %s goes first for database %q on the same server. Nothing holds that "+
				"database yet",
			client.ObjectKeyFromObject(first), database.Spec.DatabaseName,
		),
	})
}

// withSelf adds database to the claimants of its own key, unless the list
// already holds it. The index is served from status.collisionKey, so a
// Database that resolved its server for the first time is not in the list it
// just asked for: its claim is staged in memory and reaches the cluster only
// with the flush at the end of this reconcile. Without it the rule sees a
// single newer claimant, sends it to the Lease first, and the older Database
// loses the order that its creation time gives it.
func withSelf(claimants []v1.Database, database *v1.Database) []v1.Database {
	for i := range claimants {
		if claimants[i].Namespace == database.Namespace && claimants[i].Name == database.Name {
			return claimants
		}
	}

	return append(claimants, *database)
}

// finalize releases the claims of a deleted Database and removes the claim
// finalizer.
//
// The release goes first. Once the removal of the finalizer is durable the
// object is gone, and nothing but a later claimant of the same logical
// database would ever take the Lease over.
func (r *DatabaseReconciler) finalize(ctx context.Context, database *v1.Database) error {
	if !controllerutil.ContainsFinalizer(database, ClaimFinalizer) {
		return nil
	}

	if err := r.releaseHeldClaims(ctx, database, ""); err != nil {
		return err
	}

	controllerutil.RemoveFinalizer(database, ClaimFinalizer)
	if err := r.Update(ctx, database); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("removing the claim finalizer: %w", err)
	}

	return nil
}

// releaseHeldClaims deletes every claim Lease of the namespace of the
// operator that still names holder, except the one that keep names. A release
// by key reads status.collisionKey, and a Database that took a claim but
// never flushed its status carries no key: its Lease would then stay until a
// later claimant took it over. The sweep reads the holder annotations
// instead, so a Database always gives back what it holds and no longer uses.
//
// An empty keep releases every claim of the holder.
func (r *DatabaseReconciler) releaseHeldClaims(
	ctx context.Context, database *v1.Database, keep string,
) error {
	claims := r.claims()

	leases, err := claims.Held(ctx, database)
	if err != nil {
		return err
	}

	kept := ""
	if keep != "" {
		kept = claims.LeaseName(keep)
	}

	for i := range leases {
		if leases[i].Name == kept {
			continue
		}
		if err := claims.Release(ctx, &leases[i]); err != nil {
			return err
		}
	}

	return nil
}
