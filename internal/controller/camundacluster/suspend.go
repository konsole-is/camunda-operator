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

package camundacluster

import (
	"context"
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

// eventReasonWorkloadsSuspended is recorded when a failed pre-check scales a
// workload of the cluster to zero.
const eventReasonWorkloadsSuspended = "WorkloadsSuspended"

// processConditions maps the component label of a workload to the condition
// that its process reports.
var processConditions = map[string]string{
	components.ComponentZeebe:      v1.ConditionZeebeReady,
	components.ComponentGateway:    v1.ConditionGatewayReady,
	components.ComponentOperate:    v1.ConditionOperateReady,
	components.ComponentTasklist:   v1.ConditionTasklistReady,
	components.ComponentAdmin:      v1.ConditionAdminReady,
	components.ComponentConnectors: v1.ConditionConnectorsReady,
}

// suspendWorkloads scales every workload that cluster controls to zero and
// keeps everything else: the volumes, the Services, and the Secrets. A failed
// pre-check leaves no input to render from, and workloads that keep running
// on their last configuration can write a backend the cluster no longer
// resolves, so they stop until the pre-check passes. The per-process
// condition of each workload reports the suspension the way a component
// does: Suspending while replicas stop, Suspended at zero. It reports
// whether it found a workload that the cluster controls. The reconcile comes
// back through the watch on the workloads, so the conditions converge
// without a timer.
func (r *CamundaClusterReconciler) suspendWorkloads(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) (bool, error) {
	var found bool
	suspend := func(obj client.Object, replicas *int32, observed int32) error {
		if !metav1.IsControlledBy(obj, cluster) {
			return nil
		}
		found = true

		if replicas == nil || *replicas != 0 {
			if err := r.scaleToZero(ctx, cluster, obj); err != nil {
				return err
			}
		}
		stageSuspension(cluster, obj.GetLabels()[labels.ComponentKey], observed)

		return nil
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
		return false, fmt.Errorf("listing the StatefulSets of the cluster: %w", err)
	}
	for i := range sets.Items {
		if err := suspend(&sets.Items[i], sets.Items[i].Spec.Replicas, sets.Items[i].Status.Replicas); err != nil {
			return false, err
		}
	}

	var deployments appsv1.DeploymentList
	if err := r.APIReader.List(ctx, &deployments, selector...); err != nil {
		return false, fmt.Errorf("listing the Deployments of the cluster: %w", err)
	}
	for i := range deployments.Items {
		item := &deployments.Items[i]
		if err := suspend(item, item.Spec.Replicas, item.Status.Replicas); err != nil {
			return false, err
		}
	}

	return found, nil
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
		"Scaled %q to zero because the pre-check failed",
		obj.GetName(),
	)

	return nil
}

// stageSuspension sets the per-process condition of the workload with the
// given component label, in memory, the way ocf reports a suspension:
// Suspended and True once the workload observes zero replicas, Suspending
// and False while they stop. A workload without a process condition, or an
// unknown component, changes nothing.
func stageSuspension(cluster *v1.CamundaCluster, comp string, observed int32) {
	conditionType, ok := processConditions[comp]
	if !ok {
		return
	}

	condition := metav1.Condition{
		Type:               conditionType,
		Status:             metav1.ConditionTrue,
		Reason:             string(component.Suspended),
		Message:            "Scaled to zero while the pre-check of the cluster fails",
		ObservedGeneration: cluster.Generation,
	}
	if observed != 0 {
		condition.Status = metav1.ConditionFalse
		condition.Reason = string(component.Suspending)
		condition.Message = fmt.Sprintf("Waiting for %d replicas to stop", observed)
	}
	meta.SetStatusCondition(cluster.GetStatusConditions(), condition)
}
