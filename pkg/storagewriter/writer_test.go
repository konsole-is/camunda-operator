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

package storagewriter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	claimNamespace = "operator"
	backend        = "rdbms|db.apps.svc:5432/camunda"
	claim          = "camunda-storage-0123456789abcdef0123456789abcdef01234567"
)

var start = time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)

func newClient(t *testing.T) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, coordinationv1.AddToScheme(scheme))

	return fake.NewClientBuilder().WithScheme(scheme).Build()
}

func restore(name string, cluster types.UID) Writer {
	return Writer{
		Kind:       "LogicalRestoreRDBMS",
		Namespace:  "apps",
		Name:       name,
		UID:        types.UID("uid-" + name),
		ClusterUID: cluster,
	}
}

func TestRegisterMakesTheWriterLiveForOtherClusters(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	w := restore("restore", "target")

	require.NoError(t, Register(ctx, c, c, claimNamespace, backend, claim, w, start))

	live, err := Live(ctx, c, claimNamespace, backend, claim, "other-cluster", start)
	require.NoError(t, err)
	assert.Equal(t, []string{"LogicalRestoreRDBMS apps/restore"}, live)

	own, err := Live(ctx, c, claimNamespace, backend, claim, "target", start)
	require.NoError(t, err)
	assert.Empty(t, own, "a writer for the cluster itself does not hold that cluster")
}

func TestAWriterOfAnotherBackendIsNotLive(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	require.NoError(t, Register(ctx, c, c, claimNamespace, "rdbms|other:5432/camunda", claim, restore("r", "t"), start))

	live, err := Live(ctx, c, claimNamespace, backend, claim, "other-cluster", start)

	require.NoError(t, err)
	assert.Empty(t, live)
}

func TestARegistrationExpiresWithoutARenewal(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	require.NoError(t, Register(ctx, c, c, claimNamespace, backend, claim, restore("r", "t"), start))

	before, err := Live(ctx, c, claimNamespace, backend, claim, "other", start.Add(Duration-time.Second))
	require.NoError(t, err)
	assert.Len(t, before, 1)

	after, err := Live(ctx, c, claimNamespace, backend, claim, "other", start.Add(Duration))
	require.NoError(t, err)
	assert.Empty(t, after)
}

func TestRegisterRenewsOnlyAfterTheRenewInterval(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	w := restore("r", "t")
	require.NoError(t, Register(ctx, c, c, claimNamespace, backend, claim, w, start))

	require.NoError(t, Register(ctx, c, c, claimNamespace, backend, claim, w, start.Add(RenewInterval-time.Second)))
	assert.Equal(t, start, renewTime(t, c, w))

	later := start.Add(RenewInterval)
	require.NoError(t, Register(ctx, c, c, claimNamespace, backend, claim, w, later))
	assert.Equal(t, later, renewTime(t, c, w))

	live, err := Live(ctx, c, claimNamespace, backend, claim, "other", start.Add(Duration))
	require.NoError(t, err)
	assert.Len(t, live, 1, "the renewal moves the expiry")
}

func TestReleaseEndsTheRegistrationAndToleratesAMissingLease(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	w := restore("r", "t")
	require.NoError(t, Register(ctx, c, c, claimNamespace, backend, claim, w, start))

	require.NoError(t, Release(ctx, c, claimNamespace, backend, w))
	require.NoError(t, Release(ctx, c, claimNamespace, backend, w))

	live, err := Live(ctx, c, claimNamespace, backend, claim, "other", start)
	require.NoError(t, err)
	assert.Empty(t, live)
}

func TestIsWriterLease(t *testing.T) {
	writer := newLease(claimNamespace, backend, claim, restore("r", "t"), start)
	other := &coordinationv1.Lease{}
	other.SetLabels(map[string]string{"app.kubernetes.io/managed-by": "someone-else"})

	assert.True(t, IsWriterLease(writer))
	assert.False(t, IsWriterLease(other))
}

func renewTime(t *testing.T, c client.Client, w Writer) time.Time {
	t.Helper()
	var lease coordinationv1.Lease
	key := types.NamespacedName{Namespace: claimNamespace, Name: LeaseName(backend, w.UID)}
	require.NoError(t, c.Get(context.Background(), key, &lease))

	return lease.Spec.RenewTime.UTC()
}

func TestPruneExpiredDeletesOnlyExpiredRegistrationsOfTheBackend(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	stale := restore("stale", "target")
	fresh := restore("fresh", "target")
	elsewhere := restore("elsewhere", "target")

	require.NoError(t, Register(ctx, c, c, claimNamespace, backend, claim, stale, start))
	require.NoError(t, Register(ctx, c, c, claimNamespace, backend, claim, fresh, start.Add(Duration)))
	require.NoError(t, Register(ctx, c, c, claimNamespace, "rdbms|other:5432/db", claim, elsewhere, start))

	require.NoError(t, PruneExpired(ctx, c, c, claimNamespace, backend, claim, start.Add(Duration)))

	var leases coordinationv1.LeaseList
	require.NoError(t, c.List(ctx, &leases, client.InNamespace(claimNamespace)))
	names := make([]string, 0, len(leases.Items))
	for _, lease := range leases.Items {
		names = append(names, lease.Name)
	}
	want := []string{LeaseName(backend, fresh.UID), LeaseName("rdbms|other:5432/db", elsewhere.UID)}
	assert.ElementsMatch(t, want, names)
}

func TestPruneExpiredKeepsARegistrationRenewedAfterTheRead(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	w := restore("restore", "target")
	require.NoError(t, Register(ctx, c, c, claimNamespace, backend, claim, w, start))

	// The reader serves the Lease as it was before the writer renewed it.
	stale := newClient(t)
	var read coordinationv1.Lease
	require.NoError(
		t,
		c.Get(ctx, types.NamespacedName{Namespace: claimNamespace, Name: LeaseName(backend, w.UID)}, &read),
	)
	read.ResourceVersion = ""
	require.NoError(t, stale.Create(ctx, &read))
	require.NoError(t, Register(ctx, c, c, claimNamespace, backend, claim, w, start.Add(Duration)))

	require.NoError(t, PruneExpired(ctx, c, stale, claimNamespace, backend, claim, start.Add(Duration)))

	live, err := Live(ctx, c, claimNamespace, backend, claim, "other-cluster", start.Add(Duration))
	require.NoError(t, err)
	assert.Equal(t, []string{"LogicalRestoreRDBMS apps/restore"}, live)
}
