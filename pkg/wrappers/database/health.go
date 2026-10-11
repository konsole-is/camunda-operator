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

package database

import (
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"

	apiv1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/internal/readycondition"
)

// The handlers in this file are the kind-specific status logic NewBuilder
// registers by default; they replace the scaffolded defaults the ocf
// generator writes into builder.go. After regenerating the wrapper with
// --force, delete the scaffolded Default* handlers from builder.go again so
// these implementations take their place.

// DefaultOperationalStatusHandler maps the Ready condition of the Database
// to the operational state. Ready=True is Operational. Ready=False with a
// reason that ends convergence, such as Failing, is OperationFailing. Every
// other state, also a Ready of an older generation and a failed pre-check
// such as ServerIdentityUnknown, is OperationPending.
func DefaultOperationalStatusHandler(
	_ concepts.ConvergingOperation, db *apiv1.Database,
) (concepts.OperationalStatusWithReason, error) {
	return readycondition.Operational(observed(db)), nil
}

// observed is the part of the Database that the handlers read.
func observed(db *apiv1.Database) readycondition.Object {
	return readycondition.Object{
		Kind:               "Database",
		Generation:         db.Generation,
		ObservedGeneration: db.Status.ObservedGeneration,
		Conditions:         db.Status.Conditions,
	}
}

// DefaultGraceStatusHandler grades a Database that is still not operational
// after the grace period: Ready=True is Healthy, Ready=False with the reason
// Degraded is Degraded, and anything else is Down.
func DefaultGraceStatusHandler(db *apiv1.Database) (concepts.GraceStatusWithReason, error) {
	return readycondition.Grace(observed(db)), nil
}

// DefaultSuspendMutationHandler records no change. A Database runs no pods,
// so there is nothing to stop.
func DefaultSuspendMutationHandler(_ *Mutator) error {
	return nil
}

// DefaultSuspensionStatusHandler reports Suspended at once, because the
// suspension changes nothing on a Database.
func DefaultSuspensionStatusHandler(_ *apiv1.Database) (concepts.SuspensionStatusWithReason, error) {
	return concepts.SuspensionStatusWithReason{
		Status: concepts.SuspensionStatusSuspended,
		Reason: "A Database runs no pods",
	}, nil
}

// DefaultDeleteOnSuspendHandler keeps the Database on suspension.
func DefaultDeleteOnSuspendHandler(_ *apiv1.Database) bool {
	return false
}
