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
	"slices"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/keycloakadmin"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
)

// The event that the controller records when it writes the redirect URIs of
// the Optimize client. One write can add a callback, remove one, or both, so
// the vocabulary names the write and not a direction.
const (
	eventReasonOptimizeCallbacks = "OptimizeCallbacksUpdated"
	eventActionUpdate            = "Update"
)

// discoverOptimizes finds the Optimize instances that this management plane
// serves, records them in status.optimize, and puts their URLs in the render
// input.
//
// The oidc mode discovers none. The identity provider of the platform config
// holds the callback URLs there, so spec.externalUrl of a CamundaOptimize has
// no effect and a row would say the management plane did something with it.
func (r *Reconciler) discoverOptimizes(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	res *resolved,
) error {
	if res.Input.Provider.Mode == components.ModeOIDC {
		mc.Status.Optimize = nil

		return nil
	}

	optimizes, err := r.listOptimizes(ctx, res.ContractName)
	if err != nil {
		return err
	}
	mc.Status.Optimize = components.AttachedOptimizes(optimizes)
	res.Input.OptimizeURLs = components.OptimizeURLs(mc.Status.Optimize)

	return nil
}

// listOptimizes reads every CamundaOptimize of the Kubernetes cluster that
// names contract in its spec.managementAuthRef.
//
// The list is read live rather than from the cache, the way listClusters is.
// The rendered environment of Management Identity and the redirect URIs of the
// Optimize client both come out of it, so a stale cache would withdraw the
// callback of an Optimize that was created moments ago.
func (r *Reconciler) listOptimizes(ctx context.Context, contract string) ([]v1.CamundaOptimize, error) {
	var list v1.CamundaOptimizeList
	if err := r.APIReader.List(ctx, &list); err != nil {
		return nil, fmt.Errorf("listing the CamundaOptimizes: %w", err)
	}

	var named []v1.CamundaOptimize
	for _, optimize := range list.Items {
		if optimize.Spec.ManagementAuthRef == contract {
			named = append(named, optimize)
		}
	}

	return named, nil
}

