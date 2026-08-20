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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// primaryWorld is one case of the primary-storage phase: the restore, its
// progress, the client that holds the cluster, and the events the phase
// emitted.
type primaryWorld struct {
	restore  *v1.PointInTimeRestore
	client   client.Client
	scheme   *runtime.Scheme
	recorder *events.FakeRecorder
	input    PrimaryInput
}

// newPrimaryWorld builds the state that the phase starts from: three broker
// volumes that still hold the data of the cluster.
func newPrimaryWorld(t *testing.T) *primaryWorld {
	t.Helper()

	target := targetFixture()
	objects := make([]client.Object, 0, target.Brokers)
	for _, name := range target.ClaimNames() {
		objects = append(objects, existingClaim(name))
	}

	scheme := testScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
	owner := restoreOwner(1)
	recorder := events.NewFakeRecorder(16)

	return &primaryWorld{
		restore:  owner,
		client:   c,
		scheme:   scheme,
		recorder: recorder,
		input: PrimaryInput{
			Owner:        owner,
			OwnerLabel:   labels.PointInTimeRestore(restoreName),
			Target:       target,
			Size:         resource.MustParse("10Gi"),
			FieldManager: FieldManagerPointInTimeRestore,
			Recorder:     recorder,
			Args:         []string{restoreArg},
			Poll:         testPoll,
			Grace:        testGrace,
		},
	}
}

// look runs one reconcile of the primary-storage phase.
func (w *primaryWorld) look(t *testing.T) Outcome {
	t.Helper()

	outcome, err := Primary(
		t.Context(), w.client, w.client, w.scheme, &w.restore.Status.RestoreProgress, w.input,
	)
	require.NoError(t, err)

	return outcome
}

// progress returns the progress that the phase reads and writes.
func (w *primaryWorld) progress() *v1.RestoreProgress {
	return &w.restore.Status.RestoreProgress
}

// jobs returns the restore Jobs that exist now, by name.
func (w *primaryWorld) jobs(t *testing.T) map[string]batchv1.Job {
	t.Helper()

	var list batchv1.JobList
	require.NoError(t, w.client.List(t.Context(), &list, client.InNamespace("ns")))

	byName := make(map[string]batchv1.Job, len(list.Items))
	for _, job := range list.Items {
		byName[job.Name] = job
	}

	return byName
}

// claims returns the broker data volumes that exist now, by name.
func (w *primaryWorld) claims(t *testing.T) map[string]corev1.PersistentVolumeClaim {
	t.Helper()

	var list corev1.PersistentVolumeClaimList
	require.NoError(t, w.client.List(t.Context(), &list, client.InNamespace("ns")))

	byName := make(map[string]corev1.PersistentVolumeClaim, len(list.Items))
	for _, claim := range list.Items {
		byName[claim.Name] = claim
	}

	return byName
}

// runToTheJobs looks until the phase applied the restore Jobs.
func (w *primaryWorld) runToTheJobs(t *testing.T) {
	t.Helper()

	for range 4 {
		if len(w.jobs(t)) == int(w.input.Target.Brokers) {
			return
		}
		w.look(t)
	}

	require.Len(t, w.jobs(t), int(w.input.Target.Brokers), "the phase never applied its Jobs")
}

// finish writes onto a Job the bookkeeping of a finished one. No Job
// controller runs against a fake client, so the case plays it.
func finish(t *testing.T, c client.Client, name string, kind batchv1.JobConditionType) {
	t.Helper()

	var job batchv1.Job
	key := types.NamespacedName{Namespace: "ns", Name: name}
	require.NoError(t, c.Get(t.Context(), key, &job))

	now := metav1.Now()
	precursor := batchv1.JobSuccessCriteriaMet
	if kind == batchv1.JobFailed {
		precursor = batchv1.JobFailureTarget
	}
	job.Status.StartTime = &now
	job.Status.Conditions = []batchv1.JobCondition{
		{Type: precursor, Status: corev1.ConditionTrue, Reason: "Test", Message: "written by the test"},
		{Type: kind, Status: corev1.ConditionTrue, Reason: "Test", Message: "written by the test"},
	}
	if kind == batchv1.JobComplete {
		job.Status.Succeeded = 1
		job.Status.CompletionTime = &now
	}
	require.NoError(t, c.Status().Update(t.Context(), &job))
}

