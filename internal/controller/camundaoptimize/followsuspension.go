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

// keptAtZeroNote is appended to the failure message of a CamundaOptimize whose
// workloads a suspension left at zero and whose cluster resumed.
const keptAtZeroNote = ". The Optimize workloads that stopped stay at zero until the reference check passes"

// followSuspension scales the webapp and the importer to zero and keeps
// everything else: the Deployments, the Services, and the copies of the
// referenced Secrets. The condition of each workload it stops reports the wait
// that lowered it, because the components do not run on this path.
func (r *Reconciler) followSuspension(
	ctx context.Context,
	optimize *v1.CamundaOptimize,
	held wait,
) (workloadsuspend.Result, error) {
	return r.stopWorkloads(ctx, optimize, held.condition, held.workloadNote, func(string) bool { return true })
}

// keepAtZero reports the Optimize workloads that a suspension left at zero
// while a check of this instance still fails, and whether it found any. It
// writes none of them, see workloadsuspend.KeepAtZero.
func (r *Reconciler) keepAtZero(
	ctx context.Context,
	optimize *v1.CamundaOptimize,
) (bool, error) {
	return workloadsuspend.KeepAtZero(
		ctx,
		optimize,
		components.ConditionTypes(),
		func(ctx context.Context, held func(string) bool) ([]workloadsuspend.Workload, error) {
			return r.optimizeWorkloads(ctx, optimize, held)
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
	// A read that failed halfway still returns what it reached, and those
	// workloads stop. The importer writes Elasticsearch, so leaving it up
	// because the webapp could not be read is the state this path exists to
	// prevent.
	workloads, readErr := r.optimizeWorkloads(ctx, optimize, stop)

	result, stopErr := workloadsuspend.StopWorkloadsIf(
		ctx,
		r.Client,
		optimize,
		workloads,
		message,
		stop,
		func(workload string, err error) { r.recordStop(optimize, workload, reason, err) },
	)

	return result, errors.Join(readErr, stopErr)
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
	for _, comp := range components.StopOrder() {
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

// recordStop records what this pass did with the named workload: it scaled it
// to zero for the reason of the wait that lowered it, or the API server refused.
func (r *Reconciler) recordStop(
	optimize *v1.CamundaOptimize,
	workload, reason string,
	err error,
) {
	if err != nil {
		r.EventRecorder.Eventf(
			optimize,
			nil,
			corev1.EventTypeWarning,
			workloadsuspend.EventReasonWorkloadStopRefused,
			workloadsuspend.EventActionSuspend,
			"Could not scale %q to zero, so it keeps running: %v",
			workload,
			err,
		)

		return
	}

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
	for _, conditionType := range components.ConditionTypes() {
		condition := meta.FindStatusCondition(optimize.Status.Conditions, conditionType)
		if condition != nil && workloadsuspend.IsSuspensionReason(condition.Reason) {
			return true
		}
	}

	return false
}
