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
	corev1 "k8s.io/api/core/v1"
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
// the Optimize client.
const (
	eventReasonOptimizeCallbacks = "OptimizeCallbacksRegistered"
	eventActionRegister          = "Register"
)

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
// yet and removes the callbacks of an Optimize that went away. The
// KEYCLOAK_INIT_OPTIMIZE_ROOT_URL of the rendered environment is the floor: an
// Identity that restarts writes the list of its last roll, and this step puts
// the rest back on the reconcile that the restart triggers.
//
// A returned *conditions.PreCheckFailure is already staged on the condition.
// The caller folds it into Ready and comes back on the retry interval. Only a
// failure of the Kubernetes API comes back as an error.
func (r *Reconciler) syncOptimizeCallbacks(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	res resolved,
) (*conditions.PreCheckFailure, error) {
	provider := res.Input.Provider
	if provider.Mode == components.ModeOIDC {
		stageCallbacks(mc, metav1.ConditionTrue, string(component.Disabled),
			"The identity provider of the platform config holds the callback URLs of Optimize")

		return nil, nil
	}

	admin, failure, err := r.keycloakAdmin(ctx, mc, provider)
	if err != nil || failure != nil {
		return stageCallbackFailure(mc, failure), err
	}

	desired := components.OptimizeCallbacks(res.Input)
	clientID := provider.Clients.Optimize.ID

	stored, err := admin.FindClient(ctx, clientID)
	if err != nil {
		return stageCallbackFailure(mc, &conditions.PreCheckFailure{
			Reason:  v1.ReasonConnectionFailed,
			Message: conditions.BoundMessage(err.Error()),
		}), nil
	}
	if stored == nil {
		return reportNoClient(mc, clientID, desired), nil
	}

	current := stored.RedirectURIs()
	merged := keycloakadmin.MergeRedirectURIs(current, desired, components.OptimizeCallbackPath)
	if !slices.Equal(current, merged) {
		stored.SetRedirectURIs(merged)
		if err := admin.UpdateClient(ctx, stored); err != nil {
			return stageCallbackFailure(mc, &conditions.PreCheckFailure{
				Reason:  v1.ReasonWriteFailed,
				Message: conditions.BoundMessage(err.Error()),
			}), nil
		}
		r.EventRecorder.Eventf(
			mc,
			nil,
			corev1.EventTypeNormal,
			eventReasonOptimizeCallbacks,
			eventActionRegister,
			"Client %q of realm %q now carries %d redirect URIs",
			clientID,
			provider.Realm,
			len(merged),
		)
	}

	if len(desired) == 0 {
		stageCallbacks(mc, metav1.ConditionTrue, v1.ReasonNoCallbacks,
			fmt.Sprintf("No Optimize behind this management plane names a URL, so client %q carries "+
				"no login callback of this operator", clientID))

		return nil, nil
	}

	stageCallbacks(mc, metav1.ConditionTrue, v1.ReasonHealthy,
		fmt.Sprintf("Client %q of realm %q carries the login callback of every Optimize (%d)",
			clientID, provider.Realm, len(desired)))

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

// reportNoClient reports a realm that holds no Optimize client. Management
// Identity creates the client from its optimize preset on its first start
// against Keycloak, and it renders that preset only for a management plane
// that serves at least one Optimize.
func reportNoClient(
	mc *v1.CamundaManagementCluster,
	clientID string,
	desired []string,
) *conditions.PreCheckFailure {
	if len(desired) == 0 {
		stageCallbacks(mc, metav1.ConditionTrue, v1.ReasonNoCallbacks,
			fmt.Sprintf("No Optimize behind this management plane names a URL, so realm %q holds no "+
				"%q client", mc.Name, clientID))

		return nil
	}

	return stageCallbackFailure(mc, &conditions.PreCheckFailure{
		Reason: v1.ReasonOptimizeClientMissing,
		Message: fmt.Sprintf("The realm holds no %q client yet; Management Identity creates it on its "+
			"first start against Keycloak", clientID),
	})
}

// stageCallbackFailure stages failure on OptimizeCallbacksReady and returns
// it, so that a caller can report and return in one line.
func stageCallbackFailure(
	mc *v1.CamundaManagementCluster,
	failure *conditions.PreCheckFailure,
) *conditions.PreCheckFailure {
	if failure == nil {
		return nil
	}
	stageCallbacks(mc, metav1.ConditionFalse, failure.Reason, failure.Message)

	return failure
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