// ownedJob is the restore Job of one broker as this restore created it. A
// case that starts with Jobs in place uses it.
func ownedJob(t *testing.T, w *primaryWorld, ordinal int32) *batchv1.Job {
	t.Helper()

	in := JobInput{
		Target:     w.input.Target,
		Owner:      w.restore,
		OwnerLabel: w.input.OwnerLabel,
		Ordinal:    ordinal,
		Args:       w.input.Args,
	}
	job, err := BuildJob(in)
	require.NoError(t, err)
	require.NoError(t, controllerutil.SetControllerReference(w.restore, job, w.scheme))

	return job
}

// The StatefulSet owns the broker volumes. An owner reference to the restore
// deletes a live broker volume as soon as somebody deletes the restore.
func TestPrimaryRecreatesTheClaimsWithoutAnOwnerReference(t *testing.T) {
	t.Parallel()

	w := newPrimaryWorld(t)
	w.look(t)
	w.look(t)

	claims := w.claims(t)
	require.Len(t, claims, 3)
	for name, claim := range claims {
		assert.Empty(t, claim.OwnerReferences, name)
		assert.Equal(t, resource.MustParse("10Gi"), claim.Spec.Resources.Requests[corev1.ResourceStorage])
	}
}

// The count is read from the live StatefulSet and pinned before the phase
// deletes anything, so the status says how many volumes and Jobs this restore
// covers even when it fails halfway.
func TestPrimaryRecordsTheBrokerCountBeforeItDeletesAnything(t *testing.T) {
	t.Parallel()

	w := newPrimaryWorld(t)
	w.look(t)

	assert.Equal(t, int32(3), w.progress().Brokers)
}

// A volume whose deletion is not recorded is deleted again by the next look.
// That costs a pass while nothing runs, and it erases restored data once a Job
// writes, so the record covers every volume the phase touched.
func TestPrimaryRecordsEveryClaimBeforeItDeletesIt(t *testing.T) {
	t.Parallel()

	w := newPrimaryWorld(t)

	outcome := w.look(t)

	assert.Equal(t, Outcome{Wait: testPoll}, outcome)
	assert.Equal(t, w.input.Target.ClaimNames(), w.progress().RecreatedClaims)
	assert.Empty(t, w.claims(t), "every volume that the record names is gone")
	assert.Empty(t, w.jobs(t), "no Job runs before the volumes are empty")
}

// The names are derived from the restore and the ordinal, so recording them
// before the first Job exists claims exactly the Jobs that the next look
// applies.
func TestPrimaryRecordsEveryJobNameBeforeItAppliesAJob(t *testing.T) {
	t.Parallel()

	w := newPrimaryWorld(t)
	w.look(t)

	outcome := w.look(t)

	assert.Equal(t, Outcome{Wait: Shortly}, outcome)
	assert.Equal(
		t,
		[]string{"my-cluster-pitr-pitr-0", "my-cluster-pitr-pitr-1", "my-cluster-pitr-pitr-2"},
		w.progress().PrimaryJobNames,
	)
	assert.Empty(t, w.jobs(t), "the record is durable before a Job exists")
	assert.Len(t, w.claims(t), 3, "the volumes are back and empty")
}

// The per-broker event is the only signal a user gets while the restore
// application runs.
func TestPrimaryEmitsOneEventForEachBrokerThatStarts(t *testing.T) {
	t.Parallel()

	w := newPrimaryWorld(t)
	w.runToTheJobs(t)

	seen := make([]string, 0, 3)
	for range 3 {
		select {
		case event := <-w.recorder.Events:
			seen = append(seen, event)
		default:
			require.Fail(t, "the phase emitted fewer events than it started brokers")
		}
	}

	assert.Contains(t, seen[0], "broker 0")
	assert.Contains(t, seen[1], "broker 1")
	assert.Contains(t, seen[2], "broker 2")
	for _, event := range seen {
		assert.Contains(t, event, EventReasonStarted)
	}
}