// syncOptimizeCallbacks gives the Optimize client of the realm the login
// callback of every Optimize this management plane serves, and reports the
// result on OptimizeCallbacksReady.
//
// Management Identity owns that client. It writes the whole representation
// again on every start, with the redirect URIs of its own environment, so this
// step adds what the environment of the running Identity pods does not carry
// yet and removes the callback of an Optimize that went away. The
// KEYCLOAK_INIT_OPTIMIZE_ROOT_URL of the rendered environment is the floor: an
// Identity that restarts writes the list of its last roll, and this step puts
// the rest back. The reconcile that the Deployment status of the restart
// brings usually does it at once, and the converge requeue of the caller
// bounds how long any other drift in the realm lasts.
//
// The failure it returns is already staged on the condition, and the caller
// folds it into Ready. The second result asks the caller to come back on the
// retry interval. Only a failure of the Kubernetes API comes back as an error.
//
// A management plane that serves no Optimize is never held back by the realm,
// because nobody can sign in to it either way. It still comes back while a
// callback it registered before is waiting to be withdrawn, and it stops
// calling Keycloak altogether once the condition reports that nothing of this
// operator is registered.
func (r *Reconciler) syncOptimizeCallbacks(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	res resolved,
	contractErr error,
) (*conditions.PreCheckFailure, bool, error) {
	// The two states that register nothing at all answer first. A plane that
	// administers no realm, and one whose desired state is to run nothing, are
	// not waiting on anything, so neither reports a prerequisite.
	provider := res.Input.Provider
	if provider.Mode == components.ModeOIDC {
		stageCallbacks(
			mc, metav1.ConditionTrue, string(component.Disabled),
			"The identity provider of the platform config holds the callback URLs of Optimize",
		)

		return nil, false, nil
	}
	// A suspended management plane runs no Keycloak in the keycloak mode and
	// serves nobody in either, so the realm is left as it is. Dialing a
	// Keycloak that is scaled to zero would report ConnectionFailed on a
	// resource whose desired state is exactly this.
	if res.Input.Suspended {
		stageCallbacks(
			mc, metav1.ConditionTrue, string(component.Suspended),
			"The management plane is suspended, so the realm is left as it is",
		)

		return nil, false, nil
	}
	// The contract is what makes a CamundaOptimize one of this plane's. Until
	// it is written, another plane can still be the one that owns the name, so
	// a realm written from that discovery would carry callbacks of Optimize
	// instances this plane does not serve.
	if contractErr != nil {
		stageCallbacks(
			mc, metav1.ConditionFalse, string(component.PrerequisiteNotMet),
			"The ManagementAuthConfig is not written, so the Optimize instances behind it are not "+
				"settled",
		)

		return nil, false, nil
	}

	desired := components.OptimizeCallbacks(res.Input)
	clientID := provider.Clients.Optimize.ID
	if len(desired) == 0 && nothingRegistered(mc) {
		stageNoCallbacks(mc)

		return nil, false, nil
	}
	// Management Identity writes the whole client representation while it
	// starts. This step reads that representation and writes it back with the
	// redirect URIs replaced, so a write of its own between the two calls would
	// revert what Identity just wrote. Waiting for the workload keeps the two
	// writers apart: Identity is done with the realm by the time it is ready.
	rolledOut, err := r.identityRolledOut(ctx, mc)
	if err != nil {
		return nil, false, err
	}
	if !rolledOut {
		stageCallbacks(
			mc, metav1.ConditionFalse, string(component.PrerequisiteNotMet),
			"Management Identity is rolling out, and it owns the Optimize client while it starts",
		)

		return nil, true, nil
	}

	failure, err := r.convergeOptimizeCallbacks(ctx, mc, provider, clientID, desired)
	if err != nil {
		return nil, false, err
	}
	// A realm that holds no Optimize client holds no callback of this
	// operator either, so a management plane that wants none has arrived. The
	// client is absent for the same reason: a plane with no Optimize renders
	// no preset for Management Identity to create it from.
	if len(desired) == 0 && (failure == nil || failure.Reason == v1.ReasonOptimizeClientMissing) {
		stageNoCallbacks(mc)

		return nil, false, nil
	}
	if failure != nil {
		stageCallbacks(mc, metav1.ConditionFalse, failure.Reason, failure.Message)
		if len(desired) == 0 {
			return nil, true, nil
		}

		return failure, true, nil
	}

	stageCallbacks(
		mc, metav1.ConditionTrue, v1.ReasonHealthy,
		fmt.Sprintf(
			"Client %q of realm %q carries every login callback of this management plane (%d)",
			clientID, provider.Realm, len(desired),
		),
	)

	return nil, false, nil
}

// identityRolledOut reports whether every Management Identity pod is the
// current one and ready.
//
// A ready condition is not enough. It is satisfied while one pod of the
// previous revision is still ready, and the pod of the new revision is running
// its initializer against the realm at exactly that moment. Only a rollout
// with every replica updated, present and available leaves no Identity writing
// to the client.
func (r *Reconciler) identityRolledOut(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
) (bool, error) {
	key := client.ObjectKey{Namespace: mc.Namespace, Name: components.IdentityName(mc)}

	var identity appsv1.Deployment
	if err := r.APIReader.Get(ctx, key, &identity); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("reading Deployment %q: %w", key, err)
	}

	wanted := int32(1)
	if identity.Spec.Replicas != nil {
		wanted = *identity.Spec.Replicas
	}
	status := identity.Status

	return identity.Generation == status.ObservedGeneration &&
		status.UpdatedReplicas == wanted &&
		status.Replicas == wanted &&
		status.AvailableReplicas == wanted, nil
}

// nothingRegistered reports whether the last reconcile already found no login
// callback of this operator in the realm. A management plane that rests there
// makes no call to Keycloak at all.
func nothingRegistered(mc *v1.CamundaManagementCluster) bool {
	condition := meta.FindStatusCondition(
		mc.Status.Conditions, v1.ConditionOptimizeCallbacksReady,
	)

	return condition != nil && condition.Reason == v1.ReasonNoCallbacks
}

