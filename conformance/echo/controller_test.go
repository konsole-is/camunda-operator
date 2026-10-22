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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// requestName is the name of the request of the tests.
const requestName = "basic-delete"

func newFakeController(t *testing.T, objs ...client.Object) *Controller {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, pluginsv1alpha1.AddToScheme(scheme))

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
	req.Name = requestName

	require.NoError(t, c.deleteOwn(t.Context(), req, &corev1.ConfigMap{ObjectMeta: replacement.ObjectMeta}))

	assert.NoError(t, c.client.Get(t.Context(), client.ObjectKeyFromObject(replacement), &corev1.ConfigMap{}))
}

func TestDeleteRemovesAnObjectOfTheRequest(t *testing.T) {
	own := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Namespace: "ns",
		Name:      "basic-delete-echo",
		Labels: map[string]string{
			pluginsv1alpha1.LabelRegistration: "echo",
			pluginsv1alpha1.LabelRequest:      requestName,
		},
	}}
	c := newFakeController(t, own)
	req := &pluginsv1alpha1.PluginRequest{}
	req.Namespace = "ns"
	req.Name = requestName

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

func TestReconcileRequestLeavesARequestOfAnotherRegistration(t *testing.T) {
	other := &pluginsv1alpha1.PluginRequest{}
	other.Namespace = "ns"
	other.Name = requestName
	other.Spec.Registration = otherRegistration
	c := newFakeController(t, other)

	_, err := c.reconcileRequest(t.Context(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(other)})

	require.NoError(t, err)
	var got pluginsv1alpha1.PluginRequest
	require.NoError(t, c.client.Get(t.Context(), client.ObjectKeyFromObject(other), &got))
	assert.Empty(t, got.Finalizers)
}

func TestApplyObjectsRefusesAnObjectOfAnotherWriter(t *testing.T) {
	other := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: requestName + "-echo"},
		Data:       map[string]string{"owner": "someone else"},
	}
	c := newFakeController(t, other)
	req := &pluginsv1alpha1.PluginRequest{}
	req.Namespace = "ns"
	req.Name = requestName

	err := c.applyObjects(t.Context(), req)

	assert.ErrorContains(t, err, "is not an object of the request")
	var got corev1.ConfigMap
	require.NoError(t, c.client.Get(t.Context(), client.ObjectKeyFromObject(other), &got))
	assert.Equal(t, other.Data, got.Data)
	assert.Empty(t, got.Labels)
}

// newFakeGrantController is newFakeController with the grant template
// readTemplate.
func newFakeGrantController(t *testing.T, objs ...client.Object) *Controller {
	t.Helper()

	c := newFakeController(t, objs...)
	c.opts.GrantTemplates = []pluginsv1alpha1.GrantTemplate{readTemplate()}

	return c
}

// existingGrant is the Grant of grantRequest as the controller wrote it.
func existingGrant(t *testing.T) *pluginsv1alpha1.Grant {
	t.Helper()

	grants, err := newGrantController(t, DefectNone).grants(grantRequest(map[string]string{"source": "hello"}))
	require.NoError(t, err)
	require.Len(t, grants, 1)

	return grants[0]
}

func TestResourcesListEachGrant(t *testing.T) {
	c := newFakeGrantController(t)
	req := grantRequest(map[string]string{"source": "hello"})
	grants, err := c.grants(req)
	require.NoError(t, err)

	rows := c.resources(req, grants)

	assert.Contains(t, rows, pluginsv1alpha1.ResourceRef{
		Kind:     "Grant",
		Name:     grantName(requestName, "read"),
		Role:     "grant",
		OnOrphan: pluginsv1alpha1.OnOrphanKept,
	})
}

func TestCleanUpOrphanKeepsTheGrantWithoutItsOwner(t *testing.T) {
	grant := existingGrant(t)
	c := newFakeGrantController(t, grant)
	req := grantRequest(map[string]string{"source": "hello"})
	req.Spec.DeletionPolicy = pluginsv1alpha1.DeletionPolicyOrphan

	require.NoError(t, c.cleanUp(t.Context(), req))

	var got pluginsv1alpha1.Grant
	require.NoError(t, c.client.Get(t.Context(), client.ObjectKeyFromObject(grant), &got))
	assert.Empty(t, got.OwnerReferences)
	assert.Equal(t, grant.Labels, got.Labels)
}

func TestCleanUpDeleteRemovesTheGrant(t *testing.T) {
	grant := existingGrant(t)
	c := newFakeGrantController(t, grant)
	req := grantRequest(map[string]string{"source": "hello"})
	req.Spec.DeletionPolicy = pluginsv1alpha1.DeletionPolicyDelete

	require.NoError(t, c.cleanUp(t.Context(), req))

	err := c.client.Get(t.Context(), client.ObjectKeyFromObject(grant), &pluginsv1alpha1.Grant{})
	assert.True(t, apierrors.IsNotFound(err), "got %v", err)
}

func TestCleanUpOrphanReleasesAGrantWhoseInputIsNowEmpty(t *testing.T) {
	grant := existingGrant(t)
	c := newFakeGrantController(t, grant)
	req := grantRequest(nil)
	req.Spec.DeletionPolicy = pluginsv1alpha1.DeletionPolicyOrphan

	require.NoError(t, c.cleanUp(t.Context(), req))

	var got pluginsv1alpha1.Grant
	require.NoError(t, c.client.Get(t.Context(), client.ObjectKeyFromObject(grant), &got))
	assert.Empty(t, got.OwnerReferences)
}

func TestCleanUpDeletesAGrantWhoseTemplateNoLongerRenders(t *testing.T) {
	grant := existingGrant(t)
	c := newFakeGrantController(t, grant)
	c.opts.GrantTemplates[0].Template.Raw = []byte(`{"Sid":"{{ .parameters.missing }}"}`)
	req := grantRequest(map[string]string{"source": "hello"})
	req.Spec.DeletionPolicy = pluginsv1alpha1.DeletionPolicyDelete

	require.NoError(t, c.cleanUp(t.Context(), req))

	err := c.client.Get(t.Context(), client.ObjectKeyFromObject(grant), &pluginsv1alpha1.Grant{})
	assert.True(t, apierrors.IsNotFound(err), "got %v", err)
}

func TestApplyObjectsDeletesAGrantWhoseInputIsNowEmpty(t *testing.T) {
	grant := existingGrant(t)
	c := newFakeGrantController(t, grant)

	require.NoError(t, c.applyObjects(t.Context(), grantRequest(nil)))

	err := c.client.Get(t.Context(), client.ObjectKeyFromObject(grant), &pluginsv1alpha1.Grant{})
	assert.True(t, apierrors.IsNotFound(err), "got %v", err)
}
