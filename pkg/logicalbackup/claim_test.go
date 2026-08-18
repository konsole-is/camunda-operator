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

package logicalbackup_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
)

// The claim is exercised against the fake client. The holder resources are
// unstructured, because the claim reads them without a typed client. This
// test must run whichever backup kind the module has compiled in.

const claimNamespace = "ns"

var (
	first  = logicalbackup.Claimant{Kind: "LogicalBackupElasticsearch", Name: "nightly", UID: types.UID("uid-first")}
	second = logicalbackup.Claimant{Kind: "LogicalBackupRDBMS", Name: "adhoc", UID: types.UID("uid-second")}
)

func claimClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(s))
	return fake.NewClientBuilder().WithScheme(s).WithObjects(objects...).Build()
}

// holderResource fakes the backup resource of a claimant in the given
// phase. An empty phase leaves status.phase unset, as a fresh resource has
// it.
func holderResource(claimant logicalbackup.Claimant, phase v1.LogicalBackupPhase) client.Object {
	backup := &unstructured.Unstructured{}
	backup.SetGroupVersionKind(v1.GroupVersion.WithKind(claimant.Kind))
	backup.SetNamespace(claimNamespace)
	backup.SetName(claimant.Name)
	backup.SetUID(claimant.UID)
	if phase != "" {
		_ = unstructured.SetNestedField(backup.Object, string(phase), "status", "phase")
	}
	return backup
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
	kind, name, uid := annotations[logicalbackup.ClaimHolderKindAnnotation],
		annotations[logicalbackup.ClaimHolderNameAnnotation],
		annotations[logicalbackup.ClaimHolderUIDAnnotation]
	if kind != "" && name != "" && uid != "" {
		return logicalbackup.Claimant{Kind: kind, Name: name, UID: types.UID(uid)}.String()
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
			Namespace: claimNamespace, Name: logicalbackup.ClaimLeaseName("prod"),
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
func leaseHeldBy(t *testing.T, c client.Client, claimant logicalbackup.Claimant) {
	t.Helper()
	holder := claimant.HolderIdentity()
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: claimNamespace,
			Name:      logicalbackup.ClaimLeaseName("prod"),
			Annotations: map[string]string{
				logicalbackup.ClaimHolderKindAnnotation: claimant.Kind,
				logicalbackup.ClaimHolderNameAnnotation: claimant.Name,
				logicalbackup.ClaimHolderUIDAnnotation:  string(claimant.UID),
			},
		},
		Spec: coordinationv1.LeaseSpec{HolderIdentity: &holder},
	}
	require.NoError(t, c.Create(t.Context(), lease))
}

func TestClaimLeaseNameIsDeterministicAndBounded(t *testing.T) {
	assert.Equal(t, "camunda-backup-prod", logicalbackup.ClaimLeaseName("prod"))
	assert.Equal(t, logicalbackup.ClaimLeaseName("prod"), logicalbackup.ClaimLeaseName("prod"))

	long := strings.Repeat("c", 300)
	name := logicalbackup.ClaimLeaseName(long)
	assert.LessOrEqual(t, len(name), 253)
	assert.True(t, strings.HasPrefix(name, "camunda-backup-ccc"))
	assert.NotEqual(t, name, logicalbackup.ClaimLeaseName(long+"x"))
	assert.Equal(t, name, logicalbackup.ClaimLeaseName(long))
}

func TestClaimantStringRoundTrips(t *testing.T) {
	assert.Equal(t, "LogicalBackupElasticsearch/nightly/uid-first", first.String())
	assert.Equal(t, "LogicalBackupElasticsearch/nightly", first.Display())

	parsed, err := logicalbackup.ParseClaimant(first.String())
	require.NoError(t, err)
	assert.Equal(t, first, parsed)

	for _, bad := range []string{"", "nightly", "Kind/name", "Kind//uid", "a/b/c/d"} {
		_, err := logicalbackup.ParseClaimant(bad)
		assert.Error(t, err, bad)
	}
}

func TestFirstClaimWinsAndTheSecondIsBlockedWithTheHolderNamed(t *testing.T) {
	c := claimClient(t, holderResource(first, v1.LogicalBackupRunning), holderResource(second, ""))

	holder, err := logicalbackup.Claim(t.Context(), c, c, claimNamespace, "prod", first)
	require.NoError(t, err)
	assert.Empty(t, holder)
	assert.Equal(t, first.String(), leaseHolder(t, c))

	holder, err = logicalbackup.Claim(t.Context(), c, c, claimNamespace, "prod", second)
	require.NoError(t, err)
	assert.Equal(t, first.String(), holder)
	assert.Equal(t, first.String(), leaseHolder(t, c), "the Lease still names the first claimant")
}

func TestReEntryByTheHolderIsIdempotent(t *testing.T) {
	c := claimClient(t, holderResource(first, v1.LogicalBackupRunning))

	holds, err := logicalbackup.Holds(t.Context(), c, claimNamespace, "prod", first)
	require.NoError(t, err)
	assert.False(t, holds, "nothing is held before the first claim")

	for range 3 {
		holder, err := logicalbackup.Claim(t.Context(), c, c, claimNamespace, "prod", first)
		require.NoError(t, err)
		assert.Empty(t, holder)
	}
	assert.Equal(t, first.String(), leaseHolder(t, c))

	holds, err = logicalbackup.Holds(t.Context(), c, claimNamespace, "prod", first)
	require.NoError(t, err)
	assert.True(t, holds)
	holds, err = logicalbackup.Holds(t.Context(), c, claimNamespace, "prod", second)
	require.NoError(t, err)
	assert.False(t, holds, "another claimant does not hold it")
}

