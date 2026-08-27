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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/database"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// ClaimFinalizer holds a Database that can own a claim Lease. The reconcile
// adds it before the first claim attempt, which is before it knows whether
// there is a claim to take, so every Database carries it. The deletion
// releases the Leases under it. A Database that took no claim releases
// nothing and goes away at once.
const ClaimFinalizer = "core.camunda.io/database-claim"

// claimLeasePrefix starts the name of every claim Lease.
const claimLeasePrefix = "camunda-database-"

// claimComponent is the component label value of a claim Lease.
const claimComponent = "database-claim"

// maxHolderIdentityLength bounds the holderIdentity field of a claim Lease.
// The API server of Kubernetes 1.36 accepts a longer value. The field is a
// display form for a reader, and a documented bound keeps it readable and
// safe against a stricter server. The exact identity lives in the
// annotations, which every decision about ownership reads.
const maxHolderIdentityLength = 128

// The annotations of a claim Lease name the Database that holds it, and the
// claim it holds. Every decision about ownership reads them. A Lease without
// all three holder annotations is not one of ours.
const (
	claimHolderNamespaceAnnotation = "camunda.io/database-claim-holder-namespace"
	claimHolderNameAnnotation      = "camunda.io/database-claim-holder-name"
	claimHolderUIDAnnotation       = "camunda.io/database-claim-holder-uid"
	claimKeyAnnotation             = "camunda.io/database-claim-key"
)

// databaseCollisionField indexes Database CRs by the claim they recorded in
// status.collisionKey. The collision rule can then list every claimant of one
// logical database on one PostgreSQL instance, across all namespaces, with
// one field-indexed query.
const databaseCollisionField = "database.status.collisionKey"

// claimRetryInterval is the wait before a Database that lost the claim to
// its logical database looks again. Nothing watches the claim Lease, and a
// winner that moves to another logical database does not always wake the
// Databases that wait for the one it left.
const claimRetryInterval = 60 * time.Second

// errClaimLost reports that another Database holds the claim to the logical
// database of this one. It is the single pre-check failure that withdraws the
// bindings: every other one leaves a server unreachable or a reference
// unresolved, and the published bindings stay valid. A lost claim does not.
// The winner owns the logical database and rotates the role passwords, so the
// credentials of the loser open nothing.
var errClaimLost = errors.New("another Database holds the claim")

// claimHolder is the Database that a claim Lease records. The UID tells it
// apart from a later Database of the same name.
type claimHolder struct {
	types.NamespacedName
	UID types.UID
}

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

	if holder, ours := holderOf(lease); ours && holder.UID == database.UID {
		return nil
	}

	return r.takeClaim(ctx, database, key)
}

