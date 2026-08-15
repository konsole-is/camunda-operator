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

package refindex

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

const testSecretRefsField = "databaseserverconfig.spec.secretRefs"

func TestNamespacedKey(t *testing.T) {
	assert.Equal(t, "ns/s", NamespacedKey("ns", "s"))
}

func TestObjectNamespacedName(t *testing.T) {
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "s", Namespace: "ns"}}

	assert.Equal(t, NamespacedKey("ns", "s"), ObjectNamespacedName(secret))
}

func TestObjectName(t *testing.T) {
	cfg := &v1.DatabaseServerConfig{ObjectMeta: metav1.ObjectMeta{Name: "server"}}

	assert.Equal(t, "server", ObjectName(cfg))
}

// databaseServerConfigReferencing returns a cluster-scoped CR whose admin
// credentials Secret lives at namespace/name.
func databaseServerConfigReferencing(crName, namespace, name string) *v1.DatabaseServerConfig {
	return &v1.DatabaseServerConfig{
		ObjectMeta: metav1.ObjectMeta{Name: crName},
		Spec: v1.DatabaseServerConfigSpec{
			AdminCredentialsSecretRef: v1.CredentialsSecretRef{Name: name, Namespace: namespace},
		},
	}
}

// drain pops every request queued by the handler.
func drain(q workqueue.TypedRateLimitingInterface[reconcile.Request]) []reconcile.Request {
	var reqs []reconcile.Request
	for q.Len() > 0 {
		req, _ := q.Get()
		reqs = append(reqs, req)
		q.Done(req)
	}
	return reqs
}

func TestEnqueue(t *testing.T) {
	scheme, err := v1.SchemeBuilder.Build()
	require.NoError(t, err)

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&v1.DatabaseServerConfig{}, testSecretRefsField, func(o client.Object) []string {
			ref := o.(*v1.DatabaseServerConfig).Spec.AdminCredentialsSecretRef
			return []string{NamespacedKey(ref.Namespace, ref.Name)}
		}).
		WithObjects(
			databaseServerConfigReferencing("matching-a", "ns", "s"),
			databaseServerConfigReferencing("matching-b", "ns", "s"),
			databaseServerConfigReferencing("unrelated", "other", "x"),
		).
		Build()

	h := Enqueue(c, &v1.DatabaseServerConfigList{}, testSecretRefsField, ObjectNamespacedName)

	secretEvent := func(namespace, name string) event.CreateEvent {
		return event.CreateEvent{Object: &metav1.PartialObjectMetadata{
			TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		}}
	}

	t.Run("maps a referenced Secret to every CR naming it", func(t *testing.T) {
		q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
		h.Create(context.Background(), secretEvent("ns", "s"), q)

		assert.ElementsMatch(
			t, []reconcile.Request{
				{NamespacedName: types.NamespacedName{Name: "matching-a"}},
				{NamespacedName: types.NamespacedName{Name: "matching-b"}},
			}, drain(q),
		)
	})

	t.Run("maps an unreferenced Secret to no requests", func(t *testing.T) {
		q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
		h.Create(context.Background(), secretEvent("ns", "unreferenced"), q)

		assert.Empty(t, drain(q))
	})
}
