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

package clusterclaim_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/clusterclaim"
)

// The claim is exercised against the fake client. Every claimant kind of the
// operator appears here: the claim reads any of them, and a holder of a kind
// it cannot read must keep the cluster.

const claimNamespace = "ns"

var (
	first  = clusterclaim.Claimant{Kind: "LogicalBackupElasticsearch", Name: "nightly", UID: types.UID("uid-first")}
	second = clusterclaim.Claimant{Kind: "LogicalBackupRDBMS", Name: "adhoc", UID: types.UID("uid-second")}
)

func claimClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(s))
	require.NoError(t, v1.AddToScheme(s))
	return fake.NewClientBuilder().WithScheme(s).WithObjects(objects...).Build()
}

func objectMeta(claimant clusterclaim.Claimant) metav1.ObjectMeta {
	return metav1.ObjectMeta{Namespace: claimNamespace, Name: claimant.Name, UID: claimant.UID}
}

// holderResource fakes the backup resource of a claimant in the given phase.
// An empty phase leaves status.phase unset, as a fresh resource has it.
func holderResource(claimant clusterclaim.Claimant, phase v1.LogicalBackupPhase) client.Object {
	if claimant.Kind == "LogicalBackupRDBMS" {
		return &v1.LogicalBackupRDBMS{
			ObjectMeta: objectMeta(claimant),
			Status:     v1.LogicalBackupRDBMSStatus{Phase: phase},
		}
	}
	return &v1.LogicalBackupElasticsearch{
		ObjectMeta: objectMeta(claimant),
		Status:     v1.LogicalBackupElasticsearchStatus{Phase: phase},
	}
}

// readyReason builds the status conditions of a resource whose Ready
// condition carries the given reason.
func readyReason(reason string) []metav1.Condition {
	return []metav1.Condition{
		{
			Type: v1.ConditionReady, Status: metav1.ConditionFalse,
			Reason: reason, LastTransitionTime: metav1.Now(),
		},
	}
}

// resumeFailedHolder fakes a backup that ended Failed with the Ready reason
// ResumeFailed: terminal, and the cluster's exporting is still paused. The
// terminal reason is unset, so the Ready condition alone decides.
func resumeFailedHolder(claimant clusterclaim.Claimant) client.Object {
	backup := holderResource(claimant, v1.LogicalBackupFailed).(*v1.LogicalBackupElasticsearch)
	backup.Status.Conditions = readyReason(v1.ReasonResumeFailed)
	return backup
}

// restoreHolder fakes the resource of a restore claimant. terminal picks a
// phase the restore never leaves.
func restoreHolder(claimant clusterclaim.Claimant, terminal bool) client.Object {
	if claimant.Kind == "PointInTimeRestore" {
		phase := v1.PointInTimeRestoreRestoringPrimaryStorage
		if terminal {
			phase = v1.PointInTimeRestoreCompleted
		}
		return &v1.PointInTimeRestore{
			ObjectMeta: objectMeta(claimant),
			Status:     v1.PointInTimeRestoreStatus{Phase: phase},
		}
	}
	phase := v1.LogicalRestoreRestoringPrimaryStorage
	if terminal {
		phase = v1.LogicalRestoreCompleted
	}
	return &v1.LogicalRestore{
		ObjectMeta: objectMeta(claimant),
		Status:     v1.LogicalRestoreStatus{Phase: phase},
	}
}

// leaseHolder returns the exact identity that the Lease records in its
// annotations. For a Lease without them it returns the holderIdentity
// field. Without a Lease it returns "".
func leaseHolder(t *testing.T, c client.Client) string {
	t.Helper()
	lease := readLease(t, c)
	if lease == nil {
		return ""
	}
	annotations := lease.GetAnnotations()
	kind, name, uid := annotations[clusterclaim.ClaimHolderKindAnnotation],
		annotations[clusterclaim.ClaimHolderNameAnnotation],
		annotations[clusterclaim.ClaimHolderUIDAnnotation]
	if kind != "" && name != "" && uid != "" {
		return clusterclaim.Claimant{Kind: kind, Name: name, UID: types.UID(uid)}.String()
	}
	require.NotNil(t, lease.Spec.HolderIdentity)
	return *lease.Spec.HolderIdentity
}

