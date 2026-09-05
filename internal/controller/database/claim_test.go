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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/database"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// The specs of this package name the claim vocabulary of
// pkg/components/database under its short form.
const claimHolderNameAnnotation = components.ClaimHolderNameAnnotation

func claimLeaseName(key string) string { return components.ClaimSchema().LeaseName(key) }

// unclaimed returns a Database that has not recorded a claim yet, as it
// stands on the cluster before the reconcile that resolves its server. The
// collision index of the cache is served from status.collisionKey, so such a
// Database is in no claimant list.
func unclaimed(namespace, name string, created time.Time) *v1.Database {
	database := claimant(namespace, name, created)
	database.UID = types.UID(namespace + "-" + name)
	database.Status.CollisionKey = ""

	return database
}

// staged returns the copy of database that the reconcile works on once it has
// resolved the server: the claim is set in memory and reaches the cluster
// only with the status flush that ends the reconcile.
func staged(database *v1.Database) *v1.Database {
	staged := database.DeepCopy()
	staged.Status.CollisionKey = claimKey

	return staged
}

// claimReconciler returns a reconciler whose client holds the given Databases
// and serves the collision index over them, the way the manager cache does.
func claimReconciler(t *testing.T, databases ...*v1.Database) *DatabaseReconciler {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))
	require.NoError(t, coordinationv1.AddToScheme(scheme))

	objects := make([]client.Object, 0, len(databases))
	for _, database := range databases {
		objects = append(objects, database)
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&v1.Database{}, databaseCollisionField, func(o client.Object) []string {
			key := o.(*v1.Database).Status.CollisionKey
			if key == "" {
				return nil
			}

			return []string{key}
		}).
		WithObjects(objects...).
		Build()

	return &DatabaseReconciler{
		Client: c, APIReader: c, Scheme: scheme, ClaimNamespace: testClaimNamespace,
	}
}

// leaseOf reads the claim Lease of claimKey.
func leaseOf(t *testing.T, r *DatabaseReconciler) (*coordinationv1.Lease, bool) {
	t.Helper()

	var lease coordinationv1.Lease
	key := types.NamespacedName{Namespace: testClaimNamespace, Name: claimLeaseName(claimKey)}
	err := r.APIReader.Get(context.Background(), key, &lease)
	if apierrors.IsNotFound(err) {
		return nil, false
	}
	require.NoError(t, err)

	return &lease, true
}

// TestClaimSerializesTwoFirstReconciles pins the fix of the collision that the
// cached rule cannot see. Two Databases that reach their first reconcile
// together both ask the index a question that neither claim is in yet, so the
// rule hands each of them the claim. The Lease that the API server serializes
// is what leaves exactly one holder.
func TestClaimSerializesTwoFirstReconciles(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	ctx := context.Background()

	t.Run("the second claimant loses and names the first", func(t *testing.T) {
		t.Parallel()

		older := unclaimed("alpha", "older", base)
		newer := unclaimed("beta", "newer", base.Add(time.Hour))
		r := claimReconciler(t, older, newer)

		require.NoError(t, r.claim(ctx, staged(older), claimKey))

		err := r.claim(ctx, staged(newer), claimKey)
		require.ErrorIs(t, err, errClaimLost)

		var failure *conditions.PreCheckFailure
		require.ErrorAs(t, err, &failure)
		assert.Equal(t, v1.ReasonInvalidReference, failure.Reason)
		assert.Contains(t, failure.Message, "alpha/older")
	})

	t.Run("the holder keeps the claim against an older claimant", func(t *testing.T) {
		t.Parallel()

		older := unclaimed("alpha", "older", base)
		newer := unclaimed("beta", "newer", base.Add(time.Hour))
		r := claimReconciler(t, older, newer)

		require.NoError(t, r.claim(ctx, staged(newer), claimKey))

		err := r.claim(ctx, staged(older), claimKey)
		require.ErrorIs(t, err, errClaimLost)

		var failure *conditions.PreCheckFailure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Message, "beta/newer")

		lease, found := leaseOf(t, r)
		require.True(t, found)
		holder, ours := components.ClaimSchema().HolderOf(lease)
		require.True(t, ours)
		assert.Equal(t, "beta/newer", holder.String(), "the Lease must stay with its holder")
	})

	t.Run("the rule takes no claim away from its holder", func(t *testing.T) {
		t.Parallel()

		newer := unclaimed("beta", "newer", base.Add(time.Hour))
		older := unclaimed("alpha", "older", base)
		r := claimReconciler(t, newer, older)

		held := staged(newer)
		require.NoError(t, r.claim(ctx, held, claimKey))

		// The older claimant records its claim and reaches the index. Only
		// its own takeover may hand it the logical database. Until then the
		// holder bootstraps and publishes under a claim it still owns.
		older.Status.CollisionKey = claimKey
		require.NoError(t, r.Update(ctx, older))

		assert.NoError(t, r.claim(ctx, held, claimKey))
	})
}

