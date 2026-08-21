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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// longClusterName is longer than a label value admits, so the owner label of
// the rendered Deployments carries the bounded form and not the name.
var longClusterName = strings.Repeat("c", 70)

// drain pops every request queued by a handler.
func drain(q workqueue.TypedRateLimitingInterface[reconcile.Request]) []reconcile.Request {
	var reqs []reconcile.Request
	for q.Len() > 0 {
		req, _ := q.Get()
		reqs = append(reqs, req)
		q.Done(req)
	}

	return reqs
}

func TestEnqueueForOptimizeWorkload(t *testing.T) {
	scheme, err := v1.SchemeBuilder.Build()
	require.NoError(t, err)
	require.NoError(t, appsv1.AddToScheme(scheme))

	cluster := func(name string) *v1.CamundaCluster {
		return &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"}}
	}
	optimize := func(name, clusterName string) *v1.CamundaOptimize {
		return &v1.CamundaOptimize{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"},
			Spec:       v1.CamundaOptimizeSpec{ClusterRef: v1.ClusterRef{Name: clusterName}},
		}
	}

	r := &Reconciler{Client: fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&v1.CamundaOptimize{}, clusterRefField, indexers[clusterRefField]).
		WithObjects(
			cluster("short"), cluster(longClusterName),
			optimize("on-short", "short"),
			optimize("on-long-a", longClusterName),
			optimize("on-long-b", longClusterName),
		).
		Build()}

	// workload returns a Deployment as the operator renders it for Optimize
	// on the named cluster.
	workload := func(clusterName string) *appsv1.Deployment {
		return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
			Name:      "opt-webapp",
			Namespace: "ns",
			Labels:    labels.Managed(labels.Cluster(clusterName), "webapp"),
		}}
	}

	enqueue := func(deployment *appsv1.Deployment) []reconcile.Request {
		q := workqueue.NewTypedRateLimitingQueue(
			workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](),
		)
		r.enqueueForOptimizeWorkload().
			Create(context.Background(), event.CreateEvent{Object: deployment}, q)

		return drain(q)
	}

	t.Run("maps a workload of a short-named cluster to its Optimizes", func(t *testing.T) {
		assert.ElementsMatch(
			t, []reconcile.Request{
				{NamespacedName: types.NamespacedName{Namespace: "ns", Name: "on-short"}},
			}, enqueue(workload("short")),
		)
	})

	// The label value is the bounded name here, so a mapper that looked it up
	// in the clusterRef index would match nothing and the Optimize waiting
	// for a handover would never wake.
	t.Run("maps a workload of a long-named cluster to its Optimizes", func(t *testing.T) {
		deployment := workload(longClusterName)
		require.NotEqual(t, longClusterName, deployment.Labels[labels.ClusterKey])

		assert.ElementsMatch(
			t, []reconcile.Request{
				{NamespacedName: types.NamespacedName{Namespace: "ns", Name: "on-long-a"}},
				{NamespacedName: types.NamespacedName{Namespace: "ns", Name: "on-long-b"}},
			}, enqueue(deployment),
		)
	})

	t.Run("maps a workload of an unknown cluster to no request", func(t *testing.T) {
		assert.Empty(t, enqueue(workload("absent")))
	})

	t.Run("maps a workload without the owner label to no request", func(t *testing.T) {
		deployment := workload("short")
		deployment.Labels = nil

		assert.Empty(t, enqueue(deployment))
	})
}