// readLease returns the claim Lease of cluster prod, or nil.
func readLease(t *testing.T, c client.Client) *coordinationv1.Lease {
	t.Helper()
	var lease coordinationv1.Lease
	err := c.Get(
		t.Context(), client.ObjectKey{
			Namespace: claimNamespace, Name: clusterclaim.ClaimLeaseName("prod"),
		}, &lease,
	)
	if apierrors.IsNotFound(err) {
		return nil
	}
	require.NoError(t, err)
	return &lease
}

// leaseHeldBy writes the claim Lease of cluster prod the way Claim writes
// it for claimant, without going through Claim. It plays a claimant that
// took the Lease earlier and then went away without a Release.
func leaseHeldBy(t *testing.T, c client.Client, claimant clusterclaim.Claimant) {
	t.Helper()
	holder := claimant.HolderIdentity()
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: claimNamespace,
			Name:      clusterclaim.ClaimLeaseName("prod"),
			Annotations: map[string]string{
				clusterclaim.ClaimHolderKindAnnotation: claimant.Kind,
				clusterclaim.ClaimHolderNameAnnotation: claimant.Name,
				clusterclaim.ClaimHolderUIDAnnotation:  string(claimant.UID),
			},
		},
		Spec: coordinationv1.LeaseSpec{HolderIdentity: &holder},
	}
	require.NoError(t, c.Create(t.Context(), lease))
}

func TestClaimLeaseNameIsDeterministicAndBounded(t *testing.T) {
	assert.Equal(t, "camunda-cluster-prod", clusterclaim.ClaimLeaseName("prod"))
	assert.Equal(t, clusterclaim.ClaimLeaseName("prod"), clusterclaim.ClaimLeaseName("prod"))

	long := strings.Repeat("c", 300)
	name := clusterclaim.ClaimLeaseName(long)
	assert.LessOrEqual(t, len(name), 253)
	assert.True(t, strings.HasPrefix(name, "camunda-cluster-ccc"))
	assert.NotEqual(t, name, clusterclaim.ClaimLeaseName(long+"x"))
	assert.Equal(t, name, clusterclaim.ClaimLeaseName(long))
}

func TestClaimantStringRoundTrips(t *testing.T) {
	assert.Equal(t, "LogicalBackupElasticsearch/nightly/uid-first", first.String())
	assert.Equal(t, "LogicalBackupElasticsearch/nightly", first.Display())

	parsed, err := clusterclaim.ParseClaimant(first.String())
	require.NoError(t, err)
	assert.Equal(t, first, parsed)

	for _, bad := range []string{"", "nightly", "Kind/name", "Kind//uid", "a/b/c/d"} {
		_, err := clusterclaim.ParseClaimant(bad)
		assert.Error(t, err, bad)
	}
}

func TestFirstClaimWinsAndTheSecondIsBlockedWithTheHolderNamed(t *testing.T) {
	c := claimClient(t, holderResource(first, v1.LogicalBackupRunning), holderResource(second, ""))

	holder, err := clusterclaim.Claim(t.Context(), c, c, claimNamespace, "prod", first)
	require.NoError(t, err)
	assert.Empty(t, holder)
	assert.Equal(t, first.String(), leaseHolder(t, c))

	holder, err = clusterclaim.Claim(t.Context(), c, c, claimNamespace, "prod", second)
	require.NoError(t, err)
	assert.Equal(t, first.String(), holder)
	assert.Equal(t, first.String(), leaseHolder(t, c), "the Lease still names the first claimant")
}

func TestReEntryByTheHolderIsIdempotent(t *testing.T) {
	c := claimClient(t, holderResource(first, v1.LogicalBackupRunning))

	holds, err := clusterclaim.Holds(t.Context(), c, claimNamespace, "prod", first)
	require.NoError(t, err)
	assert.False(t, holds, "nothing is held before the first claim")

	for range 3 {
		holder, err := clusterclaim.Claim(t.Context(), c, c, claimNamespace, "prod", first)
		require.NoError(t, err)
		assert.Empty(t, holder)
	}
	assert.Equal(t, first.String(), leaseHolder(t, c))

	holds, err = clusterclaim.Holds(t.Context(), c, claimNamespace, "prod", first)
	require.NoError(t, err)
	assert.True(t, holds)
	holds, err = clusterclaim.Holds(t.Context(), c, claimNamespace, "prod", second)
	require.NoError(t, err)
	assert.False(t, holds, "another claimant does not hold it")
}

