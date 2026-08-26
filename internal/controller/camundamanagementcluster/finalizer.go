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
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
)

// The event vocabulary of this controller.
const (
	// eventActionFinalize is the action of the events that the controller
	// records while it deletes a CamundaManagementCluster.
	eventActionFinalize = "Finalize"
	// eventReasonAttachmentRemoved is recorded when the finalizer withdraws
	// what the management plane wrote on the orchestration clusters and
	// deletes the ManagementAuthConfig.
	eventReasonAttachmentRemoved = "AttachmentRemoved"
)

// finalize withdraws the Console ping settings and every claim, removes the
// login callbacks from the realm, deletes the ManagementAuthConfig, and
// releases the finalizer. The Deployments, the Services, and the copies of
// referenced Secrets carry an owner reference, so Kubernetes collects them;
// what the management plane wrote on an orchestration cluster, what it wrote in
// the realm, and the contract, are outside that chain.
//
// The withdrawal goes first. Once the finalizer is gone the object is gone for
// good, and no retry can free the orchestration clusters or remove a contract
// that names Secrets nothing reads.
func (r *Reconciler) finalize(ctx context.Context, mc *v1.CamundaManagementCluster) error {
	if !controllerutil.ContainsFinalizer(mc, Finalizer) {
		return nil
	}

	clusters, err := r.listClusters(ctx)
	if err != nil {
		return err
	}

	// The users go before the claims, so that no other management plane
	// adopts a web-modeler user that this one is about to remove.
	if err := r.withdrawWebModelerUsers(ctx, mc, clusters); err != nil {
		return err
	}
	if err := r.withdrawPing(ctx, mc, clusters); err != nil {
		return err
	}
	if err := r.withdrawClaims(ctx, mc, clusters); err != nil {
		return err
	}
	if err := r.withdrawContract(ctx, mc); err != nil {
		return err
	}
	r.withdrawOptimizeCallbacks(ctx, mc)
	r.EventRecorder.Eventf(
		mc,
		nil,
		corev1.EventTypeNormal,
		eventReasonAttachmentRemoved,
		eventActionFinalize,
		"Withdrew the claims and the Console ping settings on the orchestration clusters, "+
			"tried to remove the Web Modeler users and the login callbacks of Optimize (a failed "+
			"removal has a warning event or a log line of its own), and removed "+
			"ManagementAuthConfig %q",
		components.ContractName(mc),
	)

	controllerutil.RemoveFinalizer(mc, Finalizer)
	if err := r.Update(ctx, mc); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("removing the finalizer: %w", err)
	}

	return nil
}

// withdrawOptimizeCallbacks removes the login callbacks that this management
// plane registered on the Optimize client of the realm.
//
// It is best effort, and it never stops the deletion. A Keycloak that does not
// answer must not keep the resource alive, because the orchestration clusters
// it holds would stay claimed with nothing left to free them. A realm that the
// operator could not reach keeps the callbacks, and the log line says so.
//
// The oidc mode registered nothing, and a Keycloak that the operator ran goes
// with this resource, so only a Keycloak that you run keeps anything.
func (r *Reconciler) withdrawOptimizeCallbacks(ctx context.Context, mc *v1.CamundaManagementCluster) {
	if components.Mode(mc) == components.ModeOIDC {
		return
	}

	// The provider of a Keycloak mode follows from the spec alone, so the
	// deletion path needs none of the pre-checks.
	provider, err := components.ResolveIdentityProvider(components.Input{Cluster: mc})
	if err != nil {
		logf.FromContext(ctx).Error(err, "Resolving Keycloak to withdraw the Optimize callbacks")

		return
	}

	failure, err := r.convergeOptimizeCallbacks(
		ctx, mc, provider, provider.Clients.Optimize.ID, nil,
	)
	switch {
	case err != nil:
		logf.FromContext(ctx).Error(err, "Withdrawing the Optimize callbacks")
	case failure != nil && failure.Reason != v1.ReasonOptimizeClientMissing:
		logf.FromContext(ctx).Error(
			failure, "Withdrawing the Optimize callbacks", "reason", failure.Reason,
		)
	}
}
