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

package podstate_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/podstate"
)

// restoreKind is the owner kind that the handler under test looks for.
var restoreKind = v1.GroupVersion.WithKind("PointInTimeRestore").GroupKind()

// restoreJob is a Job that the restore named owner controls.
func restoreJob(name string, owner metav1.OwnerReference) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: namespace, UID: types.UID(name + "-uid"),
			OwnerReferences: []metav1.OwnerReference{owner},
		},
	}
}

// controlledBy is the controller reference that an owner of kind and name
// leaves on the resource it created.
func controlledBy(apiVersion, kind, name string) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       name,
		UID:        types.UID(name + "-uid"),
		Controller: new(true),
	}
}

// jobPod is a pod that the Job controller created for job.
func jobPod(job *batchv1.Job) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: job.Name + "-abcde", Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(job, batchv1.SchemeGroupVersion.WithKind("Job")),
			},
		},
	}
}

// enqueued drives one create event through the handler and returns every
// request it queued.
func enqueued(t *testing.T, reader client.Reader, owner schema.GroupKind, pod client.Object) []reconcile.Request {
	t.Helper()

	queue := workqueue.NewTypedRateLimitingQueue(
		workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](),
	)
	defer queue.ShutDown()

	podstate.EnqueueJobOwner(reader, owner).Create(
		context.Background(), event.CreateEvent{Object: pod}, queue,
	)

	requests := make([]reconcile.Request, 0, queue.Len())
	for queue.Len() > 0 {
		request, shutdown := queue.Get()
		require.False(t, shutdown)
		requests = append(requests, request)
		queue.Done(request)
	}

	return requests
}

// The pod of a restore Job is what reports a container that cannot start, and
// the Job reports nothing about it. The event has to reach the restore that
// owns the Job, under the name of the restore itself.
func TestEnqueueJobOwnerWakesTheRestoreThatOwnsThePod(t *testing.T) {
	t.Parallel()

	job := restoreJob("night-pitr-0", controlledBy(v1.GroupVersion.String(), "PointInTimeRestore", "night"))
	reader := fake.NewClientBuilder().WithScheme(podScheme(t)).WithObjects(job).Build()

	requests := enqueued(t, reader, restoreKind, jobPod(job))

	want := reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: namespace, Name: "night"},
	}
	assert.Equal(t, []reconcile.Request{want}, requests)
}

// The pod cache holds every pod that the operator manages, so a controller
// sees the pods of every other kind too. Only its own pods may wake it.
func TestEnqueueJobOwnerIgnoresAPodOfAnotherKind(t *testing.T) {
	t.Parallel()

	job := restoreJob("night-lrrdbms", controlledBy(v1.GroupVersion.String(), "LogicalRestoreRDBMS", "night"))
	reader := fake.NewClientBuilder().WithScheme(podScheme(t)).WithObjects(job).Build()

	assert.Empty(t, enqueued(t, reader, restoreKind, jobPod(job)))
}

// A kind name is not unique across groups. A Job of a stranger's CRD with the
// same kind name must not wake a restore.
func TestEnqueueJobOwnerIgnoresAPodOfAnotherGroup(t *testing.T) {
	t.Parallel()

	job := restoreJob("night-pitr-0", controlledBy("example.com/v1", "PointInTimeRestore", "night"))
	reader := fake.NewClientBuilder().WithScheme(podScheme(t)).WithObjects(job).Build()

	assert.Empty(t, enqueued(t, reader, restoreKind, jobPod(job)))
}

// A broker pod carries the managed-by label too, so it reaches the handler.
// Its controller is a StatefulSet, and no restore owns it.
func TestEnqueueJobOwnerIgnoresAPodThatNoJobControls(t *testing.T) {
	t.Parallel()

	reader := fake.NewClientBuilder().WithScheme(podScheme(t)).Build()

	broker := pod("camunda-zeebe-0")
	broker.OwnerReferences = []metav1.OwnerReference{
		controlledBy("apps/v1", "StatefulSet", "camunda-zeebe"),
	}

	assert.Empty(t, enqueued(t, reader, restoreKind, broker))
}

// A cache that has not caught up with a new Job answers nothing. The poll of
// the running phase reads the pods again, so the restore loses no more than
// one look.
func TestEnqueueJobOwnerIgnoresAPodWhoseJobIsAbsent(t *testing.T) {
	t.Parallel()

	job := restoreJob("night-pitr-0", controlledBy(v1.GroupVersion.String(), "PointInTimeRestore", "night"))
	reader := fake.NewClientBuilder().WithScheme(podScheme(t)).Build()

	assert.Empty(t, enqueued(t, reader, restoreKind, jobPod(job)))
}
