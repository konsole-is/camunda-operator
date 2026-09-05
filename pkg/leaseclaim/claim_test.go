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

package leaseclaim

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// testNamespace is the namespace of the operator in these tests, where every
// claim Lease of the fixture Schema lives.
const testNamespace = "camunda-system"

// testSchema is the claim of these tests. A claimant is a ConfigMap, which
// stands for any resource that takes a thing outside Kubernetes for itself.
func testSchema() Schema[*corev1.ConfigMap] {
	return Schema[*corev1.ConfigMap]{
		Prefix:                    "camunda-test-",
		Noun:                      "test claim",
		HolderNamespaceAnnotation: "camunda.io/test-claim-holder-namespace",
		HolderNameAnnotation:      "camunda.io/test-claim-holder-name",
		HolderUIDAnnotation:       "camunda.io/test-claim-holder-uid",
		KeyAnnotation:             "camunda.io/test-claim-key",
		Labels: func(name string) map[string]string {
			return labels.Managed(labels.Database(name), "test-claim")
		},
	}
}

// claimant returns the ConfigMap that stands for a claiming resource.
func claimant(namespace, name string, uid types.UID) *corev1.ConfigMap {
	return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Namespace: namespace, Name: name, UID: uid,
	}}
}

// testClient returns a fake client that holds the given objects and serves
// the Lease type.
func testClient(t *testing.T, objects ...client.Object) client.WithWatch {
	t.Helper()

	return fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(objects...).
		Build()
}

// testScheme serves the Lease and the ConfigMap that these tests write.
func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, coordinationv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	return scheme
}

// keepEvery is the HolderKeeps of a claim whose holders never go away.
func keepEvery(context.Context, Holder) (bool, error) { return true, nil }

// keepNone is the HolderKeeps of a claim whose holders are all gone.
func keepNone(context.Context, Holder) (bool, error) { return false, nil }

// taken drops the blocker of a Take that a spec expects to succeed, so the
// arrangement of that spec reads as one line.
func taken(_ *Blocker, err error) error { return err }

// leaseOf reads the claim Lease of key.
func leaseOf(t *testing.T, c client.Client, key string) (*coordinationv1.Lease, bool) {
	t.Helper()

	var lease coordinationv1.Lease
	name := types.NamespacedName{Namespace: testNamespace, Name: testSchema().LeaseName(key)}
	err := c.Get(context.Background(), name, &lease)
	if apierrors.IsNotFound(err) {
		return nil, false
	}
	require.NoError(t, err)

	return &lease, true
}

// TestTakeSerializesTwoClaimants pins the mutual exclusion that the claim
// exists for: the API server decides it, and the second claimant is told who
// holds the key.
func TestTakeSerializesTwoClaimants(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	earlier := claimant("alpha", "earlier", "uid-earlier")
	later := claimant("beta", "later", "uid-later")
	c := testClient(t, earlier, later)
	claims := New(testSchema(), c, c, testNamespace, keepEvery)

	blocker, err := claims.Take(ctx, earlier, "7000000000000000001/camunda")
	require.NoError(t, err)
	assert.Nil(t, blocker)

	blocker, err = claims.Take(ctx, later, "7000000000000000001/camunda")

	require.NoError(t, err)
	require.NotNil(t, blocker)
	assert.False(t, blocker.Foreign())
	assert.Equal(t, holderOf(earlier), blocker.Holder)
	assert.Equal(t, testSchema().LeaseName("7000000000000000001/camunda"), blocker.Lease.Name)
	assert.Equal(t, testNamespace, blocker.Lease.Namespace)

	lease, found := leaseOf(t, c, "7000000000000000001/camunda")
	require.True(t, found)
	holder, ours := testSchema().HolderOf(lease)
	require.True(t, ours)
	assert.Equal(t, holderOf(earlier), holder)
}

