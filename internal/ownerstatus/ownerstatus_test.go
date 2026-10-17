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

package ownerstatus

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

var (
	errReconcile = errors.New("reconcile failed")
	gr           = v1.GroupVersion.WithResource("objectstorageconfigs").GroupResource()
)

func TestFlushWritesTheStagedStatusAndKeepsTheResult(t *testing.T) {
	c, owner, base := newOwner(t, interceptor.Funcs{})
	stageReady(owner)
	want := ctrl.Result{RequeueAfter: 30 * time.Second}

	res, err := Flush(context.Background(), component.ReconcileContext{Client: c, Owner: owner}, base, want, nil)

	require.NoError(t, err)
	assert.Equal(t, want, res)
	assert.True(t, storedReady(t, c, owner))
}

func TestFlushKeepsTheReconcileErrorAfterAWrite(t *testing.T) {
	c, owner, base := newOwner(t, interceptor.Funcs{})
	stageReady(owner)

	_, err := Flush(
		context.Background(),
		component.ReconcileContext{Client: c, Owner: owner},
		base,
		ctrl.Result{},
		errReconcile,
	)

	assert.Equal(t, errReconcile, err)
	assert.True(t, storedReady(t, c, owner))
}

func TestFlushWritesOverAWriteOfTheMetadataOnly(t *testing.T) {
	c, owner, base := newOwner(t, interceptor.Funcs{})
	writeMetadataAsAnotherWriter(t, c, owner)
	stageReady(owner)
	want := ctrl.Result{RequeueAfter: time.Minute}

	res, err := Flush(context.Background(), component.ReconcileContext{Client: c, Owner: owner}, base, want, nil)

	require.NoError(t, err)
	assert.Equal(t, want, res)
	assert.True(t, storedReady(t, c, owner))
}

func TestFlushKeepsTheStatusOfAnotherWriterAndRequeues(t *testing.T) {
	c, owner, base := newOwner(t, interceptor.Funcs{})
	writeStatusAsAnotherWriter(t, c, owner)
	stageReady(owner)

	res, err := Flush(
		context.Background(),
		component.ReconcileContext{Client: c, APIReader: c, Owner: owner},
		base,
		ctrl.Result{RequeueAfter: time.Minute},
		nil,
	)

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{RequeueAfter: ConflictRequeue}, res)
	var stored v1.ObjectStorageConfig
	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(owner), &stored))
	assert.Equal(t, int64(7), stored.Status.ObservedGeneration)
	assert.False(t, storedReady(t, c, owner), "a conflict must write nothing")
}

func TestFlushReturnsOnlyTheReconcileErrorOnAConflict(t *testing.T) {
	c, owner, base := newOwner(t, interceptor.Funcs{})
	writeStatusAsAnotherWriter(t, c, owner)
	stageReady(owner)
	want := ctrl.Result{RequeueAfter: time.Minute}

	res, err := Flush(
		context.Background(),
		component.ReconcileContext{Client: c, Owner: owner},
		base,
		want,
		errReconcile,
	)

	assert.Equal(t, errReconcile, err)
	assert.Equal(t, want, res)
}

func TestFlushReturnsAFailedReadAfterAConflict(t *testing.T) {
	c, owner, base := newOwner(t, interceptor.Funcs{})
	writeMetadataAsAnotherWriter(t, c, owner)
	stageReady(owner)
	forbidden := apierrors.NewForbidden(gr, "s", nil)
	reader := interceptor.NewClient(c.(client.WithWatch), interceptor.Funcs{
		Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
			return forbidden
		},
	})

	res, err := Flush(
		context.Background(),
		component.ReconcileContext{Client: c, APIReader: reader, Owner: owner},
		base,
		ctrl.Result{},
		nil,
	)

	assert.True(t, apierrors.IsForbidden(err))
	assert.False(t, apierrors.IsConflict(err), "a failed read must not pass for a conflict")
	assert.Equal(t, ctrl.Result{}, res)
}

