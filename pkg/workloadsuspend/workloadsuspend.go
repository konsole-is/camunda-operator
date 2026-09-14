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
	"encoding/json"
	"fmt"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/deployment"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EventReasonWorkloadsSuspended is the event reason of a workload that a
// controller scaled to zero outside its render. The note says which suspension
// it followed, which differs per controller.
const EventReasonWorkloadsSuspended = "WorkloadsSuspended"

// EventActionSuspend is the action verb of that event.
const EventActionSuspend = "Suspend"

// MessageKeptAtZero is the message of the condition of a workload whose
// suspension ended while the controller still has nothing to render from. The
// workload stays where the suspension left it, so the condition must not keep
// naming a suspension that is over.
//
// A caller passes it to StopAtZero in place of its own suspended message. The
// workload is read again on that pass, so a drain that finished in the meantime
// is reported as finished.
const MessageKeptAtZero = "Kept at zero until the reference check passes"

// stopPatch is the body of the merge patch that stops a workload: the replicas
// it asks for, which are always none, and the UID the caller read it with.
//
// metadata.uid is immutable, so a patch that names one the object does not have
// is rejected as invalid. A workload deleted and recreated under the same name
// between the read and the patch carries another UID, and the patch fails rather
// than stopping a workload that belongs to someone else now.
type stopPatch struct {
	Metadata struct {
		UID types.UID `json:"uid"`
	} `json:"metadata"`
	Spec struct {
		Replicas int32 `json:"replicas"`
	} `json:"spec"`
}

// Outcome is what StopAtZero did with one workload, and what to report for it.
type Outcome struct {
	// Patched is true when this call lowered the desired replicas of the
	// workload. A workload that already asked for none is left alone, so a
	// caller that records an event records it once.
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
// A workload whose desired replicas are already zero is not patched, whatever it
// still observes. A workload that is gone needs nothing: the outcome reports it
// suspended and no error, because a deleted workload runs no pods whatever it
// observed before.
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

	patch, err := zeroReplicas(obj.GetUID())
	if err != nil {
		return Outcome{}, err
	}

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

// zeroReplicas builds the patch that stops the workload that uid names. The ocf
// apply takes the replicas back with force once the caller renders the workload
// again.
//
// This is the one managed write of the operator that is not a server-side apply.
// A partial apply under the field manager of the component would drop that
// manager's ownership of every field it left out, and the next render would have
// to take them all back. A merge patch touches the one field instead, under the
// default manager, which is what the controller copies this replaced did.
func zeroReplicas(uid types.UID) (client.Patch, error) {
	var body stopPatch
	body.Metadata.UID = uid

	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("building the patch that stops the workload: %w", err)
	}

	return client.RawPatch(types.MergePatchType, raw), nil
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

// IsSuspensionReason reports whether a condition reason is one of the ocf
// statuses on the way to suspended, or suspended itself. A workload that carries
// one is not running for the user, so it is one a controller staged.
func IsSuspensionReason(reason string) bool {
	return concepts.SuspensionStatus(reason).Priority() > 0
}

// Stage sets the condition of a workload that a controller holds at zero, in
// memory, from what StopAtZero did to it. The watch on the workload brings the
// reconcile back as its replicas drop.
func Stage(owner component.OperatorCRD, conditionType string, outcome Outcome) {
	meta.SetStatusCondition(owner.GetStatusConditions(), metav1.Condition{
		Type:               conditionType,
		Status:             outcome.Status,
		Reason:             outcome.Reason,
		Message:            outcome.Message,
		ObservedGeneration: owner.GetGeneration(),
	})
}
