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
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundaoptimize"
)

// TestSuspendWorkloads covers suspendWorkloads against a fake client: every
// controlled Deployment scales to zero and stages the suspension on its
// condition, a Deployment of another CamundaOptimize stays, and an instance
// without workloads reports none.
//
// The two instances carry the same managed labels, so only the owner
// reference tells their Deployments apart.
func TestSuspendWorkloads(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	optimize := suspendOptimize("my-optimize", "uid-1")
	webapp := &appsv1.Deployment{
		ObjectMeta: optimizeWorkloadMeta(optimize, components.ComponentWebapp, optimize),
		Spec:       appsv1.DeploymentSpec{Replicas: new(int32(1))},
		Status:     appsv1.DeploymentStatus{Replicas: 1},
	}
	importer := &appsv1.Deployment{
		ObjectMeta: optimizeWorkloadMeta(optimize, components.ComponentImporter, optimize),
		Spec:       appsv1.DeploymentSpec{Replicas: new(int32(0))},
		Status:     appsv1.DeploymentStatus{Replicas: 0},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(webapp, importer).Build()
	r := &Reconciler{Client: c, APIReader: c, EventRecorder: events.NewFakeRecorder(10)}

	found, err := r.suspendWorkloads(context.Background(), optimize)
	require.NoError(t, err)
	assert.True(t, found)

	var latest appsv1.Deployment
	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(webapp), &latest))
	assert.Equal(t, int32(0), *latest.Spec.Replicas, "the webapp scales to zero")

	webappCond := meta.FindStatusCondition(optimize.Status.Conditions, v1.ConditionWebappReady)
	require.NotNil(t, webappCond)
	assert.Equal(t, metav1.ConditionFalse, webappCond.Status)
	assert.Equal(t, "Suspending", webappCond.Reason, "replicas still run, so the webapp is Suspending")

	importerCond := meta.FindStatusCondition(optimize.Status.Conditions, v1.ConditionImporterReady)
	require.NotNil(t, importerCond)
	assert.Equal(t, metav1.ConditionTrue, importerCond.Status)
	assert.Equal(t, "Suspended", importerCond.Reason, "a workload at zero reports Suspended")

	t.Run("the workloads of another instance stay", func(t *testing.T) {
		other := suspendOptimize("my-optimize", "uid-2")
		found, err := r.suspendWorkloads(context.Background(), other)
		require.NoError(t, err)
		assert.False(t, found)
		assert.Empty(t, other.Status.Conditions)

		var stays appsv1.Deployment
		require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(importer), &stays))
		assert.Equal(t, int32(0), *stays.Spec.Replicas)
	})

	t.Run("an instance without workloads reports none", func(t *testing.T) {
		empty := suspendOptimize("other-optimize", "uid-3")
		found, err := r.suspendWorkloads(context.Background(), empty)
		require.NoError(t, err)
		assert.False(t, found)
		assert.Empty(t, empty.Status.Conditions)
	})
}

// suspendOptimize returns the owner of the workloads that TestSuspendWorkloads
// scales down.
func suspendOptimize(name string, uid types.UID) *v1.CamundaOptimize {
	return &v1.CamundaOptimize{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: "team-a", UID: uid, Generation: 3,
		},
	}
}

// optimizeWorkloadMeta returns the metadata of a workload of optimize with
// owner as its controller.
func optimizeWorkloadMeta(
	optimize *v1.CamundaOptimize,
	comp string,
	owner *v1.CamundaOptimize,
) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      components.WorkloadName(optimize, comp),
		Namespace: optimize.Namespace,
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: v1.GroupVersion.String(),
			Kind:       "CamundaOptimize",
			Name:       owner.Name,
			UID:        owner.UID,
			Controller: new(true),
		}},
	}
}

// TestWasSuspendingCoversEveryStageOfASuspension pins what stops the
// suspension event from being recorded twice.
//
// A suspension passes through three ocf statuses before it settles. A
// reconcile that catches the workloads mid-drain reads one of the first two,
// and it must not read them as "not suspended yet", or it records the
// transition again on every look until the drain finishes.
func TestWasSuspendingCoversEveryStageOfASuspension(t *testing.T) {
	t.Parallel()

	suspending := []string{
		string(component.PendingSuspension),
		string(component.Suspending),
		string(component.Suspended),
	}
	for _, reason := range suspending {
		assert.True(t, wasSuspending(withReadyReason(reason)), reason)
	}

	running := []string{
		v1.ReasonHealthy,
		string(component.AliveUpdating),
		string(component.Down),
		v1.ReasonClusterAlreadyAttached,
	}
	for _, reason := range running {
		assert.False(t, wasSuspending(withReadyReason(reason)), reason)
	}

	assert.False(
		t,
		wasSuspending(&v1.CamundaOptimize{}),
		"a resource with no Ready yet has not been suspended",
	)
}

// withReadyReason returns a CamundaOptimize whose Ready condition carries the
// given reason.
func withReadyReason(reason string) *v1.CamundaOptimize {
	optimize := &v1.CamundaOptimize{}
	optimize.Status.Conditions = []metav1.Condition{{
		Type:   v1.ConditionReady,
		Status: metav1.ConditionTrue,
		Reason: reason,
	}}

	return optimize
}
