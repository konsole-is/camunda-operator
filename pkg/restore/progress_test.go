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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// The pace of a held restore in these cases.
const (
	testGrace = 10 * time.Minute
	testPoll  = 5 * time.Second
)

// restoreOwner is the resource that the driver stages conditions on. Every
// restore kind embeds v1.RestoreProgress in its status, so one kind stands for
// all of them here.
func restoreOwner(generation int64) *v1.PointInTimeRestore {
	return &v1.PointInTimeRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-cluster-pitr", Namespace: "ns", Generation: generation,
		},
	}
}

func unreachable() *conditions.PreCheckFailure {
	return &conditions.PreCheckFailure{
		Reason:  v1.ReasonConnectionFailed,
		Message: "the database does not answer",
	}
}

func ready(owner *v1.PointInTimeRestore) *metav1.Condition {
	return meta.FindStatusCondition(owner.Status.Conditions, v1.ConditionReady)
}

// The first failure of a started restore starts the clock of the mid-run
// grace, and the restore holds at the pace of a running phase.
func TestHoldRunningStartsTheClockAndHolds(t *testing.T) {
	t.Parallel()

	owner := restoreOwner(3)
	now := metav1.Now()

	outcome := HoldRunning(owner, &owner.Status.RestoreProgress, unreachable(), now, testGrace, testPoll)

	assert.Equal(t, Outcome{Wait: testPoll}, outcome)
	require.NotNil(t, owner.Status.FirstFailedAt)
	assert.Equal(t, now, *owner.Status.FirstFailedAt)

	condition := ready(owner)
	require.NotNil(t, condition)
	assert.Equal(t, metav1.ConditionFalse, condition.Status)
	assert.Equal(t, v1.ReasonConnectionFailed, condition.Reason)
	assert.Equal(t, "the database does not answer", condition.Message)
	assert.Equal(t, int64(3), condition.ObservedGeneration)
}

// The clock counts from the first failure, not from the last one. A
// dependency that keeps failing does not push the deadline away.
func TestHoldRunningKeepsTheClockInsideTheGrace(t *testing.T) {
	t.Parallel()

	owner := restoreOwner(1)
	started := metav1.NewTime(time.Now().Add(-time.Minute))
	owner.Status.FirstFailedAt = &started

	outcome := HoldRunning(
		owner, &owner.Status.RestoreProgress, unreachable(), metav1.Now(), testGrace, testPoll,
	)

	assert.Equal(t, Outcome{Wait: testPoll}, outcome)
	assert.Equal(t, &started, owner.Status.FirstFailedAt)
}

// A restore that already erased a broker volume has nothing to go back to, so
// it must reach a terminal phase instead of holding for ever.
func TestHoldRunningFailsPastTheGrace(t *testing.T) {
	t.Parallel()

	owner := restoreOwner(1)
	started := metav1.NewTime(time.Now().Add(-time.Hour))
	owner.Status.FirstFailedAt = &started

	outcome := HoldRunning(
		owner, &owner.Status.RestoreProgress, unreachable(), metav1.Now(), testGrace, testPoll,
	)

	require.NotNil(t, outcome.Failure)
	assert.Equal(t, v1.ReasonFailed, outcome.Failure.Reason)
	assert.Contains(t, outcome.Failure.Message, "did not recover")
	assert.Contains(t, outcome.Failure.Message, "the database does not answer")
	assert.Zero(t, outcome.Wait)
	assert.False(t, outcome.Done)
}

// Recovered on a restore that never failed changes nothing at all.
func TestRecoveredLeavesAProgressThatNeverFailed(t *testing.T) {
	t.Parallel()

	var progress v1.RestoreProgress
	Recovered(&progress)

	assert.Equal(t, v1.RestoreProgress{}, progress)
}

// A restore that has not erased anything gets the full grace again once it
// makes progress: it has nothing to lose while it waits.
func TestRecoveredClearsTheClockBeforeTheRestoreStarts(t *testing.T) {
	t.Parallel()

	failed := metav1.Now()
	progress := v1.RestoreProgress{FirstFailedAt: &failed}

	Recovered(&progress)

	assert.Nil(t, progress.FirstFailedAt)
}