// TestClaimNamesTheHolderNotTheOldest pins which Database a lost claim names.
// The cached rule orders claimants by creation, so it would name an older
// Database that lost the race, and a reader who deletes the name it gives
// frees nothing. The Lease names the holder.
func TestClaimNamesTheHolderNotTheOldest(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	ctx := context.Background()

	oldest := unclaimed("alpha", "oldest", base)
	holder := unclaimed("beta", "holder", base.Add(time.Hour))
	latest := unclaimed("gamma", "latest", base.Add(2*time.Hour))
	r := claimReconciler(t, oldest, holder, latest)

	// The newer Database takes the claim, and the older one records the same
	// key without holding anything. The cached rule now prefers a loser.
	require.NoError(t, r.claim(ctx, staged(holder), claimKey))
	oldest.Status.CollisionKey = claimKey
	require.NoError(t, r.Update(ctx, oldest))

	err := r.claim(ctx, staged(latest), claimKey)
	require.ErrorIs(t, err, errClaimLost)

	var failure *conditions.PreCheckFailure
	require.ErrorAs(t, err, &failure)
	assert.Contains(t, failure.Message, "beta/holder")
	assert.NotContains(t, failure.Message, "alpha/oldest")
}

// TestClaimTakesOverAHolderThatIsGone covers the crash between a claim and its
// release. Nothing else would ever hand the logical database on.
func TestClaimTakesOverAHolderThatIsGone(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	ctx := context.Background()

	gone := unclaimed("alpha", "gone", base)
	next := unclaimed("beta", "next", base.Add(time.Hour))
	r := claimReconciler(t, gone, next)

	require.NoError(t, r.claim(ctx, staged(gone), claimKey))

	// The holder goes without its finalizer running, as it does when the
	// operator crashes between the claim and the release.
	var stored v1.Database
	require.NoError(t, r.Get(ctx, client.ObjectKeyFromObject(gone), &stored))
	stored.Finalizers = nil
	require.NoError(t, r.Update(ctx, &stored))
	require.NoError(t, r.Delete(ctx, &stored))

	require.NoError(t, r.claim(ctx, staged(next), claimKey))

	lease, found := leaseOf(t, r)
	require.True(t, found)
	holder, ours := components.ClaimSchema().HolderOf(lease)
	require.True(t, ours)
	assert.Equal(t, "beta/next", holder.String())
}

// TestClaimTakesOverAReplacedHolder covers a holder that a later Database
// replaced under its own name. The claim records the UID beside the name, so
// a Database of the recorded name that is not the recorded one keeps nothing.
// Without the UID the claim of a deleted Database would pass to whatever took
// its name, and the logical database would never be free again.
func TestClaimTakesOverAReplacedHolder(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	ctx := context.Background()

	first := unclaimed("alpha", "same", base)
	next := unclaimed("beta", "next", base.Add(time.Hour))
	r := claimReconciler(t, first, next)

	require.NoError(t, r.claim(ctx, staged(first), claimKey))

	// The holder goes and a new Database takes its namespace and its name.
	// The Lease still records the UID of the one that is gone.
	var stored v1.Database
	require.NoError(t, r.Get(ctx, client.ObjectKeyFromObject(first), &stored))
	stored.Finalizers = nil
	require.NoError(t, r.Update(ctx, &stored))
	require.NoError(t, r.Delete(ctx, &stored))

	replacement := unclaimed("alpha", "same", base.Add(2*time.Hour))
	replacement.UID = "alpha-same-replacement"
	require.NoError(t, r.Create(ctx, replacement))

	require.NoError(t, r.claim(ctx, staged(next), claimKey))

	lease, found := leaseOf(t, r)
	require.True(t, found)
	holder, ours := components.ClaimSchema().HolderOf(lease)
	require.True(t, ours)
	assert.Equal(t, "beta/next", holder.String())
}

