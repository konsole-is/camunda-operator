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
	"errors"
	"strings"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
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
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// suspendScheme is the scheme that every fixture of this file builds its client
// with.
func suspendScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	return scheme
}

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

// workloadMeta returns the metadata of a workload that labelOwner names and,
// when controlled, that owner controls. The two are the same cluster for a
// workload the operator rendered. They differ for the workload of a second
// cluster that carries the labels of the first, which the listing reaches and
// the owner reference rejects.
func workloadMeta(
	labelOwner, owner *v1.CamundaCluster,
	name, comp string,
	controlled bool,
) metav1.ObjectMeta {
	m := metav1.ObjectMeta{
		Name:      name,
		Namespace: labelOwner.Namespace,
		Labels: map[string]string{
			labels.ClusterKey:   labels.OwnerName(labelOwner.Name),
			labels.ComponentKey: comp,
			labels.ManagedByKey: labels.ManagedBy,
		},
	}
	if controlled {
		m.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: v1.GroupVersion.String(),
			Kind:       "CamundaCluster",
			Name:       owner.Name,
			UID:        owner.UID,
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
		found   bool
		// observed is status.replicas of every workload that this cluster
		// controls, the pods that are still up.
		observed int32
		// noOwned leaves out the workloads that this cluster controls, the
		// state of a cluster that never rendered one.
		noOwned bool
		// wantReason is the reason of the per-process condition of a scaled
		// workload. Empty expects no condition at all.
		wantReason string
		want       map[string]int32
	}{
		"a suspended cluster whose pods still run reports Suspending": {
			suspend:    true,
			found:      true,
			observed:   2,
			wantReason: string(component.Suspending),
			want: map[string]int32{
				"my-cluster-zeebe":    0,
				"my-cluster-gateway":  0,
				"my-cluster-operate":  0,
				"other-cluster-zeebe": 1,
				"adopted-zeebe":       1,
			},
		},
		"a suspended cluster whose pods stopped reports Suspended": {
			suspend:    true,
			found:      true,
			observed:   0,
			wantReason: string(component.Suspended),
			want: map[string]int32{
				"my-cluster-zeebe":    0,
				"my-cluster-gateway":  0,
				"my-cluster-operate":  0,
				"other-cluster-zeebe": 1,
				"adopted-zeebe":       1,
			},
		},
		// A cluster created with spec.suspend and a dangling reference never
		// rendered a workload, so Ready must not claim that any stopped.
		"a suspended cluster that controls no workload finds none": {
			suspend: true,
			found:   false,
			noOwned: true,
			want: map[string]int32{
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

			owned := []client.Object{
				&appsv1.StatefulSet{
					ObjectMeta: workloadMeta(cluster, cluster, "my-cluster-zeebe", "zeebe", true),
					Spec:       appsv1.StatefulSetSpec{Replicas: new(int32(1))},
					Status:     appsv1.StatefulSetStatus{Replicas: tc.observed},
				},
				// Already at zero: nothing to patch, and no event.
				&appsv1.Deployment{
					ObjectMeta: workloadMeta(cluster, cluster, "my-cluster-gateway", "gateway", true),
					Spec:       appsv1.DeploymentSpec{Replicas: new(int32(0))},
					Status:     appsv1.DeploymentStatus{Replicas: tc.observed},
				},
				&appsv1.Deployment{
					ObjectMeta: workloadMeta(cluster, cluster, "my-cluster-operate", "operate", true),
					Spec:       appsv1.DeploymentSpec{Replicas: new(int32(2))},
					Status:     appsv1.DeploymentStatus{Replicas: tc.observed},
				},
			}
			// Both carry the labels of this cluster, so the listing reaches
			// them. The owner reference rejects the first, and the second has
			// none: the operator never rendered it.
			foreign := []client.Object{
				&appsv1.StatefulSet{
					ObjectMeta: workloadMeta(cluster, other, "other-cluster-zeebe", "zeebe", true),
					Spec:       appsv1.StatefulSetSpec{Replicas: new(int32(1))},
				},
				&appsv1.StatefulSet{
					ObjectMeta: workloadMeta(cluster, cluster, "adopted-zeebe", "zeebe", false),
					Spec:       appsv1.StatefulSetSpec{Replicas: new(int32(1))},
				},
			}

			objects := foreign
			if !tc.noOwned {
				objects = append(owned, foreign...)
			}

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
			r := &CamundaClusterReconciler{
				Client:        fakeClient,
				APIReader:     fakeClient,
				Scheme:        scheme,
				EventRecorder: events.NewFakeRecorder(10),
			}

			found, err := r.suspendExplicitly(context.Background(), cluster)
			require.NoError(t, err)
			assert.Equal(t, tc.found, found)

			for workload, want := range tc.want {
				key := client.ObjectKey{Namespace: cluster.Namespace, Name: workload}
				assert.Equal(t, want, replicasOf(t, fakeClient, key), workload)
			}

			zeebe := meta.FindStatusCondition(cluster.Status.Conditions, v1.ConditionZeebeReady)
			if tc.wantReason == "" {
				assert.Nil(t, zeebe)

				return
			}
			require.NotNil(t, zeebe)
			assert.Equal(t, tc.wantReason, zeebe.Reason)
		})
	}
}

