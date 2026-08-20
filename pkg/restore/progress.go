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

// completedMessage is the Ready message of every restore that finished.
const completedMessage = "The restore finished. The cluster can be unsuspended"

// HoldRunning holds a started restore on a dependency that stopped resolving.
// It stages the failure as the Ready condition of owner, records when the
// failure started, and fails the restore once the grace is over: a restore
// that already erased a broker volume has nothing to go back to, so it must
// reach a terminal phase and tell whoever owns the cluster to act.
//
// The grace counts from the first failure and is never extended, so a
// dependency that flaps cannot hold a started restore for ever. A restore
// that has not started holds without a bound instead, through the Pending
// path of its controller.
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

// Recovered records that the restore made progress again, which gives the
// next failure the full mid-run grace.
//
// A restore that already erased a broker volume or recorded a Job keeps its
// clock. That restore has started, and a dependency that flaps in and out
// would reset the grace on every pass and hold it for ever. Only a restore
// that has touched no volume yet gets its clock back.
func Recovered(p *v1.RestoreProgress) {
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

// Fail records the terminal failure with its Ready reason and message. The
// message carries an external error whose size the operator cannot know, for
// example the reason of a Job or of a pod, so it is bounded before it reaches
// the free-form status field. The caller writes the phase of its own kind.
func Fail(p *v1.RestoreProgress, reason, message string, now metav1.Time) {
	p.CompletionTime = &now
	p.TerminalReason = reason
	p.FailureMessage = conditions.BoundMessage(message)
}

// StageTerminal stages the Ready condition of a terminal restore from the
// recorded reason and message. It is idempotent, so a terminal restore stages
// it again on every look: a write conflict on the terminal flush can restore
// a stale condition from the server, and staging heals that.
//
// Only a caller that knows the restore is terminal calls it. Completed is the
// one outcome that records no message, so every other recorded reason reports
// a failure, and a terminal restore without a recorded reason reports Failed.
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
