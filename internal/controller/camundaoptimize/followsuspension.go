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

// This file stops the Optimize workloads of a suspended cluster while a check
// of this CamundaOptimize fails. A failed check leaves nothing to render from,
// so the render that follows the suspension never runs. The importer reads
// Elasticsearch directly, and a logical restore of that Elasticsearch suspends
// the cluster to stop it, so an importer that kept running would write
// analytics from half-restored indices.

package camundaoptimize

import (
	"context"
	"errors"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundaoptimize"
	"github.com/konsole-is/camunda-operator/pkg/workloadsuspend"
)

// stopReason is what this controller says about the Optimize workloads it
// holds at zero, in the three places that report it. Two waits lower them, and
// a user acts on a different thing in each: one cluster was suspended, and the
// other does not hold the backend that its Optimize reads.
type stopReason struct {
	// failureNote is appended to the failure message on Ready. It carries one
	// verb for the name of the cluster.
	failureNote string
	// message is the condition of a workload whose pods are gone.
	message string
	// eventNote says, in the event of a scaled workload, why this controller
	// lowered it.
	eventNote string
}

var (
	// clusterSuspended is the wait on a cluster that reports itself suspended,
	// by spec.suspend or by a state the operator holds it in.
	clusterSuspended = stopReason{
		failureNote: ". The Optimize workloads are scaled to zero because CamundaCluster %q is suspended",
		message:     "Scaled to zero while the referenced cluster is suspended",
		eventNote:   "because the CamundaCluster it attaches to is suspended",
	}
	// backendClaimAwaited is the wait on the claim of the backend. The cluster
	// can report itself healthy through it, so nothing here says it is
	// suspended.
	backendClaimAwaited = stopReason{
		failureNote: ". The Optimize workloads are scaled to zero because CamundaCluster %q does not hold its backend",
		message:     "Scaled to zero while the referenced cluster does not hold its backend",
		eventNote:   "because the CamundaCluster it attaches to does not hold its backend",
	}
)

// keptAtZeroReason says, in the event of a scaled workload, that a wait which
// ended left it at zero.
const keptAtZeroReason = "because the workloads that stopped stay at zero until the reference check passes"

// stopReasonFor returns what to say about workloads that this pass holds at
// zero. Only the storage claim reaches AwaitsBackendClaim, so every other
// suspension is the cluster reporting one.
func stopReasonFor(res resolved) stopReason {
	if res.AwaitsBackendClaim {
		return backendClaimAwaited
	}

	return clusterSuspended
}

// keptAtZeroNote is appended to the failure message of a CamundaOptimize whose
// workloads a suspension left at zero and whose cluster resumed.
const keptAtZeroNote = ". The Optimize workloads that stopped stay at zero until the reference check passes"

// suspendOrder is the Optimize workloads, the importer first. It is the workload
// that writes Elasticsearch, so a webapp that a conflict or an admission rule
// keeps up must not keep the importer up with it.
var suspendOrder = []string{components.ComponentImporter, components.ComponentWebapp}

// followSuspension scales the webapp and the importer to zero and keeps
// everything else: the Deployments, the Services, and the copies of the
// referenced Secrets. The condition of each workload it stops reports the wait
// that lowered it, because the components do not run on this path.
func (r *Reconciler) followSuspension(
	ctx context.Context,
	optimize *v1.CamundaOptimize,
	stop stopReason,
) (workloadsuspend.Result, error) {
	return r.stopWorkloads(ctx, optimize, stop.message, stop.eventNote, func(string) bool { return true })
}

// keepAtZero holds the Optimize workloads that a suspension stopped while a
// check of this instance still fails, and reports whether it held any.
//
// The conditions are read first, in memory: an instance that never followed a
// suspension has nothing to hold, and reading its Deployments would answer a
// question already answered. The candidates are then read live, so a drain that
// finished since the cluster resumed is reported as finished rather than copied
// from the condition that caught it mid-drain.
func (r *Reconciler) keepAtZero(
	ctx context.Context,
	optimize *v1.CamundaOptimize,
) (workloadsuspend.Result, error) {
	if !followsSuspendedCluster(optimize) {
		return workloadsuspend.Result{}, nil
	}

	return r.stopWorkloads(
		ctx,
		optimize,
		workloadsuspend.MessageKeptAtZero,
		keptAtZeroReason,
		func(conditionType string) bool {
			return workloadsuspend.IsAlreadySuspended(optimize, conditionType)
		},
	)
}

