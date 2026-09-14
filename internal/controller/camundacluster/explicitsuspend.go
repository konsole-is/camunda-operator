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
	"slices"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
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

// suspendedNote and keptAtZeroReason say, in the event of a scaled workload, why
// this controller lowered it.
const (
	suspendedNote    = "because spec.suspend is set"
	keptAtZeroReason = "because the workloads that stopped stay at zero until the reference check passes"
)

// keptAtZeroNote is appended to the failure message of a cluster whose
// workloads a suspension left at zero and whose suspension ended.
const keptAtZeroNote = ". The workloads that stopped stay at zero until the reference check passes"

// suspendExplicitly scales every workload that cluster controls to zero when
// spec.suspend is set, and keeps everything else: the volumes, the Services, and
// the Secrets. It does nothing for a cluster that does not set the field.
func (r *CamundaClusterReconciler) suspendExplicitly(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) (bool, error) {
	if !cluster.Spec.Suspend {
		return false, nil
	}

	// A read that failed halfway still returns what it reached, and those
	// workloads stop. The one it could not read keeps running either way, so
	// leaving the rest up adds nothing but a writer on a backend that the user
	// asked to go quiet.
	workloads, readErr := r.clusterWorkloads(ctx, cluster)

	result, stopErr := workloadsuspend.StopWorkloadsIf(
		ctx,
		r.Client,
		cluster,
		workloads,
		suspendedMessage,
		func(string) bool { return true },
		func(workload string) { r.recordSuspended(cluster, workload, suspendedNote) },
	)

	return result.Found, errors.Join(readErr, stopErr)
}

// keepAtZero holds the workloads that a suspension stopped while the pre-check
// still fails, and reports whether it held any.
//
// The conditions are read first, in memory: a cluster that never suspended has
// nothing to hold, which is every failing pass of most clusters, and reading its
// workloads would answer a question already answered. The candidates are then
// read live, so a drain that finished since the suspension ended is reported as
// finished rather than copied from the condition that caught it mid-drain.
func (r *CamundaClusterReconciler) keepAtZero(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) (bool, error) {
	held := func(conditionType string) bool {
		return workloadsuspend.IsAlreadySuspended(cluster, conditionType)
	}
	if !slices.ContainsFunc(components.ConditionTypes(), held) {
		return false, nil
	}

	workloads, readErr := r.clusterWorkloads(ctx, cluster)

	result, stopErr := workloadsuspend.StopWorkloadsIf(
		ctx,
		r.Client,
		cluster,
		workloads,
		workloadsuspend.MessageKeptAtZero,
		held,
		func(workload string) { r.recordSuspended(cluster, workload, keptAtZeroReason) },
	)

	return result.Found, errors.Join(readErr, stopErr)
}

// clusterWorkloads returns the workloads of the cluster with the condition each
// one reports. A workload whose component reports no condition is left out: the
// suspension has nothing to say about it.
//
// The reads are live. The decision to stop a workload reads its replicas, and a
// render that raised them lands on the API server before an informer carries it,
// so a cached copy can report none for a workload that runs.
func (r *CamundaClusterReconciler) clusterWorkloads(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) ([]workloadsuspend.Workload, error) {
	selector := []client.ListOption{
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels(map[string]string{
			labels.ClusterKey:   labels.OwnerName(cluster.Name),
			labels.ManagedByKey: labels.ManagedBy,
		}),
	}

	var errs []error
	var workloads []workloadsuspend.Workload
	add := func(obj client.Object) {
		conditionType, ok := components.ConditionTypeFor(obj.GetLabels()[labels.ComponentKey])
		if !ok {
			return
		}
		workloads = append(workloads, workloadsuspend.Workload{Object: obj, ConditionType: conditionType})
	}

	var sets appsv1.StatefulSetList
	if err := r.APIReader.List(ctx, &sets, selector...); err != nil {
		errs = append(errs, fmt.Errorf("listing the StatefulSets of the cluster: %w", err))
	}
	for i := range sets.Items {
		add(&sets.Items[i])
	}

	var deployments appsv1.DeploymentList
	if err := r.APIReader.List(ctx, &deployments, selector...); err != nil {
		errs = append(errs, fmt.Errorf("listing the Deployments of the cluster: %w", err))
	}
	for i := range deployments.Items {
		add(&deployments.Items[i])
	}

	return workloads, errors.Join(errs...)
}

// endpointsStopped reports whether the process that serves the endpoints of the
// cluster carries a suspension, so status.gateway and status.management would
// answer nothing.
//
// The gateway serves them, or the brokers when it is embedded, which the gateway
// condition reports as Disabled, see binding.go. A workload whose stop was
// refused keeps serving, so its endpoints stay published.
func endpointsStopped(cluster *v1.CamundaCluster) bool {
	gateway := meta.FindStatusCondition(cluster.Status.Conditions, v1.ConditionGatewayReady)
	if gateway == nil {
		return false
	}

	serving := gateway
	if gateway.Reason == string(component.Disabled) {
		serving = meta.FindStatusCondition(cluster.Status.Conditions, v1.ConditionZeebeReady)
	}

	return serving != nil && workloadsuspend.IsSuspensionReason(serving.Reason)
}

// recordSuspended records that the named workload was scaled to zero, and why.
// The reason differs per path: an explicit suspend is the field of the user, and
// a hold is this controller putting back a workload that something raised while
// it has nothing to render from.
func (r *CamundaClusterReconciler) recordSuspended(
	cluster *v1.CamundaCluster,
	workload, reason string,
) {
	r.EventRecorder.Eventf(
		cluster,
		nil,
		corev1.EventTypeNormal,
		workloadsuspend.EventReasonWorkloadsSuspended,
		workloadsuspend.EventActionSuspend,
		"Scaled %q to zero %s",
		workload,
		reason,
	)
}
