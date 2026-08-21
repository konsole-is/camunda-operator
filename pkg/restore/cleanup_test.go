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

package restore

import (
	"context"
	"errors"
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// terminalOwner is a restore that reached a terminal phase with the given
// reason, and that recorded one Job per broker.
func terminalOwner(reason string) *v1.PointInTimeRestore {
	owner := restoreOwner(1)
	owner.UID = types.UID("restore-uid")
	owner.Status.TerminalReason = reason
	owner.Status.PrimaryJobNames = []string{
		"my-cluster-pitr-pitr-0", "my-cluster-pitr-pitr-1", "my-cluster-pitr-pitr-2",
	}

	return owner
}

// recordedJob is one of the Jobs of owner, as the primary-storage phase left
// it behind.
func recordedJob(name string, controller types.UID) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "ns",
			UID:       types.UID(name + "-uid"),
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: v1.GroupVersion.String(),
				Kind:       "PointInTimeRestore",
				Name:       "my-cluster-pitr",
				UID:        controller,
				Controller: new(true),
			}},
		},
	}
}

// deleteRecorder captures the options of every delete, so a case can prove
// how the Job was removed and not only that it is gone.
type deleteRecorder struct {
	options map[string]client.DeleteOptions
}

func newDeleteRecorder() *deleteRecorder {
	return &deleteRecorder{options: map[string]client.DeleteOptions{}}
}

func (d *deleteRecorder) funcs() interceptor.Funcs {
	return interceptor.Funcs{
		Delete: func(
			ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption,
		) error {
			var recorded client.DeleteOptions
			recorded.ApplyOptions(opts)
			d.options[obj.GetName()] = recorded

			return c.Delete(ctx, obj, opts...)
		},
	}
}

// collectWorld builds a client that holds the Jobs of owner.
func collectWorld(t *testing.T, recorder *deleteRecorder, jobs ...*batchv1.Job) client.Client {
	t.Helper()

	objects := make([]client.Object, 0, len(jobs))
	for _, job := range jobs {
		objects = append(objects, job)
	}

	return fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(objects...).
		WithInterceptorFuncs(recorder.funcs()).
		Build()
}

// A completed Job keeps its pod, and that pod holds the broker volume it
// mounts. The next operation on the cluster waits on that volume without end,
// so a restore that completed gives its Jobs up.
//
// The propagation policy is the point of the delete. Background propagation
// returns before the pods are gone, and the pods are what hold the volume.
func TestCollectJobsRemovesEveryJobOfACompletedRestore(t *testing.T) {
	t.Parallel()

	owner := terminalOwner(v1.ReasonCompleted)
	recorder := newDeleteRecorder()
	c := collectWorld(
		t,
		recorder,
		recordedJob("my-cluster-pitr-pitr-0", owner.UID),
		recordedJob("my-cluster-pitr-pitr-1", owner.UID),
		recordedJob("my-cluster-pitr-pitr-2", owner.UID),
	)

	ctx := context.Background()
	require.NoError(t, CollectJobs(ctx, c, c, owner, &owner.Status.RestoreProgress))

	var left batchv1.JobList
	require.NoError(t, c.List(ctx, &left, client.InNamespace("ns")))
	assert.Empty(t, left.Items)

	for _, name := range owner.Status.PrimaryJobNames {
		options, deleted := recorder.options[name]
		require.True(t, deleted, "the restore left the Job %s behind", name)
		require.NotNil(t, options.PropagationPolicy, "Job %s", name)
		assert.Equal(t, metav1.DeletePropagationForeground, *options.PropagationPolicy, "Job %s", name)
	}
}

