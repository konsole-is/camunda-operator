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

package camundaoptimize

import (
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The event vocabulary of the suspension that a CamundaOptimize follows.
const (
	// eventActionSuspend is the action of the events that the controller
	// records when the suspension of the referenced cluster changes.
	eventActionSuspend = "Suspend"
	// eventReasonClusterSuspended is recorded when the referenced cluster
	// starts suspending and the Optimize workloads follow it to zero.
	eventReasonClusterSuspended = "ClusterSuspended"
	// eventReasonClusterResumed is recorded when the referenced cluster stops
	// being suspended and the Optimize workloads start again.
	eventReasonClusterResumed = "ClusterResumed"
	// noteSuspended and noteResumed carry the name of the cluster, which is
	// what the Ready condition cannot say.
	noteSuspended = "Scaling the Optimize workloads to zero: CamundaCluster %q is suspended"
	noteResumed   = "Starting the Optimize workloads again: CamundaCluster %q is no longer suspended"
)

// recordSuspensionChange records an event when the suspension of the
// referenced cluster changes, and nothing while it holds.
//
// The condition carries the state and the event carries the transition, so a
// user reading `kubectl describe` learns why the workloads went to zero. The
// Ready condition cannot say it: ocf builds the message of a suspended
// component from its own suspension state, and the reason is Suspended, the
// same reason that a suspended CamundaCluster reports.
//
// before is what wasSuspending read at the top of the reconcile, and suspended
// is spec.suspend of the referenced cluster. The caller runs this after it
// stages the new Ready, so a reconcile that returns early on an error records
// nothing: it changed no workload, and the next reconcile still sees the same
// transition to record.
func (r *Reconciler) recordSuspensionChange(
	optimize *v1.CamundaOptimize,
	before, suspended bool,
) {
	if suspended == before {
		return
	}

	reason, note := eventReasonClusterResumed, noteResumed
	if suspended {
		reason, note = eventReasonClusterSuspended, noteSuspended
	}

	r.EventRecorder.Eventf(
		optimize,
		nil,
		corev1.EventTypeNormal,
		reason,
		eventActionSuspend,
		note,
		optimize.Spec.ClusterRef.Name,
	)
}

// wasSuspending reports whether the last reconcile left the CamundaOptimize
// on its way to suspended or already there.
//
// It reads Ready rather than a stored copy of the cluster field, because the
// controller persists no view of the cluster. Suspending counts: a reconcile
// that catches the workloads mid-drain must not record the transition a
// second time.
func wasSuspending(optimize *v1.CamundaOptimize) bool {
	ready := meta.FindStatusCondition(optimize.Status.Conditions, v1.ConditionReady)
	if ready == nil {
		return false
	}

	switch ready.Reason {
	case string(component.PendingSuspension), string(component.Suspending), string(component.Suspended):
		return true
	default:
		return false
	}
}