// TestSuspendExplicitlyStopsWhatItRead covers a read that failed halfway: the
// listing reaches the StatefulSets and fails on the Deployments. The brokers
// must stop anyway. A restore of the backend suspends the cluster to stop every
// writer, and a transient API failure must not leave one of them writing.
func TestSuspendExplicitlyStopsWhatItRead(t *testing.T) {
	scheme := suspendScheme(t)
	cluster := suspendCluster(true)
	boom := errors.New("etcd leader changed")
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			&appsv1.StatefulSet{
				ObjectMeta: workloadMeta(cluster, cluster, "my-cluster-zeebe", "zeebe", true),
				Spec:       appsv1.StatefulSetSpec{Replicas: new(int32(1))},
			},
			&appsv1.Deployment{
				ObjectMeta: workloadMeta(cluster, cluster, "my-cluster-operate", "operate", true),
				Spec:       appsv1.DeploymentSpec{Replicas: new(int32(2))},
			},
		).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(
				ctx context.Context,
				cl client.WithWatch,
				list client.ObjectList,
				opts ...client.ListOption,
			) error {
				if _, ok := list.(*appsv1.DeploymentList); ok {
					return boom
				}

				return cl.List(ctx, list, opts...)
			},
		}).
		Build()
	r := &CamundaClusterReconciler{
		Client:        fakeClient,
		APIReader:     fakeClient,
		Scheme:        scheme,
		EventRecorder: events.NewFakeRecorder(10),
	}

	found, err := r.suspendExplicitly(context.Background(), cluster)

	require.ErrorIs(t, err, boom)
	assert.Contains(t, err.Error(), "listing the Deployments", "the error names the read that failed")
	assert.True(t, found, "the StatefulSets were read, and one of them is ours")
	assert.Equal(
		t, int32(0), replicasOf(
			t, fakeClient, client.ObjectKey{Namespace: cluster.Namespace, Name: "my-cluster-zeebe"},
		), "the workload that was read is stopped",
	)
}