// A reconcile runs again on every event, so the holder reaches its own claim
// over and over. It must read as taken and never as blocked.
func TestTakeIsIdempotentForTheHolder(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	owner := claimant("alpha", "only", "uid-only")
	c := testClient(t, owner)
	claims := New(testSchema(), c, c, testNamespace, keepEvery)

	require.NoError(t, taken(claims.Take(ctx, owner, "key")))

	blocker, err := claims.Take(ctx, owner, "key")

	require.NoError(t, err)
	assert.Nil(t, blocker)
}

// TestTakeReadsBeforeItWrites pins the fast path of a steady holder. Every
// reconcile of a resource that holds its claim reaches Take, and a create that
// the API server rejects on each of them is a write nothing needs.
func TestTakeReadsBeforeItWrites(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	owner := claimant("alpha", "only", "uid-only")

	var gets, creates int
	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(owner).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(
				ctx context.Context,
				c client.WithWatch,
				key client.ObjectKey,
				obj client.Object,
				opts ...client.GetOption,
			) error {
				gets++

				return c.Get(ctx, key, obj, opts...)
			},
			Create: func(
				ctx context.Context,
				c client.WithWatch,
				obj client.Object,
				opts ...client.CreateOption,
			) error {
				creates++

				return c.Create(ctx, obj, opts...)
			},
		}).
		Build()
	claims := New(testSchema(), c, c, testNamespace, keepEvery)
	require.NoError(t, taken(claims.Take(ctx, owner, "key")))

	gets, creates = 0, 0
	require.NoError(t, taken(claims.Take(ctx, owner, "key")))

	assert.Equal(t, 1, gets, "the holder reads its own claim back")
	assert.Zero(t, creates, "the holder writes nothing")
}

// TestTakeTakesOverAHolderThatKeepsNothing covers the crash between a claim
// and its release. Nothing else hands the key on.
func TestTakeTakesOverAHolderThatKeepsNothing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	gone := claimant("alpha", "gone", "uid-gone")
	next := claimant("beta", "next", "uid-next")

	var deletes int
	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(gone, next, testSchema().NewLease(testNamespace, "key", gone)).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(
				ctx context.Context,
				c client.WithWatch,
				obj client.Object,
				opts ...client.DeleteOption,
			) error {
				deletes++

				return c.Delete(ctx, obj, opts...)
			},
		}).
		Build()

	arranged, found := leaseOf(t, c, "key")
	require.True(t, found)
	holder, ours := testSchema().HolderOf(arranged)
	require.True(t, ours)
	require.Equal(t, holderOf(gone), holder, "the stale holder must be arranged")

	blocker, err := New(testSchema(), c, c, testNamespace, keepNone).Take(ctx, next, "key")

	require.NoError(t, err)
	assert.Nil(t, blocker)
	assert.Equal(t, 1, deletes, "the stale Lease is deleted rather than reused")
	lease, found := leaseOf(t, c, "key")
	require.True(t, found)
	holder, ours = testSchema().HolderOf(lease)
	require.True(t, ours)
	assert.Equal(t, holderOf(next), holder)
}

// Two claimants that meet one stale holder both try the takeover, and the
// API server refuses the delete of the loser. The claim is decided by then, so
// the loser reports the winner rather than a failure the reconcile retries.
func TestTakeReportsTheClaimantThatWonATakeover(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schema := testSchema()
	gone := claimant("alpha", "gone", "uid-gone")
	winner := claimant("beta", "winner", "uid-winner")
	loser := claimant("gamma", "loser", "uid-loser")

	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(gone, winner, loser, schema.NewLease(testNamespace, "key", gone)).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(
				ctx context.Context,
				c client.WithWatch,
				obj client.Object,
				_ ...client.DeleteOption,
			) error {
				// The winner takes the Lease over between the read of the
				// loser and its delete, so the preconditions no longer hold.
				var lease coordinationv1.Lease
				if err := c.Get(ctx, client.ObjectKeyFromObject(obj), &lease); err != nil {
					return err
				}
				lease.Annotations[schema.HolderNamespaceAnnotation] = winner.Namespace
				lease.Annotations[schema.HolderNameAnnotation] = winner.Name
				lease.Annotations[schema.HolderUIDAnnotation] = string(winner.UID)
				if err := c.Update(ctx, &lease); err != nil {
					return err
				}

				return apierrors.NewConflict(
					coordinationv1.Resource("leases"), obj.GetName(),
					errors.New("the Lease changed"),
				)
			},
		}).
		Build()

	blocker, err := New(schema, c, c, testNamespace, keepNone).Take(ctx, loser, "key")

	require.NoError(t, err)
	require.NotNil(t, blocker)
	assert.Equal(t, holderOf(winner), blocker.Holder)
	_, found := leaseOf(t, c, "key")
	assert.True(t, found, "the Lease of the winner survives")
}

