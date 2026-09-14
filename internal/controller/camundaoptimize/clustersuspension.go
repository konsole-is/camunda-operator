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

// This file reports the suspension that a CamundaOptimize follows: the
// workloads go to zero with the cluster it is attached to, and the events say
// when that started and when it ended.

package camundaoptimize

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/workloadsuspend"
)

// The event vocabulary of the suspension that a CamundaOptimize follows.
const (
	// eventReasonClusterSuspended is recorded when the referenced cluster
	// starts suspending and the Optimize workloads follow it to zero.
	eventReasonClusterSuspended = "ClusterSuspended"
	// eventReasonClusterResumed is recorded when the referenced cluster holds
	// its backend and is not suspended, and the Optimize workloads follow their
	// spec again.
	eventReasonClusterResumed = "ClusterResumed"
	// eventReasonStorageClaimAwaited is recorded when the claim of the backend
	// scales the Optimize workloads to zero while the cluster itself reports no
	// suspension.
	eventReasonStorageClaimAwaited = "StorageClaimAwaited"
	// noteSuspended, noteResumed and noteClaimAwaited carry the name of the
	// cluster, which is what the Ready condition cannot say. They report the
	// state of that cluster and nothing about a replica count: an importer that
	// spec.importer.replicas holds at zero does not start when the cluster
	// resumes. One resume note covers both waits, because the workloads start
	// when the cluster holds its backend and is not suspended, whichever of the
	// two held them.
	noteSuspended    = "CamundaCluster %q is suspended, the Optimize workloads follow it to zero"
	noteClaimAwaited = "CamundaCluster %q does not hold its backend, or pods of another cluster " +
		"still write it, the Optimize workloads follow it to zero"
	noteResumed = "CamundaCluster %q holds its backend and is not suspended, the Optimize " +
		"workloads follow their spec"
)

// wait is one reason the Optimize workloads are at zero, with everything this
// controller says about it. Two waits lower them, and a user acts on a
// different thing in each: one cluster was suspended, and the other does not
// hold the backend that its Optimize reads.
type wait struct {
	// eventReason and eventNote report the start of this wait. The note carries
	// one verb for the name of the cluster.
	eventReason string
	eventNote   string
	// failureNote is appended to the failure message on Ready, and it carries
	// the name of the cluster the same way.
	failureNote string
	// condition is the message of the condition of a workload of this wait
	// whose pods are gone.
	condition string
	// workloadNote says, in the event of a workload this pass lowered, why it
	// did.
	workloadNote string
}

var (
	// clusterSuspended is the wait on a cluster that reports itself suspended,
	// by spec.suspend or by a state the operator holds it in.
	clusterSuspended = wait{
		eventReason:  eventReasonClusterSuspended,
		eventNote:    noteSuspended,
		failureNote:  ". The Optimize workloads are scaled to zero because CamundaCluster %q is suspended",
		condition:    "Scaled to zero while the referenced cluster is suspended",
		workloadNote: "because the CamundaCluster it attaches to is suspended",
	}
	// backendClaimAwaited is the wait on the claim of the backend. The cluster
	// can report itself healthy through it, so nothing here says it is
	// suspended.
	backendClaimAwaited = wait{
		eventReason: eventReasonStorageClaimAwaited,
		eventNote:   noteClaimAwaited,
		failureNote: ". The Optimize workloads are scaled to zero because CamundaCluster %q does not " +
			"hold its backend",
		condition:    "Scaled to zero while the referenced cluster does not hold its backend",
		workloadNote: "because the CamundaCluster it attaches to does not hold its backend",
	}
)

// waitFor returns the wait that holds the workloads at zero on this pass. Only
// the storage claim reaches AwaitsBackendClaim, so every other wait is the
// cluster reporting a suspension of its own.
func waitFor(res resolved) wait {
	if res.AwaitsBackendClaim {
		return backendClaimAwaited
	}

	return clusterSuspended
}

// hasWorkloads reports whether this CamundaOptimize ever rendered a workload.
// An instance that never did has nothing to scale, so a suspension of it is no
// transition a user acts on: an instance created beside its cluster can meet
// the storage claim before that cluster takes it, and an event of that window
// would name a wait nobody asked for.
//
// Every workload counts, not the importer alone. A pass that stopped the webapp
// and had the importer patch rejected leaves one condition of the two, and the
// instance rendered a workload either way.
func hasWorkloads(optimize *v1.CamundaOptimize) bool {
	for _, conditionType := range workloadConditions() {
		if meta.FindStatusCondition(optimize.Status.Conditions, conditionType) != nil {
			return true
		}
	}

	return false
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

	return workloadsuspend.IsSuspensionReason(ready.Reason)
}

// recordClusterSuspended records that the workloads of this CamundaOptimize
// followed the referenced cluster to zero. The pre-check failure path records
// this transition and no other, so it names the event rather than deriving it
// from a before and an after that are always false and true there.
func (r *Reconciler) recordClusterSuspended(optimize *v1.CamundaOptimize, res resolved) {
	held := waitFor(res)
	r.EventRecorder.Eventf(
		optimize,
		nil,
		corev1.EventTypeNormal,
		held.eventReason,
		workloadsuspend.EventActionSuspend,
		held.eventNote,
		optimize.Spec.ClusterRef.Name,
	)
}

// recordSuspensionChange records an event when the suspension of the
// referenced cluster changes, and nothing while it holds.
//
// The condition carries the state and the event carries the transition, so a
// user reading `kubectl describe` learns why the workloads went to zero. The
// Ready condition cannot say it: ocf builds the message of a suspended
// component from its own suspension state, and the reason is Suspended, the
// same reason that a suspended CamundaCluster reports.
//
// before is the prior suspension state that the caller read at the top of the
// reconcile, from the Ready reason through wasSuspending or from the workload
// conditions through followsSuspendedCluster. res carries why the workloads are
// at zero now: Suspended covers spec.suspend and the states in which the
// operator holds the cluster at zero, and AwaitsBackendClaim the claim of the
// backend, which lowers them while the cluster reports itself healthy.
//
// The caller runs this after it stages the new conditions. A reconcile that
// returns early on an error records nothing, and the next one still sees the
// same transition to record.
func (r *Reconciler) recordSuspensionChange(
	optimize *v1.CamundaOptimize,
	before bool,
	res resolved,
) {
	if res.Input.Suspended == before {
		return
	}

	reason, note := eventReasonClusterResumed, noteResumed
	if res.Input.Suspended {
		held := waitFor(res)
		reason, note = held.eventReason, held.eventNote
	}

	r.EventRecorder.Eventf(
		optimize,
		nil,
		corev1.EventTypeNormal,
		reason,
		workloadsuspend.EventActionSuspend,
		note,
		optimize.Spec.ClusterRef.Name,
	)
}
