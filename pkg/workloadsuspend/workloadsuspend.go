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

// Package workloadsuspend stops a workload that a controller holds at zero
// outside its render, and reports the drain the way ocf reports it.
//
// A controller whose pre-check failed has no input to render from, so the ocf
// components never run and neither the scaling nor the condition happens through
// them. The drain wording comes from the ocf suspension handlers, so a user
// reads the same sentence whether a component or a controller stopped the
// workload.
package workloadsuspend

import (
	"context"
	"fmt"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/deployment"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Outcome is what StopAtZero did with one workload, and what to report for it.
type Outcome struct {
	// Patched is true when this call lowered the replicas of the workload. A
	// workload that already ran none is left alone, so a caller that records an
	// event records it once.
	Patched bool
	// Status and Reason are the ocf suspension status read from the replicas the
	// workload still observes: Suspending while pods run, Suspended once they
	// are gone.
	Status metav1.ConditionStatus
	Reason string
	// Message is the drain wording of ocf while pods run, and the message the
	// caller passed once they are gone.
	Message string
}

// StopAtZero patches the replicas of a StatefulSet or a Deployment to zero and
// returns what to report for it. suspended is the message to carry once its pods
// are gone, which says why the caller holds this workload at zero.
//
// A workload that already runs none is not patched. A workload that is gone
// needs nothing: the outcome reports it suspended and no error, because a
// deleted workload runs no pods whatever it observed before.
func StopAtZero(
	ctx context.Context,
	writer client.Writer,
	obj client.Object,
	suspended string,
) (Outcome, error) {
	outcome, replicas, err := drain(obj, suspended)
	if err != nil {
		return Outcome{}, err
	}
	if replicas != nil && *replicas == 0 {
		return outcome, nil
	}

	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	setZero(obj)

	if err := writer.Patch(ctx, obj, patch); err != nil {
		if apierrors.IsNotFound(err) {
			// A workload deleted between the read and the patch runs no pods,
			// whatever the read observed.
			return Outcome{
				Status:  metav1.ConditionTrue,
				Reason:  string(component.Suspended),
				Message: suspended,
			}, nil
		}

		return outcome, fmt.Errorf(
			"scaling %T %s to zero: %w", obj, client.ObjectKeyFromObject(obj), err,
		)
	}
	outcome.Patched = true

	return outcome, nil
}

// drain reads the ocf suspension status of the workload and its desired
// replicas. The status comes from the observed replicas, so it is read before
// anything lowers the desired count.
func drain(obj client.Object, suspended string) (Outcome, *int32, error) {
	var status concepts.SuspensionStatusWithReason
	var replicas *int32
	var err error

	switch workload := obj.(type) {
	case *appsv1.StatefulSet:
		replicas = workload.Spec.Replicas
		status, err = statefulset.DefaultSuspensionStatusHandler(workload)
	case *appsv1.Deployment:
		replicas = workload.Spec.Replicas
		status, err = deployment.DefaultSuspensionStatusHandler(workload)
	default:
		return Outcome{}, nil, fmt.Errorf("cannot suspend a %T", obj)
	}
	if err != nil {
		return Outcome{}, nil, fmt.Errorf("reading the suspension status of %T: %w", obj, err)
	}

	outcome := Outcome{
		Status:  metav1.ConditionFalse,
		Reason:  string(component.Suspending),
		Message: status.Reason,
	}
	if status.Status == concepts.SuspensionStatusSuspended {
		outcome.Status = metav1.ConditionTrue
		outcome.Reason = string(component.Suspended)
		outcome.Message = suspended
	}

	return outcome, replicas, nil
}

// setZero lowers the desired replicas of the workload to zero. The ocf apply
// takes the field back with force once the caller renders the workload again.
func setZero(obj client.Object) {
	switch workload := obj.(type) {
	case *appsv1.StatefulSet:
		workload.Spec.Replicas = new(int32(0))
	case *appsv1.Deployment:
		workload.Spec.Replicas = new(int32(0))
	}
}