func TestFlushReturnsAnyOtherWriteFailure(t *testing.T) {
	forbidden := apierrors.NewForbidden(gr, "s", nil)
	refuse := interceptor.Funcs{
		SubResourceUpdate: func(context.Context, client.Client, string, client.Object, ...client.SubResourceUpdateOption) error {
			return forbidden
		},
	}

	t.Run("alone", func(t *testing.T) {
		c, owner, base := newOwner(t, refuse)

		res, err := Flush(
			context.Background(),
			component.ReconcileContext{Client: c, Owner: owner},
			base,
			ctrl.Result{RequeueAfter: time.Minute},
			nil,
		)

		assert.True(t, apierrors.IsForbidden(err))
		assert.Equal(t, ctrl.Result{}, res)
	})

	t.Run("joined to the reconcile error", func(t *testing.T) {
		c, owner, base := newOwner(t, refuse)

		_, err := Flush(
			context.Background(),
			component.ReconcileContext{Client: c, Owner: owner},
			base,
			ctrl.Result{},
			errReconcile,
		)

		require.ErrorIs(t, err, errReconcile)
		assert.True(t, apierrors.IsForbidden(err))
	})
}

func TestSettle(t *testing.T) {
	conflict := fmt.Errorf("updating status: %w", apierrors.NewConflict(gr, "s", errors.New("modified")))
	forbidden := apierrors.NewForbidden(gr, "s", nil)
	later := ctrl.Result{RequeueAfter: time.Minute}

	t.Run("a write that landed keeps the result and the error", func(t *testing.T) {
		res, err := Settle(later, errReconcile, nil)

		assert.Equal(t, later, res)
		assert.Equal(t, errReconcile, err)
	})

	t.Run("a wrapped conflict requeues with no error", func(t *testing.T) {
		res, err := Settle(later, nil, conflict)

		require.NoError(t, err)
		assert.Equal(t, ctrl.Result{RequeueAfter: ConflictRequeue}, res)
	})

	t.Run("a conflict beside a reconcile error keeps only that error", func(t *testing.T) {
		res, err := Settle(later, errReconcile, conflict)

		assert.Equal(t, errReconcile, err)
		assert.Equal(t, later, res)
	})

	t.Run("another failure is joined to the reconcile error", func(t *testing.T) {
		res, err := Settle(later, errReconcile, forbidden)

		require.ErrorIs(t, err, errReconcile)
		assert.True(t, apierrors.IsForbidden(err))
		assert.Equal(t, ctrl.Result{}, res)
	})
}

// newOwner returns a client that holds one owner, the owner as a reconcile
// read it, and the copy of it that the reconcile keeps as its base.
func newOwner(t *testing.T, funcs interceptor.Funcs) (client.Client, *v1.ObjectStorageConfig, *v1.ObjectStorageConfig) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))
	stored := &v1.ObjectStorageConfig{ObjectMeta: metav1.ObjectMeta{Name: "s", Namespace: "default"}}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(stored).
		WithStatusSubresource(stored).
		WithInterceptorFuncs(funcs).
		Build()

	var owner v1.ObjectStorageConfig
	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(stored), &owner))

	return c, &owner, owner.DeepCopy()
}

// writeMetadataAsAnotherWriter changes a label of the stored owner, so the
// resourceVersion that owner holds is stale and its status is not.
func writeMetadataAsAnotherWriter(t *testing.T, c client.Client, owner *v1.ObjectStorageConfig) {
	t.Helper()

	other := owner.DeepCopy()
	other.Labels = map[string]string{"writer": "other"}
	require.NoError(t, c.Update(context.Background(), other))
}

// writeStatusAsAnotherWriter changes the status of the stored owner, as a
// second controller that shares the status does.
func writeStatusAsAnotherWriter(t *testing.T, c client.Client, owner *v1.ObjectStorageConfig) {
	t.Helper()

	other := owner.DeepCopy()
	other.Status.ObservedGeneration = 7
	require.NoError(t, c.Status().Update(context.Background(), other))
}

func stageReady(owner *v1.ObjectStorageConfig) {
	meta.SetStatusCondition(&owner.Status.Conditions, metav1.Condition{
		Type:   v1.ConditionReady,
		Status: metav1.ConditionTrue,
		Reason: v1.ReasonHealthy,
	})
}

func storedReady(t *testing.T, c client.Client, owner *v1.ObjectStorageConfig) bool {
	t.Helper()

	var stored v1.ObjectStorageConfig
	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(owner), &stored))

	return meta.IsStatusConditionTrue(stored.Status.Conditions, v1.ConditionReady)
}