func TestClaimsOfDifferentClustersDoNotMeet(t *testing.T) {
	c := claimClient(t, holderResource(first, v1.LogicalBackupRunning), holderResource(second, v1.LogicalBackupRunning))

	holder, err := clusterclaim.Claim(t.Context(), c, c, claimNamespace, "prod", first)
	require.NoError(t, err)
	assert.Empty(t, holder)
	holder, err = clusterclaim.Claim(t.Context(), c, c, claimNamespace, "staging", second)
	require.NoError(t, err)
	assert.Empty(t, holder)
}

func TestInactiveHoldersAreTakenOver(t *testing.T) {
	tests := []struct {
		name    string
		objects []client.Object
	}{
		{
			name:    "the holder is terminal",
			objects: []client.Object{holderResource(first, v1.LogicalBackupCompleted)},
		},
		{
			name:    "the holder failed",
			objects: []client.Object{holderResource(first, v1.LogicalBackupFailed)},
		},
		{
			name:    "the holder resource is gone",
			objects: nil,
		},
		{
			name: "the holder resource was recreated with the same name",
			objects: []client.Object{holderResource(
				clusterclaim.Claimant{Kind: first.Kind, Name: first.Name, UID: "uid-later"},
				v1.LogicalBackupRunning,
			)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := append([]client.Object{holderResource(second, v1.LogicalBackupPending)}, tt.objects...)
			c := claimClient(t, objects...)
			// The first claimant took the Lease when it was active, then
			// went the way the case describes without a Release.
			leaseHeldBy(t, c, first)

			got, err := clusterclaim.Claim(t.Context(), c, c, claimNamespace, "prod", second)
			require.NoError(t, err)
			assert.Empty(t, got)
			assert.Equal(t, second.String(), leaseHolder(t, c))
		})
	}
}

func TestActiveHoldersAreNotTakenOver(t *testing.T) {
	for _, phase := range []v1.LogicalBackupPhase{"", v1.LogicalBackupPending, v1.LogicalBackupRunning} {
		t.Run(string(phase), func(t *testing.T) {
			c := claimClient(t, holderResource(first, phase), holderResource(second, v1.LogicalBackupPending))
			_, err := clusterclaim.Claim(t.Context(), c, c, claimNamespace, "prod", first)
			require.NoError(t, err)

			got, err := clusterclaim.Claim(t.Context(), c, c, claimNamespace, "prod", second)
			require.NoError(t, err)
			assert.Equal(t, first.String(), got)
		})
	}
}

func TestALeaseThatIsNotOursBlocksWithoutATakeover(t *testing.T) {
	// A foreign Lease carries no holder annotations. Its holderIdentity can
	// even have the shape of ours: without the annotations it is not ours,
	// and it is never taken over.
	for _, foreign := range []string{"someone-else", "LogicalBackupElasticsearch/nightly/uid-first"} {
		t.Run(foreign, func(t *testing.T) {
			c := claimClient(t, holderResource(second, v1.LogicalBackupPending))
			var lease coordinationv1.Lease
			lease.Namespace = claimNamespace
			lease.Name = clusterclaim.ClaimLeaseName("prod")
			holder := foreign
			lease.Spec.HolderIdentity = &holder
			require.NoError(t, c.Create(t.Context(), &lease))

			got, err := clusterclaim.Claim(t.Context(), c, c, claimNamespace, "prod", second)
			require.NoError(t, err)
			assert.Equal(t, foreign, got)
			assert.Equal(t, foreign, leaseHolder(t, c))
		})
	}
}

