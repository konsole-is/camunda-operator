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

package conditions

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Suspended builds a Suspended condition observed at the given generation. Its
// status is True while the workload is intentionally scaled to zero, and False
// otherwise. It sets no LastTransitionTime, because meta.SetStatusCondition
// supplies it.
func Suspended(status metav1.ConditionStatus, message string, observedGeneration int64) metav1.Condition {
	return metav1.Condition{
		Type: TypeSuspended, Status: status, Reason: ReasonSuspended,
		Message: message, ObservedGeneration: observedGeneration,
	}
}
