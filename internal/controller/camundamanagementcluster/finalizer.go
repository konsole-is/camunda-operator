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

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
	"github.com/konsole-is/camunda-operator/pkg/leaseclaim"
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

// finalize withdraws the Console ping settings and every claim on the
// orchestration clusters, stops Management Identity, removes the login
// callbacks from the realm, deletes the ManagementAuthConfig, clears the
// record of the realm, releases the claim on every realm, and releases the
// finalizer, in that order: the realm is tidied before anything that would let
// another plane into it goes. The Deployments, the Services, and the copies of
// referenced Secrets carry an owner reference, so Kubernetes collects them;
// what the management plane wrote on an orchestration cluster, what it wrote in
// the realm, the contract, and the realm claims are outside that chain.
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
	// The Deployment goes whatever the contract says. A plane whose contract
	// write never landed, and one whose contract somebody deleted, still runs
	// the Management Identity that writes the clients of the realm.
	if err := r.stopIdentity(ctx, mc); err != nil {
		return err
	}
	// The contract is the claim on its own name, and it is held until the realm
	// is tidy. Deleting it first lets a management plane parked on that name
	// take it and register its own callbacks while this withdrawal is still in
	// flight, and this withdrawal then takes the new owner's away.
	registered, err := r.registeredCallbacks(ctx, mc)
	if err != nil {
		return err
	}
	if registered {
		if err := r.withdrawOptimizeCallbacks(ctx, mc); err != nil {
			return err
		}
	}
	if err := r.withdrawContract(ctx, mc); err != nil {
		return err
	}
	// The record is what sends a pass that runs again back into the realm, so
	// it goes from the API server before the claim does. A pass that released
	// the claim and then failed to remove the finalizer runs again, and by then
	// a plane parked on the realm holds it and has registered the login
	// callbacks of the same Optimize instances there, under the same URLs.
	// With the record still in place that second pass takes those away.
	if err := r.clearCallbackRealm(ctx, mc); err != nil {
		return err
	}
	// The realm claim goes last of all. Once it is gone a plane parked on the
	// realm claims it and runs its own Management Identity against clients
	// this withdrawal may still be writing.
	claims := r.realmClaims()
	leases, err := claims.Held(ctx, leaseclaim.HolderOf(mc))
	if err != nil {
		return err
	}
	if _, err := r.releaseRealmClaims(ctx, claims, leases, nil, nil, false); err != nil {
		return err
	}
	r.EventRecorder.Eventf(
		mc,
		nil,
		corev1.EventTypeNormal,
		eventReasonAttachmentRemoved,
		eventActionFinalize,
		"Withdrew the claims and the Console ping settings on the orchestration clusters, "+
			"tried to remove the Web Modeler users and the login callbacks of Optimize (a failed "+
			"removal has a warning event or a log line of its own), removed "+
			"ManagementAuthConfig %q, and released the claim on the Keycloak realm",
		components.ContractName(mc),
	)

	controllerutil.RemoveFinalizer(mc, Finalizer)
	if err := r.Update(ctx, mc); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("removing the finalizer: %w", err)
	}

	return nil
}

// stopIdentity deletes the Management Identity Deployment and the ReplicaSets
// of it, so that no pod of it starts after this point and writes the Optimize
// client again.
//
// Kubernetes collects that Deployment through its owner reference, but only
// once the finalizer is gone, which is after the realm is tidied. Without this
// the initializer of a pod that started moments ago could put the callbacks
// back with nothing left to remove them.
//
// A pod that is already inside its initializer can still write, and the
// deletion never waits for it: a management plane must not be held open by a
// pod that is going away. The entry it leaves then names a realm client of a
// management plane that no longer exists.
//
// A Deployment of another owner at that name ends the pass with an error.
// Nothing here can stop it, and everything after it releases the realm that
// it writes.
func (r *Reconciler) stopIdentity(ctx context.Context, mc *v1.CamundaManagementCluster) error {
	key := client.ObjectKey{Namespace: mc.Namespace, Name: components.IdentityName(mc)}

	var identity appsv1.Deployment
	switch err := r.APIReader.Get(ctx, key, &identity); {
	case err == nil:
		// The name is derived from the name of this management cluster, so
		// another owner can hold it: a plane whose components never converged
		// leaves it free for anybody. Only the owner reference tells the
		// Management Identity of this plane from a workload that somebody else
		// runs, and the ReplicaSets under such a workload are not ours either.
		// The pass ends here the way the refused delete precondition below
		// ends it. The replacement is only already there, instead of arriving
		// between the read and the delete.
		if !metav1.IsControlledBy(&identity, mc) {
			return fmt.Errorf(
				"the Deployment %q has another owner and can still write the Keycloak realm, so "+
					"this deletion waits until that Deployment is gone: delete it, or give it "+
					"another name, and this management plane goes", key,
			)
		}

		// The UID is a precondition, so a Deployment that took the name
		// between the read and this call is refused rather than deleted. The
		// refusal goes back as an error: the replacement runs a Management
		// Identity that this pass knows nothing about, and the next pass reads
		// it before anything else here gives its realm away.
		err := r.Delete(ctx, &identity, client.Preconditions{UID: &identity.UID})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("deleting Deployment %q: %w", key, err)
		}
	case !apierrors.IsNotFound(err):
		return fmt.Errorf("reading Deployment %q: %w", key, err)
	}

	// The ReplicaSets go after the Deployment, because before it the Deployment
	// makes another one at once, and they go on a pass that finds the
	// Deployment already gone, because that is what a pass that stopped between
	// the two deletions leaves behind.
	return r.stopIdentitySets(ctx, mc, key.Name)
}

