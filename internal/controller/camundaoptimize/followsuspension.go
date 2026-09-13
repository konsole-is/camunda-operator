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

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundaoptimize"
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
// suspension of the referenced cluster stopped.
const suspendedMessage = "Scaled to zero while the referenced cluster is suspended"

// workloadConditions maps an Optimize workload to the condition that it
// reports.
var workloadConditions = map[string]string{
	components.ComponentWebapp:   v1.ConditionWebappReady,
	components.ComponentImporter: v1.ConditionImporterReady,
}

// followSuspension scales the webapp and the importer to zero and keeps
// everything else: the Deployments, the Services, and the copies of the
// referenced Secrets. It reports whether it found a Deployment that this
// CamundaOptimize controls, so an instance that never rendered one does not
// claim that any stopped. The condition of each workload it scales reports the
// suspension, because the components do not run on this path.
//
// The importer goes first, and both are tried with their errors joined. It is
// the workload that writes Elasticsearch, so a webapp that a conflict or an
// admission rule keeps up must not keep the importer up with it.
func (r *Reconciler) followSuspension(
	ctx context.Context,
	optimize *v1.CamundaOptimize,
) (bool, error) {
	var found bool
	var errs []error
	for _, comp := range []string{components.ComponentImporter, components.ComponentWebapp} {
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
		found = true
		stageSuspension(optimize, comp)

		if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas == 0 {
			continue
		}
		if err := r.scaleToZero(ctx, optimize, &deployment); err != nil {
			errs = append(errs, err)
		}
	}

	return found, errors.Join(errs...)
}

// stageSuspension sets the condition of the given workload, in memory, to the
// ocf Suspended status. An unknown workload changes nothing.
func stageSuspension(optimize *v1.CamundaOptimize, comp string) {
	conditionType, ok := workloadConditions[comp]
	if !ok {
		return
	}

	meta.SetStatusCondition(optimize.GetStatusConditions(), metav1.Condition{
		Type:               conditionType,
		Status:             metav1.ConditionTrue,
		Reason:             string(component.Suspended),
		Message:            suspendedMessage,
		ObservedGeneration: optimize.Generation,
	})
}

// scaleToZero patches the replicas of a Deployment to zero and records the
// event on the CamundaOptimize. The ocf apply takes the field back with force
// when the cluster resumes and the check passes.
func (r *Reconciler) scaleToZero(
	ctx context.Context,
	optimize *v1.CamundaOptimize,
	deployment *appsv1.Deployment,
) error {
	patch := client.MergeFrom(deployment.DeepCopy())
	deployment.Spec.Replicas = new(int32(0))

	if err := r.Patch(ctx, deployment, patch); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("scaling Deployment %s to zero: %w", client.ObjectKeyFromObject(deployment), err)
	}

	r.EventRecorder.Eventf(
		optimize,
		nil,
		corev1.EventTypeNormal,
		eventReasonWorkloadsSuspended,
		eventActionSuspend,
		"Scaled %q to zero because CamundaCluster %q is suspended",
		deployment.Name,
		optimize.Spec.ClusterRef.Name,
	)

	return nil
}
