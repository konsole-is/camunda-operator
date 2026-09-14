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

package camundaoptimize

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// TestDeleteControlledLeavesAReplacementAlone covers the race that the UID
// precondition closes: the object is read, something deletes it and builds
// another under the same name, and the delete that follows must not take the
// new one. The API server refuses it with a conflict, which is the wanted state
// and no error.
func TestDeleteControlledLeavesAReplacementAlone(t *testing.T) {
	scheme := suspendScheme(t)
	optimize := &v1.CamundaOptimize{ObjectMeta: metav1.ObjectMeta{
		Name: "co-a", Namespace: "team-a", UID: "uid-1",
	}}
	deployment := ownedDeployment(optimize, "co-a-webapp", 1)
	key := client.ObjectKeyFromObject(deployment)

	var preconditions *client.Preconditions
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deployment).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(
				ctx context.Context,
				cl client.WithWatch,
				obj client.Object,
				opts ...client.DeleteOption,
			) error {
				for _, opt := range opts {
					if p, ok := opt.(*client.Preconditions); ok {
						preconditions = p
					}
				}

				return apierrors.NewConflict(
					schema.GroupResource{Group: "apps", Resource: "deployments"},
					obj.GetName(),
					errors.New("the UID in the precondition does not match the UID in record"),
				)
			},
		}).
		Build()
	r := suspendReconciler(scheme, fakeClient, events.NewFakeRecorder(10))

	err := r.deleteControlled(context.Background(), key, &appsv1.Deployment{}, optimize)

	require.NoError(t, err, "a replacement under another UID is not this instance to delete")
	require.NotNil(t, preconditions, "the delete carries a precondition")
	require.NotNil(t, preconditions.UID)
	assert.Equal(t, deployment.UID, *preconditions.UID, "the UID is the one that was read")
	assert.NoError(
		t,
		fakeClient.Get(context.Background(), key, &appsv1.Deployment{}),
		"the object at that name stays",
	)
}

// TestRemoveComponentConditionsDropsWhatAParkedHolderNoLongerRenders pins the
// status of a deposed holder. It renders no component, so FlushStatus owns none
// of the component condition types and writes back whatever the object carries.
// A WebappReady of True over a deleted Deployment is the state this prevents.
func TestRemoveComponentConditionsDropsWhatAParkedHolderNoLongerRenders(t *testing.T) {
	optimize := &v1.CamundaOptimize{}
	for _, conditionType := range []string{
		v1.ConditionWebappReady,
		v1.ConditionImporterReady,
		v1.ConditionMirroredSecretsReady,
	} {
		meta.SetStatusCondition(optimize.GetStatusConditions(), metav1.Condition{
			Type:   conditionType,
			Status: metav1.ConditionTrue,
			Reason: "Healthy",
		})
	}
	meta.SetStatusCondition(optimize.GetStatusConditions(), metav1.Condition{
		Type:   v1.ConditionReady,
		Status: metav1.ConditionFalse,
		Reason: v1.ReasonClusterAlreadyAttached,
	})

	removeComponentConditions(optimize)

	assert.Nil(t, meta.FindStatusCondition(*optimize.GetStatusConditions(), v1.ConditionWebappReady))
	assert.Nil(t, meta.FindStatusCondition(*optimize.GetStatusConditions(), v1.ConditionImporterReady))
	assert.Nil(
		t, meta.FindStatusCondition(*optimize.GetStatusConditions(), v1.ConditionMirroredSecretsReady),
	)

	ready := meta.FindStatusCondition(*optimize.GetStatusConditions(), v1.ConditionReady)
	if assert.NotNil(t, ready, "Ready carries the reason the CamundaOptimize is parked") {
		assert.Equal(t, v1.ReasonClusterAlreadyAttached, ready.Reason)
	}
}

// TestRemoveComponentConditionsOnAHolderThatNeverRendered pins that the call is
// safe on a CamundaOptimize that was parked from its first reconcile. It never
// wrote a component condition, so there is nothing to drop.
func TestRemoveComponentConditionsOnAHolderThatNeverRendered(t *testing.T) {
	optimize := &v1.CamundaOptimize{}

	removeComponentConditions(optimize)

	assert.Empty(t, *optimize.GetStatusConditions())
}