// A started restore keeps its clock. A dependency that flaps in and out would
// reset the grace on every pass otherwise, and a restore that already erased a
// broker volume would hold for ever.
func TestRecoveredKeepsTheClockOfAStartedRestore(t *testing.T) {
	t.Parallel()

	failed := metav1.Now()
	for name, progress := range map[string]v1.RestoreProgress{
		"a volume was erased": {RecreatedClaims: []string{"data-my-cluster-zeebe-0"}},
		"a Job was recorded":  {PrimaryJobNames: []string{"my-cluster-pitr-pitr-0"}},
	} {
		t.Run(name, func(t *testing.T) {
			progress.FirstFailedAt = &failed

			Recovered(&progress)

			assert.Equal(t, &failed, progress.FirstFailedAt)
		})
	}
}

func TestCompleteRecordsTheTerminalSuccess(t *testing.T) {
	t.Parallel()

	var progress v1.RestoreProgress
	now := metav1.Now()

	Complete(&progress, now)

	assert.Equal(t, &now, progress.CompletionTime)
	assert.Equal(t, v1.ReasonCompleted, progress.TerminalReason)
	assert.Empty(t, progress.FailureMessage)
}

func TestFailRecordsTheReasonAndTheMessage(t *testing.T) {
	t.Parallel()

	var progress v1.RestoreProgress
	now := metav1.Now()

	Fail(&progress, v1.ReasonIncompatibleTarget, "the partition counts differ", now)

	assert.Equal(t, &now, progress.CompletionTime)
	assert.Equal(t, v1.ReasonIncompatibleTarget, progress.TerminalReason)
	assert.Equal(t, "the partition counts differ", progress.FailureMessage)
}

// The message carries an external error whose size the operator cannot know,
// for example the reason of a Job or of a pod. The status field is free form,
// so the message is bounded before it reaches it.
func TestFailBoundsTheMessage(t *testing.T) {
	t.Parallel()

	var progress v1.RestoreProgress

	Fail(&progress, v1.ReasonFailed, strings.Repeat("x", conditions.MaxMessageLength+1), metav1.Now())

	assert.LessOrEqual(t, len(progress.FailureMessage), conditions.MaxMessageLength)
	assert.Contains(t, progress.FailureMessage, "truncated")
}

// A write conflict on the terminal flush can restore a stale Ready from the
// server. Every look of a terminal restore stages the recorded reason again,
// so the weaker condition cannot survive.
func TestStageTerminalStagesTheRecordedOutcome(t *testing.T) {
	t.Parallel()

	completed := restoreOwner(4)
	Complete(&completed.Status.RestoreProgress, metav1.Now())
	StageTerminal(completed, &completed.Status.RestoreProgress)

	condition := ready(completed)
	require.NotNil(t, condition)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, v1.ReasonCompleted, condition.Reason)
	assert.Contains(t, condition.Message, "unsuspended")
	assert.Equal(t, int64(4), completed.Status.ObservedGeneration)

	failed := restoreOwner(2)
	Fail(&failed.Status.RestoreProgress, v1.ReasonIncompatibleTarget, "the versions differ", metav1.Now())
	StageTerminal(failed, &failed.Status.RestoreProgress)

	condition = ready(failed)
	require.NotNil(t, condition)
	assert.Equal(t, metav1.ConditionFalse, condition.Status)
	assert.Equal(t, v1.ReasonIncompatibleTarget, condition.Reason)
	assert.Equal(t, "the versions differ", condition.Message)
}

// A restore that reached a terminal phase without a recorded reason still
// reports a failure, because Completed is the only outcome that records
// nothing else.
func TestStageTerminalFallsBackToFailed(t *testing.T) {
	t.Parallel()

	owner := restoreOwner(1)
	StageTerminal(owner, &owner.Status.RestoreProgress)

	condition := ready(owner)
	require.NotNil(t, condition)
	assert.Equal(t, metav1.ConditionFalse, condition.Status)
	assert.Equal(t, v1.ReasonFailed, condition.Reason)
}
