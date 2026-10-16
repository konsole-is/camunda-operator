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

package camundamanagementcluster

import (
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"

	apiv1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/internal/readycondition"
)

// The handlers in this file replace the scaffolded defaults that the ocf
// generator writes into builder.go. After a run of the generator with
// --force, delete the scaffolded Default* handlers from builder.go again.

// DefaultConvergingStatusHandler reports Healthy while the CamundaManagementCluster
// reports Ready=True for its current generation, also while it is
// suspended. In every other state the reason holds the Ready reason and
// message of the CamundaManagementCluster unchanged, so the condition of the
// component shows them. Ready=False with the reason Degraded or a reason
// that ends convergence, such as Failing, is Failing. The rest is Creating
// on the first apply and Updating afterwards.
func DefaultConvergingStatusHandler(
	op concepts.ConvergingOperation, plane *apiv1.CamundaManagementCluster,
) (concepts.AliveStatusWithReason, error) {
	return readycondition.VerbatimConverging(op, observed(plane)), nil
}

// observed is the part of the CamundaManagementCluster that the handlers read.
func observed(plane *apiv1.CamundaManagementCluster) readycondition.Object {
	return readycondition.Object{
		Kind:               "CamundaManagementCluster",
		Generation:         plane.Generation,
		ObservedGeneration: plane.Status.ObservedGeneration,
		Conditions:         plane.Status.Conditions,
	}
}

// DefaultGraceStatusHandler grades a CamundaManagementCluster that is still not
// healthy after the grace period, with the reason of
// DefaultConvergingStatusHandler: Ready=False with the reason Degraded is
// Degraded, and every other state is Down.
func DefaultGraceStatusHandler(plane *apiv1.CamundaManagementCluster) (concepts.GraceStatusWithReason, error) {
	return readycondition.VerbatimGrace(observed(plane)), nil
}

// DefaultSuspendMutationHandler records no change. A claim suspends a
// CamundaManagementCluster through spec.suspend, not through the suspension
// of the component.
func DefaultSuspendMutationHandler(_ *Mutator) error {
	return nil
}

// DefaultSuspensionStatusHandler reports Suspended at once, because the
// suspension of the component changes nothing on the CamundaManagementCluster.
func DefaultSuspensionStatusHandler(_ *apiv1.CamundaManagementCluster) (concepts.SuspensionStatusWithReason, error) {
	return concepts.SuspensionStatusWithReason{
		Status: concepts.SuspensionStatusSuspended,
		Reason: "The suspension of the component leaves the CamundaManagementCluster as it is",
	}, nil
}

// DefaultDeleteOnSuspendHandler keeps the CamundaManagementCluster on suspension.
func DefaultDeleteOnSuspendHandler(_ *apiv1.CamundaManagementCluster) bool {
	return false
}
