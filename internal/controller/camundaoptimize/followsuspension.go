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

// suspendNote is appended to the failure message of a CamundaOptimize whose
// workloads the suspension of its cluster stopped.
const suspendNote = ". The Optimize workloads are scaled to zero because CamundaCluster %q is suspended"

// suspendedMessage is the message of the condition of a workload that the
// suspension of the referenced cluster stopped and whose pods are gone.
const suspendedMessage = "Scaled to zero while the referenced cluster is suspended"

// keptAtZeroNote is appended to the failure message of a CamundaOptimize whose
// workloads a suspension left at zero and whose cluster resumed.
const keptAtZeroNote = ". The Optimize workloads that stopped stay at zero until the reference check passes"

// suspendOrder is the Optimize workloads, the importer first. It is the workload
// that writes Elasticsearch, so a webapp that a conflict or an admission rule
// keeps up must not keep the importer up with it.
var suspendOrder = []string{components.ComponentImporter, components.ComponentWebapp}

// followSuspension scales the webapp and the importer to zero and keeps
// everything else: the Deployments, the Services, and the copies of the
// referenced Secrets. The condition of each workload it stops reports the
// suspension, because the components do not run on this path.
func (r *Reconciler) followSuspension(
	ctx context.Context,
	optimize *v1.CamundaOptimize,
) (workloadsuspend.Result, error) {
	return r.stopWorkloads(ctx, optimize, suspendedMessage, func(string) bool { return true })
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
	return r.stopWorkloads(
		ctx,
		optimize,
		workloadsuspend.MessageKeptAtZero,
		func(conditionType string) bool {
			return workloadsuspend.IsAlreadySuspended(optimize, conditionType)
		},
	)
}

// stopWorkloads scales the Optimize workloads that stop accepts to zero. message
// says why a workload is held there once its pods are gone.
//
// The importer goes first. It is the workload that writes Elasticsearch, so a
// webapp that a conflict or an admission rule keeps up must not keep the
// importer up with it.
func (r *Reconciler) stopWorkloads(
	ctx context.Context,
	optimize *v1.CamundaOptimize,
	message string,
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
		func(workload string) { r.recordSuspended(optimize, workload) },
	)
}

// optimizeWorkloads reads the Deployments that stop accepts, with the condition
// each one reports. A Deployment that is gone needs nothing.
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
		if !ok {
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

// recordSuspended records that the suspension of the referenced cluster stopped
// the named workload.
func (r *Reconciler) recordSuspended(optimize *v1.CamundaOptimize, workload string) {
	r.EventRecorder.Eventf(
		optimize,
		nil,
		corev1.EventTypeNormal,
		workloadsuspend.EventReasonWorkloadsSuspended,
		workloadsuspend.EventActionSuspend,
		"Scaled %q to zero because CamundaCluster %q is suspended",
		workload,
		optimize.Spec.ClusterRef.Name,
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
