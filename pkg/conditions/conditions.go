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

// Package conditions derives the aggregate Ready condition that every CRD
// reports. The condition vocabulary lives in api/v1, and every controller
// persists status through the ocf FlushStatus. This package only holds the
// derivation rule and its inputs.
package conditions

import (
	"fmt"

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

// DeriveReady derives the CR-level Ready reason and message from the pre-check
// result of the controller, the ocf component conditions, and the suspension
// flag. A pre-check failure wins outright. Otherwise suspension reports
// Suspended. Otherwise the first component condition whose status is not True
// reports Progressing, with a message that names that component. With every
// component True, the result is Healthy. An empty component list is
// Progressing too. A controller always has at least one component, so an
// empty list means that none has reported yet.
func DeriveReady(pre *PreCheckFailure, componentConds []metav1.Condition, suspended bool) (reason, message string) {
	if pre != nil {
		return pre.Reason, pre.Message
	}

	if suspended {
		return v1.ReasonSuspended, "Suspended by spec.suspend"
	}

	if len(componentConds) == 0 {
		return v1.ReasonProgressing, "Waiting for components to report"
	}

	for _, cond := range componentConds {
		if cond.Status == metav1.ConditionTrue {
			continue
		}
		detail := cond.Message
		if detail == "" {
			detail = cond.Reason
		}
		return v1.ReasonProgressing, fmt.Sprintf("Waiting for %s: %s", cond.Type, detail)
	}

	return v1.ReasonHealthy, "All components ready"
}

// Ready builds a Ready condition observed at the given generation. It sets no
// LastTransitionTime, because meta.SetStatusCondition supplies it.
func Ready(status metav1.ConditionStatus, reason, message string, observedGeneration int64) metav1.Condition {
	return metav1.Condition{
		Type: v1.ConditionReady, Status: status, Reason: reason,
		Message: message, ObservedGeneration: observedGeneration,
	}
}
