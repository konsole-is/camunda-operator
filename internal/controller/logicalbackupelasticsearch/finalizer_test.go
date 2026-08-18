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

package logicalbackupelasticsearch

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
)

// The finalizer removal must be durable before the claim is released. Once
// the removal is written, the object is gone for good and no retry of the
// finalizer can resume exporting over a sibling. An interrupted release
// leaves a Lease whose holder no longer exists, and the next claimant takes
// it over.
func TestFinalizeRemovesTheFinalizerBeforeReleasingTheClaim(t *testing.T) {
	ctx := context.Background()
	s := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(s))
	require.NoError(t, v1.AddToScheme(s))

	backup := &v1.LogicalBackupElasticsearch{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns", Name: "held", UID: types.UID("uid-held"),
			Finalizers: []string{logicalbackup.Finalizer},
		},
		Spec: v1.LogicalBackupElasticsearchSpec{ClusterRef: v1.ClusterRef{Name: "cc"}},
	}
	holder := claimant(backup)
	identity := holder.HolderIdentity()
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns", Name: logicalbackup.ClaimLeaseName("cc"),
			Annotations: map[string]string{
				logicalbackup.ClaimHolderKindAnnotation: holder.Kind,
				logicalbackup.ClaimHolderNameAnnotation: holder.Name,
				logicalbackup.ClaimHolderUIDAnnotation:  string(holder.UID),
			},
		},
		Spec: coordinationv1.LeaseSpec{HolderIdentity: &identity},
	}

	interrupted := false
	c := fake.NewClientBuilder().WithScheme(s).
		WithObjects(backup, lease).
		WithStatusSubresource(&v1.LogicalBackupElasticsearch{}).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(
				ctx context.Context,
				cl client.WithWatch,
				obj client.Object,
				opts ...client.DeleteOption,
			) error {
				if _, ok := obj.(*coordinationv1.Lease); ok && !interrupted {
					interrupted = true
					return errors.New("interrupted before the Lease delete reached the server")
				}
				return cl.Delete(ctx, obj, opts...)
			},
		}).Build()
	r := &Reconciler{Client: c, APIReader: c, EventRecorder: events.NewFakeRecorder(16)}

	// The deletion marks the object. The finalizer keeps it alive.
	require.NoError(t, c.Delete(ctx, backup))
	var deleting v1.LogicalBackupElasticsearch
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(backup), &deleting))
	require.NotNil(t, deleting.DeletionTimestamp)

	_, err := r.finalize(ctx, &deleting)
	require.NoError(t, err, "an interrupted release must not fail the reconcile")
	assert.True(t, interrupted, "the release was attempted and interrupted")

	var gone v1.LogicalBackupElasticsearch
	assert.True(
		t, apierrors.IsNotFound(c.Get(ctx, client.ObjectKeyFromObject(backup), &gone)),
		"the finalizer removal was durable first: the object is gone and can never reconcile again",
	)

	var left coordinationv1.Lease
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(lease), &left), "the Lease survived the interruption")

	// The next claimant reclaims it: the holder resource is gone.
	sibling := logicalbackup.Claimant{Kind: "LogicalBackupElasticsearch", Name: "next", UID: types.UID("uid-next")}
	blocked, err := logicalbackup.Claim(ctx, c, c, "ns", "cc", sibling)
	require.NoError(t, err)
	assert.Empty(t, blocked, "the stale holder was taken over")
}
