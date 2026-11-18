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

package camundamanagementclusterclaim

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/ownerstatus"
)

func TestReconcileRequeuesWhenTheFinalizerUpdateConflicts(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, fleetv1alpha1.AddToScheme(scheme))
	require.NoError(t, pluginsv1alpha1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))
	ready := metav1.Condition{
		Type:               v1.ConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             v1.ReasonHealthy,
		ObservedGeneration: 1,
		LastTransitionTime: metav1.Now(),
	}
	claim := &fleetv1alpha1.CamundaManagementClusterClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "my-plane", Namespace: "team-a", Generation: 2},
		Status: fleetv1alpha1.CamundaManagementClusterClaimStatus{
			ResolvedPolicy:     &fleetv1alpha1.ResolvedPolicy{},
			ObservedGeneration: 1,
			Conditions:         []metav1.Condition{ready},
		},
	}
	base := fake.NewClientBuilder().WithScheme(scheme).WithObjects(claim).WithStatusSubresource(claim).Build()
	c := interceptor.NewClient(base, interceptor.Funcs{
		Update: func(_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.UpdateOption) error {
			resource := fleetv1alpha1.GroupVersion.WithResource("claims").GroupResource()
			return apierrors.NewConflict(resource, obj.GetName(), errors.New("changed"))
		},
	})
	r := &Reconciler{Client: c, APIReader: c, Scheme: scheme, componentClient: c}

	res, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "team-a", Name: "my-plane"},
	})

	require.NoError(t, err)
	assert.Equal(t, ownerstatus.ConflictRequeue, res.RequeueAfter)
	var current fleetv1alpha1.CamundaManagementClusterClaim
	require.NoError(t, base.Get(context.Background(), client.ObjectKeyFromObject(claim), &current))
	assert.Equal(t, int64(1), current.Status.ObservedGeneration)
	got := meta.FindStatusCondition(current.Status.Conditions, v1.ConditionReady)
	require.NotNil(t, got)
	assert.Equal(t, v1.ReasonHealthy, got.Reason)
	assert.Equal(t, int64(1), got.ObservedGeneration)
}
