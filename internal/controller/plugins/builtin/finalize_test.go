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

package builtin

import (
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/configmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/internal/controller/plugins/builtin/adapter"
	"github.com/konsole-is/camunda-operator/internal/controller/plugins/builtin/existing"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
)

const testHold = "test.camunda.io/hold"

func TestFinalizeWaitsForEachStageInOrder(t *testing.T) {
	request := deletingRequest()
	first := ownedConfigMap("first", request.UID)
	first.Finalizers = []string{testHold}
	second := ownedConfigMap("second", request.UID)
	c := fakeReader(t, request, first, second)
	h := stubHandler{c: adapter.Cleanup{Stages: []adapter.CleanupStage{
		deleteStage(t, "first", request),
		deleteStage(t, "second", request),
	}}}

	assert.Equal(
		t, ctrl.Result{RequeueAfter: cleanupRequeue}, finalizeOnce(t, c, request, h), "an owned object remains",
	)

	assert.False(t, liveConfigMap(t, c, "first").DeletionTimestamp.IsZero(), "the first stage deletes first")
	assert.True(t, liveConfigMap(t, c, "second").DeletionTimestamp.IsZero(), "the second stage waits for first")
	assert.True(t, controllerutil.ContainsFinalizer(readRequest(t, c, request), pluginsv1alpha1.FinalizerCleanup))

	held := liveConfigMap(t, c, "first")
	held.Finalizers = nil
	require.NoError(t, c.Update(t.Context(), held))

	assert.Equal(t, ctrl.Result{}, finalizeOnce(t, c, request, h), "everything is gone")

	assert.True(t, apierrors.IsNotFound(c.Get(t.Context(), configMapKey("second"), &corev1.ConfigMap{})))
	assert.True(
		t, apierrors.IsNotFound(c.Get(t.Context(), client.ObjectKeyFromObject(request), request)),
		"the request goes once it has no finalizer",
	)
}

func TestFinalizeIgnoresARemainingObjectThatTheRequestDoesNotOwn(t *testing.T) {
	request := deletingRequest()
	foreign := ownedConfigMap("foreign", "")
	c := fakeReader(t, request, foreign)
	h := stubHandler{c: adapter.Cleanup{Stages: []adapter.CleanupStage{deleteStage(t, "foreign", request)}}}

	assert.Equal(t, ctrl.Result{}, finalizeOnce(t, c, request, h))

	assert.NoError(t, c.Get(t.Context(), configMapKey("foreign"), &corev1.ConfigMap{}), "a foreign object stays")
	assert.True(
		t, apierrors.IsNotFound(c.Get(t.Context(), client.ObjectKeyFromObject(request), request)),
		"a foreign object does not hold the finalizer",
	)
}

func TestFinalizeWithoutStagesRemovesTheFinalizer(t *testing.T) {
	request := deletingRequest()
	c := fakeReader(t, request)

	assert.Equal(t, ctrl.Result{}, finalizeOnce(t, c, request, stubHandler{}))

	assert.True(t, apierrors.IsNotFound(c.Get(t.Context(), client.ObjectKeyFromObject(request), request)))
}

// stubHandler returns c as its cleanup.
type stubHandler struct {
	existing.Handler
	c adapter.Cleanup
}

func (s stubHandler) Cleanup(
	*pluginsv1alpha1.PluginRequest,
	pluginsv1alpha1.DeletionPolicy,
	adapter.LiveObjects,
) (adapter.Cleanup, error) {
	return s.c, nil
}

// deletingRequest returns a request with the cleanup finalizer and a
// deletion timestamp.
func deletingRequest() *pluginsv1alpha1.PluginRequest {
	request := requestFor(pluginstartup.RegistrationExistingObjectStorage, `{"name":"my-bucket"}`)
	request.UID = "request-uid"
	request.Finalizers = []string{pluginsv1alpha1.FinalizerCleanup}
	request.DeletionTimestamp = &metav1.Time{Time: metav1.Now().Time}

	return request
}

// ownedConfigMap returns a ConfigMap name in the namespace of requestFor,
// with an owner reference to uid unless uid is empty.
func ownedConfigMap(name string, uid types.UID) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "my-cluster-ns"}}
	if uid != "" {
		cm.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: pluginsv1alpha1.GroupVersion.String(),
			Kind:       "PluginRequest",
			Name:       "orders-backup-storage",
			UID:        uid,
		}}
	}

	return cm
}

// deleteStage returns a stage that deletes the ConfigMap name if the
// request owns it, and waits for it.
func deleteStage(t *testing.T, name string, request *pluginsv1alpha1.PluginRequest) adapter.CleanupStage {
	res, err := configmap.NewBuilder(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: request.Namespace},
	}).Build()
	require.NoError(t, err)

	comp, err := component.NewComponentBuilder().
		WithName("cleanup-"+name).
		WithConditionType("Ready").
		WithResource(res, component.Delete(), component.DeleteOnlyIfOwned()).
		Build()
	require.NoError(t, err)

	return adapter.CleanupStage{
		Components: []*component.Component{comp},
		Remaining: []client.Object{
			&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: request.Namespace}},
		},
	}
}

func finalizeOnce(
	t *testing.T,
	c client.Client,
	request *pluginsv1alpha1.PluginRequest,
	h adapter.Handler,
) ctrl.Result {
	r := &Reconciler{
		Client: c, APIReader: c, Scheme: c.Scheme(), EventRecorder: events.NewFakeRecorder(100),
		handlers: map[string]adapter.Handler{request.Spec.Registration: h},
	}
	result, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(request)})
	require.NoError(t, err)
	return result
}

func configMapKey(name string) client.ObjectKey {
	return client.ObjectKey{Namespace: "my-cluster-ns", Name: name}
}

func liveConfigMap(t *testing.T, c client.Client, name string) *corev1.ConfigMap {
	var cm corev1.ConfigMap
	require.NoError(t, c.Get(t.Context(), configMapKey(name), &cm))
	return &cm
}

func readRequest(t *testing.T, c client.Client, request *pluginsv1alpha1.PluginRequest) *pluginsv1alpha1.PluginRequest {
	var live pluginsv1alpha1.PluginRequest
	require.NoError(t, c.Get(t.Context(), client.ObjectKeyFromObject(request), &live))
	return &live
}