// The logs of a failed Job are the diagnosis of the failure, and only the pod
// keeps them readable. A failed restore therefore holds the broker volumes
// until somebody deletes the restore.
func TestCollectJobsKeepsTheJobsOfAFailedRestore(t *testing.T) {
	t.Parallel()

	owner := terminalOwner(v1.ReasonFailed)
	recorder := newDeleteRecorder()
	c := collectWorld(
		t,
		recorder,
		recordedJob("my-cluster-pitr-pitr-0", owner.UID),
		recordedJob("my-cluster-pitr-pitr-1", owner.UID),
		recordedJob("my-cluster-pitr-pitr-2", owner.UID),
	)

	ctx := context.Background()
	require.NoError(t, CollectJobs(ctx, c, c, owner, &owner.Status.RestoreProgress))

	assert.Empty(t, recorder.options)

	var left batchv1.JobList
	require.NoError(t, c.List(ctx, &left, client.InNamespace("ns")))
	assert.Len(t, left.Items, 3)
}

// The terminal branch runs on every look, so the call repeats until the Jobs
// are gone. A Job that is already gone is the outcome this call wants.
func TestCollectJobsTreatsAMissingJobAsDone(t *testing.T) {
	t.Parallel()

	owner := terminalOwner(v1.ReasonCompleted)
	recorder := newDeleteRecorder()
	c := collectWorld(t, recorder, recordedJob("my-cluster-pitr-pitr-1", owner.UID))

	ctx := context.Background()
	require.NoError(t, CollectJobs(ctx, c, c, owner, &owner.Status.RestoreProgress))

	assert.Equal(t, []string{"my-cluster-pitr-pitr-1"}, keysOf(recorder.options))
}

// A restore that somebody deleted and created again under one name derives
// the names of its predecessor's Jobs. Only the controller reference proves
// that a Job under a recorded name belongs to this restore.
func TestCollectJobsLeavesAJobOfAnotherOwner(t *testing.T) {
	t.Parallel()

	owner := terminalOwner(v1.ReasonCompleted)
	recorder := newDeleteRecorder()
	c := collectWorld(
		t,
		recorder,
		recordedJob("my-cluster-pitr-pitr-0", types.UID("another-restore")),
	)

	ctx := context.Background()
	require.NoError(t, CollectJobs(ctx, c, c, owner, &owner.Status.RestoreProgress))

	assert.Empty(t, recorder.options)

	var left batchv1.JobList
	require.NoError(t, c.List(ctx, &left, client.InNamespace("ns")))
	assert.Len(t, left.Items, 1)
}

// Foreground propagation keeps the Job in place until its pods are gone, so
// every later look reads a Job that already terminates. A second delete of it
// buys nothing.
func TestCollectJobsSkipsAJobThatAlreadyTerminates(t *testing.T) {
	t.Parallel()

	owner := terminalOwner(v1.ReasonCompleted)
	terminating := recordedJob("my-cluster-pitr-pitr-0", owner.UID)
	terminating.DeletionTimestamp = new(metav1.Now())
	terminating.Finalizers = []string{metav1.FinalizerDeleteDependents}

	recorder := newDeleteRecorder()
	c := collectWorld(t, recorder, terminating)

	ctx := context.Background()
	require.NoError(t, CollectJobs(ctx, c, c, owner, &owner.Status.RestoreProgress))

	assert.Empty(t, recorder.options)
}

// keysOf returns the names of every recorded delete, in name order.
func keysOf(options map[string]client.DeleteOptions) []string {
	return slices.Sorted(maps.Keys(options))
}

// The delete carries a UID precondition, so a name that another writer takes
// between the read and the delete comes back as a Conflict. That Job is not
// this restore's to remove, and the restore goes on.
func TestCollectJobsAcceptsAConflictOnTheDelete(t *testing.T) {
	t.Parallel()

	owner := terminalOwner(v1.ReasonCompleted)
	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(recordedJob("my-cluster-pitr-pitr-0", owner.UID)).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(
				_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.DeleteOption,
			) error {
				return apierrors.NewConflict(
					schema.GroupResource{Group: "batch", Resource: "jobs"},
					obj.GetName(),
					errors.New("the UID in the precondition does not match"),
				)
			},
		}).
		Build()

	require.NoError(t, CollectJobs(
		context.Background(), c, c, owner, &owner.Status.RestoreProgress,
	))
}