// The restore acts on the brokers it recorded. A cluster that lost a broker
// while the restore waited cannot run that work, and the render error
// underneath names only the counts, so the phase names the real cause.
func TestPrimaryRefusesAnOrdinalPastTheLiveBrokerCount(t *testing.T) {
	t.Parallel()

	w := newPrimaryWorld(t)
	w.look(t)
	w.look(t)
	require.Len(t, w.progress().PrimaryJobNames, 3)

	// The live StatefulSet runs one broker now. The pinned count stays three.
	w.input.Target = targetFixture()
	w.input.Target.Brokers = 1

	outcome := w.look(t)

	require.NotNil(t, outcome.Failure)
	assert.Equal(t, v1.ReasonFailed, outcome.Failure.Reason)
	assert.Contains(t, outcome.Failure.Message, "resized")
	assert.Contains(t, outcome.Failure.Message, "1 brokers")
	assert.Contains(t, outcome.Failure.Message, "recorded 3")
}

// A second Job of one broker runs the restore application on a volume that
// the first one already wrote. The phase reports the removal instead, because
// nothing resolves it on its own.
func TestPrimaryFailsWhenARecordedJobIsGone(t *testing.T) {
	t.Parallel()

	w := newPrimaryWorld(t)
	w.runToTheJobs(t)

	gone := w.progress().PrimaryJobNames[0]
	var job batchv1.Job
	require.NoError(t, w.client.Get(
		t.Context(), types.NamespacedName{Namespace: "ns", Name: gone}, &job,
	))
	require.NoError(t, w.client.Delete(t.Context(), &job))

	outcome := w.look(t)

	require.NotNil(t, outcome.Failure)
	assert.Equal(t, v1.ReasonFailed, outcome.Failure.Reason)
	assert.Contains(t, outcome.Failure.Message, gone)
	assert.Contains(t, outcome.Failure.Message, "broker 0")
}

// A restore that crashed between two creates finds a prefix of its Jobs. The
// Jobs that are missing behind it were never created, so the phase creates
// them instead of reporting them gone.
func TestPrimaryCreatesTheJobsThatFollowTheLastOneItApplied(t *testing.T) {
	t.Parallel()

	w := newPrimaryWorld(t)
	w.look(t)
	w.look(t)
	require.Len(t, w.progress().PrimaryJobNames, 3)
	require.NoError(t, w.client.Create(t.Context(), ownedJob(t, w, 0)))

	outcome := w.look(t)

	assert.Equal(t, Outcome{Wait: testPoll}, outcome)
	assert.Len(t, w.jobs(t), 3)
}

// The restore is over when every broker restored, and not before.
func TestPrimaryIsDoneWhenEveryJobCompleted(t *testing.T) {
	t.Parallel()

	w := newPrimaryWorld(t)
	w.runToTheJobs(t)

	finish(t, w.client, w.progress().PrimaryJobNames[0], batchv1.JobComplete)
	finish(t, w.client, w.progress().PrimaryJobNames[1], batchv1.JobComplete)

	assert.Equal(t, Outcome{Wait: testPoll}, w.look(t), "one broker is still restoring")

	finish(t, w.client, w.progress().PrimaryJobNames[2], batchv1.JobComplete)

	assert.Equal(t, Outcome{Done: true}, w.look(t))
}

// One failing Job fails the restore and names its broker: the partitions of
// that broker are not restored, and a second attempt needs empty volumes
// again, which only a new restore arranges.
func TestPrimaryFailsWhenABrokerCannotRestore(t *testing.T) {
	t.Parallel()

	w := newPrimaryWorld(t)
	w.runToTheJobs(t)

	finish(t, w.client, w.progress().PrimaryJobNames[1], batchv1.JobFailed)

	outcome := w.look(t)

	require.NotNil(t, outcome.Failure)
	assert.Contains(t, outcome.Failure.Message, "broker 1")
}