// TestClaimBlocksOnAForeignLease pins that the operator never takes over a
// Lease that it did not write. A name collision with something else in the
// namespace of the operator must not lose that object its Lease.
func TestClaimBlocksOnAForeignLease(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	ctx := context.Background()

	database := unclaimed("alpha", "only", base)
	r := claimReconciler(t, database)
	require.NoError(t, r.Create(ctx, &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testClaimNamespace, Name: claimLeaseName(claimKey),
		},
	}))

	err := r.claim(ctx, staged(database), claimKey)
	require.ErrorIs(t, err, errClaimLost)

	var failure *conditions.PreCheckFailure
	require.ErrorAs(t, err, &failure)
	assert.Contains(t, failure.Message, "names no Database")

	_, found := leaseOf(t, r)
	assert.True(t, found, "a foreign Lease must survive")
}

// TestClaimReleases covers the two paths that give a claim back: the deletion
// of the holder, and a holder that moves to another logical database.
func TestClaimReleases(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	ctx := context.Background()

	t.Run("the finalizer of the holder gives the Lease back", func(t *testing.T) {
		t.Parallel()

		database := unclaimed("alpha", "only", base)
		r := claimReconciler(t, database)

		held := staged(database)
		controllerutil.AddFinalizer(held, ClaimFinalizer)
		require.NoError(t, r.Update(ctx, held))
		require.NoError(t, r.claim(ctx, held, claimKey))

		require.NoError(t, r.finalize(ctx, held))

		_, found := leaseOf(t, r)
		assert.False(t, found)
		assert.False(t, controllerutil.ContainsFinalizer(held, ClaimFinalizer))
	})

	t.Run("a holder that recorded no claim gives the Lease back", func(t *testing.T) {
		t.Parallel()

		database := unclaimed("alpha", "only", base)
		r := claimReconciler(t, database)

		held := staged(database)
		controllerutil.AddFinalizer(held, ClaimFinalizer)
		require.NoError(t, r.Update(ctx, held))
		require.NoError(t, r.claim(ctx, held, claimKey))

		// The status flush that records the claim never reached the cluster,
		// so the deleted object carries no key. The holder annotations of the
		// Lease are the only record of what it holds.
		deleted := held.DeepCopy()
		deleted.Status.CollisionKey = ""
		require.NoError(t, r.finalize(ctx, deleted))

		_, found := leaseOf(t, r)
		assert.False(t, found)
	})

	t.Run("a Database that claims another key gives the old Lease back", func(t *testing.T) {
		t.Parallel()

		database := unclaimed("alpha", "only", base)
		r := claimReconciler(t, database)

		held := staged(database)
		require.NoError(t, r.claim(ctx, held, claimKey))

		require.NoError(t, r.releaseHeldClaims(
			ctx, held, "7000000000000000001/other",
		))

		_, found := leaseOf(t, r)
		assert.False(t, found)
	})

	t.Run("a holder that recorded no claim gives it back on a move", func(t *testing.T) {
		t.Parallel()

		database := unclaimed("alpha", "only", base)
		r := claimReconciler(t, database)

		require.NoError(t, r.claim(ctx, staged(database), claimKey))

		// The status flush never recorded the claim, and the spec now names
		// another logical database. A release that reads the recorded key
		// would leave the old one held until this Database is deleted.
		moved := database.DeepCopy()
		require.NoError(t, r.releaseHeldClaims(
			ctx, moved, "7000000000000000001/other",
		))

		_, found := leaseOf(t, r)
		assert.False(t, found)
	})

	t.Run("a Database releases no Lease that another one holds", func(t *testing.T) {
		t.Parallel()

		holder := unclaimed("alpha", "holder", base)
		other := unclaimed("beta", "other", base.Add(time.Hour))
		r := claimReconciler(t, holder, other)
		require.NoError(t, r.claim(ctx, staged(holder), claimKey))

		deleted := staged(other)
		controllerutil.AddFinalizer(deleted, ClaimFinalizer)
		require.NoError(t, r.finalize(ctx, deleted))

		_, found := leaseOf(t, r)
		assert.True(t, found)
	})
}
