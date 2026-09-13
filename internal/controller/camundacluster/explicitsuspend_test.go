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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// suspendCluster is the owner of the workloads that TestSuspendExplicitly
// scales down. suspend says whether the user asked for the suspension.
func suspendCluster(suspend bool) *v1.CamundaCluster {
	return &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-cluster", Namespace: "team-a", UID: "uid-1", Generation: 3,
		},
		Spec: v1.CamundaClusterSpec{Suspend: suspend},
	}
}

// workloadMeta returns the metadata of a workload of cluster: the managed
// labels of comp and, when controlled, the controlling owner reference.
func workloadMeta(cluster *v1.CamundaCluster, name, comp string, controlled bool) metav1.ObjectMeta {
	m := metav1.ObjectMeta{
		Name:      name,
		Namespace: cluster.Namespace,
		Labels: map[string]string{
			labels.ClusterKey:   labels.OwnerName(cluster.Name),
			labels.ComponentKey: comp,
			labels.ManagedByKey: labels.ManagedBy,
		},
	}
	if controlled {
		m.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: v1.GroupVersion.String(),
			Kind:       "CamundaCluster",
			Name:       cluster.Name,
			UID:        cluster.UID,
			Controller: new(true),
		}}
	}

	return m
}

// TestSuspendExplicitly covers suspendExplicitly against a fake client. The
// two instances carry the same managed labels, so only the owner reference
// tells their workloads apart.
func TestSuspendExplicitly(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	cases := map[string]struct {
		suspend bool
		want    map[string]int32
	}{
		"a suspended cluster scales the workloads it controls": {
			suspend: true,
			want: map[string]int32{
				"my-cluster-zeebe":    0,
				"my-cluster-gateway":  0,
				"my-cluster-operate":  0,
				"other-cluster-zeebe": 1,
				"adopted-zeebe":       1,
			},
		},
		"a cluster without spec.suspend scales nothing": {
			suspend: false,
			want: map[string]int32{
				"my-cluster-zeebe":    1,
				"my-cluster-gateway":  0,
				"my-cluster-operate":  2,
				"other-cluster-zeebe": 1,
				"adopted-zeebe":       1,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cluster := suspendCluster(tc.suspend)
			other := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{
				Name: "other-cluster", Namespace: cluster.Namespace, UID: "uid-2",
			}}

			objects := []client.Object{
				&appsv1.StatefulSet{
					ObjectMeta: workloadMeta(cluster, "my-cluster-zeebe", "zeebe", true),
					Spec:       appsv1.StatefulSetSpec{Replicas: new(int32(1))},
				},
				// Already at zero: nothing to patch, and no event.
				&appsv1.Deployment{
					ObjectMeta: workloadMeta(cluster, "my-cluster-gateway", "gateway", true),
					Spec:       appsv1.DeploymentSpec{Replicas: new(int32(0))},
				},
				&appsv1.Deployment{
					ObjectMeta: workloadMeta(cluster, "my-cluster-operate", "operate", true),
					Spec:       appsv1.DeploymentSpec{Replicas: new(int32(2))},
				},
				// Same managed labels, another owner.
				&appsv1.StatefulSet{
					ObjectMeta: workloadMeta(other, "other-cluster-zeebe", "zeebe", true),
					Spec:       appsv1.StatefulSetSpec{Replicas: new(int32(1))},
				},
				// The labels of this cluster and no controller: the operator
				// never rendered it, so it is not ours to stop.
				&appsv1.StatefulSet{
					ObjectMeta: workloadMeta(cluster, "adopted-zeebe", "zeebe", false),
					Spec:       appsv1.StatefulSetSpec{Replicas: new(int32(1))},
				},
			}
			// The labels of the other owner name its own cluster, so the list
			// of this cluster must not reach it at all.
			objects[3].SetLabels(map[string]string{
				labels.ClusterKey:   labels.OwnerName(cluster.Name),
				labels.ManagedByKey: labels.ManagedBy,
			})

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
			r := &CamundaClusterReconciler{
				Client:        fakeClient,
				APIReader:     fakeClient,
				Scheme:        scheme,
				EventRecorder: events.NewFakeRecorder(10),
			}

			require.NoError(t, r.suspendExplicitly(context.Background(), cluster))

			for workload, want := range tc.want {
				key := client.ObjectKey{Namespace: cluster.Namespace, Name: workload}
				assert.Equal(t, want, replicasOf(t, fakeClient, key), workload)
			}
		})
	}
}

// replicasOf returns spec.replicas of the StatefulSet or the Deployment at
// key, whichever exists.
func replicasOf(t *testing.T, reader client.Reader, key client.ObjectKey) int32 {
	t.Helper()

	var set appsv1.StatefulSet
	if err := reader.Get(context.Background(), key, &set); err == nil {
		require.NotNil(t, set.Spec.Replicas, key.Name)

		return *set.Spec.Replicas
	}

	var deployment appsv1.Deployment
	require.NoError(t, reader.Get(context.Background(), key, &deployment), key.Name)
	require.NotNil(t, deployment.Spec.Replicas, key.Name)

	return *deployment.Spec.Replicas
}