// A refused precondition says the Lease changed, and any write changes it. A
// claimant that reads a conflict as a takeover reports the holder that keeps
// nothing as a live one. A consumer that withdraws on a lost claim then takes
// the credentials of running pods away over an annotation.
func TestTakeReportsAConflictThatLeftTheHolderInPlace(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schema := testSchema()
	stale := claimant("alpha", "stale", "uid-stale")
	next := claimant("beta", "next", "uid-next")

	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(stale, next, schema.NewLease(testNamespace, "key", stale)).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(
				ctx context.Context,
				c client.WithWatch,
				obj client.Object,
				_ ...client.DeleteOption,
			) error {
				// Anything that writes the Lease bumps its resourceVersion.
				// The holder is untouched.
				var lease coordinationv1.Lease
				if err := c.Get(ctx, client.ObjectKeyFromObject(obj), &lease); err != nil {
					return err
				}
				lease.Annotations["example.com/touched"] = "true"
				if err := c.Update(ctx, &lease); err != nil {
					return err
				}

				return apierrors.NewConflict(
					coordinationv1.Resource("leases"), obj.GetName(),
					errors.New("the Lease changed"),
				)
			},
		}).
		Build()

	blocker, err := New(schema, c, c, testNamespace, keepNone).Take(ctx, next, "key")

	require.Error(t, err, "a conflict that decided nothing is a failure to retry")
	assert.True(t, apierrors.IsConflict(err))
	assert.Nil(t, blocker, "no blocker names a holder that keeps nothing")
	lease, found := leaseOf(t, c, "key")
	require.True(t, found)
	recorded, ours := schema.HolderOf(lease)
	require.True(t, ours)
	assert.Equal(t, holderOf(stale), recorded)
}

// TestTakeAnswersALiveHolderWithoutAWrite pins the cost of a parked pass. A
// claimant that waits for a key reaches Take on every reconcile, and a create
// that the API server rejects each time is a write nothing needs.
func TestTakeAnswersALiveHolderWithoutAWrite(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schema := testSchema()
	holder := claimant("alpha", "holder", "uid-holder")
	waiting := claimant("beta", "waiting", "uid-waiting")

	var gets, creates, deletes int
	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(holder, waiting, schema.NewLease(testNamespace, "key", holder)).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(
				ctx context.Context,
				c client.WithWatch,
				key client.ObjectKey,
				obj client.Object,
				opts ...client.GetOption,
			) error {
				gets++

				return c.Get(ctx, key, obj, opts...)
			},
			Create: func(
				ctx context.Context,
				c client.WithWatch,
				obj client.Object,
				opts ...client.CreateOption,
			) error {
				creates++

				return c.Create(ctx, obj, opts...)
			},
			Delete: func(
				ctx context.Context,
				c client.WithWatch,
				obj client.Object,
				opts ...client.DeleteOption,
			) error {
				deletes++

				return c.Delete(ctx, obj, opts...)
			},
		}).
		Build()

	blocker, err := New(schema, c, c, testNamespace, keepEvery).Take(ctx, waiting, "key")

	require.NoError(t, err)
	require.NotNil(t, blocker)
	assert.Equal(t, holderOf(holder), blocker.Holder)
	assert.Equal(t, 1, gets, "one read decides a key that a live holder owns")
	assert.Zero(t, creates)
	assert.Zero(t, deletes)
}

