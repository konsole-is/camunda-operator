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

// The two forms that the Camunda restore application prints when no
// primary-storage backup covers the exporter position of the database. Both
// were read off a real restore, and both are what the operator must
// recognise.
const (
	refusalLast = "2026-08-20 11:02:33.114 [] [main] WARN  io.camunda.zeebe.restore - " +
		"Skipping range [1, 448] of backup 17 because the last log position 448 is before the " +
		"required exported position 451"
	refusalFirst = "2026-08-20 11:02:33.117 [] [main] WARN  io.camunda.zeebe.restore - " +
		"Skipping range [1, 96] of backup 18 because the first log position OptionalLong[1] is " +
		"after required exported position 0"
)

// unrelatedLog is a restore that failed for a cause of its own. Nothing in it
// names an exported position.
const unrelatedLog = "Exception in thread \"main\" java.io.IOException: No space left on device\n" +
	"\tat java.base/java.io.FileOutputStream.write(FileOutputStream.java:349)"

func TestRefusalLineFindsBothFormsOfTheRefusal(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"a backup that ends before the exported position": refusalLast,
		"a backup that starts after it":                   refusalFirst,
	}

	for name, line := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			output := "starting the restore application\n" + line + "\nrestore failed\n"

			assert.Equal(t, line, refusalLine(output))
		})
	}
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

	line := refusalLine(refusalLast + strings.Repeat("x", 4000))

	assert.Len(t, line, maxRefusalLine)
	assert.True(t, strings.HasPrefix(line, "2026-08-20"))
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

	r := &Reconciler{opts: Options{ReadJobLog: readerOf(refusalLast)}.withDefaults()}

	failure := r.jobRefusal(t.Context(), failedJob(), 1)

	require.NotNil(t, failure)
	assert.Equal(t, v1.ReasonExporterPositionNotCovered, failure.Reason)
	assert.Contains(t, failure.Message, "broker 1")
	assert.Contains(t, failure.Message, "required exported position 451")
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

		return refusalFirst, nil
	}
	r := &Reconciler{opts: Options{ReadJobLog: reader}.withDefaults()}

	require.NotNil(t, r.jobRefusal(t.Context(), failedJob(), 1))
	assert.Equal(t, types.NamespacedName{Namespace: "ns", Name: "pitr-1-pitr-1"}, asked)
}
