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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// The precondition is a rule of the API server, not of this package. Only a
// real control plane answers whether it holds, so these tests run against
// envtest. The fake client models neither the uid precondition nor
// Server-Side Apply.
func startAPIServer(t *testing.T) client.Client {
	t.Helper()

	control := &envtest.Environment{BinaryAssetsDirectory: envtestBinaryDir(t)}
	cfg, err := control.Start()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, control.Stop()) })

	c, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	require.NoError(t, err)

	return NewApplyClient(c)
}

// applyPassword applies the credential Secret named key with the given
// password, through the precondition of p.
func applyPassword(ctx context.Context, c client.Client, name string, p Password) error {
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   "default",
			Annotations: p.PreconditionAnnotations(),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{"password": []byte(p.Value)},
	}

	//nolint:staticcheck // ocf applies through the deprecated client.Apply patch
	return c.Patch(ctx, secret, client.Apply, client.ForceOwnership, client.FieldOwner("test/credentials"))
}

// This is the whole point of the precondition. A controller reads a password,
// the Secret is deleted, and the apply that republishes the password arrives
// afterwards. Without the precondition that apply recreates the Secret with the
// old password, and every later reconcile reads that password back and keeps
// it, so the delete rotates nothing. With the precondition the apply is
// rejected and the Secret stays away, so the next reconcile finds nothing and
// generates a new password.
func TestApplyOfAReusedPasswordDoesNotResurrectADeletedSecret(t *testing.T) {
	c := startAPIServer(t)
	ctx := context.Background()
	key := client.ObjectKey{Namespace: "default", Name: "resurrect"}

	require.NoError(t, applyPassword(ctx, c, key.Name, Password{Value: "first"}))

	var created corev1.Secret
	require.NoError(t, c.Get(ctx, key, &created))
	reused := Password{Value: "first", SourceUID: created.UID}

	// An apply that reuses the password of the live Secret must succeed, or
	// every reconcile of a healthy cluster would fail.
	require.NoError(t, applyPassword(ctx, c, key.Name, reused))

	require.NoError(t, c.Delete(ctx, &created))

	err := applyPassword(ctx, c, key.Name, reused)
	require.Error(t, err)
	assert.True(t, apierrors.IsConflict(err), "want a conflict, got %v", err)
	assert.True(
		t,
		apierrors.IsNotFound(c.Get(ctx, key, &corev1.Secret{})),
		"the rejected apply must not have recreated the Secret",
	)

	// The reconcile that the delete enqueues finds no Secret, generates a new
	// password, and applies it without a precondition.
	require.NoError(t, applyPassword(ctx, c, key.Name, Password{Value: "second"}))

	var recreated corev1.Secret
	require.NoError(t, c.Get(ctx, key, &recreated))
	assert.Equal(t, []byte("second"), recreated.Data["password"])
	assert.NotEqual(t, created.UID, recreated.UID)
	assert.Empty(
		t, recreated.Annotations,
		"the precondition annotation must never reach the cluster",
	)
}

// metadata.resourceVersion cannot replace metadata.uid here. The API server
// enforces it on an apply that updates an object, but ignores it on an apply
// that creates one, which is exactly the case the precondition must reject.
func TestResourceVersionDoesNotRejectAResurrectingApply(t *testing.T) {
	c := startAPIServer(t)
	ctx := context.Background()
	key := client.ObjectKey{Namespace: "default", Name: "resource-version"}

	require.NoError(t, applyPassword(ctx, c, key.Name, Password{Value: "first"}))

	var created corev1.Secret
	require.NoError(t, c.Get(ctx, key, &created))
	require.NoError(t, c.Delete(ctx, &created))

	resurrect := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{
			Name:            key.Name,
			Namespace:       key.Namespace,
			ResourceVersion: created.ResourceVersion,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{"password": []byte("first")},
	}
	//nolint:staticcheck // ocf applies through the deprecated client.Apply patch
	err := c.Patch(ctx, resurrect, client.Apply, client.ForceOwnership, client.FieldOwner("test/credentials"))

	require.NoError(t, err, "the API server ignores resourceVersion on an apply that creates")
	assert.NotEqual(t, created.UID, resurrect.UID, "the stale resourceVersion still created a new object")
}

// envtestBinaryDir mirrors the shared envtest bootstrap: the control-plane
// binaries live in the first directory under bin/k8s at the repository root,
// so the tests run without KUBEBUILDER_ASSETS in the environment. An empty
// return leaves envtest to that variable.
func envtestBinaryDir(t *testing.T) string {
	t.Helper()

	base := filepath.Join("..", "..", "bin", "k8s")
	entries, err := os.ReadDir(base)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			abs, err := filepath.Abs(filepath.Join(base, entry.Name()))
			require.NoError(t, err)

			return abs
		}
	}

	return ""
}