// A holder that cannot be read keeps what it holds. A takeover after a failed
// read hands one key to two claimants each time the API server fails.
func TestTakeKeepsTheClaimWhenTheHolderCannotBeRead(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	holder := claimant("alpha", "holder", "uid-holder")
	next := claimant("beta", "next", "uid-next")
	c := testClient(t, holder, next)

	require.NoError(
		t, taken(New(testSchema(), c, c, testNamespace, keepEvery).Take(ctx, holder, "key")),
	)

	unreadable := func(context.Context, Holder) (bool, error) {
		return false, errors.New("the API server did not answer")
	}
	blocker, err := New(testSchema(), c, c, testNamespace, unreadable).Take(ctx, next, "key")

	require.Error(t, err)
	assert.Nil(t, blocker)
	lease, found := leaseOf(t, c, "key")
	require.True(t, found)
	recorded, _ := testSchema().HolderOf(lease)
	assert.Equal(t, holderOf(holder), recorded)
}

// TestTakeBlocksOnAForeignLease pins that the protocol never takes over a
// Lease it did not write. A name collision with something else in the
// namespace of the operator must not lose that object its Lease.
func TestTakeBlocksOnAForeignLease(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	owner := claimant("alpha", "only", "uid-only")
	foreign := &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{
		Namespace: testNamespace, Name: testSchema().LeaseName("key"),
	}}
	c := testClient(t, owner, foreign)
	claims := New(testSchema(), c, c, testNamespace, keepNone)

	blocker, err := claims.Take(ctx, owner, "key")

	require.NoError(t, err)
	require.NotNil(t, blocker)
	assert.True(t, blocker.Foreign())
	assert.Equal(t, foreign.Name, blocker.Lease.Name)
	_, found := leaseOf(t, c, "key")
	assert.True(t, found, "a foreign Lease must survive")
}

// A release or a takeover can remove the Lease between the create that
// answered AlreadyExists and the read that follows it. The claimant creates
// it again rather than reporting a holder it never saw.
func TestTakeCreatesAgainWhenTheLeaseGoesAwayMidClaim(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	owner := claimant("alpha", "only", "uid-only")

	var creates int
	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(owner).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(
				ctx context.Context,
				c client.WithWatch,
				obj client.Object,
				opts ...client.CreateOption,
			) error {
				creates++
				if creates == 1 {
					return apierrors.NewAlreadyExists(
						coordinationv1.Resource("leases"), obj.GetName(),
					)
				}

				return c.Create(ctx, obj, opts...)
			},
		}).
		Build()
	claims := New(testSchema(), c, c, testNamespace, keepNone)

	blocker, err := claims.Take(ctx, owner, "key")

	require.NoError(t, err)
	assert.Nil(t, blocker)
	assert.Equal(t, 2, creates)
	_, found := leaseOf(t, c, "key")
	assert.True(t, found)
}

// A Lease that answers AlreadyExists and is gone on every read leaves the
// claim undecided. The claimant reports that rather than taking the key.
func TestTakeReportsALeaseItCannotRead(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	owner := claimant("alpha", "only", "uid-only")
	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(owner).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(
				_ context.Context,
				_ client.WithWatch,
				obj client.Object,
				_ ...client.CreateOption,
			) error {
				return apierrors.NewAlreadyExists(coordinationv1.Resource("leases"), obj.GetName())
			},
		}).
		Build()
	claims := New(testSchema(), c, c, testNamespace, keepNone)

	_, err := claims.Take(ctx, owner, "key")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "test claim Lease")
	assert.Contains(t, err.Error(), "went away twice")
}

// A nil Blocker is the success of Take, and every consumer branches on
// Foreign before it reads the holder.
func TestForeignReadsANilBlocker(t *testing.T) {
	t.Parallel()

	var blocker *Blocker

	assert.False(t, blocker.Foreign())
}

