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

// terminalWorld is one look of the terminal branch: the restore that ended,
// the cluster it suspended, the Jobs it ran, and the order in which the
// releases reached the API server.
type terminalWorld struct {
	restore *v1.PointInTimeRestore
	cluster *v1.CamundaCluster
	client  client.Client
	touched *[]string
	applies *[]applied
}

// newTerminalWorld builds a restore that ended with reason, holding
// everything a restore can hold: three Jobs, a suspended cluster it recorded
// as its own, and the claim on that cluster.
func newTerminalWorld(t *testing.T, reason string, extra ...interceptor.Funcs) *terminalWorld {
	t.Helper()

	owner := terminalOwner(reason)
	owner.Status.ClusterSuspended = true

	cluster := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cluster", Namespace: "ns", UID: clusterUID},
		Spec:       v1.CamundaClusterSpec{Suspend: true},
	}

	objects := make([]client.Object, 0, len(owner.Status.PrimaryJobNames)+1)
	objects = append(objects, cluster)
	for _, name := range owner.Status.PrimaryJobNames {
		objects = append(objects, recordedJob(name, owner.UID))
	}

	touched := &[]string{}
	applies := &[]applied{}
	funcs := interceptor.Funcs{
		Delete: func(
			ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption,
		) error {
			*touched = append(*touched, touch(obj))

			return c.Delete(ctx, obj, opts...)
		},
		Patch: func(
			ctx context.Context,
			c client.WithWatch,
			obj client.Object,
			patch client.Patch,
			opts ...client.PatchOption,
		) error {
			*touched = append(*touched, touch(obj))
			if patched, isCluster := obj.(*v1.CamundaCluster); isCluster {
				var options client.PatchOptions
				options.ApplyOptions(opts)
				*applies = append(*applies, applied{
					manager: options.FieldManager, cluster: patched.DeepCopy(),
				})
			}

			return c.Patch(ctx, obj, patch, opts...)
		},
	}
	if len(extra) == 1 {
		funcs.Get = extra[0].Get
	}

	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(objects...).
		WithInterceptorFuncs(funcs).
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

func (w *terminalWorld) finish(t *testing.T) error {
	t.Helper()

	return Finish(
		t.Context(), w.client, w.client, w.restore, &w.restore.Status.RestoreProgress, "my-cluster",
	)
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
// outcome it recorded.
func TestFinishReleasesEverythingOfACompletedRestore(t *testing.T) {
	t.Parallel()

	w := newTerminalWorld(t, v1.ReasonCompleted)

	require.NoError(t, w.finish(t))

	assert.Zero(t, w.jobsLeft(t), "the Jobs of a completed restore hold its broker volumes")
	assert.Len(t, w.withdrawals(), 1, "the restore left the cluster it suspended suspended")
	assert.Empty(t, leaseHolder(t, w.client), "the cluster is not free for the next operation")

	condition := ready(w.restore)
	require.NotNil(t, condition)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, v1.ReasonCompleted, condition.Reason)
}

// The order is the correctness of this branch. The Jobs hold the broker
// volumes, so they go before the brokers are allowed to ask for one, and both
// go before the cluster is reported free.
func TestFinishFreesTheVolumesBeforeItFreesTheCluster(t *testing.T) {
	t.Parallel()

	w := newTerminalWorld(t, v1.ReasonCompleted)

	require.NoError(t, w.finish(t))

	assert.Equal(
		t,
		[]string{"Job", "Job", "Job", "CamundaCluster", "Lease"},
		*w.touched,
		"a broker that starts before its volume is free cannot attach it",
	)
}

// A failed restore keeps its Jobs, because their logs are the diagnosis, and
// it keeps the cluster suspended, because the broker volumes can be half
// written. The claim goes back either way: the restore is over.
func TestFinishKeepsTheJobsAndTheSuspensionOfAFailedRestore(t *testing.T) {
	t.Parallel()

	w := newTerminalWorld(t, v1.ReasonFailed)

	require.NoError(t, w.finish(t))

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
	w := newTerminalWorld(t, v1.ReasonCompleted, interceptor.Funcs{
		Get: func(
			ctx context.Context,
			c client.WithWatch,
			key client.ObjectKey,
			obj client.Object,
			opts ...client.GetOption,
		) error {
			if _, isJob := obj.(*batchv1.Job); isJob {
				return unreadable
			}

			return c.Get(ctx, key, obj, opts...)
		},
	})

	require.ErrorIs(t, w.finish(t), unreadable)

	assert.Empty(t, w.withdrawals())
	assert.Equal(t, selfClaimant().String(), leaseHolder(t, w.client))
}
