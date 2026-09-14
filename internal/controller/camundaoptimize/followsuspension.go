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
	"slices"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundaoptimize"
	"github.com/konsole-is/camunda-operator/pkg/workloadsuspend"
)

// eventReasonWorkloadsSuspended is recorded for each Optimize workload that
// the suspension of the referenced cluster scales to zero while a check of
// this CamundaOptimize fails.
const eventReasonWorkloadsSuspended = "WorkloadsSuspended"

// suspendNote is appended to the failure message of a CamundaOptimize whose
// workloads the suspension of its cluster stopped, so Ready says both what
// failed and what happened to the workloads.
const suspendNote = ". The Optimize workloads are scaled to zero because CamundaCluster %q is suspended"

// suspendedMessage is the message of the condition of a workload that the
// suspension of the referenced cluster stopped and whose pods are gone.
const suspendedMessage = "Scaled to zero while the referenced cluster is suspended"

// keptAtZeroMessage is the message of that condition once the cluster resumed
// while a check of this CamundaOptimize still fails. The workload stays where
// the suspension left it, so the condition must not keep naming a suspension
// that is over.
const keptAtZeroMessage = "Kept at zero until the reference check passes"

// keptAtZeroNote is appended to the failure message of a CamundaOptimize whose
// workloads a suspension left at zero and whose cluster resumed.
const keptAtZeroNote = ". The Optimize workloads stay at zero until the reference check passes"

// suspensionOutcome reports what followSuspension did with the workloads.
type suspensionOutcome struct {
	// Found is true when a Deployment that this CamundaOptimize controls
	// exists, whatever happened to it. It gates the note on Ready, which says
	// that the workloads stopped.
	Found bool
	// Stopped is true when at least one of those Deployments is at zero, by
	// this pass or an earlier one. The transition event follows it: a pass that
	// stopped none staged no condition either, so the next retry reads no
	// suspension and would record the transition again.
	Stopped bool
}

// followsSuspendedCluster reports whether the last pass already had the
// Optimize workloads following a suspension of the referenced cluster.
//
// wasSuspending does not serve on the pre-check failure path: Ready carries the
// failure there, never a suspension reason, so it reads false on every pass and
// the transition event repeats. The condition of a workload carries the state
// instead, because followSuspension is what writes it. Every status on the way
// to suspended counts, so a reconcile that catches the drain reads it as
// already suspended.
func followsSuspendedCluster(optimize *v1.CamundaOptimize) bool {
	for _, comp := range suspendOrder() {
		conditionType, ok := components.ConditionTypeFor(comp)
		if !ok {
			continue
		}

		condition := meta.FindStatusCondition(optimize.Status.Conditions, conditionType)
		if condition != nil && suspensionReason(condition.Reason) {
			return true
		}
	}

	return false
}

// followSuspension scales the webapp and the importer to zero and keeps
// everything else: the Deployments, the Services, and the copies of the
// referenced Secrets. The condition of each workload it stops reports the
// suspension, because the components do not run on this path.
//
// The importer goes first, and both are tried with their errors joined. It is
// the workload that writes Elasticsearch, so a webapp that a conflict or an
// admission rule keeps up must not keep the importer up with it.
func (r *Reconciler) followSuspension(
	ctx context.Context,
	optimize *v1.CamundaOptimize,
) (suspensionOutcome, error) {
	var outcome suspensionOutcome
	var errs []error
	for _, comp := range suspendOrder() {
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
		if !metav1.IsControlledBy(&deployment, optimize) {
			continue
		}
		outcome.Found = true

		stopped, err := workloadsuspend.StopAtZero(ctx, r.Client, &deployment, suspendedMessage)
		if err != nil {
			errs = append(errs, err)

			continue
		}
		if stopped.Patched {
			r.recordSuspended(optimize, deployment.Name)
		}
		outcome.Stopped = true
		stageSuspension(optimize, comp, stopped)
	}

	return outcome, errors.Join(errs...)
}

// stageSuspension sets the condition of the given workload, in memory, from what
// the suspension did to it. The watch on the Deployment brings the reconcile back
// as its replicas drop. An unknown workload changes nothing.
func stageSuspension(optimize *v1.CamundaOptimize, comp string, outcome workloadsuspend.Outcome) {
	conditionType, ok := components.ConditionTypeFor(comp)
	if !ok {
		return
	}

	meta.SetStatusCondition(optimize.GetStatusConditions(), metav1.Condition{
		Type:               conditionType,
		Status:             outcome.Status,
		Reason:             outcome.Reason,
		Message:            outcome.Message,
		ObservedGeneration: optimize.Generation,
	})
}

// recordSuspended records that the suspension of the referenced cluster stopped
// the named workload.
func (r *Reconciler) recordSuspended(optimize *v1.CamundaOptimize, workload string) {
	r.EventRecorder.Eventf(
		optimize,
		nil,
		corev1.EventTypeNormal,
		eventReasonWorkloadsSuspended,
		eventActionSuspend,
		"Scaled %q to zero because CamundaCluster %q is suspended",
		workload,
		optimize.Spec.ClusterRef.Name,
	)
}

// stageKeptAtZero restages the workload conditions of a CamundaOptimize whose
// cluster resumed while a check of this instance still fails, and reports
// whether it restaged any.
//
// Nothing renders on that path, so the workloads stay where the suspension left
// them and only a pass of the check raises their replicas again. Their
// conditions would keep naming a suspension of the cluster that is over.
func stageKeptAtZero(optimize *v1.CamundaOptimize) bool {
	var kept bool
	for _, comp := range suspendOrder() {
		conditionType, ok := components.ConditionTypeFor(comp)
		if !ok {
			continue
		}

		condition := meta.FindStatusCondition(optimize.Status.Conditions, conditionType)
		if condition == nil || !suspensionReason(condition.Reason) {
			continue
		}
		kept = true

		meta.SetStatusCondition(optimize.GetStatusConditions(), metav1.Condition{
			Type:               conditionType,
			Status:             metav1.ConditionTrue,
			Reason:             string(component.Suspended),
			Message:            keptAtZeroMessage,
			ObservedGeneration: optimize.Generation,
		})
	}

	return kept
}

// suspendOrder returns the Optimize workloads with the importer first. It is the
// workload that writes Elasticsearch, so a webapp that a conflict or an
// admission rule keeps up must not keep the importer up with it.
func suspendOrder() []string {
	order := components.Workloads()
	slices.Reverse(order)

	return order
}