func TestClaimsOfDifferentClustersDoNotMeet(t *testing.T) {
	c := claimClient(t, holderResource(first, v1.LogicalBackupRunning), holderResource(second, v1.LogicalBackupRunning))

	holder, err := logicalbackup.Claim(t.Context(), c, c, claimNamespace, "prod", first)
	require.NoError(t, err)
	assert.Empty(t, holder)
	holder, err = logicalbackup.Claim(t.Context(), c, c, claimNamespace, "staging", second)
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
				logicalbackup.Claimant{Kind: first.Kind, Name: first.Name, UID: "uid-later"},
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

			got, err := logicalbackup.Claim(t.Context(), c, c, claimNamespace, "prod", second)
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
			_, err := logicalbackup.Claim(t.Context(), c, c, claimNamespace, "prod", first)
			require.NoError(t, err)

			got, err := logicalbackup.Claim(t.Context(), c, c, claimNamespace, "prod", second)
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
			lease.Name = logicalbackup.ClaimLeaseName("prod")
			holder := foreign
			lease.Spec.HolderIdentity = &holder
			require.NoError(t, c.Create(t.Context(), &lease))

			got, err := logicalbackup.Claim(t.Context(), c, c, claimNamespace, "prod", second)
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
	long := logicalbackup.Claimant{
		Kind: "LogicalBackupElasticsearch", Name: strings.Repeat("n", 253), UID: types.UID("uid-long"),
	}
	c := claimClient(t, holderResource(long, v1.LogicalBackupRunning), holderResource(second, v1.LogicalBackupPending))

	got, err := logicalbackup.Claim(t.Context(), c, c, claimNamespace, "prod", long)
	require.NoError(t, err)
	assert.Empty(t, got)
	lease := readLease(t, c)
	require.NotNil(t, lease)
	assert.LessOrEqual(t, len(*lease.Spec.HolderIdentity), 128)
	assert.True(t, strings.HasPrefix(*lease.Spec.HolderIdentity, "LogicalBackupElasticsearch/nnn"))
	assert.Equal(t, long.String(), leaseHolder(t, c))
	assert.Equal(t, strings.Repeat("n", 253), lease.Annotations[logicalbackup.ClaimHolderNameAnnotation])

	holds, err := logicalbackup.Holds(t.Context(), c, claimNamespace, "prod", long)
	require.NoError(t, err)
	assert.True(t, holds)
	got, err = logicalbackup.Claim(t.Context(), c, c, claimNamespace, "prod", second)
	require.NoError(t, err)
	assert.Equal(t, long.String(), got, "the second claimant is blocked by the exact identity")

	require.NoError(t, logicalbackup.Release(t.Context(), c, c, claimNamespace, "prod", long))
	assert.Empty(t, leaseHolder(t, c))

	// Stale: the long-named holder is terminal now, and its Lease is left
	// behind. The second claimant takes it over.
	c = claimClient(t, holderResource(long, v1.LogicalBackupCompleted), holderResource(second, v1.LogicalBackupPending))
	leaseHeldBy(t, c, long)
	got, err = logicalbackup.Claim(t.Context(), c, c, claimNamespace, "prod", second)
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Equal(t, second.String(), leaseHolder(t, c))
}

func TestHolderIdentityIsBounded(t *testing.T) {
	short := logicalbackup.Claimant{Kind: "LogicalBackupRDBMS", Name: "adhoc", UID: "u"}
	assert.Equal(t, "LogicalBackupRDBMS/adhoc", short.HolderIdentity())

	long := logicalbackup.Claimant{Kind: "LogicalBackupRDBMS", Name: strings.Repeat("x", 253), UID: "u"}
	assert.LessOrEqual(t, len(long.HolderIdentity()), 128)
	assert.Equal(t, long.HolderIdentity(), long.HolderIdentity())
	other := logicalbackup.Claimant{Kind: "LogicalBackupRDBMS", Name: strings.Repeat("x", 252) + "y", UID: "u"}
	assert.NotEqual(t, long.HolderIdentity(), other.HolderIdentity())
}

func TestReleaseOnlyByTheHolder(t *testing.T) {
	c := claimClient(t, holderResource(first, v1.LogicalBackupRunning), holderResource(second, v1.LogicalBackupPending))
	_, err := logicalbackup.Claim(t.Context(), c, c, claimNamespace, "prod", first)
	require.NoError(t, err)

	require.NoError(t, logicalbackup.Release(t.Context(), c, c, claimNamespace, "prod", second))
	assert.Equal(t, first.String(), leaseHolder(t, c), "a non-holder release is a no-op")

	require.NoError(t, logicalbackup.Release(t.Context(), c, c, claimNamespace, "prod", first))
	assert.Empty(t, leaseHolder(t, c))

	require.NoError(
		t, logicalbackup.Release(t.Context(), c, c, claimNamespace, "prod", first),
		"a release of a claim that is not held is a no-op",
	)
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

	active, err := logicalbackup.HolderActive(t.Context(), c, claimNamespace, second)
	require.NoError(t, err)
	assert.False(t, active)
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
				logicalbackup.Claimant{Kind: first.Kind, Name: first.Name, UID: "other"},
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
			active, err := logicalbackup.HolderActive(t.Context(), c, claimNamespace, first)
			require.NoError(t, err)
			assert.Equal(t, tt.active, active)
		})
	}
}
