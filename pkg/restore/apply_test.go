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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// The field manager of a restore is API surface. It appears in the
// managedFields of every Job and every recreated volume, and an operator that
// reads it there must find the documented string.
func TestFieldManagersAreStable(t *testing.T) {
	t.Parallel()

	assert.Equal(
		t,
		client.FieldOwner("camunda-operator/pointintimerestore"),
		FieldManagerPointInTimeRestore,
	)
}

// Every resource a restore manages goes through Apply, so this pins the whole
// server-side apply convention of both restore kinds in one place: an apply
// patch, forced ownership, and the field manager of the calling kind.
func TestApplyForcesOwnershipUnderTheFieldManager(t *testing.T) {
	t.Parallel()

	var (
		gotPatch client.Patch
		gotOpts  []client.PatchOption
	)
	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(
				ctx context.Context,
				cl client.WithWatch,
				obj client.Object,
				patch client.Patch,
				opts ...client.PatchOption,
			) error {
				gotPatch, gotOpts = patch, opts

				return cl.Patch(ctx, obj, patch, opts...)
			},
		}).
		Build()

	claim := &corev1.PersistentVolumeClaim{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "PersistentVolumeClaim"},
		ObjectMeta: metav1.ObjectMeta{Name: "data-my-cluster-zeebe-0", Namespace: "ns"},
	}

	require.NoError(t, Apply(context.Background(), c, claim, FieldManagerPointInTimeRestore))

	require.NotNil(t, gotPatch)
	assert.Equal(t, types.ApplyPatchType, gotPatch.Type())

	var options client.PatchOptions
	options.ApplyOptions(gotOpts)
	require.NotNil(t, options.Force, "an apply that does not force ownership stalls on a conflict")
	assert.True(t, *options.Force)
	assert.Equal(t, string(FieldManagerPointInTimeRestore), options.FieldManager)

	// The object really landed, so the convention is not only well formed.
	var applied corev1.PersistentVolumeClaim
	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(claim), &applied))
}

// Apply reports the error of the API server as it is. A caller decides
// whether that error holds a restore or fails it.
func TestApplyReturnsTheClientError(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(
				context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption,
			) error {
				return boom
			},
		}).
		Build()

	err := Apply(
		context.Background(),
		c,
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "ns"}},
		FieldManagerPointInTimeRestore,
	)
	require.ErrorIs(t, err, boom)
}
