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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"

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
		req.Name = "basic-delete"
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
				r.observe(withFinalizer("4"), finalizerKey("basic-delete"))
				r.observe(sc, objectKey("basic-delete"))
			},
		},
		{
			name: "finalizer and no object",
			events: func(r *recorder) {
				r.observe(withFinalizer("4"), finalizerKey("basic-delete"))
			},
		},
		{
			name: "an object before the finalizer, seen after it",
			events: func(r *recorder) {
				r.observe(withFinalizer("6"), finalizerKey("basic-delete"))
				r.observe(sc, objectKey("basic-delete"))
			},
			message: "StorageClass ns-basic-delete-echo appeared before the finalizer plugins.camunda.io/cleanup",
		},
		{
			name: "no finalizer",
			events: func(r *recorder) {
				r.observe(sc, objectKey("basic-delete"))
			},
			message: "the request never got the finalizer plugins.camunda.io/cleanup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &recorder{}
			tt.events(r)

			err := finalizerProblem(r, "basic-delete")

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

	r.observe(cm, objectKey("basic-delete"))

	assert.True(t, r.saw("observed"))
	assert.False(t, r.saw("other"))
}

func TestFinalizerProblemReportsAFailedWatch(t *testing.T) {
	r := &recorder{}
	r.fail(errors.New("a watch failed: too old resource version"))

	assert.ErrorContains(t, finalizerProblem(r, "basic-delete"), "too old resource version")
}
