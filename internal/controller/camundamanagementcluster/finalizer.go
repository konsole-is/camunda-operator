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
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
)

// The event vocabulary of this controller.
const (
	// eventActionFinalize is the action of the events that the controller
	// records while it deletes a CamundaManagementCluster.
	eventActionFinalize = "Finalize"
	// eventReasonAttachmentRemoved is recorded when the finalizer withdraws
	// the claims and deletes the ManagementAuthConfig.
	eventReasonAttachmentRemoved = "AttachmentRemoved"
)

// finalize withdraws every claim, deletes the ManagementAuthConfig, and
// releases the finalizer. The Deployments, the Services, and the copies of
// referenced Secrets carry an owner reference, so Kubernetes collects them;
// the claims and the contract are outside that chain.
//
// The withdrawal goes first. Once the finalizer is gone the object is gone for
// good, and no retry can free the orchestration clusters or remove a contract
// that names Secrets nothing reads.
func (r *Reconciler) finalize(ctx context.Context, mc *v1.CamundaManagementCluster) error {
	if !controllerutil.ContainsFinalizer(mc, Finalizer) {
		return nil
	}

	if err := r.withdrawClaims(ctx, mc); err != nil {
		return err
	}
	if err := r.withdrawContract(ctx, mc); err != nil {
		return err
	}
	r.EventRecorder.Eventf(
		mc,
		nil,
		corev1.EventTypeNormal,
		eventReasonAttachmentRemoved,
		eventActionFinalize,
		"Withdrew the claims on the orchestration clusters and removed ManagementAuthConfig %q",
		components.ContractName(mc),
	)

	controllerutil.RemoveFinalizer(mc, Finalizer)
	if err := r.Update(ctx, mc); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("removing the finalizer: %w", err)
	}

	return nil
}