// The name of a Job comes from the name of the restore and the ordinal, so a
// restore that somebody deleted and created again under one name finds the
// Jobs of its predecessor. Only the controller reference proves that a Job is
// this restore's own.
func TestPrimaryRefusesAJobThatAnotherRestoreOwns(t *testing.T) {
	t.Parallel()

	w := newPrimaryWorld(t)
	w.look(t)
	w.look(t)

	foreign := ownedJob(t, w, 0)
	foreign.OwnerReferences = nil
	require.NoError(t, w.client.Create(t.Context(), foreign))

	outcome := w.look(t)

	require.NotNil(t, outcome.Failure)
	assert.Contains(t, outcome.Failure.Message, foreign.Name)
	assert.Contains(t, outcome.Failure.Message, "Remove the Job of the earlier restore")
}

// The recorded name and the derived name are one truth. A restore that polls
// for a Job whose name it never derives waits for ever, so the mismatch ends
// it instead.
func TestPrimaryFailsWhenARecordedNameIsNotTheNameOfThatJob(t *testing.T) {
	t.Parallel()

	w := newPrimaryWorld(t)
	w.look(t)
	w.look(t)
	w.progress().PrimaryJobNames[0] = "not-the-name-of-this-job"

	outcome := w.look(t)

	require.NotNil(t, outcome.Failure)
	assert.Contains(t, outcome.Failure.Message, "not-the-name-of-this-job")
}

// A pod that cannot start consumes no retry of its Job, so the Job stays
// active and reports nothing. Without this look the restore waits without a
// bound on a missing Secret or an image that does not pull.
func TestPrimaryHoldsOnAPodThatCannotStart(t *testing.T) {
	t.Parallel()

	w := newPrimaryWorld(t)
	w.runToTheJobs(t)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "restore-pod",
			Namespace: "ns",
			Labels:    JobSelector(w.input.OwnerLabel),
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: ComponentRestore,
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
					Reason:  "CreateContainerConfigError",
					Message: `secret "camunda-credentials" not found`,
				}},
			}},
		},
	}
	require.NoError(t, w.client.Create(t.Context(), pod))

	outcome := w.look(t)

	assert.Equal(t, Outcome{Wait: testPoll}, outcome)
	require.NotNil(t, w.progress().FirstFailedAt, "the mid-run grace runs from here")
	condition := ready(w.restore)
	require.NotNil(t, condition)
	assert.Equal(t, v1.ReasonMissingSecret, condition.Reason)
	assert.Contains(t, condition.Message, pod.Name)
}

// A malformed target is a render failure, not a wait. Nothing changes on its
// own, and the restore already left the volumes behind.
func TestPrimaryFailsOnATargetItCannotRenderFrom(t *testing.T) {
	t.Parallel()

	w := newPrimaryWorld(t)
	w.input.Target.ClaimTemplate = nil

	outcome, err := Primary(
		context.Background(), w.client, w.client, w.scheme, w.progress(), w.input,
	)
	require.NoError(t, err)

	require.NotNil(t, outcome.Failure)
	assert.Contains(t, outcome.Failure.Message, "the broker volumes cannot be recreated")
}

// A Job whose name is free is created, never applied. A read that finds no
// Job and a forced apply after it are two calls, and a Job of an earlier
// restore can land between them.
func TestPrimaryCreatesTheJobsItOwns(t *testing.T) {
	t.Parallel()

	w := newPrimaryWorld(t)
	w.runToTheJobs(t)

	for name, job := range w.jobs(t) {
		require.Len(t, job.OwnerReferences, 1, name)
		assert.Equal(t, w.restore.Name, job.OwnerReferences[0].Name)
		assert.True(t, *job.OwnerReferences[0].Controller)
		assert.Equal(t, restoreArg, job.Spec.Template.Spec.Containers[0].Args[0])
	}

	var gone batchv1.Job
	err := w.client.Get(
		t.Context(), types.NamespacedName{Namespace: "ns", Name: "my-cluster-pitr-pitr-3"}, &gone,
	)
	assert.True(t, apierrors.IsNotFound(err), "the phase runs the brokers it recorded, no more")
}
