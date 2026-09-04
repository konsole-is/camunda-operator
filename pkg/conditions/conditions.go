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
	"unicode/utf8"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"k8s.io/apimachinery/pkg/api/meta"
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

// Owner is a CRD that stages its Ready condition: the ocf owner plus the
// observedGeneration setter that every CRD of this operator implements.
type Owner interface {
	component.OperatorCRD
	// SetObservedGeneration records the last reconciled generation in status.
	SetObservedGeneration(generation int64)
}

// MaxMessageLength bounds the message of every condition that this package
// builds or stages, and of every status string a controller passes through
// BoundMessage. The CRD schema caps a condition message at 32768 characters.
// A message that reaches the cap makes the status unwritable, and a
// controller that copies an external error into a message cannot know its
// size. 8 KiB keeps the message readable and far under the cap.
const MaxMessageLength = 8 << 10

// Stage sets ready on owner and records the current generation as observed,
// in memory. FlushStatus persists both. Every controller stages Ready through
// Stage, so observedGeneration always moves with Ready. The message of ready
// is bounded by MaxMessageLength, whichever way it was built.
func Stage(owner Owner, ready metav1.Condition) {
	ready.Message = BoundMessage(ready.Message)
	meta.SetStatusCondition(owner.GetStatusConditions(), ready)
	owner.SetObservedGeneration(owner.GetGeneration())
}

// Failed builds the Ready condition for a pre-check failure of owner.
func Failed(owner Owner, failure *PreCheckFailure) metav1.Condition {
	return Ready(metav1.ConditionFalse, failure.Reason, failure.Message, owner.GetGeneration())
}

// Ready builds a Ready condition observed at the given generation. It sets no
// LastTransitionTime, because meta.SetStatusCondition supplies it. The
// message is bounded by MaxMessageLength.
func Ready(status metav1.ConditionStatus, reason, message string, observedGeneration int64) metav1.Condition {
	return metav1.Condition{
		Type: v1.ConditionReady, Status: status, Reason: reason,
		Message: BoundMessage(message), ObservedGeneration: observedGeneration,
	}
}

// Aggregate builds the Ready condition of owner from the given ocf
// components. It is component.Aggregate under the Ready condition type: Ready
// is True only when every component condition is True, and the reason and the
// message come from the governing component, which is the one with the highest
// component.Status priority among the components that are not True, or the
// highest of all of them when they all are. A component that has not reported
// yet counts as Unknown. With no components the result is False. The caller
// decides which components make up Ready: an auxiliary component, for example
// a metrics exporter, keeps its own condition and stays out of the list. The
// message is bounded by MaxMessageLength.
func Aggregate(owner component.OperatorCRD, comps ...*component.Component) metav1.Condition {
	ready := metav1.Condition(component.Aggregate(v1.ConditionReady, owner, comps...))
	ready.Message = BoundMessage(ready.Message)
	return ready
}

// BoundMessage returns message when it is within MaxMessageLength bytes.
// Otherwise it returns a prefix of message cut on a rune boundary, followed
// by "... (truncated, N bytes)" with the original size — and the whole
// result, marker included, is at most MaxMessageLength bytes, so a caller
// can size a field to the constant.
func BoundMessage(message string) string {
	if len(message) <= MaxMessageLength {
		return message
	}
	marker := fmt.Sprintf("... (truncated, %d bytes)", len(message))
	cut := message[:MaxMessageLength-len(marker)]
	for !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut + marker
}

// Error returns the condition-ready message.
func (f *PreCheckFailure) Error() string { return f.Message }