// TestTakeOverLeavesTheLeaseOfAnotherHolder pins the guard that lets a takeover
// delete a Lease. A claimant that removes the Lease of another resource hands
// one key to two of them.
func TestTakeOverLeavesTheLeaseOfAnotherHolder(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schema := testSchema()
	holder := claimant("alpha", "holder", "uid-holder")
	other := claimant("beta", "other", "uid-other")
	c := testClient(t, holder, other)
	claims := New(schema, c, c, testNamespace, keepEvery)
	require.NoError(t, taken(claims.Take(ctx, holder, "key")))
	stale, found := leaseOf(t, c, "key")
	require.True(t, found)

	// The other claimant takes the Lease over after the read, so the
	// preconditions of the delete no longer hold.
	moved := stale.DeepCopy()
	moved.Annotations[schema.HolderNamespaceAnnotation] = other.Namespace
	moved.Annotations[schema.HolderNameAnnotation] = other.Name
	moved.Annotations[schema.HolderUIDAnnotation] = string(other.UID)
	require.NoError(t, c.Update(ctx, moved))

	require.NoError(t, claims.takeOver(ctx, "key", stale))

	lease, found := leaseOf(t, c, "key")
	require.True(t, found, "the Lease of the other holder survives")
	assert.True(t, schema.heldBy(lease, other.UID))

	require.NoError(t, claims.takeOver(ctx, "key", lease))

	_, found = leaseOf(t, c, "key")
	assert.False(t, found)
}

// A Lease that is already gone is no failure: a release runs again after a
// crash, and a takeover races other claimants.
func TestTakeOverLeavesAMissingLeaseAlone(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	owner := claimant("alpha", "only", "uid-only")
	c := testClient(t, owner)
	claims := New(testSchema(), c, c, testNamespace, keepEvery)
	require.NoError(t, taken(claims.Take(ctx, owner, "key")))
	lease, found := leaseOf(t, c, "key")
	require.True(t, found)
	require.NoError(t, claims.Release(ctx, lease))

	assert.NoError(t, claims.takeOver(ctx, "key", lease))
}

// A holder that the create discovers, after the read found no Lease, is
// judged like one the read found. A claimant that crashed between its create
// and its release must not block the key for a pass.
func TestTakeTakesOverAHolderThatTheCreateDiscovers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schema := testSchema()
	gone := claimant("alpha", "gone", "uid-gone")
	next := claimant("beta", "next", "uid-next")

	var creates, deletes int
	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(next).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(
				ctx context.Context,
				c client.WithWatch,
				obj client.Object,
				opts ...client.CreateOption,
			) error {
				creates++
				// The gone claimant wins the first create in the window
				// between the read and this create, and is deleted right
				// after.
				if creates == 1 {
					if err := c.Create(ctx, schema.NewLease(testNamespace, "key", gone)); err != nil {
						return err
					}

					return apierrors.NewAlreadyExists(
						coordinationv1.Resource("leases"), obj.GetName(),
					)
				}

				return c.Create(ctx, obj, opts...)
			},
			Delete: func(
				ctx context.Context,
				c client.WithWatch,
				obj client.Object,
				opts ...client.DeleteOption,
			) error {
				deletes++

				return c.Delete(ctx, obj, opts...)
			},
		}).
		Build()

	blocker, err := New(schema, c, c, testNamespace, keepNone).Take(ctx, next, "key")

	require.NoError(t, err)
	assert.Nil(t, blocker, "the dead holder is taken over in the same pass")
	assert.Equal(t, 1, deletes)
	assert.Equal(t, 2, creates)
	lease, found := leaseOf(t, c, "key")
	require.True(t, found)
	assert.True(t, schema.heldBy(lease, next.UID))
}

// The rule of TakeUnclaimed runs only while no Lease holds the key, and its
// error stops the claim before any write.
func TestTakeUnclaimedRunsTheRuleOnAFreeKeyOnly(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	holder := claimant("alpha", "holder", "uid-holder")
	waiting := claimant("beta", "waiting", "uid-waiting")
	c := testClient(t, holder, waiting)
	claims := New(testSchema(), c, c, testNamespace, keepEvery)

	var runs int
	count := func(context.Context) error {
		runs++

		return nil
	}

	require.NoError(t, taken(claims.TakeUnclaimed(ctx, holder, "key", count)))
	assert.Equal(t, 1, runs, "a free key runs the rule")

	require.NoError(t, taken(claims.TakeUnclaimed(ctx, holder, "key", count)))
	assert.Equal(t, 1, runs, "the holder of the key runs no rule")

	blocker, err := claims.TakeUnclaimed(ctx, waiting, "key", count)
	require.NoError(t, err)
	require.NotNil(t, blocker)
	assert.Equal(t, holderOf(holder), blocker.Holder)
	assert.Equal(t, 1, runs, "a held key runs no rule")
}

