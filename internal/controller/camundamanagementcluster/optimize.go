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

// The event vocabulary of this file. Every event of the realm records the
// same Update action under a reason of its own.
const (
	// eventReasonOptimizeCallbacks is recorded when the controller writes the
	// redirect URIs of the Optimize client. One write can add a callback,
	// remove one, or both, so the reason names the write and not a direction.
	eventReasonOptimizeCallbacks = "OptimizeCallbacksUpdated"
	// eventActionUpdate is the action of every event of this file.
	eventActionUpdate = "Update"
	// eventReasonCallbacksLeftBehind is recorded when the
	// ForgetCallbackRealmAnnotation lets go of a realm that still carries the
	// login callbacks of this management plane.
	eventReasonCallbacksLeftBehind = "OptimizeCallbacksLeftBehind"
	// eventReasonForgetIgnored is recorded when a ForgetCallbackRealmAnnotation
	// names another realm than the recorded one and is removed unused.
	eventReasonForgetIgnored = "ForgetCallbackRealmIgnored"
	// eventReasonForgetRemoved is recorded when a ForgetCallbackRealmAnnotation
	// is removed from a management plane that records no realm.
	eventReasonForgetRemoved = "ForgetCallbackRealmRemoved"
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
// status.callbackRealm records the realm of a Keycloak that you run from the
// moment Reconcile points Management Identity at it, and this step writes it
// again when the realm converges, so it names the Secrets of the spec.
// Reconcile withdraws from a recorded realm the spec no longer names before
// the components run, and withdrawal is what that pass found. A
// Keycloak-to-Keycloak move with a pending withdrawal stops the pass in
// Reconcile while the plane serves an Optimize, so this step never registers
// beside a realm that is still being left. The oidc mode and a plane that
// serves no Optimize reach it with the pending failure to report.
func (r *Reconciler) syncOptimizeCallbacks(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	res resolved,
	contractErr error,
	withdrawal *conditions.PreCheckFailure,
) (*conditions.PreCheckFailure, bool, error) {
	// The two states that register nothing at all answer first. A plane that
	// administers no realm, and one whose desired state is to run nothing, are
	// not waiting on anything, so neither reports a prerequisite.
	provider := res.Input.Provider
	if provider.Mode == components.ModeOIDC {
		// A plane that registered callbacks in a Keycloak before this mode is
		// the one case where this mode still has a realm to tidy.
		if withdrawal != nil {
			stageCallbacks(mc, metav1.ConditionFalse, withdrawal.Reason, withdrawal.Message)

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
	// A pending withdrawal that reaches this point belongs to a plane that
	// serves no Optimize: every other one gated the pass in Reconcile. The
	// realm being left is reported here and Ready stays with the components,
	// because nobody can sign in to an Optimize that does not exist, and the
	// new realm gets no callbacks either way.
	if withdrawal != nil {
		stageCallbacks(mc, metav1.ConditionFalse, withdrawal.Reason, withdrawal.Message)

		return nil, true, nil
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
		// NoCallbacks stops the calls to Keycloak, so it must not rest on a
		// realm that a starting pod can still write. The Deployment status
		// that the rollout wait above reads can be complete while a pod of
		// the old revision is still terminating inside its start, and that
		// pod would put the callback list of its environment back unseen.
		pods, err := r.identityPods(ctx, mc)
		if err != nil {
			return nil, false, err
		}
		if target := components.RealmTarget(provider); target != nil &&
			components.IdentityWritesRealm(pods, *target) {
			stageCallbacks(
				mc, metav1.ConditionFalse, string(component.PrerequisiteNotMet),
				fmt.Sprintf(
					"A Management Identity pod is starting against realm %q of Keycloak %q, and "+
						"it owns the Optimize client of that realm while it starts",
					target.Realm, components.RealmURL(*target),
				),
			)

			return nil, true, nil
		}
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
	// change, and its realm goes with it. A record that survives into the
	// keycloak mode named this same realm from an externalKeycloak spec
	// before, and it ends here: the managed mode owns the realm now.
	if provider.Mode == components.ModeExternalKeycloak {
		mc.Status.CallbackRealm = components.RealmTarget(provider)
	} else {
		mc.Status.CallbackRealm = nil
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

// nothingRegistered reports whether the last reconcile already found no login
// callback of this operator in the realm, and no realm is recorded. A
// management plane that rests there makes no call to Keycloak at all.
//
// The record is what makes the second half necessary. It is written before
// the components start Management Identity, and a pass that ends before its
// conditions reach the API server leaves that record beside the reason of an
// older pass. Only a plane with no record can rest on the reason alone.
func nothingRegistered(mc *v1.CamundaManagementCluster) bool {
	if mc.Status.CallbackRealm != nil {
		return false
	}
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

// recordCallbackRealm records the realm that this management plane points
// Management Identity at, once the withdrawal from any realm it is leaving is
// over, and writes the record before it returns. Identity registers the login
// callbacks of its realm itself, while it starts, so the realm is on the API
// server before the caller applies the components: a record written after the
// first registration converged would miss a retarget during that first start,
// and the callbacks would stay in a realm that no record names.
//
// Only the state that lets Identity register anything is recorded: a Keycloak
// that you run, a plane that is not suspended, and one that serves an
// Optimize, because Identity creates the Optimize client from the preset of
// the Optimize instances the plane serves. A plane that serves none registers
// nothing, and syncOptimizeCallbacks would take a record of it away again.
//
// target is the realm of the spec. A realm that is already recorded keeps the
// Secrets it was written with, however the spec rotates them: nothing here
// has signed in with the new ones yet, and a Secret that exists can still
// hold the wrong password. syncOptimizeCallbacks writes the record again once
// Keycloak accepted them, and the deletion path tries both.
func (r *Reconciler) recordCallbackRealm(
	ctx context.Context,
	rec component.ReconcileContext,
	mc *v1.CamundaManagementCluster,
	res resolved,
	target *v1.KeycloakRealmTarget,
) error {
	if mc.Status.CallbackRealm != nil || target == nil ||
		res.Input.Provider.Mode != components.ModeExternalKeycloak ||
		res.Input.Suspended || len(res.Input.OptimizeURLs) == 0 {
		return nil
	}
	mc.Status.CallbackRealm = target

	if err := component.FlushStatus(ctx, rec, nil); err != nil {
		return stepRecordCallbackRealm.stop(mc, err)
	}

	return nil
}

// withdrawStopped is the withdrawal from the recorded realm on a pass that
// stops before it registers anywhere: a failed pre-check, or a plane parked
// on a realm that another management plane holds. It needs nothing the
// pre-check resolves: the realm to leave comes from status.callbackRealm, and
// the realm of the spec, read from the spec alone, only says whether the
// plane is leaving. A retarget whose new administrator Secret is still
// missing therefore tidies the old realm all the same. It reports whether the
// caller has to come back on the retry interval.
//
// stopped says what stops the pass, for the condition message of a plane
// whose callbacks left the old realm and are registered nowhere yet.
func (r *Reconciler) withdrawStopped(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	stopped string,
) (bool, error) {
	before := mc.Status.CallbackRealm
	target, err := specRealmTarget(mc)
	if err != nil {
		return false, err
	}
	failure, consumed, err := r.withdrawRetargeted(ctx, mc, target, mc.Spec.Suspend)
	if err != nil {
		return false, err
	}
	if failure != nil {
		stageCallbacks(mc, metav1.ConditionFalse, failure.Reason, failure.Message)

		return true, nil
	}
	// The annotation let go of the realm with the callbacks still in it, so
	// the condition must not report that they left. The caller comes back,
	// and the next pass removes the spent annotation.
	if consumed {
		stageCallbacks(
			mc, metav1.ConditionFalse, string(component.PrerequisiteNotMet),
			fmt.Sprintf(
				"Realm %q of Keycloak %q keeps the login callbacks of this management plane, as "+
					"the annotation %s asked. They are registered nowhere, because %s",
				before.Realm, components.RealmURL(*before), components.ForgetCallbackRealmAnnotation,
				stopped,
			),
		)

		return true, nil
	}
	// The callbacks left the old realm, and this pass registers nothing, so
	// the condition must stop reporting a realm that now holds none of them.
	if before != nil && mc.Status.CallbackRealm == nil {
		stageCallbacks(
			mc, metav1.ConditionFalse, string(component.PrerequisiteNotMet),
			fmt.Sprintf(
				"The login callbacks left realm %q of Keycloak %q, and %s, so they are "+
					"registered nowhere",
				before.Realm, components.RealmURL(*before), stopped,
			),
		)
	}

	return false, nil
}

// withdrawRetargeted removes the login callbacks of this management plane from
// the realm that status.callbackRealm names, once the spec names another realm
// or the oidc mode. target is the realm of the spec, nil in the oidc mode.
//
// A suspended management plane leaves every realm as it is, and so does one
// whose spec still names the recorded realm.
//
// Once the realm is empty, it stops what can still write it: the Management
// Identity Deployment whose template points at the realm is deleted, and the
// record is kept until that Deployment, every ReplicaSet of it, and every
// pod that points at the realm are gone, because any of them can put the
// callbacks back. Only the pass that finds none clears the record. A pod that
// is starting against the realm holds the realm write back and stops that
// Deployment all the same, so the wait ends whether or not the pod ever
// becomes ready.
//
// The failure names the recorded realm, and nothing but the record reaches
// that realm, so the caller reports the failure and comes back on the retry
// interval. Reconcile runs this before the components, and a plane that
// serves an Optimize does not move to a new Keycloak while a failure stands,
// so the callbacks never fill the new realm beside a realm that still signs
// people in.
//
// The second result reports that the ForgetCallbackRealmAnnotation consumed
// the record on this pass. The caller ends the pass then: registering in the
// new realm right away would replace the cleared record before it persisted,
// and the next pass would read the spent annotation as one that names a
// foreign realm.
func (r *Reconciler) withdrawRetargeted(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	target *v1.KeycloakRealmTarget,
	suspended bool,
) (*conditions.PreCheckFailure, bool, error) {
	if err := r.dropSpentForgetAnnotation(ctx, mc); err != nil {
		return nil, false, err
	}
	recorded := mc.Status.CallbackRealm
	if recorded == nil || suspended {
		return nil, false, nil
	}
	if target != nil && components.SameRealm(*recorded, *target) {
		return nil, false, nil
	}
	if r.forgetCallbackRealm(mc) {
		return nil, true, nil
	}
	pods, err := r.identityPods(ctx, mc)
	if err != nil {
		return nil, false, err
	}
	// A pod that is starting against the recorded realm writes its Optimize
	// client, so a write of this operator between its read and its write
	// would be lost. Whether the pods of the new revision ever become ready
	// says nothing about the old realm, and a wait on them would hold the
	// callbacks there for as long as the new Keycloak is broken.
	if components.IdentityWritesRealm(pods, *recorded) {
		// The Deployment of the recorded realm is what starts that pod again.
		// A Management Identity that never becomes ready, because the Keycloak
		// it starts against is gone, would otherwise wait on a pod that its own
		// Deployment keeps making, and the realm would never be left.
		if _, err := r.stopOldIdentityWriters(ctx, mc, *recorded); err != nil {
			return nil, false, err
		}

		return &conditions.PreCheckFailure{
			Reason: string(component.PrerequisiteNotMet),
			Message: fmt.Sprintf(
				"A Management Identity pod is starting against realm %q of Keycloak %q, and it "+
					"owns the Optimize client of that realm while it starts. That Management "+
					"Identity is stopped, and this operator empties the realm once the pod is "+
					"gone. If that Keycloak is gone for good, set the annotation %s=%q on this "+
					"resource to leave the login callbacks there",
				recorded.Realm, components.RealmURL(*recorded),
				components.ForgetCallbackRealmAnnotation, components.RealmIdentity(*recorded),
			),
		}, false, nil
	}

	old := components.RealmProvider(*recorded)
	failure, err := r.convergeOptimizeCallbacks(ctx, mc, old, old.Clients.Optimize.ID, nil)
	if err != nil {
		return nil, false, err
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
				recorded.Realm, components.RealmURL(*recorded), failure.Message,
				components.ForgetCallbackRealmAnnotation, components.RealmIdentity(*recorded),
			),
		}, false, nil
	}
	writers, err := r.stopOldIdentityWriters(ctx, mc, *recorded)
	if err != nil {
		return nil, false, err
	}
	if writers {
		return &conditions.PreCheckFailure{
			Reason: string(component.PrerequisiteNotMet),
			Message: fmt.Sprintf(
				"Realm %q of Keycloak %q holds no login callback of this management plane any "+
					"more, and the Management Identity of that realm is not fully stopped; the "+
					"plane moves to the new identity provider when nothing of it is left",
				recorded.Realm, components.RealmURL(*recorded),
			),
		}, false, nil
	}
	mc.Status.CallbackRealm = nil

	return nil, false, nil
}

// stopOldIdentityWriters stops what can still write the recorded realm and
// reports whether anything of it is left. The Management Identity Deployment
// whose template points at the realm is deleted here: a snapshot of the pods
// alone does not bound the future, because its ReplicaSet can start a
// replacement pod right after any list, even one that found no pod at all. A
// ReplicaSet that still wants a replica counts the same way, and so does
// every pod that points at the realm.
//
// The pods are listed last, after the Deployment and the ReplicaSets. A pod
// that a ReplicaSet started before the garbage collector removed both is
// still an object at that moment, terminating at worst, and a pod whose
// object is gone can no longer run, so nothing slips between the lists.
func (r *Reconciler) stopOldIdentityWriters(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	recorded v1.KeycloakRealmTarget,
) (bool, error) {
	var writers bool

	key := client.ObjectKey{Namespace: mc.Namespace, Name: components.IdentityName(mc)}
	var identity appsv1.Deployment
	err := r.APIReader.Get(ctx, key, &identity)
	switch {
	case apierrors.IsNotFound(err):
	case err != nil:
		return false, fmt.Errorf("reading Deployment %q: %w", key, err)
	case components.IdentityTemplatePointsAtRealm(&identity.Spec.Template.Spec, recorded):
		// A Deployment at this name starts a pod against the realm its
		// template names whoever owns it, so it holds the record either way.
		// Only the delete asks who owns it: a workload of another owner is
		// not ours to stop, and the record waits for whoever removes it.
		writers = true
		// The UID is a precondition, so a Deployment that took the name
		// between the read and this call is refused rather than deleted, the
		// way stopIdentity of the finalizer deletes it.
		if metav1.IsControlledBy(&identity, mc) && identity.DeletionTimestamp == nil {
			err := r.Delete(ctx, &identity, client.Preconditions{UID: &identity.UID})
			if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
				return false, fmt.Errorf("deleting Deployment %q: %w", key, err)
			}
		}
	}

	var sets appsv1.ReplicaSetList
	if err := r.APIReader.List(
		ctx,
		&sets,
		client.InNamespace(mc.Namespace),
		client.MatchingLabels(components.IdentityPodLabels(mc)),
	); err != nil {
		return false, fmt.Errorf("listing the Management Identity ReplicaSets: %w", err)
	}
	for i := range sets.Items {
		set := &sets.Items[i]
		// A Deployment keeps the ReplicaSet of every revision it rolled over,
		// at zero replicas. Such a revision starts no pod, and a pod it has
		// left is in the list below. An absent count means one replica.
		//
		// The replica count alone answers it here. scaledDown says why this
		// wait must not ask whether a controller has read that count.
		if set.Spec.Replicas != nil && *set.Spec.Replicas == 0 {
			continue
		}
		if components.IdentityTemplatePointsAtRealm(&set.Spec.Template.Spec, recorded) {
			writers = true
		}
	}

	pods, err := r.identityPods(ctx, mc)
	if err != nil {
		return false, err
	}
	if components.IdentityPointsAtRealm(pods, recorded) {
		writers = true
	}

	return writers, nil
}

// forgetCallbackRealm lets go of the realm that status.callbackRealm records
// when the ForgetCallbackRealmAnnotation names it, with the login callbacks
// still in it, and reports whether it did. The Warning event is the one
// trace of the callbacks that stay behind.
func (r *Reconciler) forgetCallbackRealm(mc *v1.CamundaManagementCluster) bool {
	recorded := mc.Status.CallbackRealm
	// A value that is no realm identity names no realm, whatever it folds to.
	// The hatch leaves the callbacks behind for good, so it answers to the
	// exact value the condition message prints and to nothing else.
	named, identity := components.NormalizeRealmIdentity(
		mc.Annotations[components.ForgetCallbackRealmAnnotation],
	)
	if !identity || named != components.RealmIdentity(*recorded) {
		return false
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
		components.RealmURL(*recorded),
		components.ForgetCallbackRealmAnnotation,
	)
	// The annotation stays until the next pass. Removing it now, with the
	// cleared record still waiting on the deferred status flush, would
	// consume it even when that flush fails, and the record would come back
	// with the annotation gone. The next pass finds the record gone and
	// removes it.
	mc.Status.CallbackRealm = nil

	return true
}

// dropSpentForgetAnnotation removes a ForgetCallbackRealmAnnotation that
// names no recorded realm: the realm it named is let go of and its record is
// gone, or it names another realm than the recorded one and applies to
// nothing. Both record an event, because somebody set the annotation by hand
// and it would otherwise vanish without a word. The one that names another
// realm than the recorded one is a Warning: nothing was let go of and the
// annotation asked for something. With no record at all, the removal is the
// last step of a realm that was let go of on the pass before, so the event is
// a Normal one.
func (r *Reconciler) dropSpentForgetAnnotation(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
) error {
	value, ok := mc.Annotations[components.ForgetCallbackRealmAnnotation]
	if !ok {
		return nil
	}
	// The annotation is written by hand, and a Keycloak URL admits a user with
	// a password, so the event carries the folded identity of the value. A
	// value that does not parse as a URL is folded of nothing, and the event
	// says only that it is not a realm.
	folded, identity := components.NormalizeRealmIdentity(value)
	recorded := mc.Status.CallbackRealm
	// A value that is no realm identity is one forgetCallbackRealm refuses, so
	// it is spent here and removed, whatever it folds to.
	if recorded != nil && identity && folded == components.RealmIdentity(*recorded) {
		return nil
	}

	carried := "a value that is not a realm identity"
	if identity {
		carried = fmt.Sprintf("realm %q", folded)
	}
	if recorded == nil {
		r.EventRecorder.Eventf(
			mc,
			nil,
			corev1.EventTypeNormal,
			eventReasonForgetRemoved,
			eventActionUpdate,
			"Removed the annotation %s, which carries %s. status.callbackRealm names no realm, so "+
				"no realm is let go of on this pass",
			components.ForgetCallbackRealmAnnotation,
			carried,
		)

		return r.removeAnnotation(ctx, mc, components.ForgetCallbackRealmAnnotation)
	}
	r.EventRecorder.Eventf(
		mc,
		nil,
		corev1.EventTypeWarning,
		eventReasonForgetIgnored,
		eventActionUpdate,
		"Annotation %s carries %s and status.callbackRealm names realm %q, so the "+
			"annotation is removed and nothing is let go of",
		components.ForgetCallbackRealmAnnotation,
		carried,
		components.RealmIdentity(*recorded),
	)

	return r.removeAnnotation(ctx, mc, components.ForgetCallbackRealmAnnotation)
}

// specRealmTarget is the realm that the spec names, read from the spec alone,
// or nil in the oidc mode. The two Keycloak modes resolve without a
// pre-check, the way the finalizer reads them.
func specRealmTarget(mc *v1.CamundaManagementCluster) (*v1.KeycloakRealmTarget, error) {
	if components.Mode(mc) == components.ModeOIDC {
		return nil, nil
	}

	provider, err := components.ResolveIdentityProvider(components.Input{Cluster: mc})
	if err != nil {
		return nil, fmt.Errorf("resolving the identity provider of the spec: %w", err)
	}
	target := components.RealmTarget(provider)
	// A nil target in a Keycloak mode would read as the oidc mode and
	// withdraw on a spec that changed nothing.
	if target == nil {
		return nil, errors.New("the identity provider of the spec names no Keycloak administrator")
	}

	return target, nil
}

// identityPods lists the Management Identity pods of this management plane.
// The read goes through APIReader, for the reason startedInitialClaim gives.
func (r *Reconciler) identityPods(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
) ([]corev1.Pod, error) {
	var pods corev1.PodList
	if err := r.APIReader.List(
		ctx,
		&pods,
		client.InNamespace(mc.Namespace),
		client.MatchingLabels(components.IdentityPodLabels(mc)),
	); err != nil {
		return nil, fmt.Errorf("listing the Management Identity pods: %w", err)
	}

	return pods.Items, nil
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
