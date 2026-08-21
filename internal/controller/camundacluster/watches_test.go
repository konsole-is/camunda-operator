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

package camundacluster

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
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
// its resources carries the bounded form and not the name.
var longClusterName = strings.Repeat("c", 70)

// brokerClaim returns a PersistentVolumeClaim labeled as the operator labels
// a broker volume of the named cluster.
func brokerClaim(namespace, cluster string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name:      "data-" + cluster + "-zeebe-0",
		Namespace: namespace,
		Labels:    labels.Managed(labels.Cluster(cluster), "zeebe"),
	}}
}

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

func TestEnqueueForBrokerClaim(t *testing.T) {
	scheme, err := v1.SchemeBuilder.Build()
	require.NoError(t, err)
	require.NoError(t, corev1.AddToScheme(scheme))

	cluster := func(name string) *v1.CamundaCluster {
		return &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"}}
	}

	r := &CamundaClusterReconciler{Client: fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cluster("short"), cluster(longClusterName), cluster("other")).
		Build()}

	enqueue := func(claim *corev1.PersistentVolumeClaim) []reconcile.Request {
		q := workqueue.NewTypedRateLimitingQueue(
			workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](),
		)
		r.enqueueForBrokerClaim().Create(context.Background(), event.CreateEvent{Object: claim}, q)

		return drain(q)
	}

	t.Run("maps a claim of a short-named cluster to that cluster", func(t *testing.T) {
		assert.Equal(
			t, []reconcile.Request{
				{NamespacedName: types.NamespacedName{Namespace: "ns", Name: "short"}},
			}, enqueue(brokerClaim("ns", "short")),
		)
	})

	// The label value is the bounded name here, so a mapper that enqueued it
	// as a name would ask for a cluster that does not exist and the volume
	// status of every long-named cluster would stop moving.
	t.Run("maps a claim of a long-named cluster to the unbounded name", func(t *testing.T) {
		claim := brokerClaim("ns", longClusterName)
		require.NotEqual(t, longClusterName, claim.Labels[labels.ClusterKey])

		assert.Equal(
			t, []reconcile.Request{
				{NamespacedName: types.NamespacedName{Namespace: "ns", Name: longClusterName}},
			}, enqueue(claim),
		)
	})

	t.Run("maps a claim of an unknown cluster to no request", func(t *testing.T) {
		assert.Empty(t, enqueue(brokerClaim("ns", "absent")))
	})

	t.Run("maps a claim in another namespace to no request", func(t *testing.T) {
		assert.Empty(t, enqueue(brokerClaim("elsewhere", "short")))
	})

	t.Run("maps a claim without the owner label to no request", func(t *testing.T) {
		claim := brokerClaim("ns", "short")
		claim.Labels = nil

		assert.Empty(t, enqueue(claim))
	})
}
