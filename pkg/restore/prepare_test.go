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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// testPrepPoll paces a preparation step that waits for the cluster.
const testPrepPoll = 3 * time.Second

// clusterUID is the identity that every apply of a preparation step carries
// as its precondition.
const clusterUID = types.UID("cluster-uid")

// applied is one apply that a preparation step made.
type applied struct {
	manager string
	cluster *v1.CamundaCluster
}

// prepareWorld is one case of the preparation step: the restore, the cluster
// it prepares, the target it reads, and the applies it made.
type prepareWorld struct {
	restore *v1.PointInTimeRestore
	cluster *v1.CamundaCluster
	client  client.Client
	input   PrepareInput
	applies *[]applied
}

// newPrepareWorld builds a suspended cluster whose brokers already stopped
// and that runs the version of its backup. Every case moves one fact of it.
func newPrepareWorld(t *testing.T) *prepareWorld {
	t.Helper()

	cluster := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cluster", Namespace: "ns", UID: clusterUID},
		Spec:       v1.CamundaClusterSpec{Version: "8.9.9", Suspend: true},
	}

	applies := &[]applied{}
	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(cluster).
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(
				ctx context.Context,
				cl client.WithWatch,
				obj client.Object,
				patch client.Patch,
				opts ...client.PatchOption,
			) error {
				var options client.PatchOptions
				options.ApplyOptions(opts)
				if patched, ok := obj.(*v1.CamundaCluster); ok {
					*applies = append(*applies, applied{
						manager: options.FieldManager, cluster: patched.DeepCopy(),
					})
				}

				return cl.Patch(ctx, obj, patch, opts...)
			},
		}).
		Build()

	target := targetFixture()
	target.StatefulSet.Status.Replicas = 0
	owner := restoreOwner(1)

	return &prepareWorld{
		restore: owner,
		cluster: cluster,
		client:  c,
		applies: applies,
		input: PrepareInput{
			Owner:   owner,
			Cluster: cluster,
			Target:  target,
			Version: "8.9.9",
			Poll:    testPrepPoll,
		},
	}
}

// look runs one reconcile of the preparation step.
func (w *prepareWorld) look(t *testing.T) Outcome {
	t.Helper()

	outcome, err := Prepare(t.Context(), w.client, &w.restore.Status.RestoreProgress, w.input)
	require.NoError(t, err)

	return outcome
}

// progress returns the record that the step reads and writes.
func (w *prepareWorld) progress() *v1.RestoreProgress {
	return &w.restore.Status.RestoreProgress
}

// unsuspended makes the cluster of the world one that still runs, in the
// store and in the copy that the step reads.
func (w *prepareWorld) unsuspended(t *testing.T) {
	t.Helper()

	w.cluster.Spec.Suspend = false
	require.NoError(t, w.client.Update(t.Context(), w.cluster))
}

// live reads the cluster as it is now.
func (w *prepareWorld) live(t *testing.T) *v1.CamundaCluster {
	t.Helper()

	var current v1.CamundaCluster
	key := client.ObjectKeyFromObject(w.cluster)
	require.NoError(t, w.client.Get(t.Context(), key, &current))

	return &current
}

// The names appear in the managedFields of every cluster that a restore
// prepares. A GitOps tool and the layer above this operator both read them
// there, so they are API surface.
func TestTargetFieldManagersAreStable(t *testing.T) {
	t.Parallel()

	assert.Equal(t, client.FieldOwner("camunda-operator/restore-suspend"), FieldManagerTargetSuspend)
	assert.Equal(t, client.FieldOwner("camunda-operator/restore-version"), FieldManagerTargetVersion)
}

// A crash between the write and the record would leave a suspended cluster
// that nothing ever unsuspends again, so the record comes first and the write
// waits for the look that follows its flush.
func TestPrepareRecordsTheSuspensionBeforeItWritesIt(t *testing.T) {
	t.Parallel()

	w := newPrepareWorld(t)
	w.unsuspended(t)

	outcome := w.look(t)

	assert.False(t, outcome.Done)
	assert.True(t, w.progress().ClusterSuspended)
	assert.Empty(t, *w.applies, "the step wrote before its record was durable")
	assert.False(t, w.live(t).Spec.Suspend)
}

