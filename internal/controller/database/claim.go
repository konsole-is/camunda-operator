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

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/database"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
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
// Database that lost the race as often as it is the holder. The rule runs
// only while no Lease exists, where it decides which claimant tries first.
//
// A claim that another Database holds returns an error that wraps
// errClaimLost, so the reconcile withdraws the bindings and reports the
// holder.
func (r *DatabaseReconciler) claim(ctx context.Context, database *v1.Database, key string) error {
	lease, found, err := r.readClaim(ctx, key)
	if err != nil {
		return err
	}

	if !found {
		if err := r.checkCollision(ctx, database, key); err != nil {
			return err
		}

		return r.takeClaim(ctx, database, key)
	}

	if holder, ours := components.ClaimHolderOf(lease); ours && holder.UID == database.UID {
		return nil
	}

	return r.takeClaim(ctx, database, key)
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

// takeClaim creates the claim Lease of the logical database. The API server
// serializes the create, so of two Databases that reach this together exactly
// one holds the claim, whatever the cached rule answered them.
//
// It returns nil when database holds the claim after the call. A claim that
// another Database holds returns an error that wraps errClaimLost. Only a
// holder that is gone, or that a later Database replaced under the same name,
// is taken over.
func (r *DatabaseReconciler) takeClaim(ctx context.Context, database *v1.Database, key string) error {
	holder, err := r.createClaim(ctx, database, key)
	if err != nil || holder == nil {
		return err
	}

	keeps, err := r.holderKeeps(ctx, *holder)
	if err != nil {
		return err
	}
	if !keeps {
		if err := r.dropClaim(ctx, key, *holder); err != nil {
			return err
		}
		if holder, err = r.createClaim(ctx, database, key); err != nil || holder == nil {
			return err
		}
	}

	return fmt.Errorf("%w: %w", errClaimLost, &conditions.PreCheckFailure{
		Reason: v1.ReasonInvalidReference,
		Message: fmt.Sprintf(
			"Database %s already claims database %q on the same server",
			holder.NamespacedName, database.Spec.DatabaseName,
		),
	})
}

// createClaim creates the claim Lease of database. It returns nil when
// database holds the claim after the call, which covers the Lease it created
// and the one it held already. Otherwise it returns the holder that the Lease
// records.
//
// A Lease that carries no holder annotations is not one of ours. It blocks
// without a takeover, and the error names it.
func (r *DatabaseReconciler) createClaim(
	ctx context.Context, database *v1.Database, key string,
) (*components.ClaimHolder, error) {
	// The Lease can go away between the create and the read, when a release
	// or a takeover races this claimant. The second pass then creates it.
	for range 2 {
		err := r.Create(ctx, components.NewClaimLease(r.ClaimNamespace, key, database))
		if err == nil {
			return nil, nil
		}
		if !apierrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("creating the claim Lease of %q: %w", key, err)
		}

		lease, found, err := r.readClaim(ctx, key)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}

		holder, ours := components.ClaimHolderOf(lease)
		if !ours {
			return nil, fmt.Errorf("%w: %w", errClaimLost, &conditions.PreCheckFailure{
				Reason: v1.ReasonInvalidReference,
				Message: fmt.Sprintf(
					"Lease %s/%s claims database %q on the same server and names no Database. "+
						"Delete it if nothing else uses it",
					r.ClaimNamespace, lease.Name, database.Spec.DatabaseName,
				),
			})
		}
		if holder.UID == database.UID {
			return nil, nil
		}

		return &holder, nil
	}

	return nil, fmt.Errorf("the claim Lease of %q exists but is not readable yet", key)
}

