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
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// completedMessage says nothing about the suspension of the cluster, because
// a later look after the terminal phase withdraws the suspension.
const completedMessage = "The restore finished"

// HoldRunning holds a started restore on a dependency that stopped resolving.
// It sets p.FirstFailedAt on the first failure. Until grace has passed since
// p.FirstFailedAt, it stages failure as the Ready condition of owner and
// reports Outcome.Wait of poll. Once grace has passed, it reports a Failure
// with v1.ReasonFailed instead.
// HoldRunning never extends the grace, so a dependency that flaps cannot hold
// a started restore for ever.
func HoldRunning(
	owner conditions.Owner,
	p *v1.RestoreProgress,
	failure *conditions.PreCheckFailure,
	now metav1.Time,
	grace, poll time.Duration,
) Outcome {
	if p.FirstFailedAt == nil {
		p.FirstFailedAt = &now
	}

	// A started restore has nothing to go back to, so it must end and tell
	// the owner of the cluster to act.
	if now.Sub(p.FirstFailedAt.Time) > grace {
		return Outcome{Failure: &conditions.PreCheckFailure{
			Reason: v1.ReasonFailed,
			Message: fmt.Sprintf(
				"a dependency stopped resolving and did not recover: %s", failure.Message,
			),
		}}
	}

	conditions.Stage(owner, conditions.Failed(owner, failure))

	// A started restore looks again at the pace of a running phase, not at the
	// slower pace of an admission hold. The grace is measured in this loop, so
	// the loop has to run inside it.
	return Outcome{Wait: poll}
}

// Recovered records that the restore made progress again, so the next failure
// gets the full grace of HoldRunning. A restore that recorded a recreated
// claim or a Job keeps its clock. Recovered does not see the indices that the
// secondary-storage phase deletes, so call it there only before the first
// delete.
func Recovered(p *v1.RestoreProgress) {
	// Without the guard, a dependency that flaps resets the grace on every pass.
	if len(p.RecreatedClaims) > 0 || len(p.PrimaryJobNames) > 0 {
		return
	}

	p.FirstFailedAt = nil
}

// Complete records the terminal success. The caller writes the phase of its
// own kind.
func Complete(p *v1.RestoreProgress, now metav1.Time) {
	p.CompletionTime = &now
	p.TerminalReason = v1.ReasonCompleted
}

// Fail records the terminal failure with its Ready reason and message. It
// bounds the size of message, so message can carry an external error. The
// caller writes the phase of its own kind.
func Fail(p *v1.RestoreProgress, reason, message string, now metav1.Time) {
	p.CompletionTime = &now
	p.TerminalReason = reason
	p.FailureMessage = conditions.BoundMessage(message)
}

// StageTerminal stages the Ready condition of a terminal restore from the
// recorded reason and message. Call it only for a terminal restore, and again
// on every look. Every reason other than Completed reports a failure, and a
// restore without a recorded reason reports Failed.
func StageTerminal(owner conditions.Owner, p *v1.RestoreProgress) {
	if p.TerminalReason == v1.ReasonCompleted {
		conditions.Stage(owner, conditions.Ready(
			metav1.ConditionTrue, v1.ReasonCompleted, completedMessage, owner.GetGeneration(),
		))

		return
	}

	reason := p.TerminalReason
	if reason == "" {
		reason = v1.ReasonFailed
	}

	conditions.Stage(owner, conditions.Ready(
		metav1.ConditionFalse, reason, p.FailureMessage, owner.GetGeneration(),
	))
}