// A CR name can be 253 characters. The identity of such a claimant does not
// fit a bounded holderIdentity. The Lease carries the exact identity in its
// annotations and a bounded display form in the field.
func TestALongNamedClaimantClaimsReleasesAndIsTakenOver(t *testing.T) {
	long := clusterclaim.Claimant{
		Kind: "LogicalBackupElasticsearch", Name: strings.Repeat("n", 253), UID: types.UID("uid-long"),
	}
	c := claimClient(t, holderResource(long, v1.LogicalBackupRunning), holderResource(second, v1.LogicalBackupPending))

	got, err := clusterclaim.Claim(t.Context(), c, c, claimNamespace, "prod", long)
	require.NoError(t, err)
	assert.Empty(t, got)
	lease := readLease(t, c)
	require.NotNil(t, lease)
	assert.LessOrEqual(t, len(*lease.Spec.HolderIdentity), 128)
	assert.True(t, strings.HasPrefix(*lease.Spec.HolderIdentity, "LogicalBackupElasticsearch/nnn"))
	assert.Equal(t, long.String(), leaseHolder(t, c))
	assert.Equal(t, strings.Repeat("n", 253), lease.Annotations[clusterclaim.ClaimHolderNameAnnotation])

	holds, err := clusterclaim.Holds(t.Context(), c, claimNamespace, "prod", long)
	require.NoError(t, err)
	assert.True(t, holds)
	got, err = clusterclaim.Claim(t.Context(), c, c, claimNamespace, "prod", second)
	require.NoError(t, err)
	assert.Equal(t, long.String(), got, "the second claimant is blocked by the exact identity")

	require.NoError(t, clusterclaim.Release(t.Context(), c, c, claimNamespace, "prod", long))
	assert.Empty(t, leaseHolder(t, c))

	// Stale: the long-named holder is terminal now, and its Lease is left
	// behind. The second claimant takes it over.
	c = claimClient(t, holderResource(long, v1.LogicalBackupCompleted), holderResource(second, v1.LogicalBackupPending))
	leaseHeldBy(t, c, long)
	got, err = clusterclaim.Claim(t.Context(), c, c, claimNamespace, "prod", second)
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Equal(t, second.String(), leaseHolder(t, c))
}

func TestHolderIdentityIsBounded(t *testing.T) {
	short := clusterclaim.Claimant{Kind: "LogicalBackupRDBMS", Name: "adhoc", UID: "u"}
	assert.Equal(t, "LogicalBackupRDBMS/adhoc", short.HolderIdentity())

	long := clusterclaim.Claimant{Kind: "LogicalBackupRDBMS", Name: strings.Repeat("x", 253), UID: "u"}
	assert.LessOrEqual(t, len(long.HolderIdentity()), 128)
	assert.Equal(t, long.HolderIdentity(), long.HolderIdentity())
	other := clusterclaim.Claimant{Kind: "LogicalBackupRDBMS", Name: strings.Repeat("x", 252) + "y", UID: "u"}
	assert.NotEqual(t, long.HolderIdentity(), other.HolderIdentity())
}

func TestReleaseOnlyByTheHolder(t *testing.T) {
	c := claimClient(t, holderResource(first, v1.LogicalBackupRunning), holderResource(second, v1.LogicalBackupPending))
	_, err := clusterclaim.Claim(t.Context(), c, c, claimNamespace, "prod", first)
	require.NoError(t, err)

	require.NoError(t, clusterclaim.Release(t.Context(), c, c, claimNamespace, "prod", second))
	assert.Equal(t, first.String(), leaseHolder(t, c), "a non-holder release is a no-op")

	require.NoError(t, clusterclaim.Release(t.Context(), c, c, claimNamespace, "prod", first))
	assert.Empty(t, leaseHolder(t, c))

	require.NoError(
		t, clusterclaim.Release(t.Context(), c, c, claimNamespace, "prod", first),
		"a release of a claim that is not held is a no-op",
	)
}

