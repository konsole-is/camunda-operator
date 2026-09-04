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
	assert.Equal(t, HolderOf(earlier), blocker.Holder)
	assert.Equal(t, testSchema().LeaseName("7000000000000000001/camunda"), blocker.Lease.Name)
	assert.Equal(t, testNamespace, blocker.Lease.Namespace)

	lease, found := leaseOf(t, c, "7000000000000000001/camunda")
	require.True(t, found)
	holder, ours := testSchema().HolderOf(lease)
	require.True(t, ours)
	assert.Equal(t, HolderOf(earlier), holder)
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
	require.Equal(t, HolderOf(gone), holder, "the stale holder must be arranged")

	blocker, err := New(testSchema(), c, c, testNamespace, keepNone).Take(ctx, next, "key")

	require.NoError(t, err)
	assert.Nil(t, blocker)
	assert.Equal(t, 1, deletes, "the stale Lease is deleted rather than reused")
	lease, found := leaseOf(t, c, "key")
	require.True(t, found)
	holder, ours = testSchema().HolderOf(lease)
	require.True(t, ours)
	assert.Equal(t, HolderOf(next), holder)
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
	assert.Equal(t, HolderOf(winner), blocker.Holder)
	_, found := leaseOf(t, c, "key")
	assert.True(t, found, "the Lease of the winner survives")
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
	assert.Equal(t, HolderOf(holder), recorded)
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
	assert.Contains(t, err.Error(), "not readable yet")
}

// A nil Blocker is the success of Take, and every consumer branches on
// Foreign before it reads the holder.
func TestForeignReadsANilBlocker(t *testing.T) {
	t.Parallel()

	var blocker *Blocker

	assert.False(t, blocker.Foreign())
}

// TestDropLeavesTheLeaseOfAnotherHolder pins the guard that lets a takeover
// delete a Lease. A claimant that removes the Lease of another resource hands
// one key to two of them.
func TestDropLeavesTheLeaseOfAnotherHolder(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	holder := claimant("alpha", "holder", "uid-holder")
	other := claimant("beta", "other", "uid-other")
	c := testClient(t, holder, other)
	claims := New(testSchema(), c, c, testNamespace, keepEvery)
	require.NoError(t, taken(claims.Take(ctx, holder, "key")))

	require.NoError(t, claims.Drop(ctx, "key", HolderOf(other)))

	_, found := leaseOf(t, c, "key")
	assert.True(t, found)

	require.NoError(t, claims.Drop(ctx, "key", HolderOf(holder)))

	_, found = leaseOf(t, c, "key")
	assert.False(t, found)
}

// A Lease that is already gone is no failure: a release runs again after a
// crash, and a takeover races other claimants.
func TestDropLeavesAMissingLeaseAlone(t *testing.T) {
	t.Parallel()

	owner := claimant("alpha", "only", "uid-only")
	c := testClient(t, owner)
	claims := New(testSchema(), c, c, testNamespace, keepEvery)

	assert.NoError(t, claims.Drop(context.Background(), "key", HolderOf(owner)))
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

	held, err := claims.Held(ctx, HolderOf(owner))

	require.NoError(t, err)
	require.Len(t, held, 2)
	assert.ElementsMatch(
		t,
		[]string{claims.LeaseName("first"), claims.LeaseName("second")},
		[]string{held[0].Name, held[1].Name},
	)
}

// TestHoldsReadsTheWholeIdentity pins the predicate that a deletion asks
// before it writes the thing behind the claim.
func TestHoldsReadsTheWholeIdentity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	owner := claimant("alpha", "only", "uid-only")
	c := testClient(t, owner)
	claims := New(testSchema(), c, c, testNamespace, keepEvery)

	held, err := claims.Holds(ctx, "key", HolderOf(owner))
	require.NoError(t, err)
	assert.False(t, held, "a Lease that is gone holds nothing")

	require.NoError(t, taken(claims.Take(ctx, owner, "key")))

	held, err = claims.Holds(ctx, "key", HolderOf(owner))
	require.NoError(t, err)
	assert.True(t, held)

	replacement := claimant("alpha", "only", "uid-replacement")
	held, err = claims.Holds(ctx, "key", HolderOf(replacement))
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

// TestNewRefusesWiringThatCannotDecideOwnership pins the guard on the
// arguments of New. A nil HolderKeeps runs only where two claimants meet,
// which is the case the package exists for.
func TestNewRefusesWiringThatCannotDecideOwnership(t *testing.T) {
	t.Parallel()

	c := testClient(t)
	cases := map[string]func(){
		"a Schema with nothing in it": func() {
			New(Schema[*corev1.ConfigMap]{}, c, c, testNamespace, keepEvery)
		},
		"a Schema with two equal annotation keys": func() {
			schema := testSchema()
			schema.KeyAnnotation = schema.HolderUIDAnnotation
			New(schema, c, c, testNamespace, keepEvery)
		},
		"no client":      func() { New(testSchema(), nil, c, testNamespace, keepEvery) },
		"no reader":      func() { New(testSchema(), c, nil, testNamespace, keepEvery) },
		"no HolderKeeps": func() { New(testSchema(), c, c, testNamespace, nil) },
	}

	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Panics(t, build)
		})
	}
}

// TestSchemaValidate names every field that a claim Lease needs to be read
// back as a claim.
func TestSchemaValidate(t *testing.T) {
	t.Parallel()

	require.NoError(t, testSchema().Validate())

	cases := map[string]func(*Schema[*corev1.ConfigMap]){
		"no prefix":             func(s *Schema[*corev1.ConfigMap]) { s.Prefix = "" },
		"no noun":               func(s *Schema[*corev1.ConfigMap]) { s.Noun = "" },
		"no Labels":             func(s *Schema[*corev1.ConfigMap]) { s.Labels = nil },
		"no holder namespace":   func(s *Schema[*corev1.ConfigMap]) { s.HolderNamespaceAnnotation = "" },
		"no holder name":        func(s *Schema[*corev1.ConfigMap]) { s.HolderNameAnnotation = "" },
		"no holder uid":         func(s *Schema[*corev1.ConfigMap]) { s.HolderUIDAnnotation = "" },
		"no key":                func(s *Schema[*corev1.ConfigMap]) { s.KeyAnnotation = "" },
		"two equal holder keys": func(s *Schema[*corev1.ConfigMap]) { s.HolderNameAnnotation = s.HolderUIDAnnotation },
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
