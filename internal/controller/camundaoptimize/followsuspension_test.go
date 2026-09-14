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
	"slices"
	"strings"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundaoptimize"
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

// ownedDeployment returns a Deployment of owner with the given name, asking for
// one replica and observing observed of them.
func ownedDeployment(owner *v1.CamundaOptimize, name string, observed int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: owner.Namespace,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: v1.GroupVersion.String(),
				Kind:       "CamundaOptimize",
				Name:       owner.Name,
				UID:        owner.UID,
				Controller: new(true),
			}},
		},
		Spec:   appsv1.DeploymentSpec{Replicas: new(int32(1))},
		Status: appsv1.DeploymentStatus{Replicas: observed},
	}
}

// suspendReconciler returns a Reconciler that reads and writes through c and
// records into recorder.
func suspendReconciler(
	scheme *runtime.Scheme,
	c client.Client,
	recorder *events.FakeRecorder,
) *Reconciler {
	return &Reconciler{
		Client:          c,
		APIReader:       c,
		Scheme:          scheme,
		EventRecorder:   recorder,
		componentClient: c,
	}
}

// TestFollowSuspensionJoinsPatchErrors covers the error path: the importer is
// the workload that writes Elasticsearch, so a webapp that a conflict keeps up
// must not keep the importer up with it. A workload whose patch was rejected
// keeps its pods, so it reports no suspension.
func TestFollowSuspensionJoinsPatchErrors(t *testing.T) {
	scheme := suspendScheme(t)

	optimize := &v1.CamundaOptimize{
		ObjectMeta: metav1.ObjectMeta{
			Name: "co-a", Namespace: "team-a", UID: "uid-1", Generation: 3,
		},
		Spec: v1.CamundaOptimizeSpec{ClusterRef: v1.ClusterRef{Name: "my-cluster"}},
	}
	webappName := components.WorkloadName(optimize, components.ComponentWebapp)
	importerName := components.WorkloadName(optimize, components.ComponentImporter)
	boom := errors.New("admission webhook denied the request")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			ownedDeployment(optimize, webappName, 0),
			ownedDeployment(optimize, importerName, 0),
		).
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(
				ctx context.Context,
				cl client.WithWatch,
				obj client.Object,
				patch client.Patch,
				opts ...client.PatchOption,
			) error {
				if obj.GetName() == webappName {
					return boom
				}

				return cl.Patch(ctx, obj, patch, opts...)
			},
		}).
		Build()
	r := suspendReconciler(scheme, fakeClient, events.NewFakeRecorder(10))

	outcome, err := r.followSuspension(context.Background(), optimize, clusterSuspended)

	require.ErrorIs(t, err, boom)
	assert.True(t, outcome.Found)
	assert.True(t, outcome.Stopped, "the importer stopped, so the transition started")
	assert.Contains(t, err.Error(), webappName, "the error names the workload that stayed up")

	var importer appsv1.Deployment
	require.NoError(t, fakeClient.Get(
		context.Background(),
		client.ObjectKey{Namespace: optimize.Namespace, Name: importerName},
		&importer,
	))
	assert.Equal(t, int32(0), *importer.Spec.Replicas, "the importer is scaled anyway")

	assert.Nil(
		t,
		meta.FindStatusCondition(optimize.Status.Conditions, v1.ConditionWebappReady),
		"a workload whose patch was rejected keeps its pods, so it reports no suspension",
	)
	staged := meta.FindStatusCondition(optimize.Status.Conditions, v1.ConditionImporterReady)
	require.NotNil(t, staged)
	assert.Equal(t, string(component.Suspended), staged.Reason)
}