// Every claimant kind answers the same question, through the Terminal method
// that api/v1 gives all four. A live holder of any kind blocks. A terminal
// or a deleted holder of any kind is taken over. A kind the claim cannot
// read blocks and is never taken over.
func TestTakeoverIsTheSameForEveryClaimantKind(t *testing.T) {
	for _, kind := range []string{
		"LogicalBackupElasticsearch", "LogicalBackupRDBMS", "LogicalRestore", "PointInTimeRestore",
	} {
		t.Run(kind, func(t *testing.T) {
			holder := clusterclaim.Claimant{Kind: kind, Name: "holder", UID: types.UID("uid-holder")}
			live, ended := kindHolders(holder)

			t.Run("a live holder blocks", func(t *testing.T) {
				c := claimClient(t, live, holderResource(second, v1.LogicalBackupPending))
				leaseHeldBy(t, c, holder)

				active, err := clusterclaim.HolderActive(t.Context(), c, claimNamespace, holder)
				require.NoError(t, err)
				assert.True(t, active)

				got, err := clusterclaim.Claim(t.Context(), c, c, claimNamespace, "prod", second)
				require.NoError(t, err)
				assert.Equal(t, holder.String(), got)
				assert.Equal(t, holder.String(), leaseHolder(t, c), "not taken over")
			})

			t.Run("a terminal holder is taken over", func(t *testing.T) {
				c := claimClient(t, ended, holderResource(second, v1.LogicalBackupPending))
				leaseHeldBy(t, c, holder)

				active, err := clusterclaim.HolderActive(t.Context(), c, claimNamespace, holder)
				require.NoError(t, err)
				assert.False(t, active)

				got, err := clusterclaim.Claim(t.Context(), c, c, claimNamespace, "prod", second)
				require.NoError(t, err)
				assert.Empty(t, got)
				assert.Equal(t, second.String(), leaseHolder(t, c))
			})

			t.Run("a deleted holder is taken over", func(t *testing.T) {
				c := claimClient(t, holderResource(second, v1.LogicalBackupPending))
				leaseHeldBy(t, c, holder)

				active, err := clusterclaim.HolderActive(t.Context(), c, claimNamespace, holder)
				require.NoError(t, err)
				assert.False(t, active)

				got, err := clusterclaim.Claim(t.Context(), c, c, claimNamespace, "prod", second)
				require.NoError(t, err)
				assert.Empty(t, got)
				assert.Equal(t, second.String(), leaseHolder(t, c))
			})

			t.Run("a holder that ended normally leaves the cluster running", func(t *testing.T) {
				c := claimClient(t, ended)
				paused, err := clusterclaim.HolderKeepsClusterPaused(t.Context(), c, claimNamespace, holder)
				require.NoError(t, err)
				assert.False(t, paused)
			})
		})
	}
}

// kindHolders fakes two resources of the claimant's kind: one that still
// runs, and one that reached a phase it never leaves.
func kindHolders(claimant clusterclaim.Claimant) (live, ended client.Object) {
	switch claimant.Kind {
	case "LogicalBackupElasticsearch", "LogicalBackupRDBMS":
		return holderResource(claimant, v1.LogicalBackupRunning),
			holderResource(claimant, v1.LogicalBackupCompleted)
	default:
		return restoreHolder(claimant, false), restoreHolder(claimant, true)
	}
}

// A holder of a kind the claim does not know keeps the cluster. It can be
// running for all the claim knows, so no claimant takes the Lease from it.
func TestAHolderOfAnUnknownKindBlocksAndIsNeverTakenOver(t *testing.T) {
	unknown := clusterclaim.Claimant{Kind: "SomeFutureKind", Name: "x", UID: types.UID("uid-unknown")}
	c := claimClient(t, holderResource(second, v1.LogicalBackupPending))
	leaseHeldBy(t, c, unknown)

	active, err := clusterclaim.HolderActive(t.Context(), c, claimNamespace, unknown)
	require.NoError(t, err)
	assert.True(t, active, "an unreadable holder is never declared inactive")

	paused, err := clusterclaim.HolderKeepsClusterPaused(t.Context(), c, claimNamespace, unknown)
	require.NoError(t, err)
	assert.False(t, paused)

	got, err := clusterclaim.Claim(t.Context(), c, c, claimNamespace, "prod", second)
	require.NoError(t, err)
	assert.Equal(t, unknown.String(), got)
	assert.Equal(t, unknown.String(), leaseHolder(t, c), "not taken over")
}

