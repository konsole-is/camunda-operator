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
func testSchema() Schema {
	return Schema{
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

// holderOf is the holder identity of a claimant.
func holderOf(owner client.Object) Holder {
	return Holder{
		NamespacedName: client.ObjectKeyFromObject(owner),
		UID:            owner.GetUID(),
	}
}

// testClient returns a fake client that holds the given objects and serves
// the Lease type.
func testClient(t *testing.T, objects ...client.Object) client.WithWatch {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, coordinationv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

// keepEvery is the HolderKeeps of a claim whose holders never go away.
func keepEvery(context.Context, Holder) (bool, error) { return true, nil }

// keepNone is the HolderKeeps of a claim whose holders are all gone.
func keepNone(context.Context, Holder) (bool, error) { return false, nil }

// first drops the blocker of a Take that a spec expects to succeed, so the
// arrangement of that spec reads as one line.
func first(_ *Blocker, err error) error { return err }

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
	first := claimant("alpha", "first", "uid-first")
	second := claimant("beta", "second", "uid-second")
	c := testClient(t, first, second)
	claims := New(testSchema(), c, c, testNamespace, keepEvery)

	blocker, err := claims.Take(ctx, first, "7000000000000000001/camunda")
	require.NoError(t, err)
	assert.Nil(t, blocker)

	blocker, err = claims.Take(ctx, second, "7000000000000000001/camunda")

	require.NoError(t, err)
	require.NotNil(t, blocker)
	assert.False(t, blocker.Foreign())
	assert.Equal(t, holderOf(first), blocker.Holder)
	assert.Equal(t, testSchema().LeaseName("7000000000000000001/camunda"), blocker.Lease.Name)
	assert.Equal(t, testNamespace, blocker.Lease.Namespace)

	lease, found := leaseOf(t, c, "7000000000000000001/camunda")
	require.True(t, found)
	holder, ours := testSchema().HolderOf(lease)
	require.True(t, ours)
	assert.Equal(t, holderOf(first), holder)
}

// A reconcile runs again on every event, so the holder reaches its own claim
// over and over. It must read as taken and never as blocked.
func TestTakeIsIdempotentForTheHolder(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	owner := claimant("alpha", "only", "uid-only")
	c := testClient(t, owner)
	claims := New(testSchema(), c, c, testNamespace, keepEvery)

	require.NoError(t, first(claims.Take(ctx, owner, "key")))

	blocker, err := claims.Take(ctx, owner, "key")

	require.NoError(t, err)
	assert.Nil(t, blocker)
}

// TestTakeTakesOverAHolderThatKeepsNothing covers the crash between a claim
// and its release. Nothing else hands the key on.
func TestTakeTakesOverAHolderThatKeepsNothing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	gone := claimant("alpha", "gone", "uid-gone")
	next := claimant("beta", "next", "uid-next")
	c := testClient(t, gone, next)

	require.NoError(t, first(New(testSchema(), c, c, testNamespace, keepEvery).Take(ctx, gone, "key")))

	blocker, err := New(testSchema(), c, c, testNamespace, keepNone).Take(ctx, next, "key")

	require.NoError(t, err)
	assert.Nil(t, blocker)
	lease, found := leaseOf(t, c, "key")
	require.True(t, found)
	holder, ours := testSchema().HolderOf(lease)
	require.True(t, ours)
	assert.Equal(t, holderOf(next), holder)
}

// A holder that cannot be read keeps what it holds. A takeover after a failed
// read hands one key to two claimants each time the API server fails.
func TestTakeKeepsTheClaimWhenTheHolderCannotBeRead(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	holder := claimant("alpha", "holder", "uid-holder")
	next := claimant("beta", "next", "uid-next")
	c := testClient(t, holder, next)

	require.NoError(t, first(New(testSchema(), c, c, testNamespace, keepEvery).Take(ctx, holder, "key")))

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
	scheme := runtime.NewScheme()
	require.NoError(t, coordinationv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	var creates int
	c := fake.NewClientBuilder().
		WithScheme(scheme).
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
	scheme := runtime.NewScheme()
	require.NoError(t, coordinationv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	c := fake.NewClientBuilder().
		WithScheme(scheme).
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
	require.NoError(t, first(claims.Take(ctx, holder, "key")))

	require.NoError(t, claims.Drop(ctx, "key", holderOf(other)))

	_, found := leaseOf(t, c, "key")
	assert.True(t, found)

	require.NoError(t, claims.Drop(ctx, "key", holderOf(holder)))

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

	assert.NoError(t, claims.Drop(context.Background(), "key", holderOf(owner)))
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
	require.NoError(t, first(claims.Take(ctx, owner, "first")))
	require.NoError(t, first(claims.Take(ctx, owner, "second")))
	require.NoError(t, first(claims.Take(ctx, twin, "third")))

	held, err := claims.Held(ctx, holderOf(owner))

	require.NoError(t, err)
	require.Len(t, held, 2)
	assert.ElementsMatch(
		t,
		[]string{testSchema().LeaseName("first"), testSchema().LeaseName("second")},
		[]string{held[0].Name, held[1].Name},
	)
}

// TestReleaseDeletesUnderThePreconditions pins the guard that keeps a
// takeover from removing a Lease that changed since it was read.
func TestReleaseDeletesUnderThePreconditions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	owner := claimant("alpha", "only", "uid-only")
	c := testClient(t, owner)
	claims := New(testSchema(), c, c, testNamespace, keepEvery)
	require.NoError(t, first(claims.Take(ctx, owner, "key")))

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
