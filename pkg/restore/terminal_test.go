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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// terminalWorld is the state a terminal restore looks at: the restore that
// ended, the cluster it suspended, the Jobs it ran, and the writes that its
// looks made.
type terminalWorld struct {
	restore *v1.PointInTimeRestore
	cluster *v1.CamundaCluster
	client  client.Client
	touched *[]string
	applies *[]applied
}

// terminalOptions shape one case of the terminal branch.
type terminalOptions struct {
	// reason is the recorded terminal reason of the restore.
	reason string
	// terminating keeps every Job in the state that foreground propagation
	// leaves behind: deleted, and held by the foreground finalizer until its
	// pods are gone.
	terminating bool
	// jobReadError fails every read of a Job.
	jobReadError error
}

// newTerminalWorld builds a restore that ended, holding everything a restore
// can hold: three Jobs, a suspended cluster it recorded as its own, and the
// claim on that cluster.
func newTerminalWorld(t *testing.T, opts terminalOptions) *terminalWorld {
	t.Helper()

	owner := terminalOwner(opts.reason)
	owner.Status.ClusterSuspended = true

	cluster := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cluster", Namespace: "ns", UID: clusterUID},
		Spec:       v1.CamundaClusterSpec{Suspend: true},
	}

	objects := make([]client.Object, 0, len(owner.Status.PrimaryJobNames)+1)
	objects = append(objects, cluster)
	for _, name := range owner.Status.PrimaryJobNames {
		job := recordedJob(name, owner.UID)
		if opts.terminating {
			// The API server stamps the finalizer and keeps the Job until the
			// collector reports its pods gone. Until then the pods can exist,
			// and a pod is what holds a broker volume.
			job.DeletionTimestamp = new(metav1.NewTime(time.Now()))
			job.Finalizers = []string{metav1.FinalizerDeleteDependents}
		}
		objects = append(objects, job)
	}

	touched := &[]string{}
	applies := &[]applied{}

	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(objects...).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(
				ctx context.Context,
				cl client.WithWatch,
				key client.ObjectKey,
				obj client.Object,
				opt ...client.GetOption,
			) error {
				if _, isJob := obj.(*batchv1.Job); isJob && opts.jobReadError != nil {
					return opts.jobReadError
				}

				return cl.Get(ctx, key, obj, opt...)
			},
			Delete: func(
				ctx context.Context, cl client.WithWatch, obj client.Object, opt ...client.DeleteOption,
			) error {
				*touched = append(*touched, touch(obj))

				return cl.Delete(ctx, obj, opt...)
			},
			Patch: func(
				ctx context.Context,
				cl client.WithWatch,
				obj client.Object,
				patch client.Patch,
				opt ...client.PatchOption,
			) error {
				*touched = append(*touched, touch(obj))
				if patched, isCluster := obj.(*v1.CamundaCluster); isCluster {
					var options client.PatchOptions
					options.ApplyOptions(opt)
					*applies = append(*applies, applied{
						manager: options.FieldManager, cluster: patched.DeepCopy(),
					})
				}

				return cl.Patch(ctx, obj, patch, opt...)
			},
		}).
		Build()

	// The restore holds the claim from its admission on, so the terminal
	// branch has one to give back.
	_, err := Take(t.Context(), c, c, owner, "my-cluster")
	require.NoError(t, err)
	*touched = nil

	return &terminalWorld{
		restore: owner, cluster: cluster, client: c, touched: touched, applies: applies,
	}
}

// touch names the kind that one write reached, which is what the order of the
// terminal branch is about.
func touch(obj client.Object) string {
	switch obj.(type) {
	case *batchv1.Job:
		return "Job"
	case *v1.CamundaCluster:
		return "CamundaCluster"
	case *coordinationv1.Lease:
		return "Lease"
	default:
		return "other"
	}
}

// look runs one look of the terminal branch and returns what it wrote, in
// order, beside the outcome. Each look reports its own writes, so a case can
// prove which look made them.
func (w *terminalWorld) look(t *testing.T) (Outcome, []string, error) {
	t.Helper()

	*w.touched = nil
	outcome, err := Finish(
		t.Context(), w.client, w.client, w.restore, &w.restore.Status.RestoreProgress, "my-cluster",
	)

	return outcome, *w.touched, err
}

// jobsLeft counts the recorded Jobs that still exist.
func (w *terminalWorld) jobsLeft(t *testing.T) int {
	t.Helper()

	left := 0
	for _, name := range w.restore.Status.PrimaryJobNames {
		var job batchv1.Job
		key := types.NamespacedName{Namespace: "ns", Name: name}
		if err := w.client.Get(t.Context(), key, &job); err == nil {
			left++
		}
	}

	return left
}