// checkCollision orders the claimants of the logical database that key names
// by first creation. The claim belongs to the PostgreSQL instance, so the
// list covers every namespace: an older Database that reaches the same
// instance through another contract goes first, and the failure names it. A
// lost claim wraps errClaimLost, which the reconcile answers by withdrawing
// the bindings.
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

	winner := components.CollisionWinner(withSelf(list.Items, database))
	if winner == nil ||
		(winner.Namespace == database.Namespace && winner.Name == database.Name) {
		return nil
	}

	return fmt.Errorf("%w: %w", errClaimLost, &conditions.PreCheckFailure{
		Reason: v1.ReasonInvalidReference,
		Message: fmt.Sprintf(
			"Database %s already claims database %q on the same server",
			client.ObjectKeyFromObject(winner), database.Spec.DatabaseName,
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
) (*claimHolder, error) {
	// The Lease can go away between the create and the read, when a release
	// or a takeover races this claimant. The second pass then creates it.
	for range 2 {
		err := r.Create(ctx, newClaimLease(r.ClaimNamespace, key, database))
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

		holder, ours := holderOf(lease)
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
func (r *DatabaseReconciler) holderKeeps(ctx context.Context, holder claimHolder) (bool, error) {
	var other v1.Database
	if err := r.APIReader.Get(ctx, holder.NamespacedName, &other); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, fmt.Errorf("reading the claim holder %s: %w", holder.NamespacedName, err)
	}

	return other.UID == holder.UID, nil
}

// releaseStaleClaims gives back every claim that database holds other than
// key, which it holds now. It reads the holder annotations rather than
// status.collisionKey: a Database that took a claim and stopped before the
// status flush records none, and a release by the recorded key would leave
// that one held until the Database is deleted.
//
// The caller runs it only once database holds key and nothing it published
// names another logical database any more. A Database that gave an old claim
// up earlier would leave its published bindings on a logical database that
// another Database could take and rotate the roles of.
//
// recorded is the claim that status.collisionKey held before this reconcile.
// A Database that already named this logical database has nothing to give
// back, which is every reconcile after the first.
func (r *DatabaseReconciler) releaseStaleClaims(
	ctx context.Context, database *v1.Database, recorded, key string,
) error {
	if recorded == key {
		return nil
	}

	return r.releaseHeldClaims(ctx, selfHolder(database), key)
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
// The label selector is the one that newClaimLease writes, so the list covers
// the claims of this Database alone. The holder annotations then tell a Lease
// of a later Database of the same name apart from one of this Database.
func (r *DatabaseReconciler) releaseHeldClaims(
	ctx context.Context, holder claimHolder, keep string,
) error {
	var leases coordinationv1.LeaseList
	err := r.APIReader.List(
		ctx, &leases,
		client.InNamespace(r.ClaimNamespace),
		client.MatchingLabels(labels.Managed(labels.Database(holder.Name), claimComponent)),
	)
	if err != nil {
		return fmt.Errorf("listing the claim Leases of %s: %w", holder.NamespacedName, err)
	}

	kept := ""
	if keep != "" {
		kept = claimLeaseName(keep)
	}

	for i := range leases.Items {
		lease := &leases.Items[i]
		if lease.Name == kept {
			continue
		}
		if recorded, ours := holderOf(lease); !ours || recorded != holder {
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
func (r *DatabaseReconciler) dropClaim(ctx context.Context, key string, holder claimHolder) error {
	if key == "" {
		return nil
	}

	lease, found, err := r.readClaim(ctx, key)
	if err != nil || !found {
		return err
	}

	if recorded, ours := holderOf(lease); !ours || recorded != holder {
		return nil
	}

	return r.deleteClaim(ctx, lease)
}

// deleteClaim deletes lease under the UID and the resourceVersion that it was
// read with. A Lease that is gone, or that changed in between, is left alone.
func (r *DatabaseReconciler) deleteClaim(ctx context.Context, lease *coordinationv1.Lease) error {
	err := r.Delete(ctx, lease, client.Preconditions{
		UID: &lease.UID, ResourceVersion: &lease.ResourceVersion,
	})
	if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
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
	name := types.NamespacedName{Namespace: r.ClaimNamespace, Name: claimLeaseName(key)}

	var lease coordinationv1.Lease
	if err := r.APIReader.Get(ctx, name, &lease); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}

		return nil, false, fmt.Errorf("reading the claim Lease %s: %w", name, err)
	}

	return &lease, true, nil
}

// holderOf returns the Database that the annotations of the Lease name, and
// whether all three of them are there. Only the annotations carry ownership.
// The holderIdentity of the Lease is a display form for a reader.
func holderOf(lease *coordinationv1.Lease) (claimHolder, bool) {
	annotations := lease.GetAnnotations()
	holder := claimHolder{
		NamespacedName: types.NamespacedName{
			Namespace: annotations[claimHolderNamespaceAnnotation],
			Name:      annotations[claimHolderNameAnnotation],
		},
		UID: types.UID(annotations[claimHolderUIDAnnotation]),
	}
	if holder.Namespace == "" || holder.Name == "" || holder.UID == "" {
		return claimHolder{}, false
	}

	return holder, true
}

// selfHolder is the holder identity of database.
func selfHolder(database *v1.Database) claimHolder {
	return claimHolder{
		NamespacedName: client.ObjectKeyFromObject(database),
		UID:            database.UID,
	}
}

// newClaimLease builds the claim Lease of key for database.
func newClaimLease(namespace, key string, database *v1.Database) *coordinationv1.Lease {
	holder := labels.BoundedName(
		database.Namespace+"/"+database.Name, maxHolderIdentityLength,
	)
	now := metav1.NowMicro()

	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      claimLeaseName(key),
			Labels:    labels.Managed(labels.Database(database.Name), claimComponent),
			Annotations: map[string]string{
				claimHolderNamespaceAnnotation: database.Namespace,
				claimHolderNameAnnotation:      database.Name,
				claimHolderUIDAnnotation:       string(database.UID),
				claimKeyAnnotation:             key,
			},
		},
		Spec: coordinationv1.LeaseSpec{HolderIdentity: &holder, AcquireTime: &now},
	}
}

// claimLeaseName returns the name of the Lease that claims key:
// "camunda-database-<hash of the key>". A claim key holds a system identifier
// and a database name, which together are no DNS subdomain, so the name is
// built from a hash of it. Every claimant of one logical database therefore
// meets on one Lease, and the claim key annotation says which one that is.
func claimLeaseName(key string) string {
	sum := sha256.Sum256([]byte(key))

	return claimLeasePrefix + hex.EncodeToString(sum[:])[:40]
}