// TestSuspendExplicitlyJoinsPatchErrors covers the error path: one workload
// that a conflict or an admission rule keeps up must not leave the rest of
// them running.
func TestSuspendExplicitlyJoinsPatchErrors(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	cluster := suspendCluster(true)
	boom := errors.New("admission webhook denied the request")
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			&appsv1.StatefulSet{
				ObjectMeta: workloadMeta(cluster, cluster, "my-cluster-zeebe", "zeebe", true),
				Spec:       appsv1.StatefulSetSpec{Replicas: new(int32(1))},
			},
			&appsv1.Deployment{
				ObjectMeta: workloadMeta(cluster, cluster, "my-cluster-operate", "operate", true),
				Spec:       appsv1.DeploymentSpec{Replicas: new(int32(2))},
			},
		).
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(
				ctx context.Context,
				cl client.WithWatch,
				obj client.Object,
				patch client.Patch,
				opts ...client.PatchOption,
			) error {
				if obj.GetName() == "my-cluster-zeebe" {
					return boom
				}

				return cl.Patch(ctx, obj, patch, opts...)
			},
		}).
		Build()
	recorder := events.NewFakeRecorder(10)
	r := &CamundaClusterReconciler{
		Client:        fakeClient,
		APIReader:     fakeClient,
		Scheme:        scheme,
		EventRecorder: recorder,
	}

	found, err := r.suspendExplicitly(context.Background(), cluster)

	require.ErrorIs(t, err, boom)
	assert.True(t, found)
	assert.Contains(t, err.Error(), "my-cluster-zeebe", "the error names the workload that stayed up")
	assert.Equal(
		t, int32(0), replicasOf(
			t, fakeClient, client.ObjectKey{Namespace: cluster.Namespace, Name: "my-cluster-operate"},
		), "the other workload is scaled anyway",
	)

	assert.Nil(
		t,
		meta.FindStatusCondition(cluster.Status.Conditions, v1.ConditionZeebeReady),
		"a workload whose patch was rejected keeps its pods, so it reports no suspension",
	)
	operate := meta.FindStatusCondition(cluster.Status.Conditions, v1.ConditionOperateReady)
	require.NotNil(t, operate)
	assert.Equal(t, string(component.Suspended), operate.Reason)

	// The condition of a refused workload says nothing about the refusal, so
	// the event is where a user reads why it still runs.
	var refusals []string
	for range len(recorder.Events) {
		recorded := <-recorder.Events
		if strings.Contains(recorded, "WorkloadStopRefused") {
			refusals = append(refusals, recorded)
		}
	}
	require.Len(t, refusals, 1)
	assert.Contains(t, refusals[0], "Warning")
	assert.Contains(t, refusals[0], "my-cluster-zeebe")
	assert.Contains(t, refusals[0], boom.Error(), "the event carries the reason the API server gave")
}

// TestSuspendExplicitlyReportsAFailedList covers a listing that fails: the
// suspension scales nothing, and the error says which listing failed.
func TestSuspendExplicitlyReportsAFailedList(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	cluster := suspendCluster(true)
	boom := errors.New("connection refused")
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(&appsv1.StatefulSet{
			ObjectMeta: workloadMeta(cluster, cluster, "my-cluster-zeebe", "zeebe", true),
			Spec:       appsv1.StatefulSetSpec{Replicas: new(int32(1))},
		}).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(
				context.Context, client.WithWatch, client.ObjectList, ...client.ListOption,
			) error {
				return boom
			},
		}).
		Build()
	r := &CamundaClusterReconciler{
		Client:        fakeClient,
		APIReader:     fakeClient,
		Scheme:        scheme,
		EventRecorder: events.NewFakeRecorder(10),
	}

	found, err := r.suspendExplicitly(context.Background(), cluster)

	require.ErrorIs(t, err, boom)
	assert.False(t, found, "a listing that failed found nothing")
	assert.Contains(t, err.Error(), "listing the StatefulSets of the cluster")
	assert.Equal(
		t, int32(1), replicasOf(
			t, fakeClient, client.ObjectKey{Namespace: cluster.Namespace, Name: "my-cluster-zeebe"},
		), "a listing that failed scales nothing",
	)
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

