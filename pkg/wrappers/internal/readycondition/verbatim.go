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

package readycondition

import (
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1 "github.com/konsole-is/camunda-operator/api/v1"
)

// VerbatimConverging maps the Ready condition to a convergence status for a
// consumer that shows the Ready reason and message of the resource as they
// are. Ready=True of the observed generation is Healthy, whatever its
// reason, so a suspended resource is Healthy too. The reason it returns
// holds the Ready reason and message unchanged. Ready=False with the reason
// Degraded or a reason that ends convergence, such as Failing, is Failing.
// Every other state, also a Ready of an older generation, is Creating on
// the first apply and Updating afterwards.
func VerbatimConverging(op concepts.ConvergingOperation, o Object) concepts.AliveStatusWithReason {
	reason := verbatim(o)
	if stale := concepts.StaleGenerationStatus(op, observed(o), o.Generation, o.Kind); stale != nil {
		stale.Reason = reason

		return *stale
	}

	ready := meta.FindStatusCondition(o.Conditions, apiv1.ConditionReady)
	switch {
	case ready != nil && ready.Status == metav1.ConditionTrue:
		return concepts.AliveStatusWithReason{Status: concepts.AliveConvergingStatusHealthy, Reason: reason}
	case ready != nil && ready.Status == metav1.ConditionFalse && endsConvergence(ready.Reason):
		return concepts.AliveStatusWithReason{Status: concepts.AliveConvergingStatusFailing, Reason: reason}
	}

	status := concepts.AliveConvergingStatusUpdating
	if op == concepts.ConvergingOperationCreated {
		status = concepts.AliveConvergingStatusCreating
	}

	return concepts.AliveStatusWithReason{Status: status, Reason: reason}
}

// verbatim gives the Ready reason and message, unchanged. It says so when
// the operator has not observed the current generation yet.
func verbatim(o Object) string {
	text := o.Kind + " has not reported Ready yet"
	if ready := meta.FindStatusCondition(o.Conditions, apiv1.ConditionReady); ready != nil {
		text = ready.Reason
		if ready.Message != "" {
			text += ": " + ready.Message
		}
	}

	if observed(o) < o.Generation {
		text = unobserved(o) + ". Its last Ready condition was " + text
	}

	return text
}

// endsConvergence reports whether a Ready=False reason means that the
// resource stopped converging.
func endsConvergence(reason string) bool {
	_, failing := failingReasons[reason]

	return failing || reason == string(component.Degraded)
}

// VerbatimGrace grades the resource at the end of the grace period of its
// consumer, with the reason of VerbatimConverging. Ready=True of the
// observed generation is Healthy, Ready=False with the reason Degraded is
// Degraded, and every other state is Down.
func VerbatimGrace(o Object) concepts.GraceStatusWithReason {
	ready := currentReady(o)
	reason := verbatim(o)

	switch {
	case ready != nil && ready.Status == metav1.ConditionTrue:
		return concepts.GraceStatusWithReason{Status: concepts.GraceStatusHealthy, Reason: reason}
	case ready != nil && ready.Status == metav1.ConditionFalse && ready.Reason == string(component.Degraded):
		return concepts.GraceStatusWithReason{Status: concepts.GraceStatusDegraded, Reason: reason}
	}

	return concepts.GraceStatusWithReason{Status: concepts.GraceStatusDown, Reason: reason}
}

// currentReady returns the Ready condition when the operator observed the
// current generation, and nil otherwise.
func currentReady(o Object) *metav1.Condition {
	if observed(o) < o.Generation {
		return nil
	}

	return meta.FindStatusCondition(o.Conditions, apiv1.ConditionReady)
}