// stopIdentitySets deletes every ReplicaSet of the Management Identity
// Deployment that name names. The caller has established that no Deployment of
// another owner holds that name.
//
// Kubernetes collects them with the Deployment, but in the background, and one
// that still asks for a pod starts another Management Identity while it waits.
// That pod would write the clients of the realm this deletion is about to give
// back.
func (r *Reconciler) stopIdentitySets(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	name string,
) error {
	var sets appsv1.ReplicaSetList
	if err := r.APIReader.List(
		ctx,
		&sets,
		client.InNamespace(mc.Namespace),
		client.MatchingLabels(components.IdentityPodLabels(mc)),
	); err != nil {
		return fmt.Errorf("listing the Management Identity ReplicaSets: %w", err)
	}

	var errs []error
	for i := range sets.Items {
		set := &sets.Items[i]
		// The labels are the discovery labels of this plane, which anybody can
		// write. The owner reference is what tells a ReplicaSet of the
		// Management Identity Deployment from a workload of somebody else.
		owner := metav1.GetControllerOf(set)
		if owner == nil || owner.APIVersion != appsv1.SchemeGroupVersion.String() ||
			owner.Kind != "Deployment" || owner.Name != name {
			continue
		}

		// A ReplicaSet that took the name between the list and this call is
		// refused, and the refusal goes back as an error for the reason the
		// Deployment gives.
		err := r.Delete(ctx, set, client.Preconditions{UID: &set.UID})
		if err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf(
				"deleting ReplicaSet %q: %w", client.ObjectKeyFromObject(set), err,
			))
		}
	}

	return errors.Join(errs...)
}

// registeredCallbacks reports whether this management plane can have login
// callbacks of its own in a realm, which is what authorizes the withdrawal.
//
// status.callbackRealm names the realm that Management Identity was pointed
// at. It is written before the components apply, so a plane whose contract
// write never landed still holds the record of the realm its Identity
// registered in. A ManagementAuthConfig that this plane holds answers for the
// keycloak mode, which records no realm because the operator deletes that
// Keycloak with the plane. A plane with neither served no Optimize anywhere.
func (r *Reconciler) registeredCallbacks(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
) (bool, error) {
	if mc.Status.CallbackRealm != nil {
		return true, nil
	}

	return r.ownsContract(ctx, mc)
}

// ownsContract reports whether a ManagementAuthConfig that this management
// plane holds exists, under the name it writes now or the name its status
// recorded.
//
// An absent contract answers no: a plane that was deleted before it ever wrote
// one registered nothing. A contract of another owner answers no as well, so
// the deletion never removes a callback of that owner on a guess. A read that
// fails is an error, the way the read of the realm claim is, because the pass
// deletes the contract and releases the realm right after and a no would leave
// the callbacks in there.
func (r *Reconciler) ownsContract(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
) (bool, error) {
	names := []string{components.ContractName(mc)}
	if previous := mc.Status.ManagementAuthConfig; previous != "" {
		names = append(names, previous)
	}

	for _, name := range names {
		var existing v1.ManagementAuthConfig
		if err := r.APIReader.Get(ctx, client.ObjectKey{Name: name}, &existing); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}

			return false, fmt.Errorf("reading ManagementAuthConfig %q: %w", name, err)
		}
		if ownedBy(&existing, mc) {
			return true, nil
		}
	}

	return false, nil
}

// withdrawOptimizeCallbacks removes the login callbacks that this management
// plane registered on the Optimize client of the realm.
//
// A Keycloak that does not answer is best effort and never stops the deletion,
// because the orchestration clusters this resource holds would stay claimed
// with nothing left to free them. A realm that the operator could not reach
// keeps the callbacks, and the log line says so. A failure of the Kubernetes
// API does stop the deletion: it leaves the realms to withdraw from unknown,
// and the caller releases the claim on every one of them next.
//
// A Keycloak that the operator ran goes with this resource, so only a Keycloak
// that you run keeps anything. The caller decides whether this plane ever
// registered a callback, through registeredCallbacks. Without that, a plane
// parked on a name another owner holds takes the callbacks of the holder with
// it.
func (r *Reconciler) withdrawOptimizeCallbacks(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
) error {
	realms, err := r.withdrawalRealms(ctx, mc)
	if err != nil {
		return err
	}

	for _, provider := range realms {
		failure, err := r.convergeOptimizeCallbacks(
			ctx, mc, provider, provider.Clients.Optimize.ID, nil,
		)
		switch {
		case err != nil:
			// The URL of the spec admits a user with a password, and this
			// error reaches a log, so the message names the realm alone.
			return fmt.Errorf(
				"withdrawing the Optimize callbacks of realm %q: %w", provider.Realm, err,
			)
		case failure != nil && failure.Reason != v1.ReasonOptimizeClientMissing:
			logf.FromContext(ctx).Error(
				failure, "Withdrawing the Optimize callbacks", "reason", failure.Reason,
			)
		default:
			return nil
		}
	}

	return nil
}