// TestKeepAtZeroFinishesTheDrain covers the workload that a suspension left
// mid-drain. The condition caught it at Suspending, and its pods have stopped
// since, so the pass that holds it at zero reads it again rather than copying a
// state that has moved on.
func TestKeepAtZeroFinishesTheDrain(t *testing.T) {
	scheme := suspendScheme(t)
	cluster := suspendCluster(false)
	zeebe := &appsv1.StatefulSet{
		ObjectMeta: workloadMeta(cluster, cluster, "my-cluster-zeebe", "zeebe", true),
		// Already at zero, with no pods left: the drain the condition caught is
		// over.
		Spec: appsv1.StatefulSetSpec{Replicas: new(int32(0))},
	}
	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:    v1.ConditionZeebeReady,
		Status:  metav1.ConditionFalse,
		Reason:  string(component.Suspending),
		Message: "Waiting for replicas to scale down, 2 replicas still running.",
	})

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(zeebe).Build()
	r := &CamundaClusterReconciler{
		Client:        fakeClient,
		APIReader:     fakeClient,
		Scheme:        scheme,
		EventRecorder: events.NewFakeRecorder(10),
	}

	kept, err := r.keepAtZero(context.Background(), cluster)

	require.NoError(t, err)
	assert.True(t, kept)
	condition := meta.FindStatusCondition(cluster.Status.Conditions, v1.ConditionZeebeReady)
	require.NotNil(t, condition)
	assert.Equal(t, string(component.Suspended), condition.Reason, "the pods are gone")
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, "Kept at zero until the reference check passes", condition.Message)
}

// TestKeepAtZeroDropsAStaleSuspension covers a condition that outlived the
// state it reported: a render raised the workload and the status flush that
// would have said so was lost to a conflict, so the persisted condition still
// reads Suspended over a running workload.
//
// The hold must not believe it and patch that workload back to zero. It drops
// the condition instead, so nothing else reads a suspension from it, the
// endpoints stay published, and the next render stages it again.
func TestKeepAtZeroDropsAStaleSuspension(t *testing.T) {
	scheme := suspendScheme(t)
	cluster := suspendCluster(false)
	raised := &appsv1.Deployment{
		ObjectMeta: workloadMeta(cluster, cluster, "my-cluster-gateway", "gateway", true),
		Spec:       appsv1.DeploymentSpec{Replicas: new(int32(3))},
		Status:     appsv1.DeploymentStatus{Replicas: 3},
	}
	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:   v1.ConditionGatewayReady,
		Status: metav1.ConditionTrue,
		Reason: string(component.Suspended),
	})

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(raised).Build()
	recorder := events.NewFakeRecorder(10)
	r := &CamundaClusterReconciler{
		Client:        fakeClient,
		APIReader:     fakeClient,
		Scheme:        scheme,
		EventRecorder: recorder,
	}

	kept, err := r.keepAtZero(context.Background(), cluster)

	require.NoError(t, err)
	assert.False(t, kept)
	assert.Equal(
		t, int32(3), replicasOf(
			t, fakeClient, client.ObjectKey{Namespace: cluster.Namespace, Name: "my-cluster-gateway"},
		), "the render owns a workload it raised",
	)
	assert.Nil(
		t,
		meta.FindStatusCondition(cluster.Status.Conditions, v1.ConditionGatewayReady),
		"the stale suspension is gone",
	)
	assert.False(t, endpointsStopped(cluster), "the gateway serves, so its endpoints stay published")
	assert.Empty(t, recorder.Events, "nothing was scaled, so nothing is recorded")
}

