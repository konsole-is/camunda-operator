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
	"context"
	"errors"
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
	"sigs.k8s.io/controller-runtime/pkg/manager"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	clustercomponents "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/storagewriter"
)

const renewNamespace = "operator"

func renewClient(t *testing.T) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, coordinationv1.AddToScheme(scheme))

	return fake.NewClientBuilder().WithScheme(scheme).Build()
}

func restoreInto(name string) Registration {
	return Registration{
		Owner: &v1.LogicalRestoreRDBMS{
			TypeMeta:   metav1.TypeMeta{Kind: "LogicalRestoreRDBMS"},
			ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: name, UID: types.UID("uid-" + name)},
		},
		Backend: "rdbms|db.apps.svc:5432/" + name,
		Target:  "uid-target",
	}
}

func renewedAt(t *testing.T, c client.Client, reg Registration) *time.Time {
	t.Helper()
	var lease coordinationv1.Lease
	key := types.NamespacedName{
		Namespace: renewNamespace,
		Name:      storagewriter.LeaseName(reg.Backend, reg.Owner.GetUID()),
	}
	err := c.Get(context.Background(), key, &lease)
	if apierrors.IsNotFound(err) {
		return nil
	}
	require.NoError(t, err)

	return &lease.Spec.RenewTime.Time
}

func TestRenewerRenewsEveryListedRegistrationAndCreatesNone(t *testing.T) {
	ctx := context.Background()
	c := renewClient(t)
	registered := restoreInto("registered")
	released := restoreInto("released")
	start := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	claim := clustercomponents.StorageClaimSchema().LeaseName(registered.Backend)
	w := writerOf(registered.Owner, registered.Target)
	require.NoError(t, storagewriter.Register(ctx, c, c, renewNamespace, registered.Backend, claim, w, start))

	renewer := &Renewer{
		Client:         c,
		Reader:         c,
		ClaimNamespace: renewNamespace,
		List: func(context.Context) ([]Registration, error) {
			return []Registration{registered, released}, nil
		},
	}
	now := start.Add(storagewriter.Duration)

	require.NoError(t, renewer.renew(ctx, now))

	renewed := renewedAt(t, c, registered)
	require.NotNil(t, renewed)
	assert.True(t, renewed.Equal(now), "the registration is renewed")
	assert.Nil(t, renewedAt(t, c, released), "a restore with no registration gets none")
}

func TestRenewerReportsAFailedList(t *testing.T) {
	renewer := &Renewer{
		List: func(context.Context) ([]Registration, error) { return nil, errors.New("list failed") },
	}

	assert.ErrorContains(t, renewer.renew(context.Background(), time.Now()), "list failed")
}

func TestRenewableSkipsFinishedAndDeletedRestores(t *testing.T) {
	deleted := metav1.Now()

	assert.True(t, Renewable("rdbms|db:5432/camunda", false, nil), "a running restore with a backend")
	assert.False(t, Renewable("", false, nil), "no backend registered yet")
	assert.False(t, Renewable("rdbms|db:5432/camunda", true, nil), "a terminal restore released its registration")
	assert.False(t, Renewable("rdbms|db:5432/camunda", false, &deleted), "a deleted restore no longer runs")
}

func TestTheRenewerRunsOnlyOnTheLeader(t *testing.T) {
	var renewer manager.LeaderElectionRunnable = &Renewer{}

	assert.True(t, renewer.NeedLeaderElection())
}

func TestOtherWritersKeepsARegistrationLiveInsideTheLeadGrace(t *testing.T) {
	ctx := context.Background()
	c := renewClient(t)
	earlier := restoreInto("earlier")
	claim := clustercomponents.StorageClaimSchema().LeaseName(earlier.Backend)
	renewed := time.Now().Add(-storagewriter.Duration - time.Minute)
	require.NoError(t, storagewriter.Register(
		ctx, c, c, renewNamespace, earlier.Backend, claim, writerOf(earlier.Owner, earlier.Target), renewed,
	))
	next := restoreInto("next").Owner

	led := time.Now().Add(-time.Minute)
	writers, err := OtherWriters(ctx, c, renewNamespace, earlier.Backend, next, led)
	require.NoError(t, err)
	assert.Equal(t, []string{"LogicalRestoreRDBMS apps/earlier"}, writers)

	writers, err = OtherWriters(ctx, c, renewNamespace, earlier.Backend, next, time.Time{})
	require.NoError(t, err)
	assert.Empty(t, writers, "without the lead time, the registration reads as expired")
}
