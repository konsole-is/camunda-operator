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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// suspendCluster is the owner of the workloads that TestSuspendWorkloads
// scales down.
func suspendCluster() *v1.CamundaCluster {
	return &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-cluster", Namespace: "team-a", UID: "uid-1", Generation: 3,
		},
	}
}

// workloadMeta returns the metadata of a workload of cluster: the managed
// labels of comp and the controlling owner reference.
func workloadMeta(cluster *v1.CamundaCluster, name, comp string, controlled bool) metav1.ObjectMeta {
	m := metav1.ObjectMeta{
		Name:      name,
		Namespace: cluster.Namespace,
		Labels: map[string]string{
			labels.ClusterKey:   cluster.Name,
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

// TestSuspendWorkloads covers suspendWorkloads against a fake client: every
// controlled workload scales to zero and stages the suspension on its
// process condition, a workload of another owner stays, and a cluster
// without workloads reports none.
func TestSuspendWorkloads(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	cluster := suspendCluster()
	foreign := suspendCluster()
	foreign.Name, foreign.UID = "other-cluster", "uid-2"

	sts := &appsv1.StatefulSet{
		ObjectMeta: workloadMeta(cluster, "my-cluster-zeebe", "zeebe", true),
		Spec:       appsv1.StatefulSetSpec{Replicas: new(int32(3))},
		Status:     appsv1.StatefulSetStatus{Replicas: 3},
	}
	gateway := &appsv1.Deployment{
		ObjectMeta: workloadMeta(cluster, "my-cluster-gateway", "gateway", true),
		Spec:       appsv1.DeploymentSpec{Replicas: new(int32(0))},
		Status:     appsv1.DeploymentStatus{Replicas: 0},
	}
	unowned := &appsv1.Deployment{
		ObjectMeta: workloadMeta(cluster, "my-cluster-connectors", "connectors", false),
		Spec:       appsv1.DeploymentSpec{Replicas: new(int32(1))},
	}
	unowned.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: v1.GroupVersion.String(),
		Kind:       "CamundaCluster",
		Name:       foreign.Name,
		UID:        foreign.UID,
		Controller: new(true),
	}}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sts, gateway, unowned).Build()
	r := &CamundaClusterReconciler{
		Client: c, APIReader: c, EventRecorder: events.NewFakeRecorder(10),
	}

	found, err := r.suspendWorkloads(context.Background(), cluster)
	require.NoError(t, err)
	assert.True(t, found)

	var latestSts appsv1.StatefulSet
	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(sts), &latestSts))
	assert.Equal(t, int32(0), *latestSts.Spec.Replicas, "the broker StatefulSet scales to zero")

	var latestUnowned appsv1.Deployment
	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(unowned), &latestUnowned))
	assert.Equal(t, int32(1), *latestUnowned.Spec.Replicas, "a workload of another owner stays")

	zeebe := meta.FindStatusCondition(cluster.Status.Conditions, v1.ConditionZeebeReady)
	require.NotNil(t, zeebe)
	assert.Equal(t, metav1.ConditionFalse, zeebe.Status)
	assert.Equal(t, "Suspending", zeebe.Reason, "replicas still run, so the broker condition is Suspending")

	gatewayCond := meta.FindStatusCondition(cluster.Status.Conditions, v1.ConditionGatewayReady)
	require.NotNil(t, gatewayCond)
	assert.Equal(t, metav1.ConditionTrue, gatewayCond.Status)
	assert.Equal(t, "Suspended", gatewayCond.Reason, "a workload at zero reports Suspended")

	assert.Nil(
		t, meta.FindStatusCondition(cluster.Status.Conditions, v1.ConditionConnectorsReady),
		"a workload of another owner stages nothing",
	)

	t.Run("a cluster without workloads reports none", func(t *testing.T) {
		empty := suspendCluster()
		empty.Namespace = "empty-ns"
		found, err := r.suspendWorkloads(context.Background(), empty)
		require.NoError(t, err)
		assert.False(t, found)
		assert.Empty(t, empty.Status.Conditions)
	})
}