func TestPrepareSuspendsTheClusterOnTheLookAfterTheRecord(t *testing.T) {
	t.Parallel()

	w := newPrepareWorld(t)
	w.unsuspended(t)

	w.look(t)
	outcome := w.look(t)

	assert.False(t, outcome.Done)
	assert.Equal(t, testPrepPoll, outcome.Wait)
	require.Len(t, *w.applies, 1)
	assert.Equal(t, string(FieldManagerTargetSuspend), (*w.applies)[0].manager)
	assert.True(t, (*w.applies)[0].cluster.Spec.Suspend)
	assert.Equal(t, clusterUID, (*w.applies)[0].cluster.UID, "the apply carries no precondition")
	assert.True(t, w.live(t).Spec.Suspend)
}

// A cluster that its owner suspended keeps no record of this restore, so the
// restore leaves it suspended when it finishes.
func TestPrepareRecordsNothingForAClusterThatIsAlreadySuspended(t *testing.T) {
	t.Parallel()

	w := newPrepareWorld(t)

	outcome := w.look(t)

	assert.True(t, outcome.Done)
	assert.False(t, w.progress().ClusterSuspended)
	assert.Empty(t, *w.applies)
}

// spec.suspend says what was asked for. The StatefulSet says what happened. A
// version that reaches the brokers while they still run is the downgrade of a
// running cluster that the whole order exists to avoid.
func TestPrepareWaitsUntilTheBrokersAreGone(t *testing.T) {
	t.Parallel()

	w := newPrepareWorld(t)
	w.input.Target.StatefulSet.Status.Replicas = 2
	w.input.Version = "8.9.8"

	outcome := w.look(t)

	assert.False(t, outcome.Done)
	assert.Equal(t, testPrepPoll, outcome.Wait)
	assert.Empty(t, *w.applies, "the step wrote the version while the brokers still ran")
	assert.Contains(t, ready(w.restore).Message, "2 of its brokers still run")
}

func TestPrepareWritesTheVersionOfTheBackup(t *testing.T) {
	t.Parallel()

	w := newPrepareWorld(t)
	w.input.Version = "8.9.8"

	outcome := w.look(t)

	assert.False(t, outcome.Done, "the brokers still carry the old version")
	require.Len(t, *w.applies, 1)
	assert.Equal(t, string(FieldManagerTargetVersion), (*w.applies)[0].manager)
	assert.Equal(t, "8.9.8", (*w.applies)[0].cluster.Spec.Version)
	assert.False(t, (*w.applies)[0].cluster.Spec.Suspend, "the version apply also claims spec.suspend")
	assert.Equal(t, "8.9.8", w.live(t).Spec.Version)
}

// The step is done only when spec.version and the tag of the broker image
// both carry the version of the backup.
func TestPrepareIsDoneWhenTheSpecAndTheImageBothCarryTheVersion(t *testing.T) {
	t.Parallel()

	w := newPrepareWorld(t)

	outcome := w.look(t)

	assert.True(t, outcome.Done)
	assert.Empty(t, *w.applies)
}

// A cluster that takes its version from a preset carries no spec.version at
// all. The restore writes the field, so the cluster keeps the version of the
// backup once the restore is over.
func TestPrepareWritesTheVersionOfAClusterThatDeclaresNone(t *testing.T) {
	t.Parallel()

	w := newPrepareWorld(t)
	w.cluster.Spec.Version = ""
	require.NoError(t, w.client.Update(t.Context(), w.cluster))

	outcome := w.look(t)

	assert.False(t, outcome.Done)
	require.Len(t, *w.applies, 1)
	assert.Equal(t, "8.9.9", (*w.applies)[0].cluster.Spec.Version)
}

// The write is made once. A step that applied the same value on every poll
// would write to the cluster for as long as the rollout takes.
func TestPrepareWaitsForTheImageWithoutWritingAgain(t *testing.T) {
	t.Parallel()

	w := newPrepareWorld(t)
	w.input.Target.Version = "8.10.0"

	outcome := w.look(t)

	assert.False(t, outcome.Done)
	assert.Equal(t, testPrepPoll, outcome.Wait)
	assert.Empty(t, *w.applies, "the step wrote a version that the cluster already declares")
	assert.Contains(t, ready(w.restore).Message, "brokers carry 8.10.0")
}

