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
	components "github.com/konsole-is/camunda-operator/pkg/components/camundaoptimize"
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
	noteClaimAwaited = "CamundaCluster %q does not hold its backend, or pods of another cluster or " +
		"of a previous instance, or a restore into another cluster, still write it, the Optimize " +
		"workloads follow it to zero"
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
	// backendClaimAwaited is the wait on the claim of the backend: the cluster
	// does not hold the claim, or it holds it while another writer still writes
	// the backend. The cluster can report itself healthy through either, so
	// nothing here says it is suspended.
	backendClaimAwaited = wait{
		eventReason: eventReasonStorageClaimAwaited,
		eventNote:   noteClaimAwaited,
		failureNote: ". The Optimize workloads are scaled to zero because CamundaCluster %q does not " +
			"hold its backend, or pods of another cluster or of a previous instance, or a restore into " +
			"another cluster, still write it",
		condition: "Scaled to zero while the referenced cluster does not hold its backend, or pods of " +
			"another cluster or of a previous instance, or a restore into another cluster, still write it",
		workloadNote: "because the CamundaCluster it attaches to does not hold its backend, or pods of " +
			"another cluster or of a previous instance, or a restore into another cluster, still write it",
	}
)

// suspension is the suspension of the referenced cluster that a
// CamundaOptimize followed when the last pass ended.
type suspension struct {
	// by is status.suspendedBy.
	by v1.OptimizeSuspension
	// rendered reports whether the instance rendered a workload before this
	// pass.
	rendered bool
}

// readSuspension returns the suspension that optimize carries into this pass.
// Call it before anything stages a condition of this pass.
func readSuspension(optimize *v1.CamundaOptimize) suspension {
	// Not the conditions: a failed apply or a conflicting flush rewrites them.
	return suspension{by: optimize.Status.SuspendedBy, rendered: hasWorkloads(optimize)}
}

// suspendedBy returns why res holds the workloads at zero on this pass, or
// empty when they follow their spec.
func suspendedBy(res resolved) v1.OptimizeSuspension {
	switch {
	case !res.Input.Suspended:
		return ""
	case res.AwaitsBackendClaim:
		return v1.OptimizeSuspensionStorageClaim
	default:
		return v1.OptimizeSuspensionCluster
	}
}

// waitFor returns everything this controller says about the wait by. by is not
// empty.
func waitFor(by v1.OptimizeSuspension) wait {
	if by == v1.OptimizeSuspensionStorageClaim {
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
	for _, conditionType := range components.ConditionTypes() {
		if meta.FindStatusCondition(optimize.Status.Conditions, conditionType) != nil {
			return true
		}
	}

	return false
}

// recordSuspensionChange stages now as status.suspendedBy, and records an event
// when the workloads start to follow a suspension or stop following one. It
// records nothing when prior rendered no workload. The event marks the decision
// of this pass, not its outcome.
func (r *Reconciler) recordSuspensionChange(
	optimize *v1.CamundaOptimize,
	prior suspension,
	now v1.OptimizeSuspension,
) {
	optimize.Status.SuspendedBy = now
	if !prior.rendered || (prior.by != "") == (now != "") {
		return
	}

	reason, note := eventReasonClusterResumed, noteResumed
	if now != "" {
		held := waitFor(now)
		reason, note = held.eventReason, held.eventNote
	}

	// Ready cannot name the wait: its reason is Suspended, as for a suspended
	// CamundaCluster.
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