func TestHolderOfAKindTheServerDoesNotServeIsInactive(t *testing.T) {
	// The fake client cannot answer NoMatch by itself; an interceptor
	// plays the API server of an operator that ships one backup kind only.
	c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
			gvk := obj.GetObjectKind().GroupVersionKind()
			return &meta.NoKindMatchError{GroupKind: gvk.GroupKind(), SearchedVersions: []string{gvk.Version}}
		},
	}).Build()

	active, err := clusterclaim.HolderActive(t.Context(), c, claimNamespace, second)
	require.NoError(t, err)
	assert.False(t, active)
}

// A ResumeFailed holder ended with exporting still paused. Its claim is
// what keeps a sibling from backing up a paused cluster, so it stays active:
// it blocks, and it is never taken over.
func TestAResumeFailedHolderBlocksAndIsNotTakenOver(t *testing.T) {
	c := claimClient(t, resumeFailedHolder(first), holderResource(second, v1.LogicalBackupPending))
	leaseHeldBy(t, c, first)

	got, err := clusterclaim.Claim(t.Context(), c, c, claimNamespace, "prod", second)
	require.NoError(t, err)
	assert.Equal(t, first.String(), got, "the ResumeFailed holder blocks")
	assert.Equal(t, first.String(), leaseHolder(t, c), "and was not taken over")

	active, err := clusterclaim.HolderActive(t.Context(), c, claimNamespace, first)
	require.NoError(t, err)
	assert.True(t, active)

	paused, err := clusterclaim.HolderKeepsClusterPaused(t.Context(), c, claimNamespace, first)
	require.NoError(t, err)
	assert.True(t, paused)
}

// HolderKeepsClusterPaused says no for every holder that is not a terminal
// ResumeFailed backup: active ones, ordinary terminal ones, and gone ones.
func TestHolderKeepsClusterPausedOnlyForResumeFailed(t *testing.T) {
	tests := []struct {
		name   string
		object client.Object
	}{
		{"running", holderResource(first, v1.LogicalBackupRunning)},
		{"completed", holderResource(first, v1.LogicalBackupCompleted)},
		{"failed via the normal resume", holderResource(first, v1.LogicalBackupFailed)},
		{"gone", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objects []client.Object
			if tt.object != nil {
				objects = append(objects, tt.object)
			}
			c := claimClient(t, objects...)
			paused, err := clusterclaim.HolderKeepsClusterPaused(t.Context(), c, claimNamespace, first)
			require.NoError(t, err)
			assert.False(t, paused)
		})
	}
}

// A long cluster name that is cut at a "." or "-" must not leave the
// separator in front of the hash suffix. The result must stay a valid DNS
// subdomain. Otherwise the Lease can never be created, and admission never
// passes.
func TestClaimLeaseNameOfADottedNameIsAValidDNSSubdomain(t *testing.T) {
	// The head of "camunda-cluster-" plus this name is cut at 244. That is
	// exactly the "." of this name, so the cut lands on the separator.
	dotted := strings.Repeat("a", 227) + "." + strings.Repeat("b", 200)

	name := clusterclaim.ClaimLeaseName(dotted)

	assert.LessOrEqual(t, len(name), 253)
	assert.Empty(t, apimachineryvalidation.NameIsDNSSubdomain(name, false), "the Lease name is a valid DNS subdomain")
	assert.Equal(t, name, clusterclaim.ClaimLeaseName(dotted))
	assert.NotEqual(t, name, clusterclaim.ClaimLeaseName(dotted+"x"))

	// A cut inside a run of separators trims them all.
	messy := strings.Repeat("a", 220) + strings.Repeat(".", 24) + strings.Repeat("b", 100)
	assert.Empty(t, apimachineryvalidation.NameIsDNSSubdomain(clusterclaim.ClaimLeaseName(messy), false))
}