// withdrawalRealms lists what to take the login callbacks out of, in the order
// to try it, and nothing for a management plane that registered none anywhere.
//
// status.callbackRealm is the realm they went into, which is the one to tidy
// even after the spec was pointed at another Keycloak. The spec goes first
// when it names that same realm, because a rotation of the Secrets changes no
// realm and the record can still name the Secret that was replaced. The
// record follows as a second try, for a spec whose Secrets are the broken
// ones. A plane that recorded no realm falls back to the spec, which
// specRealmWithdrawal decides. The provider of a Keycloak mode follows from
// the spec alone, so the deletion path needs none of the pre-checks.
func (r *Reconciler) withdrawalRealms(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
) ([]components.IdentityProvider, error) {
	recorded := mc.Status.CallbackRealm
	if components.Mode(mc) == components.ModeOIDC {
		if recorded == nil {
			return nil, nil
		}

		return []components.IdentityProvider{components.RealmProvider(*recorded)}, nil
	}

	provider, err := components.ResolveIdentityProvider(components.Input{Cluster: mc})
	if err != nil {
		if recorded != nil {
			return []components.IdentityProvider{components.RealmProvider(*recorded)}, nil
		}
		logf.FromContext(ctx).Error(err, "Resolving Keycloak to withdraw the Optimize callbacks")

		return nil, nil
	}
	if recorded == nil {
		return r.specRealmWithdrawal(ctx, mc, provider)
	}
	target := components.RealmTarget(provider)
	if target == nil || !components.SameRealm(*recorded, *target) {
		return []components.IdentityProvider{components.RealmProvider(*recorded)}, nil
	}
	if apiequality.Semantic.DeepEqual(*recorded, *target) {
		return []components.IdentityProvider{provider}, nil
	}

	return []components.IdentityProvider{provider, components.RealmProvider(*recorded)}, nil
}

// specRealmWithdrawal is the withdrawal of a plane that recorded no realm. It
// takes the realm of the spec, and nothing when this plane cannot be shown to
// have been in that realm.
//
// A plane that serves no Optimize runs a Management Identity against the realm
// the spec names and records nothing of it, so the spec is the only way to a
// client that Identity made there.
func (r *Reconciler) specRealmWithdrawal(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	provider components.IdentityProvider,
) ([]components.IdentityProvider, error) {
	// The operator runs the Keycloak of the keycloak mode, and it is deleted
	// with this plane, so the realm behind it is this plane's whatever the
	// claim says.
	if components.Mode(mc) == components.ModeKeycloak {
		return []components.IdentityProvider{provider}, nil
	}

	// A plane parked on the realm of another plane keeps the contract of the
	// realm it left, and its spec names a realm it never entered. Its claim is
	// what says this plane was ever in there.
	target := components.RealmTarget(provider)
	if target == nil {
		return nil, nil
	}
	holds, err := r.holdsRealmClaim(ctx, mc, *target)
	if err != nil {
		return nil, err
	}
	if !holds {
		logf.FromContext(ctx).Info(
			"Left the login callbacks of the realm alone, this management plane holds no claim on it",
			"realm", components.RealmIdentity(*target),
		)

		return nil, nil
	}

	return []components.IdentityProvider{provider}, nil
}

// holdsRealmClaim reports whether the claim Lease of target names this
// management plane. A Lease that is gone and one of another holder answer no,
// so a deletion never writes a realm that this plane cannot show it entered.
//
// A read that fails is an error and not a no. The caller releases the claim on
// every realm right after, so an unread claim that answered no would leave the
// callbacks of this plane in a realm the next plane takes.
func (r *Reconciler) holdsRealmClaim(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	target v1.KeycloakRealmTarget,
) (bool, error) {
	return r.realmClaims().Holds(
		ctx, components.RealmIdentity(target), leaseclaim.HolderOf(mc),
	)
}

// clearCallbackRealm takes the record of the realm off the API server. It
// writes the status of mc once, the way recordCallbackRealm does, and a plane
// that recorded no realm writes nothing. An object that is gone answers
// NotFound, which is no error here, as in the removal of the finalizer.
func (r *Reconciler) clearCallbackRealm(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
) error {
	if mc.Status.CallbackRealm == nil {
		return nil
	}
	mc.Status.CallbackRealm = nil

	rec := component.ReconcileContext{
		Client:        r.Client,
		Scheme:        r.Scheme,
		EventRecorder: r.EventRecorder,
		Metrics:       r.Metrics,
		APIReader:     r.APIReader,
		Owner:         mc,
	}
	if err := component.FlushStatus(ctx, rec, nil); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("clearing the record of the realm of the login callbacks: %w", err)
	}

	return nil
}
