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

package echo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

func newFakeController(t *testing.T, objs ...client.Object) *Controller {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))

	c, err := New(validOptions())
	require.NoError(t, err)
	c.client = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()

	return c
}

func TestReleaseOwnerNeverCreatesAMissingObject(t *testing.T) {
	c := newFakeController(t)
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "basic-orphan-echo"}}

	require.NoError(t, c.releaseOwner(t.Context(), cm, "request-uid"))

	err := c.client.Get(t.Context(), client.ObjectKeyFromObject(cm), &corev1.ConfigMap{})
	assert.True(t, apierrors.IsNotFound(err), "got %v", err)
}

func TestReleaseOwnerKeepsTheContentAndOtherOwners(t *testing.T) {
	other := metav1.OwnerReference{APIVersion: "v1", Kind: "Secret", Name: "other", UID: "other-uid"}
	existing := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns",
			Name:      "basic-orphan-echo",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "plugins.camunda.io/v1alpha1",
					Kind:       "PluginRequest",
					Name:       "basic-orphan",
					UID:        "request-uid",
				},
				other,
			},
		},
		Data: map[string]string{"message": "hi"},
	}
	c := newFakeController(t, existing)

	require.NoError(t, c.releaseOwner(t.Context(), &corev1.ConfigMap{ObjectMeta: existing.ObjectMeta}, "request-uid"))

	var got corev1.ConfigMap
	require.NoError(t, c.client.Get(t.Context(), client.ObjectKeyFromObject(existing), &got))
	assert.Equal(t, []metav1.OwnerReference{other}, got.OwnerReferences)
	assert.Equal(t, map[string]string{"message": "hi"}, got.Data)
}

func TestDeleteKeepsAnObjectThatIsNotOfTheRequest(t *testing.T) {
	replacement := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "basic-delete-echo"}}
	c := newFakeController(t, replacement)
	req := &pluginsv1alpha1.PluginRequest{}
	req.Namespace = "ns"
	req.Name = "basic-delete"

	require.NoError(t, c.deleteOwn(t.Context(), req, &corev1.ConfigMap{ObjectMeta: replacement.ObjectMeta}))

	assert.NoError(t, c.client.Get(t.Context(), client.ObjectKeyFromObject(replacement), &corev1.ConfigMap{}))
}

func TestDeleteRemovesAnObjectOfTheRequest(t *testing.T) {
	own := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Namespace: "ns",
		Name:      "basic-delete-echo",
		Labels: map[string]string{
			pluginsv1alpha1.LabelRegistration: "echo",
			pluginsv1alpha1.LabelRequest:      "basic-delete",
		},
	}}
	c := newFakeController(t, own)
	req := &pluginsv1alpha1.PluginRequest{}
	req.Namespace = "ns"
	req.Name = "basic-delete"

	require.NoError(
		t,
		c.deleteOwn(
			t.Context(),
			req,
			&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: own.Name}},
		),
	)

	err := c.client.Get(t.Context(), client.ObjectKeyFromObject(own), &corev1.ConfigMap{})
	assert.True(t, apierrors.IsNotFound(err), "got %v", err)
}