// errNotFirst stands for the order a caller puts in front of the claim.
var errNotFirst = errors.New("another claimant goes first")

func TestTakeUnclaimedReturnsTheErrorOfTheRule(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	owner := claimant("alpha", "only", "uid-only")

	var creates int
	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(owner).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(
				ctx context.Context,
				c client.WithWatch,
				obj client.Object,
				opts ...client.CreateOption,
			) error {
				creates++

				return c.Create(ctx, obj, opts...)
			},
		}).
		Build()
	claims := New(testSchema(), c, c, testNamespace, keepEvery)

	blocker, err := claims.TakeUnclaimed(ctx, owner, "key", func(context.Context) error {
		return fmt.Errorf("ordering the claimants: %w", errNotFirst)
	})

	require.ErrorIs(t, err, errNotFirst)
	assert.Nil(t, blocker)
	assert.Zero(t, creates, "a refused order writes nothing")
}

// OwnerExists answers for a holder by its UID: the resource that stands under
// the recorded name keeps the claim only while it is the recorded one.
func TestOwnerExists(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	owner := claimant("alpha", "owner", "uid-owner")
	c := testClient(t, owner)
	keeps := OwnerExists(c, testSchema())

	kept, err := keeps(ctx, holderOf(owner))
	require.NoError(t, err)
	assert.True(t, kept, "the recorded resource keeps the claim")

	kept, err = keeps(ctx, holderOf(claimant("alpha", "owner", "uid-later")))
	require.NoError(t, err)
	assert.False(t, kept, "a later resource of the same name keeps nothing")

	kept, err = keeps(ctx, holderOf(claimant("alpha", "gone", "uid-gone")))
	require.NoError(t, err)
	assert.False(t, kept, "a resource that is gone keeps nothing")
}

func TestOwnerExistsReportsAReadThatFails(t *testing.T) {
	t.Parallel()

	owner := claimant("alpha", "owner", "uid-owner")
	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(owner).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(
				context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption,
			) error {
				return errors.New("the API server is away")
			},
		}).
		Build()

	kept, err := OwnerExists(c, testSchema())(context.Background(), holderOf(owner))

	require.Error(t, err)
	assert.False(t, kept)
	assert.Contains(t, err.Error(), "reading the test claim holder alpha/owner")
}

// TestHeldReadsTheHolderAnnotations pins that the label selector alone does
// not decide what a resource holds. The labels carry the name of the holder
// with no namespace and no UID, so they also match the Leases of a resource
// of another namespace with this name.
func TestHeldReadsTheHolderAnnotations(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	owner := claimant("alpha", "shared", "uid-owner")
	twin := claimant("beta", "shared", "uid-twin")
	c := testClient(t, owner, twin)
	claims := New(testSchema(), c, c, testNamespace, keepEvery)
	require.NoError(t, taken(claims.Take(ctx, owner, "first")))
	require.NoError(t, taken(claims.Take(ctx, owner, "second")))
	require.NoError(t, taken(claims.Take(ctx, twin, "third")))

	held, err := claims.Held(ctx, owner)

	require.NoError(t, err)
	require.Len(t, held, 2)
	assert.ElementsMatch(
		t,
		[]string{claims.LeaseName("first"), claims.LeaseName("second")},
		[]string{held[0].Name, held[1].Name},
	)
}

