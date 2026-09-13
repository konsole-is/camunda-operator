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

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// eventReasonWorkloadsSuspended is recorded for each workload that an explicit
// suspend scales to zero while the pre-check of the cluster fails.
const eventReasonWorkloadsSuspended = "WorkloadsSuspended"

// suspendNote is appended to the failure message of a cluster whose workloads
// an explicit suspend stopped, so Ready says both what failed and what
// happened to the workloads.
const suspendNote = ". The workloads are scaled to zero because spec.suspend is set"

// suspendedMessage is the message of the per-process condition of a workload
// that an explicit suspend stopped.
const suspendedMessage = "Scaled to zero because spec.suspend is set"

// processConditions maps the component label of a workload to the condition
// that its process reports. It mirrors Process.ConditionType in
// pkg/components/camundacluster: a failed pre-check has no effective spec to
// build the topology from, so the pairs are read from the label instead.
var processConditions = map[string]string{
	components.ComponentZeebe:      v1.ConditionZeebeReady,
	components.ComponentGateway:    v1.ConditionGatewayReady,
	components.ComponentOperate:    v1.ConditionOperateReady,
	components.ComponentTasklist:   v1.ConditionTasklistReady,
	components.ComponentAdmin:      v1.ConditionAdminReady,
	components.ComponentConnectors: v1.ConditionConnectorsReady,
}

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
	suspend := func(obj client.Object, replicas *int32) {
		if !metav1.IsControlledBy(obj, cluster) {
			return
		}
		found = true

		// The components do not run on this path, so nothing else refreshes
		// the per-process conditions and they would report health over zero
		// pods.
		stageSuspension(cluster, obj.GetLabels()[labels.ComponentKey])

		if replicas != nil && *replicas == 0 {
			return
		}
		if err := r.scaleToZero(ctx, cluster, obj); err != nil {
			errs = append(errs, err)
		}
	}

	selector := []client.ListOption{
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels(map[string]string{
			labels.ClusterKey:   labels.OwnerName(cluster.Name),
			labels.ManagedByKey: labels.ManagedBy,
		}),
	}

	var sets appsv1.StatefulSetList
	if err := r.APIReader.List(ctx, &sets, selector...); err != nil {
		errs = append(errs, fmt.Errorf("listing the StatefulSets of the cluster: %w", err))
	}
	for i := range sets.Items {
		suspend(&sets.Items[i], sets.Items[i].Spec.Replicas)
	}

	var deployments appsv1.DeploymentList
	if err := r.APIReader.List(ctx, &deployments, selector...); err != nil {
		errs = append(errs, fmt.Errorf("listing the Deployments of the cluster: %w", err))
	}
	for i := range deployments.Items {
		suspend(&deployments.Items[i], deployments.Items[i].Spec.Replicas)
	}

	return found, errors.Join(errs...)
}

// scaleToZero patches the replicas of a workload to zero and records the
// event on the cluster. A workload that is already gone needs nothing. The
// ocf apply takes the field back with force when the cluster resumes.
func (r *CamundaClusterReconciler) scaleToZero(
	ctx context.Context,
	cluster *v1.CamundaCluster,
	obj client.Object,
) error {
	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	var kind string
	switch workload := obj.(type) {
	case *appsv1.StatefulSet:
		kind = "StatefulSet"
		workload.Spec.Replicas = new(int32(0))
	case *appsv1.Deployment:
		kind = "Deployment"
		workload.Spec.Replicas = new(int32(0))
	default:
		return fmt.Errorf("cannot scale a %T to zero", obj)
	}

	if err := r.Patch(ctx, obj, patch); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("scaling %s %s to zero: %w", kind, client.ObjectKeyFromObject(obj), err)
	}

	r.EventRecorder.Eventf(
		cluster,
		nil,
		corev1.EventTypeNormal,
		eventReasonWorkloadsSuspended,
		eventActionReconcile,
		"Scaled %q to zero because spec.suspend is set",
		obj.GetName(),
	)

	return nil
}

// stageSuspension sets the per-process condition of the workload with the
// given component label, in memory, to the ocf Suspended status. A workload
// whose component reports no condition changes nothing.
func stageSuspension(cluster *v1.CamundaCluster, comp string) {
	conditionType, ok := processConditions[comp]
	if !ok {
		return
	}

	meta.SetStatusCondition(cluster.GetStatusConditions(), metav1.Condition{
		Type:               conditionType,
		Status:             metav1.ConditionTrue,
		Reason:             string(component.Suspended),
		Message:            suspendedMessage,
		ObservedGeneration: cluster.Generation,
	})
}