// A terminal-write conflict can persist phase and terminalReason while it
// restores an older Ready condition. The recorded terminalReason decides,
// so such a holder still counts as paused and its claim is never taken over.
func TestATerminalReasonResumeFailedDecidesOverAStaleReadyCondition(t *testing.T) {
	backup := holderResource(first, v1.LogicalBackupFailed).(*v1.LogicalBackupElasticsearch)
	backup.Status.TerminalReason = v1.ReasonResumeFailed
	// The stale Ready of an earlier write: still Progressing.
	backup.Status.Conditions = readyReason("Progressing")
	c := claimClient(t, backup, holderResource(second, v1.LogicalBackupPending))
	leaseHeldBy(t, c, first)

	active, err := clusterclaim.HolderActive(t.Context(), c, claimNamespace, first)
	require.NoError(t, err)
	assert.True(t, active, "the recorded terminal reason wins over the stale condition")

	paused, err := clusterclaim.HolderKeepsClusterPaused(t.Context(), c, claimNamespace, first)
	require.NoError(t, err)
	assert.True(t, paused)

	got, err := clusterclaim.Claim(t.Context(), c, c, claimNamespace, "prod", second)
	require.NoError(t, err)
	assert.Equal(t, first.String(), got)
	assert.Equal(t, first.String(), leaseHolder(t, c), "not taken over")
}

// A terminalReason other than ResumeFailed decides too: the holder resumed
// exporting, so it is inactive whatever a stale Ready condition says.
func TestATerminalReasonFailedIsInactiveDespiteAStaleResumeFailedReady(t *testing.T) {
	backup := holderResource(first, v1.LogicalBackupFailed).(*v1.LogicalBackupElasticsearch)
	backup.Status.TerminalReason = "Failed"
	backup.Status.Conditions = readyReason(v1.ReasonResumeFailed)
	c := claimClient(t, backup)

	active, err := clusterclaim.HolderActive(t.Context(), c, claimNamespace, first)
	require.NoError(t, err)
	assert.False(t, active)
}

// A foreign Lease whose unannotated holderIdentity happens to spell the
// identity of this claimant is still foreign. It is never self, it blocks,
// and it is never taken over.
func TestAForeignLeaseSpellingOurIdentityIsNotSelf(t *testing.T) {
	c := claimClient(t, holderResource(first, v1.LogicalBackupRunning))
	var lease coordinationv1.Lease
	lease.Namespace = claimNamespace
	lease.Name = clusterclaim.ClaimLeaseName("prod")
	holder := first.String()
	lease.Spec.HolderIdentity = &holder
	require.NoError(t, c.Create(t.Context(), &lease))

	holds, err := clusterclaim.Holds(t.Context(), c, claimNamespace, "prod", first)
	require.NoError(t, err)
	assert.False(t, holds, "an unannotated Lease is never held by us")

	got, err := clusterclaim.Claim(t.Context(), c, c, claimNamespace, "prod", first)
	require.NoError(t, err)
	assert.Equal(t, first.String(), got, "it blocks even the claimant it spells")

	var after coordinationv1.Lease
	require.NoError(t, c.Get(
		t.Context(), client.ObjectKey{
			Namespace: claimNamespace, Name: clusterclaim.ClaimLeaseName("prod"),
		}, &after,
	))
	assert.Empty(t, after.GetAnnotations(), "the foreign Lease was not taken over or rewritten")
}

func TestHolderActive(t *testing.T) {
	tests := []struct {
		name   string
		object client.Object
		active bool
	}{
		{"fresh resource without a phase", holderResource(first, ""), true},
		{"pending", holderResource(first, v1.LogicalBackupPending), true},
		{"running", holderResource(first, v1.LogicalBackupRunning), true},
		{"completed", holderResource(first, v1.LogicalBackupCompleted), false},
		{"failed", holderResource(first, v1.LogicalBackupFailed), false},
		{"gone", nil, false},
		{
			"another UID",
			holderResource(
				clusterclaim.Claimant{Kind: first.Kind, Name: first.Name, UID: "other"},
				v1.LogicalBackupRunning,
			),
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objects []client.Object
			if tt.object != nil {
				objects = append(objects, tt.object)
			}
			c := claimClient(t, objects...)
			active, err := clusterclaim.HolderActive(t.Context(), c, claimNamespace, first)
			require.NoError(t, err)
			assert.Equal(t, tt.active, active)
		})
	}
}
