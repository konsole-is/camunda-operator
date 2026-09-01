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
	"errors"
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
// the vocabulary names the write and not a direction. Every other write to
// the realm records the same action under a reason of its own.
const (
	eventReasonOptimizeCallbacks = "OptimizeCallbacksUpdated"
	eventActionUpdate            = "Update"
	// eventReasonCallbacksLeftBehind is recorded when the annotation
	// ForgetCallbackRealmAnnotation lets go of a realm that still carries
	// the login callbacks of this management plane.
	eventReasonCallbacksLeftBehind = "OptimizeCallbacksLeftBehind"
)

// rollingOut is what the condition says while Management Identity starts. It
// owns the Optimize client of the realm then, so the operator writes no realm
// at all until the rollout is over.
const rollingOut = "Management Identity is rolling out, and it owns the Optimize client while it starts"

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
// callback of every Optimize this management plane serves, gives the first
// administrator the Optimize role, and reports the result on
// OptimizeCallbacksReady.
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
//
// status.callbackRealm records the realm of a Keycloak that you run once the
// callbacks are there. A spec that names another realm, or the oidc mode,
// withdraws from the recorded realm before this step registers anywhere
// else, so a realm the spec no longer names never keeps signing people in.
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
		// A plane that registered callbacks in a Keycloak before this mode is
		// the one case where this mode still has a realm to tidy.
		failure, err := r.withdrawRetargeted(ctx, mc, res)
		if err != nil {
			return nil, false, err
		}
		if failure != nil {
			stageCallbacks(mc, metav1.ConditionFalse, failure.Reason, failure.Message)

			// Everybody signs in through the identity provider of the platform
			// config now, so the realm that is left over holds nobody back.
			return nil, true, nil
		}

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
	// The realm that the spec named before goes first. A realm that keeps the
	// callbacks signs people in to the same Optimize instances as the new one,
	// and no later reconcile can reach it once the record is gone.
	retargeted, err := r.withdrawRetargeted(ctx, mc, res)
	if err != nil {
		return nil, false, err
	}
	if retargeted != nil {
		stageCallbacks(mc, metav1.ConditionFalse, retargeted.Reason, retargeted.Message)
		if len(desired) == 0 {
			return nil, true, nil
		}

		return retargeted, true, nil
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
		stageCallbacks(mc, metav1.ConditionFalse, string(component.PrerequisiteNotMet), rollingOut)
		// A rollout that surges keeps IdentityReady True from the pod of the
		// previous revision, so no component reports that the realm is behind.
		// Ready would otherwise read Healthy over a callback that nobody can
		// sign in through yet. A withdrawal is not held back the same way: the
		// plane already serves every Optimize it has to serve.
		if len(desired) == 0 {
			return nil, true, nil
		}

		return &conditions.PreCheckFailure{
			Reason:  string(component.PrerequisiteNotMet),
			Message: rollingOut,
		}, true, nil
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

	// The realm carries the callbacks, so it is the realm to take them out of
	// when the spec names another one later. Only a Keycloak that you run is
	// recorded: the Keycloak that the operator runs is deleted by the same
	// change, and its realm goes with it.
	if provider.Mode == components.ModeExternalKeycloak {
		mc.Status.CallbackRealm = components.RealmTarget(provider)
	}

	// The Optimize client is there and the Optimize role came with it. The
	// first administrator can be one from before that role existed, and this
	// is where they are given it.
	grantFailure, err := r.grantAdminOptimizeRole(ctx, mc, provider)
	if err != nil {
		return nil, false, err
	}
	if grantFailure != nil {
		stageCallbacks(mc, metav1.ConditionFalse, grantFailure.Reason, grantFailure.Message)

		return grantFailure, true, nil
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

// withdrawRetargeted removes the login callbacks of this management plane from
// the realm that status.callbackRealm names, once the spec names another realm
// or the oidc mode. It forgets the record when that realm holds nothing of
// this operator any more, and the caller then registers in the realm of the
// spec.
//
// A suspended management plane leaves every realm as it is, and so does one
// whose spec still names the recorded realm.
//
// The failure it returns names the recorded realm. Nothing but this record
// reaches that realm, so the caller reports the failure, leaves the realm of
// the spec as it is, and comes back on the retry interval.
func (r *Reconciler) withdrawRetargeted(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	res resolved,
) (*conditions.PreCheckFailure, error) {
	recorded := mc.Status.CallbackRealm
	if recorded == nil || res.Input.Suspended {
		return nil, nil
	}

	if target := components.RealmTarget(res.Input.Provider); target != nil &&
		components.SameRealm(*recorded, *target) {
		return nil, nil
	}
	forgotten, err := r.forgetCallbackRealm(ctx, mc)
	if err != nil || forgotten {
		return nil, err
	}
	// The pods of the revision before the change still write the recorded
	// realm while they start, so a withdrawal that overlaps one of them is put
	// back by it.
	rolledOut, err := r.identityRolledOut(ctx, mc)
	if err != nil {
		return nil, err
	}
	if !rolledOut {
		return &conditions.PreCheckFailure{
			Reason:  string(component.PrerequisiteNotMet),
			Message: rollingOut,
		}, nil
	}

	old := components.RealmProvider(*recorded)
	failure, err := r.convergeOptimizeCallbacks(ctx, mc, old, old.Clients.Optimize.ID, nil)
	if err != nil {
		return nil, err
	}
	// A realm that holds no Optimize client holds no callback of this operator
	// either, so that realm is as tidy as a withdrawal leaves it.
	if failure != nil && failure.Reason != v1.ReasonOptimizeClientMissing {
		return &conditions.PreCheckFailure{
			Reason: failure.Reason,
			Message: fmt.Sprintf(
				"Realm %q of Keycloak %q still carries the login callbacks of this management plane, "+
					"and this operator could not remove them: %s. If that Keycloak is gone for good, "+
					"set the annotation %s=%q on this resource to leave them there",
				recorded.Realm, recorded.URL, failure.Message,
				components.ForgetCallbackRealmAnnotation, components.RealmIdentity(*recorded),
			),
		}, nil
	}
	mc.Status.CallbackRealm = nil

	return nil, nil
}

// forgetCallbackRealm lets go of the realm that status.callbackRealm records
// when the ForgetCallbackRealmAnnotation names it, with the login callbacks
// still in it, and removes the annotation. It reports whether the record is
// gone. The caller has found that the spec no longer names the recorded
// realm, so an annotation that names another realm stays where it is: it
// says nothing about the realm the plane is leaving now.
//
// The Warning event is the one trace of the callbacks that stay behind. A
// Keycloak that is gone for good has nothing to withdraw from, and one that
// is only down comes back with the callbacks still on its Optimize client.
func (r *Reconciler) forgetCallbackRealm(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
) (bool, error) {
	recorded := mc.Status.CallbackRealm
	named, ok := mc.Annotations[components.ForgetCallbackRealmAnnotation]
	if !ok || named != components.RealmIdentity(*recorded) {
		return false, nil
	}
	if err := r.removeAnnotation(ctx, mc, components.ForgetCallbackRealmAnnotation); err != nil {
		return false, err
	}
	r.EventRecorder.Eventf(
		mc,
		nil,
		corev1.EventTypeWarning,
		eventReasonCallbacksLeftBehind,
		eventActionUpdate,
		"Left the login callbacks of this management plane on client %q of realm %q of Keycloak %q, "+
			"as the annotation %s asked",
		components.RealmProvider(*recorded).Clients.Optimize.ID,
		recorded.Realm,
		recorded.URL,
		components.ForgetCallbackRealmAnnotation,
	)
	mc.Status.CallbackRealm = nil

	return true, nil
}

// identityRolledOut reports whether every pod of the Management Identity that
// this management cluster owns is the current one and ready. A Deployment of
// another owner at the same name answers no.
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
	// The name is derived from the name of this management cluster, so another
	// owner can hold it while the components of this plane never converged.
	// A workload of somebody else says nothing about who writes this realm.
	if !metav1.IsControlledBy(&identity, mc) {
		return false, nil
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
// management plane, and forgets the realm that status.callbackRealm named. No
// realm holds a callback of this operator in that state, so no realm is left
// to withdraw from later.
func stageNoCallbacks(mc *v1.CamundaManagementCluster) {
	mc.Status.CallbackRealm = nil
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

	trust, failure, err := r.keycloakTrust(ctx, mc, provider)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	return keycloakadmin.New(
		provider.KeycloakURL,
		provider.Realm,
		string(secret.Data[ref.UsernameKey]),
		string(secret.Data[ref.PasswordKey]),
		trust...,
	), nil, nil
}

// keycloakTrust reads the certificate authority that the identity provider
// names and returns the options that make the administration client trust it.
// A provider that names none returns no option, so the client verifies
// Keycloak against the trust store of the operator image alone.
//
// The Secret is read live, the way the administrator credentials are, because
// the controller watches Secrets metadata only.
func (r *Reconciler) keycloakTrust(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	provider components.IdentityProvider,
) ([]keycloakadmin.Option, *conditions.PreCheckFailure, error) {
	ref := provider.CABundle
	if ref == nil {
		return nil, nil, nil
	}

	key := client.ObjectKey{Namespace: mc.Namespace, Name: ref.Name}
	secret, msg, err := secretref.Get(ctx, r.APIReader, key, ref.Key)
	if err != nil {
		return nil, nil, fmt.Errorf("reading Secret %q: %w", key, err)
	}
	if msg != "" {
		return nil, &conditions.PreCheckFailure{Reason: v1.ReasonMissingSecret, Message: msg}, nil
	}

	// Only the bundle is the user's to correct. A trust store of the operator
	// image that cannot be read is a fault of the operator, so it comes back
	// as an error and never as a state of this resource.
	pool, err := keycloakadmin.ParseCABundle(secret.Data[ref.Key])
	switch {
	case errors.Is(err, keycloakadmin.ErrNoCertificates):
		return nil, &conditions.PreCheckFailure{
			Reason: v1.ReasonInvalidCABundle,
			Message: conditions.BoundMessage(fmt.Sprintf(
				"Key %q of Secret %s: %s", ref.Key, key, err,
			)),
		}, nil
	case err != nil:
		return nil, nil, fmt.Errorf("building the certificate pool of Secret %q: %w", key, err)
	}

	return []keycloakadmin.Option{keycloakadmin.WithRootCAs(pool)}, nil, nil
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
