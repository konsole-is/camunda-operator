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

package pointintimerestore

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The two per-range lines that the restore point resolver logs while it
// rejects a candidate range. Both were read off a real restore. The resolver
// takes the first range that passes, so a run that prints either of these can
// still select a later range and succeed, and the operator must not read them
// as a refusal.
const (
	skippedLast = "2026-08-20 11:02:33.114 [] [main] WARN  io.camunda.zeebe.restore - " +
		"Skipping range [1, 448] because the last log position 448 is before the required " +
		"exported position 451"
	skippedFirst = "2026-08-20 11:02:33.117 [] [main] WARN  io.camunda.zeebe.restore - " +
		"Skipping range [1, 96] because the first log position OptionalLong[1] is after " +
		"required exported position 0"
)

// noUsableRange is the terminal failure of the resolver. It is thrown only
// when every range of a partition was rejected, so it is the one line that
// proves no range was selected.
const noUsableRange = "java.lang.IllegalStateException: No usable range found for partition 1 " +
	"with from=null, exportedPosition=451. Available ranges: [Range[start=1, end=17]]"

// noUsableRangeWithoutPosition is the same terminal failure on a run that got
// no exporter position. Its cause is another one, and the remedy this operator
// names does not answer it.
const noUsableRangeWithoutPosition = "java.lang.IllegalStateException: No usable range found " +
	"for partition 1 with from=2026-08-20T10:00:00Z, exportedPosition=null. Available ranges: []"

// unrelatedLog is a restore that failed for a cause of its own. Nothing in it
// names an exported position.
const unrelatedLog = "Exception in thread \"main\" java.io.IOException: No space left on device\n" +
	"\tat java.base/java.io.FileOutputStream.write(FileOutputStream.java:349)"

func TestRefusalLineFindsTheTerminalFailure(t *testing.T) {
	t.Parallel()

	output := "starting the restore application\n" + skippedLast + "\n" + noUsableRange + "\n"

	assert.Equal(t, noUsableRange, refusalLine(output))
}

// The resolver logs one line for every range it rejects and then takes the
// first range that passes. A run that prints those lines and then succeeds, or
// fails for another cause, is not this refusal.
func TestRefusalLineIgnoresARangeThatWasOnlySkipped(t *testing.T) {
	t.Parallel()

	for name, output := range map[string]string{
		"a range skipped for its last position":  skippedLast,
		"a range skipped for its first position": skippedFirst,
		"both, and then another failure": skippedFirst + "\n" + skippedLast + "\n" +
			"java.lang.IllegalStateException: Unexpected data gaps between backup 4 and backup 7",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, refusalLine(output))
		})
	}
}

// The terminal message reports the exporter position it was given. A run that
// got none failed for another cause.
func TestRefusalLineIgnoresATerminalFailureWithoutAPosition(t *testing.T) {
	t.Parallel()

	assert.Empty(t, refusalLine(noUsableRangeWithoutPosition))
}

// A restore that failed for another cause carries no marker. Reporting the
// refusal for it would send the user after the wrong remedy.
func TestRefusalLineIgnoresAnUnrelatedFailure(t *testing.T) {
	t.Parallel()

	assert.Empty(t, refusalLine(unrelatedLog))
	assert.Empty(t, refusalLine(""))
}

// The line reaches a status field that a user reads, so it is bounded.
func TestRefusalLineBoundsAVeryLongLine(t *testing.T) {
	t.Parallel()

	line := refusalLine(noUsableRange + strings.Repeat("x", 4000))

	assert.Len(t, line, maxRefusalLine)
	assert.True(t, strings.HasPrefix(line, "java.lang.IllegalStateException"))
}

// failedJob is the Job of one broker that already reported failed.
func failedJob() *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "pitr-1-pitr-1"},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: "BackoffLimitExceeded"},
			},
		},
	}
}

// readerOf returns a log reader that answers output for every Job.
func readerOf(output string) ReadJobLog {
	return func(context.Context, types.NamespacedName) (string, error) {
		return output, nil
	}
}

func TestJobRefusalNamesTheCauseAndTheRemedy(t *testing.T) {
	t.Parallel()

	r := &Reconciler{opts: Options{ReadJobLog: readerOf(noUsableRange)}.withDefaults()}

	failure := r.jobRefusal(t.Context(), failedJob(), 1)

	require.NotNil(t, failure)
	assert.Equal(t, v1.ReasonExporterPositionNotCovered, failure.Reason)
	assert.Contains(t, failure.Message, "broker 1")
	assert.Contains(t, failure.Message, "No usable range found for partition 1")
	assert.Contains(t, failure.Message, "Roll the database back")
}

// A failure of another kind keeps the message that the phase already found,
// which the nil answer leaves in place.
func TestJobRefusalAnswersNothingForAnotherFailure(t *testing.T) {
	t.Parallel()

	r := &Reconciler{opts: Options{ReadJobLog: readerOf(unrelatedLog)}.withDefaults()}

	assert.Nil(t, r.jobRefusal(t.Context(), failedJob(), 0))
}

// A log that the operator cannot read must never change a failure into
// something else.
func TestJobRefusalAnswersNothingWhenTheLogIsUnreadable(t *testing.T) {
	t.Parallel()

	unreadable := func(context.Context, types.NamespacedName) (string, error) {
		return "", errors.New("the pod is gone")
	}
	r := &Reconciler{opts: Options{ReadJobLog: unreadable}.withDefaults()}

	assert.Nil(t, r.jobRefusal(t.Context(), failedJob(), 0))
}

// A Reconciler that never reached SetupWithManager has no reader. It reports
// the generic failure rather than taking the manager down.
func TestJobRefusalAnswersNothingWithoutAReader(t *testing.T) {
	t.Parallel()

	r := &Reconciler{opts: Options{}.withDefaults()}

	assert.Nil(t, r.jobRefusal(t.Context(), failedJob(), 0))
}

// The reader gets the key of the Job that failed, so it reads the log of that
// broker and no other.
func TestJobRefusalReadsTheLogOfTheFailedJob(t *testing.T) {
	t.Parallel()

	var asked types.NamespacedName
	reader := func(_ context.Context, job types.NamespacedName) (string, error) {
		asked = job

		return noUsableRange, nil
	}
	r := &Reconciler{opts: Options{ReadJobLog: reader}.withDefaults()}

	require.NotNil(t, r.jobRefusal(t.Context(), failedJob(), 1))
	assert.Equal(t, types.NamespacedName{Namespace: "ns", Name: "pitr-1-pitr-1"}, asked)
}