// TestKeepAtZeroReadsNoWorkloadWithoutASuspension covers the early return: a
// cluster that never suspended has nothing to hold, which is every failing pass
// of most clusters, so it must not read its workloads to find that out.
func TestKeepAtZeroReadsNoWorkloadWithoutASuspension(t *testing.T) {
	scheme := suspendScheme(t)
	cluster := suspendCluster(false)
	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:   v1.ConditionZeebeReady,
		Status: metav1.ConditionTrue,
		Reason: v1.ReasonHealthy,
	})

	var listed int
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(
				ctx context.Context,
				cl client.WithWatch,
				list client.ObjectList,
				opts ...client.ListOption,
			) error {
				listed++

				return cl.List(ctx, list, opts...)
			},
		}).
		Build()
	r := &CamundaClusterReconciler{
		Client:        fakeClient,
		APIReader:     fakeClient,
		Scheme:        scheme,
		EventRecorder: events.NewFakeRecorder(10),
	}

	kept, err := r.keepAtZero(context.Background(), cluster)

	require.NoError(t, err)
	assert.False(t, kept)
	assert.Zero(t, listed, "the conditions answered it, so no workload was read")
}

// TestKeepAtZeroLeavesAWorkloadItNeverStopped covers the guard: a workload whose
// condition carries no suspension is running for the user, and a pass that held
// it at zero would stop it for a reason the user never gave.
func TestKeepAtZeroLeavesAWorkloadItNeverStopped(t *testing.T) {
	scheme := suspendScheme(t)
	cluster := suspendCluster(false)
	zeebe := &appsv1.StatefulSet{
		ObjectMeta: workloadMeta(cluster, cluster, "my-cluster-zeebe", "zeebe", true),
		Spec:       appsv1.StatefulSetSpec{Replicas: new(int32(1))},
	}
	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:   v1.ConditionZeebeReady,
		Status: metav1.ConditionTrue,
		Reason: v1.ReasonHealthy,
	})

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(zeebe).Build()
	r := &CamundaClusterReconciler{
		Client:        fakeClient,
		APIReader:     fakeClient,
		Scheme:        scheme,
		EventRecorder: events.NewFakeRecorder(10),
	}

	kept, err := r.keepAtZero(context.Background(), cluster)

	require.NoError(t, err)
	assert.False(t, kept)
	assert.Equal(
		t, int32(1), replicasOf(
			t, fakeClient, client.ObjectKey{Namespace: cluster.Namespace, Name: "my-cluster-zeebe"},
		), "a running workload stays running",
	)
}

// TestEndpointsStopped covers which process decides that the endpoints answer
// nothing: the gateway, or the brokers when the gateway is embedded. A workload
// whose stop was refused keeps serving, so its endpoints stay published.
func TestEndpointsStopped(t *testing.T) {
	t.Parallel()

	condition := func(conditionType, reason string) metav1.Condition {
		return metav1.Condition{Type: conditionType, Status: metav1.ConditionTrue, Reason: reason}
	}

	cases := map[string]struct {
		conditions []metav1.Condition
		stopped    bool
	}{
		"the standalone gateway stopped": {
			conditions: []metav1.Condition{
				condition(v1.ConditionGatewayReady, string(component.Suspended)),
				condition(v1.ConditionZeebeReady, string(component.Suspended)),
			},
			stopped: true,
		},
		"the standalone gateway kept running": {
			conditions: []metav1.Condition{
				condition(v1.ConditionGatewayReady, v1.ReasonHealthy),
				condition(v1.ConditionZeebeReady, string(component.Suspended)),
			},
			stopped: false,
		},
		"an embedded gateway whose brokers stopped": {
			conditions: []metav1.Condition{
				condition(v1.ConditionGatewayReady, string(component.Disabled)),
				condition(v1.ConditionZeebeReady, string(component.Suspended)),
			},
			stopped: true,
		},
		"an embedded gateway whose brokers kept running": {
			conditions: []metav1.Condition{
				condition(v1.ConditionGatewayReady, string(component.Disabled)),
				condition(v1.ConditionZeebeReady, v1.ReasonHealthy),
			},
			stopped: false,
		},
		"a cluster that reports nothing yet": {stopped: false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cluster := &v1.CamundaCluster{}
			for _, c := range tc.conditions {
				meta.SetStatusCondition(&cluster.Status.Conditions, c)
			}

			assert.Equal(t, tc.stopped, endpointsStopped(cluster))
		})
	}
}