// TestFollowSuspensionStopsWhatItRead covers a read that failed halfway: the
// importer is read and the webapp read fails. The importer must stop anyway. It
// is the workload that writes Elasticsearch, and a restore of that Elasticsearch
// is why the cluster is suspended.
func TestFollowSuspensionStopsWhatItRead(t *testing.T) {
	scheme := suspendScheme(t)

	optimize := &v1.CamundaOptimize{
		ObjectMeta: metav1.ObjectMeta{Name: "co-a", Namespace: "team-a", UID: "uid-1"},
		Spec:       v1.CamundaOptimizeSpec{ClusterRef: v1.ClusterRef{Name: "my-cluster"}},
	}
	webapp := components.WorkloadName(optimize, components.ComponentWebapp)
	importer := components.WorkloadName(optimize, components.ComponentImporter)
	boom := errors.New("etcd leader changed")
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			ownedDeployment(optimize, importer, 1),
			ownedDeployment(optimize, webapp, 1),
		).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(
				ctx context.Context,
				cl client.WithWatch,
				key client.ObjectKey,
				obj client.Object,
				opts ...client.GetOption,
			) error {
				if key.Name == webapp {
					return boom
				}

				return cl.Get(ctx, key, obj, opts...)
			},
		}).
		Build()
	r := suspendReconciler(scheme, fakeClient, events.NewFakeRecorder(10))

	outcome, err := r.followSuspension(context.Background(), optimize, clusterSuspended)

	require.ErrorIs(t, err, boom)
	assert.Contains(t, err.Error(), webapp, "the error names the read that failed")
	assert.True(t, outcome.Found)
	var stopped appsv1.Deployment
	require.NoError(t, fakeClient.Get(
		context.Background(),
		client.ObjectKey{Namespace: optimize.Namespace, Name: importer},
		&stopped,
	))
	assert.Equal(t, int32(0), *stopped.Spec.Replicas, "the workload that was read is stopped")
}

// TestFollowSuspensionFindsNoWorkloadOfAnotherOwner covers the owner guard: two
// CamundaOptimizes of one cluster carry the same managed labels, so only the
// owner reference tells their Deployments apart.
func TestFollowSuspensionFindsNoWorkloadOfAnotherOwner(t *testing.T) {
	scheme := suspendScheme(t)

	optimize := &v1.CamundaOptimize{ObjectMeta: metav1.ObjectMeta{
		Name: "co-a", Namespace: "team-a", UID: "uid-1",
	}}
	foreign := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      components.WorkloadName(optimize, components.ComponentImporter),
			Namespace: optimize.Namespace,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: v1.GroupVersion.String(),
				Kind:       "CamundaOptimize",
				Name:       "co-b",
				UID:        apitypes.UID("uid-2"),
				Controller: new(true),
			}},
		},
		Spec: appsv1.DeploymentSpec{Replicas: new(int32(1))},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(foreign).Build()
	r := suspendReconciler(scheme, fakeClient, events.NewFakeRecorder(10))

	outcome, err := r.followSuspension(context.Background(), optimize, clusterSuspended)

	require.NoError(t, err)
	assert.False(t, outcome.Found)
	assert.False(t, outcome.Stopped)

	var kept appsv1.Deployment
	require.NoError(t, fakeClient.Get(context.Background(), client.ObjectKeyFromObject(foreign), &kept))
	assert.Equal(t, int32(1), *kept.Spec.Replicas)
}

// TestFollowSuspensionReportsTheDrain covers the condition vocabulary: ocf
// reports Suspending while the pods of a scaled workload are still up, and
// Suspended once they are gone. The watch on the Deployment brings the reconcile
// back as they drop.
func TestFollowSuspensionReportsTheDrain(t *testing.T) {
	scheme := suspendScheme(t)

	cases := map[string]struct {
		observed int32
		status   metav1.ConditionStatus
		reason   string
	}{
		"pods still up": {observed: 2, status: metav1.ConditionFalse, reason: string(component.Suspending)},
		"pods stopped":  {observed: 0, status: metav1.ConditionTrue, reason: string(component.Suspended)},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			optimize := &v1.CamundaOptimize{ObjectMeta: metav1.ObjectMeta{
				Name: "co-a", Namespace: "team-a", UID: "uid-1",
			}}
			importer := ownedDeployment(
				optimize, components.WorkloadName(optimize, components.ComponentImporter), tc.observed,
			)

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(importer).Build()
			r := suspendReconciler(scheme, fakeClient, events.NewFakeRecorder(10))

			outcome, err := r.followSuspension(context.Background(), optimize, clusterSuspended)

			require.NoError(t, err)
			assert.True(t, outcome.Found)
			assert.True(t, outcome.Stopped)
			staged := meta.FindStatusCondition(optimize.Status.Conditions, v1.ConditionImporterReady)
			require.NotNil(t, staged)
			assert.Equal(t, tc.status, staged.Status)
			assert.Equal(t, tc.reason, staged.Reason)
		})
	}
}

// TestFollowSuspensionFindsNothingWithoutWorkloads covers an instance that
// rendered no Deployment yet. found gates the note on Ready, so it must report
// none and stage no condition.
func TestFollowSuspensionFindsNothingWithoutWorkloads(t *testing.T) {
	scheme := suspendScheme(t)

	optimize := &v1.CamundaOptimize{ObjectMeta: metav1.ObjectMeta{
		Name: "co-a", Namespace: "team-a", UID: "uid-1",
	}}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	r := suspendReconciler(scheme, fakeClient, events.NewFakeRecorder(10))

	outcome, err := r.followSuspension(context.Background(), optimize, clusterSuspended)

	require.NoError(t, err)
	assert.False(t, outcome.Found)
	assert.False(t, outcome.Stopped)
	assert.Empty(t, optimize.Status.Conditions)
}

