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

// This file stops the workloads of a cluster that spec.suspend suspends while
// its pre-check fails. A failed pre-check leaves nothing to render from, so the
// suspended render never runs. Without this, the instruction of the user waits
// for a reference that it does not depend on. A broken Secret is exactly when a
// user reaches for suspend.

package camundacluster

import (
	"context"
	"errors"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/workloadsuspend"
)

// suspendNote is appended to the failure message of a cluster whose workloads
// an explicit suspend stopped, so Ready says both what failed and what
// happened to the workloads.
const suspendNote = ". The workloads are scaled to zero because spec.suspend is set"

// suspendedMessage is the message of the per-process condition of a workload
// that an explicit suspend stopped and whose pods are gone.
const suspendedMessage = "Scaled to zero because spec.suspend is set"

// keptAtZeroNote is appended to the failure message of a cluster whose
// workloads a suspension left at zero and whose suspension ended.
const keptAtZeroNote = ". The workloads that stopped stay at zero until the reference check passes"

// suspendExplicitly scales every workload that cluster controls to zero when
// spec.suspend is set, and keeps everything else: the volumes, the Services,
// and the Secrets. It does nothing for a cluster that does not set the field.
// It reports whether it found a workload that the cluster controls, so a
// cluster that never rendered one does not claim that any stopped.
//
// Every workload is tried and the errors are joined. One workload that a
// conflict or an admission rule keeps up must not leave the rest of them
// running.
func (r *CamundaClusterReconciler) suspendExplicitly(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) (bool, error) {
	if !cluster.Spec.Suspend {
		return false, nil
	}

	var found bool
	var errs []error
	suspend := func(obj client.Object) {
		if !metav1.IsControlledBy(obj, cluster) {
			return
		}
		found = true

		outcome, err := workloadsuspend.StopAtZero(ctx, r.Client, obj, suspendedMessage)
		if err != nil {
			errs = append(errs, err)

			return
		}
		if outcome.Patched {
			r.recordSuspended(cluster, obj.GetName())
		}
		// The components do not run on this path, so nothing else refreshes the
		// per-process conditions and they would report health over zero pods.
		stageSuspension(cluster, obj.GetLabels()[labels.ComponentKey], outcome)
	}

	selector := []client.ListOption{
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels(map[string]string{
			labels.ClusterKey:   labels.OwnerName(cluster.Name),
			labels.ManagedByKey: labels.ManagedBy,
		}),
	}

	// The reads are live. The decision to stop a workload reads its replicas,
	// and a render that raised them lands on the API server before an informer
	// carries it, so a cached copy can report none for a workload that runs.
	var sets appsv1.StatefulSetList
	if err := r.APIReader.List(ctx, &sets, selector...); err != nil {
		errs = append(errs, fmt.Errorf("listing the StatefulSets of the cluster: %w", err))
	}
	for i := range sets.Items {
		suspend(&sets.Items[i])
	}

	var deployments appsv1.DeploymentList
	if err := r.APIReader.List(ctx, &deployments, selector...); err != nil {
		errs = append(errs, fmt.Errorf("listing the Deployments of the cluster: %w", err))
	}
	for i := range deployments.Items {
		suspend(&deployments.Items[i])
	}

	return found, errors.Join(errs...)
}

// recordSuspended records that an explicit suspend stopped the named workload.
func (r *CamundaClusterReconciler) recordSuspended(cluster *v1.CamundaCluster, workload string) {
	r.EventRecorder.Eventf(
		cluster,
		nil,
		corev1.EventTypeNormal,
		workloadsuspend.EventReasonWorkloadsSuspended,
		workloadsuspend.EventActionSuspend,
		"Scaled %q to zero because spec.suspend is set",
		workload,
	)
}

// stageSuspension sets the per-process condition of the workload with the given
// component label. A workload whose component reports no condition changes
// nothing.
func stageSuspension(cluster *v1.CamundaCluster, comp string, outcome workloadsuspend.Outcome) {
	conditionType, ok := components.ConditionTypeFor(comp)
	if !ok {
		return
	}

	workloadsuspend.Stage(cluster, conditionType, outcome)
}