// A holder that reads a claim as its own must be able to give it back. The
// UID decides both, so a name annotation that somebody edited by hand leaves
// no claim that this resource holds and never sweeps.
func TestHeldListsAClaimWhoseNameAnnotationChanged(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schema := testSchema()
	owner := claimant("alpha", "only", "uid-only")
	c := testClient(t, owner)
	claims := New(schema, c, c, testNamespace, keepEvery)
	require.NoError(t, taken(claims.Take(ctx, owner, "key")))

	lease, found := leaseOf(t, c, "key")
	require.True(t, found)
	lease.Annotations[schema.HolderNameAnnotation] = "edited-by-hand"
	require.NoError(t, c.Update(ctx, lease))

	holds, err := claims.Holds(ctx, "key", owner)
	require.NoError(t, err)
	assert.True(t, holds, "the recorded UID is the owner")

	held, err := claims.Held(ctx, owner)

	require.NoError(t, err)
	assert.Len(t, held, 1, "a claim it holds is a claim it can give back")
}

// TestHoldsReadsTheRecordedUID pins the predicate that a deletion asks
// before it writes the thing behind the claim.
func TestHoldsReadsTheRecordedUID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	owner := claimant("alpha", "only", "uid-only")
	c := testClient(t, owner)
	claims := New(testSchema(), c, c, testNamespace, keepEvery)

	held, err := claims.Holds(ctx, "key", owner)
	require.NoError(t, err)
	assert.False(t, held, "a Lease that is gone holds nothing")

	require.NoError(t, taken(claims.Take(ctx, owner, "key")))

	held, err = claims.Holds(ctx, "key", owner)
	require.NoError(t, err)
	assert.True(t, held)

	replacement := claimant("alpha", "only", "uid-replacement")
	held, err = claims.Holds(ctx, "key", replacement)
	require.NoError(t, err)
	assert.False(t, held, "a later resource of the same name holds nothing")
}

// TestReleaseDeletesUnderThePreconditions pins the guard that keeps a
// takeover from removing a Lease that changed since it was read.
func TestReleaseDeletesUnderThePreconditions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	owner := claimant("alpha", "only", "uid-only")
	c := testClient(t, owner)
	claims := New(testSchema(), c, c, testNamespace, keepEvery)
	require.NoError(t, taken(claims.Take(ctx, owner, "key")))

	lease, found := leaseOf(t, c, "key")
	require.True(t, found)
	stale := lease.DeepCopy()
	stale.ResourceVersion = "999999"

	require.Error(t, claims.Release(ctx, stale))
	_, found = leaseOf(t, c, "key")
	assert.True(t, found)

	require.NoError(t, claims.Release(ctx, lease))
	_, found = leaseOf(t, c, "key")
	assert.False(t, found)
}

// The fake client applies the resourceVersion precondition and ignores the
// UID one, so only an assertion on the request itself keeps the UID guard.
// Without it a Lease that was released and created again under the same name
// is deleted by the claimant that read the first one.
func TestReleaseSendsTheUIDPrecondition(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schema := testSchema()
	owner := claimant("alpha", "only", "uid-only")
	lease := schema.NewLease(testNamespace, "key", owner)
	lease.UID = "lease-uid"

	var sent *metav1.Preconditions
	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(owner, lease).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(
				ctx context.Context,
				c client.WithWatch,
				obj client.Object,
				opts ...client.DeleteOption,
			) error {
				options := &client.DeleteOptions{}
				options.ApplyOptions(opts)
				sent = options.Preconditions

				return c.Delete(ctx, obj, opts...)
			},
		}).
		Build()
	claims := New(schema, c, c, testNamespace, keepEvery)

	read, found := leaseOf(t, c, "key")
	require.True(t, found)
	require.NoError(t, claims.Release(ctx, read))

	require.NotNil(t, sent, "the delete carries preconditions")
	require.NotNil(t, sent.UID, "the delete carries the UID precondition")
	assert.Equal(t, read.UID, *sent.UID)
	require.NotNil(t, sent.ResourceVersion)
	assert.Equal(t, read.ResourceVersion, *sent.ResourceVersion)
}

