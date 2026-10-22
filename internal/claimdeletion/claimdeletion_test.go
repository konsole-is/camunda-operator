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

package claimdeletion

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

const namespace = "team-a"

func newScheme(t *testing.T) *runtime.Scheme {
	scheme := runtime.NewScheme()
	require.NoError(t, fleetv1alpha1.AddToScheme(scheme))
	require.NoError(t, pluginsv1alpha1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	return scheme
}

func deletedClaim() *fleetv1alpha1.CamundaManagementClusterClaim {
	return &fleetv1alpha1.CamundaManagementClusterClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "my-plane",
			Namespace:         namespace,
			UID:               "claim-uid",
			Finalizers:        []string{fleetv1alpha1.FinalizerCleanup},
			DeletionTimestamp: ptr.To(metav1.Now()),
		},
	}
}

// controlledBy returns obj with a controller reference to claim.
func controlledBy[T client.Object](obj T, claim client.Object) T {
	obj.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: fleetv1alpha1.GroupVersion.String(),
		Kind:       "CamundaManagementClusterClaim",
		Name:       claim.GetName(),
		UID:        claim.GetUID(),
		Controller: ptr.To(true),
	}})

	return obj
}

func request(name string) *pluginsv1alpha1.PluginRequest {
	return &pluginsv1alpha1.PluginRequest{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: pluginsv1alpha1.PluginRequestSpec{
			Section:        "access",
			Registration:   "test-access",
			DeletionPolicy: pluginsv1alpha1.DeletionPolicyDelete,
		},
	}
}

func plane(name string) *v1.CamundaManagementCluster {
	return &v1.CamundaManagementCluster{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
}

func registration(policies ...pluginsv1alpha1.DeletionPolicy) *pluginsv1alpha1.PluginRegistration {
	return &pluginsv1alpha1.PluginRegistration{
		ObjectMeta: metav1.ObjectMeta{Name: "test-access"},
		Spec:       pluginsv1alpha1.PluginRegistrationSpec{DeletionPolicies: policies},
	}
}

func newDeleter(t *testing.T, objs ...client.Object) (Deleter, client.Client) {
	c := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(objs...).Build()

	return Deleter{Client: c, APIReader: c, Scheme: c.Scheme(), EventRecorder: events.NewFakeRecorder(16)}, c
}

func input(claim client.Object, stages ...[]client.Object) Input {
	return Input{
		Claim:              claim,
		FieldManagerPrefix: fleetv1alpha1.FieldManagerPrefixPlaneClaim,
		DeletionPolicy:     pluginsv1alpha1.DeletionPolicyDelete,
		Selections:         []fleetv1alpha1.PluginSelection{{Section: "access", Registration: "test-access"}},
		Stages:             stages,
	}
}

func exists(t *testing.T, c client.Client, obj client.Object) bool {
	err := c.Get(context.Background(), client.ObjectKeyFromObject(obj), obj)
	if apierrors.IsNotFound(err) {
		return false
	}
	require.NoError(t, err)

	return true
}

func TestFinalizeDeletesTheGeneratedObjectsBeforeTheRequests(t *testing.T) {
	claim := deletedClaim()
	generated := controlledBy(plane("my-plane"), claim)
	access := controlledBy(request("my-plane-access"), claim)
	d, c := newDeleter(t, claim, generated, access, registration(pluginsv1alpha1.DeletionPolicyDelete))

	remaining, err := d.Finalize(context.Background(), input(claim, []client.Object{plane("my-plane")}))

	require.NoError(t, err)
	assert.Equal(t, []string{"CamundaManagementCluster my-plane", "PluginRequest my-plane-access"}, remaining)
	assert.False(t, exists(t, c, plane("my-plane")))
	assert.True(t, exists(t, c, request("my-plane-access")))
}

func TestFinalizeDeletesTheRequestsWhenTheGeneratedObjectsAreGone(t *testing.T) {
	claim := deletedClaim()
	access := controlledBy(request("my-plane-access"), claim)
	d, c := newDeleter(t, claim, access, registration(pluginsv1alpha1.DeletionPolicyDelete))

	remaining, err := d.Finalize(context.Background(), input(claim, []client.Object{plane("my-plane")}))

	require.NoError(t, err)
	assert.Equal(t, []string{"PluginRequest my-plane-access"}, remaining)
	assert.False(t, exists(t, c, request("my-plane-access")))
}

func TestFinalizeCountsAnObjectThatTheClaimDoesNotControlAsGone(t *testing.T) {
	claim := deletedClaim()
	foreign := plane("my-plane")
	d, c := newDeleter(t, claim, foreign, registration(pluginsv1alpha1.DeletionPolicyDelete))

	remaining, err := d.Finalize(context.Background(), input(claim, []client.Object{plane("my-plane")}))

	require.NoError(t, err)
	assert.Empty(t, remaining)
	assert.True(t, exists(t, c, plane("my-plane")), "an object of another owner is never deleted")
}

func TestFinalizeRemovesTheFinalizerWhenNothingIsLeft(t *testing.T) {
	claim := deletedClaim()
	d, c := newDeleter(t, claim, registration(pluginsv1alpha1.DeletionPolicyDelete))

	remaining, err := d.Finalize(context.Background(), input(claim, []client.Object{plane("my-plane")}))

	require.NoError(t, err)
	assert.Empty(t, remaining)
	assert.False(t, exists(t, c, deletedClaim()), "the claim goes once its finalizer is removed")
}

func TestFinalizeDoesNothingWithoutTheFinalizer(t *testing.T) {
	claim := deletedClaim()
	claim.Finalizers = []string{"example.com/other"}
	generated := controlledBy(plane("my-plane"), claim)
	d, c := newDeleter(t, claim, generated, registration(pluginsv1alpha1.DeletionPolicyDelete))

	remaining, err := d.Finalize(context.Background(), input(claim, []client.Object{plane("my-plane")}))

	require.NoError(t, err)
	assert.Nil(t, remaining)
	assert.True(t, exists(t, c, plane("my-plane")))
}

func TestFinalizeRefusesAnOrphanPolicyThatARegistrationDoesNotSupport(t *testing.T) {
	claim := deletedClaim()
	generated := controlledBy(plane("my-plane"), claim)
	d, c := newDeleter(t, claim, generated, registration(pluginsv1alpha1.DeletionPolicyDelete))
	in := input(claim, []client.Object{plane("my-plane")})
	in.DeletionPolicy = pluginsv1alpha1.DeletionPolicyOrphan

	_, err := d.Finalize(context.Background(), in)

	var failure *conditions.PreCheckFailure
	require.True(t, errors.As(err, &failure))
	assert.Equal(t, fleetv1alpha1.ReasonNotAllowed, failure.Reason)
	assert.True(t, exists(t, c, plane("my-plane")), "a refused policy deletes nothing")
}
