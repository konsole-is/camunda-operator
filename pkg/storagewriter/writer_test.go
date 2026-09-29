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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	require.NoError(t, Register(ctx, c, c, claimNamespace, backend, claim, w))

	live, err := Live(ctx, c, claimNamespace, backend, claim, "other-cluster")
	require.NoError(t, err)
	assert.Equal(t, []string{"LogicalRestoreRDBMS apps/restore"}, live)

	own, err := Live(ctx, c, claimNamespace, backend, claim, "target")
	require.NoError(t, err)
	assert.Empty(t, own, "a writer for the cluster itself does not hold that cluster")
}

func TestLiveExceptLeavesOutOnlyTheRegistrationOfTheWriter(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	earlier := restore("restore", "target")
	again := earlier
	again.UID = "uid-restore-again"
	require.NoError(t, Register(ctx, c, c, claimNamespace, backend, claim, earlier))
	require.NoError(t, Register(ctx, c, c, claimNamespace, backend, claim, again))

	live, err := LiveExcept(ctx, c, claimNamespace, backend, claim, again)

	require.NoError(t, err)
	assert.Equal(
		t,
		[]string{"LogicalRestoreRDBMS apps/restore"},
		live,
		"a writer of the same name and another UID counts, the one for the same cluster too",
	)
}

func TestAWriterOfAnotherBackendIsNotLive(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	require.NoError(t, Register(ctx, c, c, claimNamespace, "rdbms|other:5432/camunda", claim, restore("r", "t")))

	live, err := Live(ctx, c, claimNamespace, backend, claim, "other-cluster")

	require.NoError(t, err)
	assert.Empty(t, live)
}

// A registration carries no time that a reader measures, so its age never
// frees the backend.
func TestARegistrationHoldsTheBackendWhateverItsAge(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	w := restore("restore", "target")
	old := metav1.NewMicroTime(metav1.Now().AddDate(-1, 0, 0))
	lease := newLease(claimNamespace, backend, claim, w)
	lease.Spec.AcquireTime = &old
	lease.Spec.RenewTime = &old
	lease.Spec.LeaseDurationSeconds = new(int32(1))
	require.NoError(t, c.Create(ctx, lease))

	live, err := Live(ctx, c, claimNamespace, backend, claim, "other-cluster")

	require.NoError(t, err)
	assert.Equal(t, []string{"LogicalRestoreRDBMS apps/restore"}, live)
}

func TestRegisterTwiceKeepsOneRegistration(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	w := restore("r", "t")

	require.NoError(t, Register(ctx, c, c, claimNamespace, backend, claim, w))
	require.NoError(t, Register(ctx, c, c, claimNamespace, backend, claim, w))

	var leases coordinationv1.LeaseList
	require.NoError(t, c.List(ctx, &leases, client.InNamespace(claimNamespace)))
	assert.Len(t, leases.Items, 1)
}

func TestReleaseEndsTheRegistrationAndToleratesAMissingLease(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	w := restore("r", "t")
	require.NoError(t, Register(ctx, c, c, claimNamespace, backend, claim, w))

	require.NoError(t, Release(ctx, c, claimNamespace, backend, w))
	require.NoError(t, Release(ctx, c, claimNamespace, backend, w))

	live, err := Live(ctx, c, claimNamespace, backend, claim, "other")
	require.NoError(t, err)
	assert.Empty(t, live)
}

func TestReleaseAllEndsEveryRegistrationOfTheWriterAndNoOther(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	w := restore("restore", "target")
	other := restore("other", "target")
	moved := "rdbms|moved.apps.svc:5432/camunda"
	require.NoError(t, Register(ctx, c, c, claimNamespace, backend, claim, w))
	require.NoError(t, Register(ctx, c, c, claimNamespace, moved, "other-claim", w))
	require.NoError(t, Register(ctx, c, c, claimNamespace, backend, claim, other))

	require.NoError(t, ReleaseAll(ctx, c, c, claimNamespace, w.UID))
	require.NoError(t, ReleaseAll(ctx, c, c, claimNamespace, w.UID), "a writer with no registration is fine")

	var leases coordinationv1.LeaseList
	require.NoError(t, c.List(ctx, &leases, client.InNamespace(claimNamespace)))
	require.Len(t, leases.Items, 1)
	assert.Equal(t, LeaseName(backend, other.UID), leases.Items[0].Name)
}

func TestIsWriterLease(t *testing.T) {
	writer := newLease(claimNamespace, backend, claim, restore("r", "t"))
	other := &coordinationv1.Lease{}
	other.SetLabels(map[string]string{"app.kubernetes.io/managed-by": "someone-else"})

	assert.True(t, IsWriterLease(writer))
	assert.False(t, IsWriterLease(other))
}

func TestRegisterRestoresTheIdentityOfATamperedLease(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	w := restore("restore", "target")
	tampered := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{Namespace: claimNamespace, Name: LeaseName(backend, w.UID)},
	}
	require.NoError(t, c.Create(ctx, tampered))

	require.NoError(t, Register(ctx, c, c, claimNamespace, backend, claim, w))

	live, err := Live(ctx, c, claimNamespace, backend, claim, "other-cluster")
	require.NoError(t, err)
	assert.Equal(t, []string{"LogicalRestoreRDBMS apps/restore"}, live)

	require.NoError(t, ReleaseAll(ctx, c, c, claimNamespace, w.UID))
	live, err = Live(ctx, c, claimNamespace, backend, claim, "other-cluster")
	require.NoError(t, err)
	assert.Empty(t, live, "the restored registration carries the UID of its writer")
}
