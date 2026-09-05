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
	"path/filepath"
	"strings"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/testing/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// claimScheme registers the Lease type that the claim reader lists.
func claimScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, coordinationv1.AddToScheme(scheme))

	return scheme
}

// claimLease builds the claim Lease of key that holder holds.
func claimLease(namespace, key string, holder ClaimHolder) *coordinationv1.Lease {
	return ClaimSchema().NewLease(namespace, key, &v1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: holder.Namespace,
			Name:      holder.Name,
			UID:       holder.UID,
		},
	})
}

func testHolder(namespace, name string, uid types.UID) ClaimHolder {
	return ClaimHolder{
		NamespacedName: types.NamespacedName{Namespace: namespace, Name: name},
		UID:            uid,
	}
}

func TestClaimLeaseName(t *testing.T) {
	name := ClaimSchema().LeaseName("7000000000000000001/camunda")

	assert.True(t, strings.HasPrefix(name, "camunda-database-"))
	assert.Equal(t, name, ClaimSchema().LeaseName("7000000000000000001/camunda"))
	assert.NotEqual(t, name, ClaimSchema().LeaseName("7000000000000000002/camunda"))
	assert.NotEqual(t, name, ClaimSchema().LeaseName("7000000000000000001/other"))
}

func TestNewClaimLease(t *testing.T) {
	holder := testHolder("apps", "orders", "uid-1")

	lease := claimLease("camunda-system", "7000000000000000001/camunda", holder)

	assert.Equal(t, ClaimSchema().LeaseName("7000000000000000001/camunda"), lease.Name)
	assert.Equal(t, "camunda-system", lease.Namespace)
	assert.Equal(t, ClaimLeaseLabels("orders"), lease.Labels)
	assert.Equal(t, "7000000000000000001/camunda", lease.Annotations[ClaimKeyAnnotation])
	require.NotNil(t, lease.Spec.HolderIdentity)
	assert.Equal(t, "apps/orders", *lease.Spec.HolderIdentity)
}

func TestClaimSchemaHolderOf(t *testing.T) {
	holder := testHolder("apps", "orders", "uid-1")
	lease := claimLease("camunda-system", "7000000000000000001/camunda", holder)

	recorded, ours := ClaimSchema().HolderOf(lease)

	assert.True(t, ours)
	assert.Equal(t, holder, recorded)
}

func TestClaimSchemaHolderOfRejectsAPartialRecord(t *testing.T) {
	for _, missing := range []string{
		ClaimHolderNamespaceAnnotation,
		ClaimHolderNameAnnotation,
		ClaimHolderUIDAnnotation,
	} {
		t.Run(missing, func(t *testing.T) {
			holder := testHolder("apps", "orders", "uid-1")
			lease := claimLease("camunda-system", "7000000000000000001/camunda", holder)
			delete(lease.Annotations, missing)

			recorded, ours := ClaimSchema().HolderOf(lease)

			assert.False(t, ours)
			assert.Equal(t, ClaimHolder{}, recorded)
		})
	}
}

func TestListClaims(t *testing.T) {
	holder := testHolder("apps", "orders", "uid-1")
	other := testHolder("shop", "carts", "uid-2")
	c := fake.NewClientBuilder().
		WithScheme(claimScheme(t)).
		WithObjects(
			claimLease("camunda-system", "7000000000000000001/camunda", holder),
			claimLease("camunda-system", "7000000000000000002/carts", other),
		).
		Build()

	claims, err := ListClaims(context.Background(), c, "camunda-system")

	require.NoError(t, err)
	want := []Claim{
		{Holder: holder, Key: "7000000000000000001/camunda"},
		{Holder: other, Key: "7000000000000000002/carts"},
	}
	assert.ElementsMatch(t, want, claims)
}

func TestListClaimsReadsOneNamespace(t *testing.T) {
	holder := testHolder("apps", "orders", "uid-1")
	c := fake.NewClientBuilder().
		WithScheme(claimScheme(t)).
		WithObjects(claimLease("elsewhere", "7000000000000000001/camunda", holder)).
		Build()

	claims, err := ListClaims(context.Background(), c, "camunda-system")

	require.NoError(t, err)
	assert.Empty(t, claims)
}

