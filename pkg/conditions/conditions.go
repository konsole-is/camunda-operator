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

// Package conditions builds the aggregate Ready condition of a CRD that runs
// ocf components. The condition vocabulary lives in api/v1, and every
// controller persists status through the ocf FlushStatus.
package conditions

import (
	"fmt"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// PreCheckFailure is a failed reconciliation pre-check, for example an
// unresolved reference, a missing Secret, or an unreachable server. It maps
// the failure to its documented Ready reason and a condition-ready message. It
// is an error, so pre-checks return it through the ordinary error path. The
// reconciler picks it out with errors.As and reports it as a Ready condition,
// not as a reconcile error.
type PreCheckFailure struct {
	// Reason is the documented Ready condition reason for the failure.
	Reason string
	// Message is the condition-ready failure message.
	Message string
}

// Error returns the condition-ready message.
func (f *PreCheckFailure) Error() string { return f.Message }

// Ready builds a Ready condition observed at the given generation. It sets no
// LastTransitionTime, because meta.SetStatusCondition supplies it.
func Ready(status metav1.ConditionStatus, reason, message string, observedGeneration int64) metav1.Condition {
	return metav1.Condition{
		Type: v1.ConditionReady, Status: status, Reason: reason,
		Message: message, ObservedGeneration: observedGeneration,
	}
}

// Aggregate builds the Ready condition of owner from its ocf components. It
// mirrors the representative component: the one whose condition reason has
// the highest component.Status priority, with the first one winning a tie.
// Ready takes the status and reason of that component, and its message names
// the component. A component that has not reported yet counts as Unknown,
// which component.GetCondition supplies. With no components the result is
// Unknown.
func Aggregate(owner component.OperatorCRD, comps ...*component.Component) metav1.Condition {
	generation := owner.GetGeneration()
	if len(comps) == 0 {
		return Ready(metav1.ConditionFalse, string(component.Unknown), "No component has reported yet", generation)
	}

	representative := comps[0].GetCondition(owner)
	for _, comp := range comps[1:] {
		cond := comp.GetCondition(owner)
		if cond.ComponentStatus().Priority() > representative.ComponentStatus().Priority() {
			representative = cond
		}
	}

	return Ready(
		representative.Status,
		representative.Reason,
		fmt.Sprintf("%s: %s", representative.Type, representative.Message),
		generation,
	)
}