// withdrawals are the applies that took spec.suspend off the cluster. A
// server-side apply is what withdraws it, so the apply is the observable
// fact, as it is for Resume itself.
func (w *terminalWorld) withdrawals() []applied {
	withdrawn := make([]applied, 0, len(*w.applies))
	for _, apply := range *w.applies {
		if apply.manager == string(FieldManagerTargetSuspend) && !apply.cluster.Spec.Suspend {
			withdrawn = append(withdrawn, apply)
		}
	}

	return withdrawn
}

// A restore that completed gives back everything it held, and it stages the
// outcome it recorded. It takes two looks: the first asks for the Jobs, and
// the second finds them gone.
func TestFinishReleasesEverythingOfACompletedRestore(t *testing.T) {
	t.Parallel()

	w := newTerminalWorld(t, terminalOptions{reason: v1.ReasonCompleted})

	asked, _, err := w.look(t)
	require.NoError(t, err)
	assert.False(t, asked.Done, "the volumes are not free while the Jobs are still being removed")
	assert.Equal(t, Shortly, asked.Wait)

	done, _, err := w.look(t)
	require.NoError(t, err)
	assert.True(t, done.Done)

	assert.Zero(t, w.jobsLeft(t), "the Jobs of a completed restore hold its broker volumes")
	assert.Len(t, w.withdrawals(), 1, "the restore left the cluster it suspended suspended")
	assert.Empty(t, leaseHolder(t, w.client), "the cluster is not free for the next operation")

	condition := ready(w.restore)
	require.NotNil(t, condition)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, v1.ReasonCompleted, condition.Reason)
}

// The order is the correctness of this branch, and it spans the two looks.
// The Jobs hold the broker volumes, so nothing else is written until a look
// finds them gone.
func TestFinishFreesTheVolumesBeforeItFreesTheCluster(t *testing.T) {
	t.Parallel()

	w := newTerminalWorld(t, terminalOptions{reason: v1.ReasonCompleted})

	_, first, err := w.look(t)
	require.NoError(t, err)
	assert.Equal(
		t,
		[]string{"Job", "Job", "Job"},
		first,
		"a broker that starts before its volume is free cannot attach it",
	)

	_, second, err := w.look(t)
	require.NoError(t, err)
	assert.Equal(
		t,
		[]string{"CamundaCluster", "Lease"},
		second,
		"the cluster is reported free before its volumes are",
	)
}

// The gate is the point of the collection answer. A Job that still carries the
// foreground finalizer can still have pods, and those pods hold the broker
// volumes, so the unsuspend waits however many looks that takes.
func TestFinishDoesNotUnsuspendWhileAJobStillTerminates(t *testing.T) {
	t.Parallel()

	w := newTerminalWorld(t, terminalOptions{reason: v1.ReasonCompleted, terminating: true})

	for range 3 {
		outcome, written, err := w.look(t)
		require.NoError(t, err)

		assert.False(t, outcome.Done)
		assert.Equal(t, Shortly, outcome.Wait)
		assert.Empty(t, written, "a Job that already terminates is asked for again")
	}

	assert.Equal(t, len(w.restore.Status.PrimaryJobNames), w.jobsLeft(t))
	assert.Empty(t, w.withdrawals(), "the brokers started over volumes that the Job pods still hold")
	assert.Equal(t, selfClaimant().String(), leaseHolder(t, w.client))
}

// A failed restore keeps its Jobs, because their logs are the diagnosis, and
// it keeps the cluster suspended, because the broker volumes can be half
// written. The claim goes back either way: the restore is over. Nothing is
// collected, so one look finishes it.
func TestFinishKeepsTheJobsAndTheSuspensionOfAFailedRestore(t *testing.T) {
	t.Parallel()

	w := newTerminalWorld(t, terminalOptions{reason: v1.ReasonFailed})

	outcome, written, err := w.look(t)
	require.NoError(t, err)

	assert.True(t, outcome.Done)
	assert.Equal(
		t, []string{"Lease"}, written, "a failed restore wrote to the cluster it left suspended",
	)
	assert.Equal(t, len(w.restore.Status.PrimaryJobNames), w.jobsLeft(t))
	assert.Empty(t, w.withdrawals())
	assert.Empty(t, leaseHolder(t, w.client))
}

// The claim is what tells the next operation that the cluster is free. A step
// that failed leaves it in place, so the branch runs again on the next look
// instead of handing over a cluster whose volumes are still held.
func TestFinishKeepsTheClaimWhenAStepFails(t *testing.T) {
	t.Parallel()

	unreadable := errors.New("the API server does not answer")
	w := newTerminalWorld(t, terminalOptions{
		reason: v1.ReasonCompleted, jobReadError: unreadable,
	})

	outcome, _, err := w.look(t)
	require.ErrorIs(t, err, unreadable)

	assert.False(t, outcome.Done)
	assert.Empty(t, w.withdrawals())
	assert.Equal(t, selfClaimant().String(), leaseHolder(t, w.client))
}