// TestKeepAtZeroReadsNoWorkloadWithoutASuspension covers the early return: an
// instance that never followed a suspension has nothing to hold, so it must not
// read its Deployments to find that out.
func TestKeepAtZeroReadsNoWorkloadWithoutASuspension(t *testing.T) {
	scheme := suspendScheme(t)

	optimize := &v1.CamundaOptimize{ObjectMeta: metav1.ObjectMeta{
		Name: "co-a", Namespace: "team-a", UID: "uid-1",
	}}
	meta.SetStatusCondition(&optimize.Status.Conditions, metav1.Condition{
		Type:   v1.ConditionWebappReady,
		Status: metav1.ConditionTrue,
		Reason: v1.ReasonHealthy,
	})

	var read int
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(
				ctx context.Context,
				cl client.WithWatch,
				key client.ObjectKey,
				obj client.Object,
				opts ...client.GetOption,
			) error {
				read++

				return cl.Get(ctx, key, obj, opts...)
			},
		}).
		Build()
	r := suspendReconciler(scheme, fakeClient, events.NewFakeRecorder(10))

	outcome, err := r.keepAtZero(context.Background(), optimize)

	require.NoError(t, err)
	assert.False(t, outcome.Found)
	assert.Zero(t, read, "the conditions answered it, so no Deployment was read")
}

// TestFollowsSuspendedCluster pins what counts as a suspension that the
// conditions already carry. Every ocf status on the way to suspended counts: a
// reconcile that catches the drain must not record the transition a second
// time.
func TestFollowsSuspendedCluster(t *testing.T) {
	t.Parallel()

	withCondition := func(conditionType, reason string) *v1.CamundaOptimize {
		optimize := &v1.CamundaOptimize{}
		meta.SetStatusCondition(&optimize.Status.Conditions, metav1.Condition{
			Type:   conditionType,
			Status: metav1.ConditionTrue,
			Reason: reason,
		})

		return optimize
	}

	following := []string{
		string(component.PendingSuspension),
		string(component.Suspending),
		string(component.Suspended),
	}
	for _, reason := range following {
		assert.True(t, followsSuspendedCluster(withCondition(v1.ConditionImporterReady, reason)), reason)
		assert.True(t, followsSuspendedCluster(withCondition(v1.ConditionWebappReady, reason)), reason)
	}

	running := []string{v1.ReasonHealthy, string(component.AliveUpdating), string(component.Down)}
	for _, reason := range running {
		assert.False(t, followsSuspendedCluster(withCondition(v1.ConditionImporterReady, reason)), reason)
	}

	assert.False(
		t,
		followsSuspendedCluster(&v1.CamundaOptimize{}),
		"a resource with no workload condition follows no suspension",
	)
	assert.False(
		t,
		followsSuspendedCluster(withCondition(v1.ConditionReady, string(component.Suspended))),
		"Ready is the business of wasSuspending",
	)
}