// TestReadReportsAMissingLease pins the found flag that every caller branches
// on before it reads a holder.
func TestReadReportsAMissingLease(t *testing.T) {
	t.Parallel()

	c := testClient(t)
	claims := New(testSchema(), c, c, testNamespace, keepEvery)

	lease, found, err := claims.Read(context.Background(), "key")

	require.NoError(t, err)
	assert.False(t, found)
	assert.Nil(t, lease)
}

// TestSchemaValidate names every field that a claim Lease needs to be read
// back as a claim.
func TestSchemaValidate(t *testing.T) {
	t.Parallel()

	require.NoError(t, testSchema().Validate())

	cases := map[string]func(*Schema[*corev1.ConfigMap]){
		"no prefix":               func(s *Schema[*corev1.ConfigMap]) { s.Prefix = "" },
		"a prefix with no hyphen": func(s *Schema[*corev1.ConfigMap]) { s.Prefix = "camunda" },
		"no noun":                 func(s *Schema[*corev1.ConfigMap]) { s.Noun = "" },
		"no Labels":               func(s *Schema[*corev1.ConfigMap]) { s.Labels = nil },
		"no holder namespace":     func(s *Schema[*corev1.ConfigMap]) { s.HolderNamespaceAnnotation = "" },
		"no holder name":          func(s *Schema[*corev1.ConfigMap]) { s.HolderNameAnnotation = "" },
		"no holder uid":           func(s *Schema[*corev1.ConfigMap]) { s.HolderUIDAnnotation = "" },
		"no key":                  func(s *Schema[*corev1.ConfigMap]) { s.KeyAnnotation = "" },
		"two equal holder keys":   func(s *Schema[*corev1.ConfigMap]) { s.HolderNameAnnotation = s.HolderUIDAnnotation },
	}

	for name, breakSchema := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			schema := testSchema()
			breakSchema(&schema)

			assert.Error(t, schema.Validate())
		})
	}
}

// TestLeaseName pins the name that every claimant of one key meets on. A
// claim key is no DNS subdomain, and the Lease name has to be one.
func TestLeaseName(t *testing.T) {
	t.Parallel()

	schema := testSchema()

	name := schema.LeaseName("7000000000000000001/camunda")

	assert.True(t, strings.HasPrefix(name, "camunda-test-"))
	assert.Empty(t, validation.IsDNS1123Subdomain(name))
	assert.Equal(t, name, schema.LeaseName("7000000000000000001/camunda"))
	assert.NotEqual(t, name, schema.LeaseName("7000000000000000002/camunda"))
}

// TestNewLeaseBoundsTheHolderIdentity pins that a long name reaches the
// annotations whole while the display form stays inside the bound.
func TestNewLeaseBoundsTheHolderIdentity(t *testing.T) {
	t.Parallel()

	schema := testSchema()
	long := strings.Repeat("n", 200)

	lease := schema.NewLease(testNamespace, "key", claimant("apps", long, "uid-1"))

	require.NotNil(t, lease.Spec.HolderIdentity)
	assert.LessOrEqual(t, len(*lease.Spec.HolderIdentity), MaxHolderIdentityLength)
	assert.Equal(t, long, lease.Annotations[schema.HolderNameAnnotation])
	assert.Equal(t, "key", lease.Annotations[schema.KeyAnnotation])
}

// TestHolderOfRejectsAPartialRecord pins that ownership needs all three
// annotations. A Lease that carries fewer is not one of ours, and the
// protocol never takes it over.
func TestHolderOfRejectsAPartialRecord(t *testing.T) {
	t.Parallel()

	schema := testSchema()
	for _, missing := range []string{
		schema.HolderNamespaceAnnotation,
		schema.HolderNameAnnotation,
		schema.HolderUIDAnnotation,
	} {
		t.Run(missing, func(t *testing.T) {
			t.Parallel()

			lease := schema.NewLease(testNamespace, "key", claimant("apps", "orders", "uid-1"))
			delete(lease.Annotations, missing)

			holder, ours := schema.HolderOf(lease)

			assert.False(t, ours)
			assert.Equal(t, Holder{}, holder)
		})
	}
}
