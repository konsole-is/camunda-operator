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

package keycloak

import (
	"fmt"

	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
)

// The handlers in this file are the kind-specific status logic NewBuilder
// registers by default; they replace the scaffolded defaults the ocf
// generator writes into builder.go. After regenerating the wrapper with
// --force, delete the scaffolded Default* handlers from builder.go again so
// these implementations take their place.

// conditionTrue is the value of a Keycloak condition that holds. The Keycloak
// Operator writes the status of a condition as a string, not as a boolean.
const conditionTrue = "True"

// DefaultConvergingStatusHandler maps the conditions of the Keycloak custom
// resource to the component convergence state: HasErrors is Failing, Ready is
// Healthy, and anything else is still converging (Creating on the first
// apply, Updating afterwards). The Keycloak Operator reports Ready=False
// while it rolls the pods of a healthy Keycloak, so only HasErrors marks a
// failure.
func DefaultConvergingStatusHandler(
	op concepts.ConvergingOperation, kc *Keycloak,
) (concepts.AliveStatusWithReason, error) {
	if errors, found := condition(kc, ConditionHasErrors); found && errors.Status == conditionTrue {
		return concepts.AliveStatusWithReason{
			Status: concepts.AliveConvergingStatusFailing,
			Reason: conditionMessage("Keycloak reports errors", errors),
		}, nil
	}

	if ready, found := condition(kc, ConditionReady); found && ready.Status == conditionTrue {
		return concepts.AliveStatusWithReason{
			Status: concepts.AliveConvergingStatusHealthy,
			Reason: "Keycloak is ready",
		}, nil
	}

	status := concepts.AliveConvergingStatusUpdating
	if op == concepts.ConvergingOperationCreated {
		status = concepts.AliveConvergingStatusCreating
	}

	return concepts.AliveStatusWithReason{Status: status, Reason: notReadyReason(kc)}, nil
}

// DefaultGraceStatusHandler grades a Keycloak that is still not converged
// after the grace period: ready is Healthy, anything else is Down. A Keycloak
// serves requests or it does not, so it has no degraded state of its own.
func DefaultGraceStatusHandler(kc *Keycloak) (concepts.GraceStatusWithReason, error) {
	if ready, found := condition(kc, ConditionReady); found && ready.Status == conditionTrue {
		return concepts.GraceStatusWithReason{
			Status: concepts.GraceStatusHealthy,
			Reason: "Keycloak is ready",
		}, nil
	}

	return concepts.GraceStatusWithReason{
		Status: concepts.GraceStatusDown,
		Reason: notReadyReason(kc),
	}, nil
}

// DefaultSuspendMutationHandler scales the Keycloak to zero instances.
// Keycloak keeps every realm, client, and user in its PostgreSQL database, so
// a Keycloak without pods loses nothing and a resume brings the same server
// back.
func DefaultSuspendMutationHandler(m *Mutator) error {
	m.SetInstances(0)

	return nil
}

// DefaultSuspensionStatusHandler reports Suspended once no Keycloak pod is
// left: the applied resource asks for zero instances, the Keycloak Operator
// has seen that generation, and it reports no ready instance.
func DefaultSuspensionStatusHandler(kc *Keycloak) (concepts.SuspensionStatusWithReason, error) {
	if kc.Spec.Instances == nil || *kc.Spec.Instances != 0 {
		return concepts.SuspensionStatusWithReason{
			Status: concepts.SuspensionStatusPending,
			Reason: "Keycloak is not scaled to zero instances yet",
		}, nil
	}

	if kc.Status.ObservedGeneration < kc.Generation {
		return concepts.SuspensionStatusWithReason{
			Status: concepts.SuspensionStatusSuspending,
			Reason: "the Keycloak Operator has not observed the current generation yet",
		}, nil
	}

	if kc.Status.Instances > 0 {
		return concepts.SuspensionStatusWithReason{
			Status: concepts.SuspensionStatusSuspending,
			Reason: fmt.Sprintf("Keycloak still runs %d instances", kc.Status.Instances),
		}, nil
	}

	return concepts.SuspensionStatusWithReason{
		Status: concepts.SuspensionStatusSuspended,
		Reason: "Keycloak runs no instances",
	}, nil
}

// DefaultDeleteOnSuspendHandler keeps the Keycloak custom resource on
// suspension. The suspension mutation scales it to zero, and the database
// behind it is not the operator's to delete.
func DefaultDeleteOnSuspendHandler(_ *Keycloak) bool {
	return false
}

// condition returns the condition of the given type that the Keycloak
// Operator reported.
func condition(kc *Keycloak, conditionType string) (KeycloakCondition, bool) {
	for _, c := range kc.Status.Conditions {
		if c.Type == conditionType {
			return c, true
		}
	}

	return KeycloakCondition{}, false
}

// notReadyReason explains why a Keycloak is not ready, from its Ready
// condition when it reported one.
func notReadyReason(kc *Keycloak) string {
	ready, found := condition(kc, ConditionReady)
	if !found {
		return "Keycloak has not reported readiness yet"
	}

	return conditionMessage("Keycloak is not ready", ready)
}

// conditionMessage appends the message of a condition to a summary, when the
// condition carries one.
func conditionMessage(summary string, c KeycloakCondition) string {
	if c.Message == "" {
		return summary
	}

	return summary + ": " + c.Message
}