// TestReconcileRecordsTheSuspensionWhenAPatchFails covers the event bookkeeping
// of a failed patch. A pass that stopped one Deployment started the transition,
// so it is recorded even though the other stayed up: the next retry reads the
// suspension off the flushed condition of the one that stopped. A pass that
// stopped none has no suspension to report, and recording one would repeat on
// every retry.
func TestReconcileRecordsTheSuspensionWhenAPatchFails(t *testing.T) {
	scheme := suspendScheme(t)

	optimize := &v1.CamundaOptimize{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "co-a",
			Namespace:  "team-a",
			UID:        "uid-1",
			Finalizers: []string{Finalizer},
		},
		Spec: v1.CamundaOptimizeSpec{
			Version:           "8.9.4",
			ManagementAuthRef: "mac",
			ClusterRef:        v1.ClusterRef{Name: "my-cluster"},
		},
	}
	webappName := components.WorkloadName(optimize, components.ComponentWebapp)
	importerName := components.WorkloadName(optimize, components.ComponentImporter)

	cases := map[string]struct {
		rejected   []string
		wantEvents int
	}{
		"one patch fails":   {rejected: []string{webappName}, wantEvents: 1},
		"every patch fails": {rejected: []string{webappName, importerName}, wantEvents: 0},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// The cluster is suspended and its storageRef names nothing, so the
			// pre-check fails after it read the suspension.
			cluster := &v1.CamundaCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-cluster", Namespace: optimize.Namespace, UID: "uid-c",
				},
				Spec: v1.CamundaClusterSpec{
					Version:    "8.9.4",
					Suspend:    true,
					StorageRef: "no-such-contract",
				},
			}
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(
					optimize.DeepCopy(),
					cluster, ownedDeployment(optimize, webappName, 0), ownedDeployment(optimize, importerName, 0),
				).
				WithStatusSubresource(&v1.CamundaOptimize{}).
				WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(
						ctx context.Context,
						cl client.WithWatch,
						obj client.Object,
						patch client.Patch,
						opts ...client.PatchOption,
					) error {
						if slices.Contains(tc.rejected, obj.GetName()) {
							return errors.New("admission webhook denied the request")
						}

						return cl.Patch(ctx, obj, patch, opts...)
					},
				}).
				Build()
			recorder := events.NewFakeRecorder(16)
			r := suspendReconciler(scheme, fakeClient, recorder)

			key := apitypes.NamespacedName{Namespace: optimize.Namespace, Name: optimize.Name}
			_, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: key})

			require.Error(t, err, "the rejected patch is returned, so the reconcile retries")
			assert.Equal(t, tc.wantEvents, countRecorded(recorder, eventReasonClusterSuspended))
		})
	}
}

// countRecorded drains recorder and returns how many of its events carry the
// given reason.
func countRecorded(recorder *events.FakeRecorder, reason string) int {
	var count int
	for {
		select {
		case recorded := <-recorder.Events:
			if strings.Contains(recorded, reason) {
				count++
			}
		default:
			return count
		}
	}
}

// TestReconcileFollowsTheSuspensionWhenTheAttachmentCheckFails covers the order
// of the pre-check: the suspension of the cluster is read before the attachment
// checks, so a failure there still stops the workloads.
//
// A logical restore of Elasticsearch suspends the cluster to stop every writer.
// An importer left running through a handover would write analytics from
// half-restored indices.
func TestReconcileFollowsTheSuspensionWhenTheAttachmentCheckFails(t *testing.T) {
	scheme := suspendScheme(t)

	optimize := &v1.CamundaOptimize{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "co-a",
			Namespace:  "team-a",
			UID:        "uid-1",
			Finalizers: []string{Finalizer},
		},
		Spec: v1.CamundaOptimizeSpec{
			Version:           "8.9.4",
			ManagementAuthRef: "mac",
			ClusterRef:        v1.ClusterRef{Name: "my-cluster"},
		},
	}
	cluster := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cluster", Namespace: optimize.Namespace, UID: "uid-c"},
		Spec:       v1.CamundaClusterSpec{Version: "8.9.4", Suspend: true, StorageRef: "contract"},
	}
	// The importer Deployment of a previous instance, which no owner reference
	// of this one covers, so the pre-check fails with WaitingForHandover.
	leftover := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "co-previous-importer",
			Namespace: optimize.Namespace,
			Labels: labels.Managed(
				labels.Cluster(optimize.Spec.ClusterRef.Name), components.ComponentImporter,
			),
		},
		Spec: appsv1.DeploymentSpec{Replicas: new(int32(1))},
	}
	webappName := components.WorkloadName(optimize, components.ComponentWebapp)
	importerName := components.WorkloadName(optimize, components.ComponentImporter)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			optimize, cluster, leftover,
			ownedDeployment(optimize, webappName, 0),
			ownedDeployment(optimize, importerName, 0),
		).
		WithStatusSubresource(&v1.CamundaOptimize{}).
		Build()
	r := suspendReconciler(scheme, fakeClient, events.NewFakeRecorder(16))

	key := apitypes.NamespacedName{Namespace: optimize.Namespace, Name: optimize.Name}
	_, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: key})
	require.NoError(t, err)

	for _, name := range []string{webappName, importerName} {
		var deployment appsv1.Deployment
		require.NoError(t, fakeClient.Get(
			t.Context(), client.ObjectKey{Namespace: optimize.Namespace, Name: name}, &deployment,
		))
		assert.Equal(t, int32(0), *deployment.Spec.Replicas, name)
	}

	var latest v1.CamundaOptimize
	require.NoError(t, fakeClient.Get(t.Context(), key, &latest))
	ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
	require.NotNil(t, ready)
	assert.Equal(t, v1.ReasonWaitingForHandover, ready.Reason)
}
