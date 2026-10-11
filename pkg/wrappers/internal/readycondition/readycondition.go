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

// Package readycondition reads the Ready condition that this operator writes
// on its own kinds, for the wrappers that apply those kinds from another
// controller.
package readycondition

import (
	"strconv"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1 "github.com/konsole-is/camunda-operator/api/v1"
)

// Object is the part of an operator-owned resource that the handlers read.
// Kind names the resource in every reason, for example "ElasticsearchCluster".
type Object struct {
	Kind               string
	Generation         int64
	ObservedGeneration int64
	Conditions         []metav1.Condition
}

// state is the meaning of a Ready condition for a consumer of the resource.
type state int

const (
	pending state = iota
	serving
	suspended
	degraded
	failing
)

// failingReasons are the Ready reasons of a resource that stopped converging:
// those of ocf, and ArchiveFailing, which a DatabaseServer reports after its
// own grace period. Every other Ready=False reason, also a failed pre-check,
// stays converging until the grace period of the consumer ends.
var failingReasons = map[string]struct{}{
	string(component.AliveFailing):      {},
	string(component.OperationFailing):  {},
	string(component.CompletionFailing): {},
	string(component.Down):              {},
	string(component.Error):             {},
	string(component.FeatureGateError):  {},
	apiv1.ReasonArchiveFailing:          {},
}

// Converging maps the Ready condition to a convergence status. Ready=True is
// Healthy, unless its reason is Suspended. Ready=False with a reason that ends
// convergence, such as Failing, Down, or ArchiveFailing, is Failing. Every
// other state, also a Ready of an older generation, is Creating on the first
// apply and Updating afterwards.
func Converging(op concepts.ConvergingOperation, o Object) concepts.AliveStatusWithReason {
	if stale := concepts.StaleGenerationStatus(op, o.ObservedGeneration, o.Generation, o.Kind); stale != nil {
		stale.Reason = unobserved(o)

		return *stale
	}

	s, reason := read(o)
	switch s {
	case serving:
		return concepts.AliveStatusWithReason{Status: concepts.AliveConvergingStatusHealthy, Reason: reason}
	case degraded, failing:
		return concepts.AliveStatusWithReason{Status: concepts.AliveConvergingStatusFailing, Reason: reason}
	}

	status := concepts.AliveConvergingStatusUpdating
	if op == concepts.ConvergingOperationCreated {
		status = concepts.AliveConvergingStatusCreating
	}

	return concepts.AliveStatusWithReason{Status: status, Reason: reason}
}

// unobserved explains a Ready condition that is older than the spec.
func unobserved(o Object) string {
	return "The operator has not observed generation " + strconv.FormatInt(o.Generation, 10) +
		" of the " + o.Kind + " yet"
}

// read gives the state of the Ready condition and a reason that names the
// kind, the Ready reason, and the Ready message.
func read(o Object) (state, string) {
	ready := meta.FindStatusCondition(o.Conditions, apiv1.ConditionReady)
	switch {
	case ready == nil || ready.Status == metav1.ConditionUnknown:
		return pending, o.Kind + " has not reported Ready yet"
	case ready.Status == metav1.ConditionTrue && ready.Reason == string(component.Suspended):
		return suspended, o.Kind + " is suspended"
	case ready.Status == metav1.ConditionTrue:
		return serving, o.Kind + " is ready"
	}

	reason := o.Kind + " is not ready (" + ready.Reason + ")"
	if ready.Message != "" {
		reason += ": " + ready.Message
	}

	if ready.Reason == string(component.Degraded) {
		return degraded, reason
	}

	if _, ok := failingReasons[ready.Reason]; ok {
		return failing, reason
	}

	return pending, reason
}

// Operational maps the Ready condition to an operational status, by the
// rules of Converging: serving is Operational, Failing is OperationFailing,
// and every other state is OperationPending.
func Operational(o Object) concepts.OperationalStatusWithReason {
	if o.ObservedGeneration < o.Generation {
		return concepts.OperationalStatusWithReason{
			Status: concepts.OperationalStatusPending,
			Reason: unobserved(o),
		}
	}

	s, reason := read(o)
	switch s {
	case serving:
		return concepts.OperationalStatusWithReason{Status: concepts.OperationalStatusOperational, Reason: reason}
	case degraded, failing:
		return concepts.OperationalStatusWithReason{Status: concepts.OperationalStatusFailing, Reason: reason}
	}

	return concepts.OperationalStatusWithReason{Status: concepts.OperationalStatusPending, Reason: reason}
}

// Grace grades a resource that is not converged at the end of the grace
// period of its consumer: serving is Healthy, Ready=False with the reason
// Degraded is Degraded, and every other state is Down. A Ready of an older
// generation is Down too, so Grace never reports Healthy where Converging
// does not.
func Grace(o Object) concepts.GraceStatusWithReason {
	if o.ObservedGeneration < o.Generation {
		return concepts.GraceStatusWithReason{Status: concepts.GraceStatusDown, Reason: unobserved(o)}
	}

	s, reason := read(o)
	switch s {
	case serving:
		return concepts.GraceStatusWithReason{Status: concepts.GraceStatusHealthy, Reason: reason}
	case degraded:
		return concepts.GraceStatusWithReason{Status: concepts.GraceStatusDegraded, Reason: reason}
	}

	return concepts.GraceStatusWithReason{Status: concepts.GraceStatusDown, Reason: reason}
}

// Suspension reports the progress of a suspension. requested says whether
// the applied object asks for it. It is Pending until the object asks,
// Suspending until the operator observed that generation and reports
// Ready=True with the reason Suspended, and Suspended then.
func Suspension(o Object, requested bool) concepts.SuspensionStatusWithReason {
	if !requested {
		return concepts.SuspensionStatusWithReason{
			Status: concepts.SuspensionStatusPending,
			Reason: o.Kind + " does not ask for suspension yet",
		}
	}

	if o.ObservedGeneration < o.Generation {
		return concepts.SuspensionStatusWithReason{Status: concepts.SuspensionStatusSuspending, Reason: unobserved(o)}
	}

	s, reason := read(o)
	if s == suspended {
		return concepts.SuspensionStatusWithReason{Status: concepts.SuspensionStatusSuspended, Reason: reason}
	}

	return concepts.SuspensionStatusWithReason{Status: concepts.SuspensionStatusSuspending, Reason: reason}
}