// stopWorkloads scales the Optimize workloads that stop accepts to zero. message
// says why a workload is held there once its pods are gone, and reason says the
// same in the event of each workload it lowered.
//
// The importer goes first. It is the workload that writes Elasticsearch, so a
// webapp that a conflict or an admission rule keeps up must not keep the
// importer up with it.
func (r *Reconciler) stopWorkloads(
	ctx context.Context,
	optimize *v1.CamundaOptimize,
	message, reason string,
	stop func(conditionType string) bool,
) (workloadsuspend.Result, error) {
	workloads, err := r.optimizeWorkloads(ctx, optimize, stop)
	if err != nil {
		return workloadsuspend.Result{}, err
	}

	return workloadsuspend.StopWorkloadsIf(
		ctx,
		r.Client,
		optimize,
		workloads,
		message,
		stop,
		func(workload string) { r.recordSuspended(optimize, workload, reason) },
	)
}

// optimizeWorkloads reads the Deployments that stop accepts, with the condition
// each one reports. One that stop refuses is never read, and one that is gone
// needs nothing.
//
// The reads are live. The decision to stop a workload reads its replicas, and a
// render that raised them lands on the API server before an informer carries it,
// so a cached copy can report none for a workload that runs.
func (r *Reconciler) optimizeWorkloads(
	ctx context.Context,
	optimize *v1.CamundaOptimize,
	stop func(conditionType string) bool,
) ([]workloadsuspend.Workload, error) {
	var errs []error
	var workloads []workloadsuspend.Workload
	for _, comp := range suspendOrder {
		conditionType, ok := components.ConditionTypeFor(comp)
		if !ok || !stop(conditionType) {
			continue
		}

		key := client.ObjectKey{
			Namespace: optimize.Namespace,
			Name:      components.WorkloadName(optimize, comp),
		}

		var deployment appsv1.Deployment
		if err := r.APIReader.Get(ctx, key, &deployment); err != nil {
			if !apierrors.IsNotFound(err) {
				errs = append(errs, fmt.Errorf("reading Deployment %q: %w", key, err))
			}

			continue
		}
		workloads = append(
			workloads,
			workloadsuspend.Workload{Object: &deployment, ConditionType: conditionType},
		)
	}

	return workloads, errors.Join(errs...)
}

// recordSuspended records that the named workload was scaled to zero, and why.
// The reason differs per path: a cluster that is suspended, or this controller
// putting back a workload that something raised while it has nothing to render
// from.
func (r *Reconciler) recordSuspended(optimize *v1.CamundaOptimize, workload, reason string) {
	r.EventRecorder.Eventf(
		optimize,
		nil,
		corev1.EventTypeNormal,
		workloadsuspend.EventReasonWorkloadsSuspended,
		workloadsuspend.EventActionSuspend,
		"Scaled %q to zero %s",
		workload,
		reason,
	)
}

// followsSuspendedCluster reports whether the workload conditions already carry
// a suspension of the referenced cluster.
func followsSuspendedCluster(optimize *v1.CamundaOptimize) bool {
	for _, conditionType := range workloadConditions() {
		condition := meta.FindStatusCondition(optimize.Status.Conditions, conditionType)
		if condition != nil && workloadsuspend.IsSuspensionReason(condition.Reason) {
			return true
		}
	}

	return false
}

// workloadConditions returns the condition that each Optimize workload reports.
func workloadConditions() []string {
	types := make([]string, 0, len(suspendOrder))
	for _, comp := range suspendOrder {
		if conditionType, ok := components.ConditionTypeFor(comp); ok {
			types = append(types, conditionType)
		}
	}

	return types
}