// stageNoCallbacks reports a realm that holds no login callback of this
// management plane.
func stageNoCallbacks(mc *v1.CamundaManagementCluster) {
	stageCallbacks(
		mc, metav1.ConditionTrue, v1.ReasonNoCallbacks,
		"No Optimize behind this management plane names a URL, so no login callback of this "+
			"operator is registered in the realm",
	)
}

// convergeOptimizeCallbacks reads the client that clientID names and writes
// the merged redirect URIs back when they differ from the stored ones. It
// stages no condition; the caller decides what a failure means.
func (r *Reconciler) convergeOptimizeCallbacks(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	provider components.IdentityProvider,
	clientID string,
	desired []string,
) (*conditions.PreCheckFailure, error) {
	admin, failure, err := r.keycloakAdmin(ctx, mc, provider)
	if err != nil || failure != nil {
		return failure, err
	}

	stored, err := admin.FindClient(ctx, clientID)
	if err != nil {
		return &conditions.PreCheckFailure{
			Reason:  v1.ReasonConnectionFailed,
			Message: conditions.BoundMessage(err.Error()),
		}, nil
	}
	if stored == nil {
		return &conditions.PreCheckFailure{
			Reason: v1.ReasonOptimizeClientMissing,
			Message: fmt.Sprintf("The realm holds no %q client; Management Identity creates it "+
				"while it starts, so restart Management Identity to have it created again",
				clientID),
		}, nil
	}

	current := stored.RedirectURIs()
	merged := keycloakadmin.MergeRedirectURIs(current, desired, components.OptimizeCallbackPath)
	if slices.Equal(current, merged) {
		return nil, nil
	}

	stored.SetRedirectURIs(merged)
	if err := admin.UpdateClient(ctx, stored); err != nil {
		return &conditions.PreCheckFailure{
			Reason:  v1.ReasonWriteFailed,
			Message: conditions.BoundMessage(err.Error()),
		}, nil
	}
	r.EventRecorder.Eventf(
		mc,
		nil,
		corev1.EventTypeNormal,
		eventReasonOptimizeCallbacks,
		eventActionUpdate,
		"Client %q of realm %q now carries %d redirect URIs",
		clientID,
		provider.Realm,
		len(merged),
	)

	return nil, nil
}

// keycloakAdmin builds the administration client of the Keycloak that
// Management Identity bootstraps the realm in. It signs in with the same
// administrator and reaches the same URL that the rendered environment of
// Management Identity names, so nothing else has to be configured.
//
// The Secret of that administrator is absent until Keycloak runs, which is
// normal on a management plane that is still starting, so a missing one is a
// reported state and not an error.
func (r *Reconciler) keycloakAdmin(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	provider components.IdentityProvider,
) (*keycloakadmin.Client, *conditions.PreCheckFailure, error) {
	ref := provider.AdminCredentials
	if ref == nil {
		return nil, &conditions.PreCheckFailure{
			Reason:  v1.ReasonInvalidReference,
			Message: "The identity provider names no Keycloak administrator",
		}, nil
	}

	key := client.ObjectKey{Namespace: mc.Namespace, Name: ref.Name}
	secret, msg, err := secretref.Get(ctx, r.APIReader, key, ref.UsernameKey, ref.PasswordKey)
	if err != nil {
		return nil, nil, fmt.Errorf("reading Secret %q: %w", key, err)
	}
	if msg != "" {
		return nil, &conditions.PreCheckFailure{Reason: v1.ReasonMissingSecret, Message: msg}, nil
	}

	return keycloakadmin.New(
		provider.KeycloakURL,
		provider.Realm,
		string(secret.Data[ref.UsernameKey]),
		string(secret.Data[ref.PasswordKey]),
	), nil, nil
}

// stageCallbacks sets OptimizeCallbacksReady on the in-memory CR. The deferred
// FlushStatus of the reconcile persists it.
func stageCallbacks(
	mc *v1.CamundaManagementCluster,
	status metav1.ConditionStatus,
	reason, message string,
) {
	meta.SetStatusCondition(mc.GetStatusConditions(), metav1.Condition{
		Type:               v1.ConditionOptimizeCallbacksReady,
		Status:             status,
		Reason:             reason,
		Message:            conditions.BoundMessage(message),
		ObservedGeneration: mc.GetGeneration(),
	})
}
