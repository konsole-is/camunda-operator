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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// claimKey is the recorded claim that every fixture of this file contests.
const claimKey = "7000000000000000001/camunda"

// claimant returns a Database that recorded claimKey at the given creation
// time.
func claimant(namespace, name string, created time.Time) *v1.Database {
	return &v1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			CreationTimestamp: metav1.NewTime(created),
		},
		Spec:   v1.DatabaseSpec{ServerRef: "server", DatabaseName: "camunda"},
		Status: v1.DatabaseStatus{CollisionKey: claimKey},
	}
}

// collisionReconciler returns a reconciler whose client serves the collision
// index over indexed, the way the manager cache serves it from the claims that
// reached the cluster.
func collisionReconciler(t *testing.T, indexed ...*v1.Database) *DatabaseReconciler {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))

	objects := make([]client.Object, 0, len(indexed))
	for _, database := range indexed {
		objects = append(objects, database)
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&v1.Database{}, databaseCollisionField, func(o client.Object) []string {
			key := o.(*v1.Database).Status.CollisionKey
			if key == "" {
				return nil
			}

			return []string{key}
		}).
		WithObjects(objects...).
		Build()

	return &DatabaseReconciler{Client: c, APIReader: c, Scheme: scheme}
}

// TestCheckCollisionCountsTheDatabaseItRunsFor pins the rule against an index
// that has not caught up. A Database records its claim in status, so the index
// holds it only after the flush at the end of the reconcile that resolved the
// server. On that reconcile the list comes back without it, and first creation
// still has to win.
func TestCheckCollisionCountsTheDatabaseItRunsFor(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

	t.Run("the older Database wins before its own claim is indexed", func(t *testing.T) {
		t.Parallel()

		newer := claimant("beta", "newer", base.Add(time.Hour))
		older := claimant("alpha", "older", base)
		r := collisionReconciler(t, newer)

		assert.NoError(t, r.checkCollision(context.Background(), older, claimKey))
	})

	t.Run("the newer Database still loses before its own claim is indexed", func(t *testing.T) {
		t.Parallel()

		older := claimant("alpha", "older", base)
		newer := claimant("beta", "newer", base.Add(time.Hour))
		r := collisionReconciler(t, older)

		err := r.checkCollision(context.Background(), newer, claimKey)
		require.ErrorIs(t, err, errClaimNotFirst)

		// The rejection is an order, not a loss. Reconcile withdraws the
		// bindings and gives back every claim on errClaimLost, and a Database
		// that waits its turn for a name nobody holds keeps both.
		require.NotErrorIs(t, err, errClaimLost)

		var failure *conditions.PreCheckFailure
		require.ErrorAs(t, err, &failure)
		assert.Equal(t, v1.ReasonInvalidReference, failure.Reason)
		assert.Contains(t, failure.Message, "alpha/older")
	})

	t.Run("an indexed Database is not counted twice", func(t *testing.T) {
		t.Parallel()

		older := claimant("alpha", "older", base)
		newer := claimant("beta", "newer", base.Add(time.Hour))
		r := collisionReconciler(t, older, newer)

		assert.NoError(t, r.checkCollision(context.Background(), older, claimKey))
		require.ErrorIs(
			t, r.checkCollision(context.Background(), newer, claimKey), errClaimNotFirst,
		)
	})

	t.Run("the only claimant wins", func(t *testing.T) {
		t.Parallel()

		only := claimant("alpha", "only", base)
		r := collisionReconciler(t)

		assert.NoError(t, r.checkCollision(context.Background(), only, claimKey))
	})
}
