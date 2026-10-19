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

package conformance

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

func configMap(name, resourceVersion string) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{}
	cm.Kind = "ConfigMap"
	cm.Namespace = "ns"
	cm.Name = name
	cm.ResourceVersion = resourceVersion
	return cm
}

func TestRecorderKeepsTheEarliestResourceVersionOfAKey(t *testing.T) {
	r := &recorder{}

	r.observe(configMap("late", "20"), "object/basic")
	r.observe(configMap("early", "9"), "object/basic")
	r.observe(configMap("later", "30"), "object/basic")

	got, ok := r.first("object/basic")
	assert.True(t, ok)
	assert.Equal(t, "ConfigMap ns/early", got.ref)
}

func TestFinalizerProblem(t *testing.T) {
	withFinalizer := func(resourceVersion string) *pluginsv1alpha1.PluginRequest {
		req := &pluginsv1alpha1.PluginRequest{}
		req.Kind = kindPluginRequest
		req.Name = testRequestName
		req.ResourceVersion = resourceVersion
		return req
	}
	sc := &storagev1.StorageClass{}
	sc.Kind = "StorageClass"
	sc.Name = "ns-basic-delete-echo"
	sc.ResourceVersion = "5"

	tests := []struct {
		name    string
		events  func(r *recorder)
		message string
	}{
		{
			name: "finalizer before the first object",
			events: func(r *recorder) {
				r.observe(withFinalizer("4"), finalizerKey(testRequestName))
				r.observe(sc, objectKey(testRequestName))
			},
		},
		{
			name: "finalizer and no object",
			events: func(r *recorder) {
				r.observe(withFinalizer("4"), finalizerKey(testRequestName))
			},
		},
		{
			name: "an object before the finalizer, seen after it",
			events: func(r *recorder) {
				r.observe(withFinalizer("6"), finalizerKey(testRequestName))
				r.observe(sc, objectKey(testRequestName))
			},
			message: "StorageClass ns-basic-delete-echo appeared before the finalizer plugins.camunda.io/cleanup",
		},
		{
			name: "no finalizer",
			events: func(r *recorder) {
				r.observe(sc, objectKey(testRequestName))
			},
			message: "the request never got the finalizer plugins.camunda.io/cleanup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &recorder{}
			tt.events(r)

			err := finalizerProblem(r, testRequestName)

			if tt.message == "" {
				assert.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tt.message)
		})
	}
}

func TestRecorderSawOnlyTheObjectsThatItObserved(t *testing.T) {
	r := &recorder{}
	cm := configMap("x", "4")
	cm.UID = "observed"

	r.observe(cm, objectKey(testRequestName))

	assert.True(t, r.saw("observed"))
	assert.False(t, r.saw("other"))
}

func TestFinalizerProblemReportsAFailedWatch(t *testing.T) {
	r := &recorder{}
	r.fail(errors.New("a watch failed: too old resource version"))

	assert.ErrorContains(t, finalizerProblem(r, testRequestName), "too old resource version")
}

func TestRecorderCaughtUpComparesTheLastResourceVersionOfAStream(t *testing.T) {
	r := &recorder{}
	r.advance("registration", "10")

	assert.True(t, r.caughtUp("registration", "9"))
	assert.True(t, r.caughtUp("registration", "10"))
	assert.False(t, r.caughtUp("registration", "11"))
	assert.False(t, r.caughtUp("other", "1"))
}

func TestRecorderWatchWithoutAStartObservesTheCurrentObjects(t *testing.T) {
	scheme, err := newScheme()
	require.NoError(t, err)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(configMap("existing", "")).Build()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	r := &recorder{}

	err = r.watch(
		ctx, c, func() client.ObjectList { return &corev1.ConfigMapList{} }, func(client.Object) string {
			return "current"
		}, "",
	)

	require.NoError(t, err)
	got, ok := r.first("current")
	require.True(t, ok)
	assert.Contains(t, got.ref, "ns/existing")
}

func TestRecorderKeepsTheNewestCopyOfEachObservedObject(t *testing.T) {
	r := &recorder{}
	first := &unstructured.Unstructured{}
	first.SetKind("ConfigMap")
	first.SetName("x")
	first.SetUID("x-uid")
	first.SetResourceVersion("4")
	second := first.DeepCopy()
	second.SetResourceVersion("7")

	r.observe(first, objectKey("basic-orphan"))
	r.observe(second, objectKey("basic-orphan"))

	got := r.objectsOf(objectKey("basic-orphan"))
	require.Len(t, got, 1)
	assert.Equal(t, "7", got[0].GetResourceVersion())
	assert.Empty(t, r.objectsOf(objectKey("other")))
}
