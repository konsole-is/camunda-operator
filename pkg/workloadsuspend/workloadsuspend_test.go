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

package workloadsuspend_test

import (
	"context"
	"errors"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/konsole-is/camunda-operator/pkg/workloadsuspend"
)

const suspended = "Scaled to zero because the caller says so"

// TestStopAtZero covers both workload kinds over the states a caller meets: a
// workload with pods still up drains, one whose pods stopped is suspended, and
// one that already asks for no replicas is left alone.
func TestStopAtZero(t *testing.T) {
	objectMeta := metav1.ObjectMeta{Name: "workload", Namespace: "ns"}

	cases := map[string]struct {
		object    client.Object
		wantPatch bool
		status    metav1.ConditionStatus
		reason    string
		message   string
	}{
		"a StatefulSet whose pods still run": {
			object: &appsv1.StatefulSet{
				ObjectMeta: objectMeta,
				Spec:       appsv1.StatefulSetSpec{Replicas: new(int32(3))},
				Status:     appsv1.StatefulSetStatus{Replicas: 2},
			},
			wantPatch: true,
			status:    metav1.ConditionFalse,
			reason:    string(component.Suspending),
			message:   "Waiting for replicas to scale down, 2 replicas still running.",
		},
		"a StatefulSet whose pods stopped": {
			object: &appsv1.StatefulSet{
				ObjectMeta: objectMeta,
				Spec:       appsv1.StatefulSetSpec{Replicas: new(int32(3))},
			},
			wantPatch: true,
			status:    metav1.ConditionTrue,
			reason:    string(component.Suspended),
			message:   suspended,
		},
		"a Deployment whose pods still run": {
			object: &appsv1.Deployment{
				ObjectMeta: objectMeta,
				Spec:       appsv1.DeploymentSpec{Replicas: new(int32(1))},
				Status:     appsv1.DeploymentStatus{Replicas: 1},
			},
			wantPatch: true,
			status:    metav1.ConditionFalse,
			reason:    string(component.Suspending),
			message:   "Waiting for replicas to scale down, 1 replicas still running.",
		},
		"a Deployment that already asks for none": {
			object: &appsv1.Deployment{
				ObjectMeta: objectMeta,
				Spec:       appsv1.DeploymentSpec{Replicas: new(int32(0))},
			},
			wantPatch: false,
			status:    metav1.ConditionTrue,
			reason:    string(component.Suspended),
			message:   suspended,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, clientgoscheme.AddToScheme(scheme))
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tc.object.DeepCopyObject().(client.Object)).
				Build()

			outcome, err := workloadsuspend.StopAtZero(
				context.Background(), fakeClient, tc.object, suspended,
			)

			require.NoError(t, err)
			assert.Equal(t, tc.wantPatch, outcome.Patched)
			assert.Equal(t, tc.status, outcome.Status)
			assert.Equal(t, tc.reason, outcome.Reason)
			assert.Equal(t, tc.message, outcome.Message)
			assert.Equal(t, int32(0), replicas(t, tc.object), "the workload asks for no replicas")
		})
	}
}

// TestStopAtZeroToleratesAWorkloadThatIsGone pins the race with a delete: a
// workload that vanished between the read and the patch runs no pods, whatever
// it observed when it was read, so it reports suspended and no error.
func TestStopAtZeroToleratesAWorkloadThatIsGone(t *testing.T) {
	observed := map[string]int32{"observed no pods": 0, "observed pods still running": 2}

	for name, replicas := range observed {
		t.Run(name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, clientgoscheme.AddToScheme(scheme))
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

			outcome, err := workloadsuspend.StopAtZero(
				context.Background(),
				fakeClient,
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: "gone", Namespace: "ns"},
					Spec:       appsv1.DeploymentSpec{Replicas: new(int32(1))},
					Status:     appsv1.DeploymentStatus{Replicas: replicas},
				},
				suspended,
			)

			require.NoError(t, err)
			assert.False(t, outcome.Patched)
			assert.Equal(t, metav1.ConditionTrue, outcome.Status)
			assert.Equal(t, string(component.Suspended), outcome.Reason)
			assert.Equal(t, suspended, outcome.Message)
		})
	}
}

// TestStopAtZeroWrapsAFailedPatch covers the caller contract on a rejected
// patch: the error names the workload, and Patched stays false so the caller
// records no event for a workload that still runs.
func TestStopAtZeroWrapsAFailedPatch(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	boom := errors.New("admission webhook denied the request")
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "workload", Namespace: "ns"},
		Spec:       appsv1.DeploymentSpec{Replicas: new(int32(1))},
	}
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deployment.DeepCopy()).
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(
				context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption,
			) error {
				return boom
			},
		}).
		Build()

	outcome, err := workloadsuspend.StopAtZero(context.Background(), fakeClient, deployment, suspended)

	require.ErrorIs(t, err, boom)
	assert.False(t, outcome.Patched)
	assert.Contains(t, err.Error(), "ns/workload")
}

// TestStopAtZeroRejectsAnotherKind pins that only the two workload kinds are
// supported, so a caller that passes anything else learns it.
func TestStopAtZeroRejectsAnotherKind(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	_, err := workloadsuspend.StopAtZero(
		context.Background(),
		fakeClient,
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc", Namespace: "ns"}},
		suspended,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot suspend")
}

// replicas returns spec.replicas of the workload under test.
func replicas(t *testing.T, obj client.Object) int32 {
	t.Helper()

	var asked *int32
	switch workload := obj.(type) {
	case *appsv1.StatefulSet:
		asked = workload.Spec.Replicas
	case *appsv1.Deployment:
		asked = workload.Spec.Replicas
	default:
		require.Fail(t, "the fixture carries no workload kind")
	}
	require.NotNil(t, asked)

	return *asked
}
