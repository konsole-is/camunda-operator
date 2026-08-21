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

package restore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/clusterclaim"
)

// The cluster that every case here claims, and the restore that claims it.
const claimNamespace = "ns"

// selfOwner is the restore that claims the cluster. Take and Give derive the
// claimant from it, so selfClaimant is what the Lease must record afterwards.
func selfOwner() *v1.PointInTimeRestore {
	owner := restoreOwner(1)
	owner.UID = "restore-uid"

	return owner
}

func selfClaimant() clusterclaim.Claimant {
	return clusterclaim.Claimant{
		Kind: "PointInTimeRestore", Name: "my-cluster-pitr", UID: "restore-uid",
	}
}

func backupClaimant() clusterclaim.Claimant {
	return clusterclaim.Claimant{
		Kind: "LogicalBackupRDBMS", Name: "nightly", UID: "backup-uid",
	}
}

// backupHolder is the resource of the backup that holds the claim. A backup
// that is not terminal still needs the cluster.
func backupHolder(phase v1.LogicalBackupPhase) client.Object {
	return &v1.LogicalBackupRDBMS{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: claimNamespace, Name: backupClaimant().Name, UID: backupClaimant().UID,
		},
		Status: v1.LogicalBackupRDBMSStatus{Phase: phase},
	}
}

// heldLease is the claim Lease of the cluster, as the backup left it.
func heldLease() *coordinationv1.Lease {
	holder := backupClaimant().HolderIdentity()

	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: claimNamespace,
			Name:      clusterclaim.ClaimLeaseName("my-cluster"),
			Annotations: map[string]string{
				clusterclaim.ClaimHolderKindAnnotation: backupClaimant().Kind,
				clusterclaim.ClaimHolderNameAnnotation: backupClaimant().Name,
				clusterclaim.ClaimHolderUIDAnnotation:  string(backupClaimant().UID),
			},
		},
		Spec: coordinationv1.LeaseSpec{HolderIdentity: &holder},
	}
}

func claimClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()

	return fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(objects...).Build()
}

// leaseHolder returns the exact identity that the claim Lease records, or ""
// when no Lease exists.
func leaseHolder(t *testing.T, c client.Client) string {
	t.Helper()

	var lease coordinationv1.Lease
	key := types.NamespacedName{
		Namespace: claimNamespace, Name: clusterclaim.ClaimLeaseName("my-cluster"),
	}
	if err := c.Get(t.Context(), key, &lease); err != nil {
		require.True(t, apierrors.IsNotFound(err))

		return ""
	}
	annotations := lease.GetAnnotations()

	return clusterclaim.Claimant{
		Kind: annotations[clusterclaim.ClaimHolderKindAnnotation],
		Name: annotations[clusterclaim.ClaimHolderNameAnnotation],
		UID:  types.UID(annotations[clusterclaim.ClaimHolderUIDAnnotation]),
	}.String()
}

// A cluster that nobody holds is the restore's, and the Lease says so. Every
// phase that touches storage runs behind this claim.
func TestTakeClaimsAnUnclaimedCluster(t *testing.T) {
	t.Parallel()

	c := claimClient(t)

	outcome, err := Take(t.Context(), c, c, selfOwner(), "my-cluster")
	require.NoError(t, err)

	assert.Equal(t, Outcome{Done: true}, outcome)
	assert.Equal(t, selfClaimant().String(), leaseHolder(t, c))
}

// The restore waits while another operation of any kind holds the cluster.
// The reason names no kind, because the holder can be a backup or another
// restore.
func TestTakeHoldsWhileAnotherOperationRuns(t *testing.T) {
	t.Parallel()

	c := claimClient(t, heldLease(), backupHolder(v1.LogicalBackupRunning))

	outcome, err := Take(t.Context(), c, c, selfOwner(), "my-cluster")
	require.NoError(t, err)

	require.NotNil(t, outcome.Failure)
	assert.Equal(t, v1.ReasonClusterClaimed, outcome.Failure.Reason)
	assert.Contains(t, outcome.Failure.Message, "LogicalBackupRDBMS/nightly")
	assert.Contains(t, outcome.Failure.Message, "my-cluster")
	assert.False(t, outcome.Done)
	assert.Equal(t, backupClaimant().String(), leaseHolder(t, c), "the holder keeps its claim")
}

// Nothing bounds the hold. The restore starts on its own when the holder
// reaches a terminal phase, because the next look takes the claim over.
func TestTakeTakesOverATerminalHolder(t *testing.T) {
	t.Parallel()

	c := claimClient(t, heldLease(), backupHolder(v1.LogicalBackupCompleted))

	outcome, err := Take(t.Context(), c, c, selfOwner(), "my-cluster")
	require.NoError(t, err)

	assert.Equal(t, Outcome{Done: true}, outcome)
	assert.Equal(t, selfClaimant().String(), leaseHolder(t, c))
}

// A restore that re-enters after a failed status flush finds itself as the
// holder and goes on.
func TestTakeIsIdempotentForTheHolder(t *testing.T) {
	t.Parallel()

	c := claimClient(t)
	_, err := Take(t.Context(), c, c, selfOwner(), "my-cluster")
	require.NoError(t, err)

	outcome, err := Take(t.Context(), c, c, selfOwner(), "my-cluster")
	require.NoError(t, err)

	assert.Equal(t, Outcome{Done: true}, outcome)
	assert.Equal(t, selfClaimant().String(), leaseHolder(t, c))
}

func TestGiveReleasesTheClaimOfTheHolder(t *testing.T) {
	t.Parallel()

	c := claimClient(t)
	_, err := Take(t.Context(), c, c, selfOwner(), "my-cluster")
	require.NoError(t, err)

	require.NoError(t, Give(t.Context(), c, c, selfOwner(), "my-cluster"))
	assert.Empty(t, leaseHolder(t, c))
}

// Give runs on every look of a terminal restore, so a claim that another
// operation holds must survive it.
func TestGiveLeavesTheClaimOfAnotherHolder(t *testing.T) {
	t.Parallel()

	c := claimClient(t, heldLease(), backupHolder(v1.LogicalBackupRunning))

	require.NoError(t, Give(t.Context(), c, c, selfOwner(), "my-cluster"))
	assert.Equal(t, backupClaimant().String(), leaseHolder(t, c))
}