// A Lease of the leader election of an operator carries neither the labels of
// a claim nor its annotations. It claims no logical database.
func TestListClaimsLeavesOutALeaseOfSomebodyElse(t *testing.T) {
	foreign := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{Namespace: "camunda-system", Name: "leader-election"},
	}
	c := fake.NewClientBuilder().WithScheme(claimScheme(t)).WithObjects(foreign).Build()

	claims, err := ListClaims(context.Background(), c, "camunda-system")

	require.NoError(t, err)
	assert.Empty(t, claims)
}

func TestListClaimsLeavesOutALeaseThatNamesNoDatabase(t *testing.T) {
	holder := testHolder("apps", "orders", "uid-1")
	lease := claimLease("camunda-system", "7000000000000000001/camunda", holder)
	delete(lease.Annotations, ClaimHolderUIDAnnotation)
	c := fake.NewClientBuilder().WithScheme(claimScheme(t)).WithObjects(lease).Build()

	claims, err := ListClaims(context.Background(), c, "camunda-system")

	require.NoError(t, err)
	assert.Empty(t, claims)
}

func TestListClaimsLeavesOutALeaseThatNamesNoKey(t *testing.T) {
	holder := testHolder("apps", "orders", "uid-1")
	lease := claimLease("camunda-system", "7000000000000000001/camunda", holder)
	delete(lease.Annotations, ClaimKeyAnnotation)
	c := fake.NewClientBuilder().WithScheme(claimScheme(t)).WithObjects(lease).Build()

	claims, err := ListClaims(context.Background(), c, "camunda-system")

	require.NoError(t, err)
	assert.Empty(t, claims)
}

func TestListClaimsReportsAFailedRead(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()

	_, err := ListClaims(context.Background(), c, "camunda-system")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "camunda-system")
}

// The claims of one Database carry its name, so a release can select them
// without a read of every claim in the namespace.
func TestClaimLeaseLabelsSelectOneDatabase(t *testing.T) {
	holder := testHolder("apps", "orders", "uid-1")
	c := fake.NewClientBuilder().
		WithScheme(claimScheme(t)).
		WithObjects(
			claimLease("camunda-system", "7000000000000000001/camunda", holder),
			claimLease("camunda-system", "7000000000000000002/carts", testHolder("shop", "carts", "uid-2")),
		).
		Build()

	var leases coordinationv1.LeaseList
	err := c.List(
		context.Background(), &leases,
		client.InNamespace("camunda-system"),
		client.MatchingLabels(ClaimLeaseLabels("orders")),
	)

	require.NoError(t, err)
	require.Len(t, leases.Items, 1)
	assert.Equal(t, ClaimSchema().LeaseName("7000000000000000001/camunda"), leases.Items[0].Name)
}

// TestNewClaimLeaseMatchesTheGolden pins the claim Lease against the shape
// the operator wrote before the protocol moved to pkg/leaseclaim. A change of
// its name, its labels or its annotations loses the operator every claim it
// holds. The running holder keeps a Lease that no later claimant reads.
func TestNewClaimLeaseMatchesTheGolden(t *testing.T) {
	cases := map[string]struct {
		file     string
		key      string
		database *v1.Database
	}{
		"claimlease": {
			file: "claimlease.yaml",
			key:  "7000000000000000001/camunda",
			database: &v1.Database{ObjectMeta: metav1.ObjectMeta{
				Namespace: "apps", Name: "orders", UID: "uid-1",
			}},
		},
		"a name that the bounds cut": {
			file: "claimlease-longname.yaml",
			key:  "7000000000000000002/other",
			database: &v1.Database{ObjectMeta: metav1.ObjectMeta{
				Namespace: strings.Repeat("s", 60),
				Name:      strings.Repeat("n", 200),
				UID:       "uid-2",
			}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			lease := ClaimSchema().NewLease("camunda-system", tc.key, tc.database)

			require.NotNil(t, lease.Spec.AcquireTime)
			golden.AssertYAML(
				t,
				filepath.Join("testdata", "golden", tc.file),
				leasePreview{lease: lease},
				golden.WithScheme(leaseGoldenScheme(t)),
				golden.Update(*update),
			)
		})
	}
}

// leasePreview renders a claim Lease for the golden comparison. The acquire
// time is the wall clock of the render, so it is zeroed.
type leasePreview struct {
	lease *coordinationv1.Lease
}

func (p leasePreview) Preview() (client.Object, error) {
	lease := p.lease.DeepCopy()
	lease.Spec.AcquireTime = nil

	return lease, nil
}

// leaseGoldenScheme serves the Lease that the golden serializer renders.
func leaseGoldenScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, coordinationv1.AddToScheme(scheme))

	return scheme
}
