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

package credentials

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// recordingClient returns a client that records the object of every Patch call
// instead of sending it, together with a pointer to the record.
func recordingClient() (client.Client, *[]client.Object) {
	var patched []client.Object
	recorder := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Patch: func(
			_ context.Context, _ client.WithWatch, obj client.Object,
			_ client.Patch, _ ...client.PatchOption,
		) error {
			patched = append(patched, obj)
			return nil
		},
	}).Build()

	return NewApplyClient(recorder), &patched
}

// The precondition annotation must become metadata.uid on the patch, and must
// not reach the cluster. metadata.uid is the only precondition the API server
// enforces on an apply that would create the object, which is the case this
// wrapper exists to reject.
func TestApplyClientTurnsThePreconditionIntoTheUID(t *testing.T) {
	t.Parallel()

	c, patched := recordingClient()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "creds",
			Namespace:   "ns",
			Annotations: map[string]string{PreconditionAnnotation: "uid-1", "keep": "me"},
		},
	}

	//nolint:staticcheck // ocf applies through the deprecated client.Apply patch
	require.NoError(t, c.Patch(context.Background(), secret, client.Apply))

	require.Len(t, *patched, 1)
	applied, ok := (*patched)[0].(*corev1.Secret)
	require.True(t, ok)
	assert.Equal(t, types.UID("uid-1"), applied.UID)
	assert.Equal(t, map[string]string{"keep": "me"}, applied.Annotations)
}

// A Secret whose only annotation was the precondition must be applied without
// an annotation map at all, so the wrapper never publishes an empty one.
func TestApplyClientClearsAnAnnotationsOnlyPrecondition(t *testing.T) {
	t.Parallel()

	c, patched := recordingClient()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "creds",
			Namespace:   "ns",
			Annotations: map[string]string{PreconditionAnnotation: "uid-1"},
		},
	}

	//nolint:staticcheck // ocf applies through the deprecated client.Apply patch
	require.NoError(t, c.Patch(context.Background(), secret, client.Apply))

	require.Len(t, *patched, 1)
	assert.Nil(t, (*patched)[0].GetAnnotations())
}

// Everything the wrapper is not about passes through untouched: a Secret
// without the annotation, another kind, and a patch that is not an apply.
func TestApplyClientPassesEverythingElseThrough(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		obj   client.Object
		patch client.Patch
	}{
		{
			name: "Secret without the precondition",
			obj: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: "ns"},
			},
			patch: client.Apply, //nolint:staticcheck // ocf applies through the deprecated client.Apply patch
		},
		{
			name: "another kind carrying the annotation",
			obj: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "zeebe",
					Namespace:   "ns",
					Annotations: map[string]string{PreconditionAnnotation: "uid-1"},
				},
			},
			patch: client.Apply, //nolint:staticcheck // ocf applies through the deprecated client.Apply patch
		},
		{
			name: "merge patch of a Secret carrying the annotation",
			obj: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "creds",
					Namespace:   "ns",
					Annotations: map[string]string{PreconditionAnnotation: "uid-1"},
				},
			},
			patch: client.Merge,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, patched := recordingClient()
			before := tt.obj.DeepCopyObject().(client.Object)

			require.NoError(t, c.Patch(context.Background(), tt.obj, tt.patch))

			require.Len(t, *patched, 1)
			assert.Equal(t, before.GetAnnotations(), (*patched)[0].GetAnnotations())
			assert.Empty(t, (*patched)[0].GetUID())
		})
	}
}
