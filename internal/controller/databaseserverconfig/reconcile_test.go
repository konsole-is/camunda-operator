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

package databaseserverconfig

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// laggingCache hands out the contract as it stood before the last status
// write, the way the informer cache of the manager does until it catches up
// with that write. Every other read, and every write, goes to the live store.
type laggingCache struct {
	client.Client
	behind *v1.DatabaseServerConfig
}

// Get returns the copy the cache holds for the contract, and the live copy for
// everything else.
func (c laggingCache) Get(
	ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption,
) error {
	cfg, isConfig := obj.(*v1.DatabaseServerConfig)
	if isConfig && key == client.ObjectKeyFromObject(c.behind) {
		c.behind.DeepCopyInto(cfg)

		return nil
	}

	return c.Client.Get(ctx, key, obj, opts...)
}

// TestReconcilePublishesHealthyWhileTheCacheLagsTheLastWrite pins that the
// contract flips to Healthy on the reconcile the Secret wakes, even when the
// cache of the controller still holds the copy from before it recorded the
// missing Secret. A status write from that copy is refused with a conflict,
// and the repair of a conflict drops the Ready of this pass. Nothing wakes
// the controller to stage it again until the probe interval is up.
func TestReconcilePublishesHealthyWhileTheCacheLagsTheLastWrite(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	cfg := &v1.DatabaseServerConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "dbsc", Namespace: "databases"},
		Spec: v1.DatabaseServerConfigSpec{
			Engine: v1.DatabaseEnginePostgres,
			Host:   "postgres.databases.svc",
			Port:   5432,
			AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
				Name: "admin-creds", UsernameKey: "username", PasswordKey: "password",
			},
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "admin-creds", Namespace: "databases"},
		Data:       map[string][]byte{"username": []byte("postgres"), "password": []byte("secret")},
	}
	live := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cfg, secret).
		WithStatusSubresource(&v1.DatabaseServerConfig{}).
		Build()

	ctx := context.Background()
	key := client.ObjectKeyFromObject(cfg)

	// The copy the cache holds: the contract as it was before the reconcile
	// that found no Secret recorded that.
	var behind v1.DatabaseServerConfig
	require.NoError(t, live.Get(ctx, key, &behind))

	// The write the cache has not caught up with.
	var recorded v1.DatabaseServerConfig
	require.NoError(t, live.Get(ctx, key, &recorded))
	conditions.Stage(&recorded, conditions.Ready(
		metav1.ConditionFalse, v1.ReasonMissingSecret,
		"Secret databases/admin-creds not found", recorded.Generation,
	))
	require.NoError(t, live.Status().Update(ctx, &recorded))
	require.NotEqual(t, behind.ResourceVersion, recorded.ResourceVersion)

	reconciler := &DatabaseServerConfigReconciler{
		Client:    laggingCache{Client: live, behind: &behind},
		APIReader: live,
		Scheme:    scheme,
		probe: func(
			context.Context, *v1.DatabaseServerConfig, string, string,
		) (version, systemIdentifier string, err error) {
			return "17", "7412345678901234567", nil
		},
	}

	_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	require.NoError(t, err)

	var got v1.DatabaseServerConfig
	require.NoError(t, live.Get(ctx, key, &got))
	ready := meta.FindStatusCondition(got.Status.Conditions, v1.ConditionReady)
	require.NotNil(t, ready)
	assert.Equal(t, metav1.ConditionTrue, ready.Status)
	assert.Equal(t, v1.ReasonHealthy, ready.Reason)
	assert.Equal(t, "17", got.Status.ServerVersion)
	assert.Equal(t, "7412345678901234567", got.Status.SystemIdentifier)
}
