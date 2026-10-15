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

package backupschedule

import (
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"

	apiv1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/internal/readycondition"
)

// The handlers in this file replace the scaffolded defaults that the ocf
// generator writes into builder.go. After a run of the generator with
// --force, delete the scaffolded Default* handlers from builder.go again.

// DefaultConvergingStatusHandler maps the Ready condition of the
// BackupSchedule to the alive states. Ready=True is Healthy, unless its
// reason is Suspended. Ready=False with a reason that ends convergence,
// such as Failing, is Failing. Every other state, also a Ready of an older
// generation, is Creating on the first apply and Updating afterwards.
func DefaultConvergingStatusHandler(
	op concepts.ConvergingOperation, schedule *apiv1.BackupSchedule,
) (concepts.AliveStatusWithReason, error) {
	return readycondition.Converging(op, observed(schedule)), nil
}

// observed is the part of the BackupSchedule that the handlers read.
func observed(schedule *apiv1.BackupSchedule) readycondition.Object {
	return readycondition.Object{
		Kind:               "BackupSchedule",
		Generation:         schedule.Generation,
		ObservedGeneration: schedule.Status.ObservedGeneration,
		Conditions:         schedule.Status.Conditions,
	}
}

// DefaultGraceStatusHandler grades a BackupSchedule that is still not
// healthy after the grace period: Ready=True is Healthy, Ready=False with
// the reason Degraded is Degraded, and every other state is Down.
func DefaultGraceStatusHandler(schedule *apiv1.BackupSchedule) (concepts.GraceStatusWithReason, error) {
	return readycondition.Grace(observed(schedule)), nil
}

// DefaultSuspendMutationHandler records no change. The suspension of the
// component leaves the BackupSchedule as it is.
func DefaultSuspendMutationHandler(_ *Mutator) error {
	return nil
}

// DefaultSuspensionStatusHandler reports Suspended at once, because the
// suspension of the component changes nothing on the BackupSchedule.
func DefaultSuspensionStatusHandler(_ *apiv1.BackupSchedule) (concepts.SuspensionStatusWithReason, error) {
	return concepts.SuspensionStatusWithReason{
		Status: concepts.SuspensionStatusSuspended,
		Reason: "The suspension of the component leaves the BackupSchedule as it is",
	}, nil
}

// DefaultDeleteOnSuspendHandler keeps the BackupSchedule on suspension.
func DefaultDeleteOnSuspendHandler(_ *apiv1.BackupSchedule) bool {
	return false
}