// holderKeeps reports whether the Database that the Lease names still owns
// the claim. It does while it exists under the recorded UID. A holder that is
// gone, or that a later Database replaced under the same name, keeps nothing,
// so a crash between the claim and the release never blocks the logical
// database forever.
//
// A holder that is there keeps the claim against every other claimant, even
// an older one. The logical database it bootstrapped is in use, and its
// passwords are the ones the published Secrets carry. To hand it on would
// reset those passwords under a running cluster.
func (r *DatabaseReconciler) holderKeeps(ctx context.Context, holder components.ClaimHolder) (bool, error) {
	var other v1.Database
	if err := r.APIReader.Get(ctx, holder.NamespacedName, &other); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, fmt.Errorf("reading the claim holder %s: %w", holder.NamespacedName, err)
	}

	return other.UID == holder.UID, nil
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

	if err := r.releaseHeldClaims(ctx, selfHolder(database), ""); err != nil {
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
//
// The label selector is the one that NewClaimLease writes. It carries the
// name of the Database alone. The list therefore also holds the claims of a
// Database of another namespace with this name, and of a later Database. The
// holder annotations tell them apart.
func (r *DatabaseReconciler) releaseHeldClaims(
	ctx context.Context, holder components.ClaimHolder, keep string,
) error {
	var leases coordinationv1.LeaseList
	err := r.APIReader.List(
		ctx, &leases,
		client.InNamespace(r.ClaimNamespace),
		client.MatchingLabels(components.ClaimLeaseLabels(holder.Name)),
	)
	if err != nil {
		return fmt.Errorf("listing the claim Leases of %s: %w", holder.NamespacedName, err)
	}

	kept := ""
	if keep != "" {
		kept = components.ClaimLeaseName(keep)
	}

	for i := range leases.Items {
		lease := &leases.Items[i]
		if lease.Name == kept {
			continue
		}
		if recorded, ours := components.ClaimHolderOf(lease); !ours || recorded != holder {
			continue
		}
		if err := r.deleteClaim(ctx, lease); err != nil {
			return err
		}
	}

	return nil
}

// dropClaim deletes the claim Lease of key while its annotations still name
// holder. A Lease that is gone, or that another Database holds, is left
// alone.
func (r *DatabaseReconciler) dropClaim(ctx context.Context, key string, holder components.ClaimHolder) error {
	if key == "" {
		return nil
	}

	lease, found, err := r.readClaim(ctx, key)
	if err != nil || !found {
		return err
	}

	if recorded, ours := components.ClaimHolderOf(lease); !ours || recorded != holder {
		return nil
	}

	return r.deleteClaim(ctx, lease)
}

// deleteClaim deletes lease under the UID and the resourceVersion that it was
// read with. A Lease that is gone is left alone.
//
// A Lease that changed in between fails the preconditions, which means it was
// not deleted. The error goes back to the caller, so a release keeps its
// finalizer and reads the Lease again on the next look. To report a conflict
// as a release would let the Database go while its claim stayed.
func (r *DatabaseReconciler) deleteClaim(ctx context.Context, lease *coordinationv1.Lease) error {
	err := r.Delete(ctx, lease, client.Preconditions{
		UID: &lease.UID, ResourceVersion: &lease.ResourceVersion,
	})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf(
			"deleting the claim Lease %s: %w", client.ObjectKeyFromObject(lease), err,
		)
	}

	return nil
}

// readClaim reads the claim Lease of key live. Every claim decision reads the
// API server. A claim decided from a cache is no serialization.
func (r *DatabaseReconciler) readClaim(
	ctx context.Context, key string,
) (*coordinationv1.Lease, bool, error) {
	name := types.NamespacedName{Namespace: r.ClaimNamespace, Name: components.ClaimLeaseName(key)}

	var lease coordinationv1.Lease
	if err := r.APIReader.Get(ctx, name, &lease); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}

		return nil, false, fmt.Errorf("reading the claim Lease %s: %w", name, err)
	}

	return &lease, true, nil
}

// selfHolder is the holder identity of database.
func selfHolder(database *v1.Database) components.ClaimHolder {
	return components.ClaimHolder{
		NamespacedName: client.ObjectKeyFromObject(database),
		UID:            database.UID,
	}
}