// A cluster part way through an upgrade declares the newer version and still
// runs the older image. Reading the image alone would call it converged, and
// the cluster controller would then roll the newer image in under the
// restore.
func TestPrepareWritesTheVersionOfAClusterThatIsMidUpgrade(t *testing.T) {
	t.Parallel()

	w := newPrepareWorld(t)
	w.cluster.Spec.Version = "8.10.0"
	require.NoError(t, w.client.Update(t.Context(), w.cluster))
	w.input.Target.Version = "8.9.9"
	w.input.Version = "8.9.9"

	outcome := w.look(t)

	assert.False(t, outcome.Done, "the cluster still declares the version it was upgrading to")
	require.Len(t, *w.applies, 1)
	assert.Equal(t, "8.9.9", (*w.applies)[0].cluster.Spec.Version)
}

// A backup that recorded no version, and one whose recorded version is not a
// version, name nothing that the restore can write. The version rule of the
// restore kind reports what such a backup means.
func TestPrepareWritesNoVersionItCannotWrite(t *testing.T) {
	t.Parallel()

	for name, version := range map[string]string{
		"no recorded version": "",
		"not a version":       "latest",
		"not three segments":  "8.9",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			w := newPrepareWorld(t)
			w.input.Version = version

			outcome := w.look(t)

			assert.True(t, outcome.Done)
			assert.Empty(t, *w.applies)
		})
	}
}

// Every entry point that renders from a Target rejects an incomplete one. A
// nil target takes the manager down inside a reconcile.
func TestPrepareRejectsAnIncompleteTarget(t *testing.T) {
	t.Parallel()

	w := newPrepareWorld(t)
	w.input.Target = nil

	outcome := w.look(t)

	require.NotNil(t, outcome.Failure)
	assert.Equal(t, v1.ReasonFailed, outcome.Failure.Reason)
	assert.Contains(t, outcome.Failure.Message, "cannot prepare its cluster")
}

// A restore that did not suspend its cluster withdraws nothing. That is what
// keeps a cluster suspended that its owner suspended.
func TestResumeWritesNothingWithoutTheRecord(t *testing.T) {
	t.Parallel()

	w := newPrepareWorld(t)

	require.NoError(t, Resume(
		t.Context(), w.client, w.client, w.progress(), client.ObjectKeyFromObject(w.cluster),
	))

	assert.Empty(t, *w.applies)
	assert.True(t, w.live(t).Spec.Suspend)
}

// The withdrawal applies an object without spec.suspend, so server-side apply
// removes the field that this manager owns.
func TestResumeWithdrawsTheSuspensionItApplied(t *testing.T) {
	t.Parallel()

	w := newPrepareWorld(t)
	w.progress().ClusterSuspended = true

	require.NoError(t, Resume(
		t.Context(), w.client, w.client, w.progress(), client.ObjectKeyFromObject(w.cluster),
	))

	require.Len(t, *w.applies, 1)
	assert.Equal(t, string(FieldManagerTargetSuspend), (*w.applies)[0].manager)
	assert.False(t, (*w.applies)[0].cluster.Spec.Suspend)
	assert.Equal(t, clusterUID, (*w.applies)[0].cluster.UID)
}

// An apply against a cluster that is gone would put an empty CamundaCluster
// in its place, so a cluster that no longer exists needs no withdrawal.
func TestResumeWritesNothingForAClusterThatIsGone(t *testing.T) {
	t.Parallel()

	w := newPrepareWorld(t)
	w.progress().ClusterSuspended = true
	require.NoError(t, w.client.Delete(t.Context(), w.cluster))

	require.NoError(t, Resume(
		t.Context(), w.client, w.client, w.progress(),
		types.NamespacedName{Namespace: "ns", Name: "my-cluster"},
	))

	assert.Empty(t, *w.applies)
}
