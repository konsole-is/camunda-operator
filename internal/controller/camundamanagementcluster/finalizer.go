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
// orchestration clusters, removes the login callbacks from the realm, deletes
// the ManagementAuthConfig, releases the claim on every realm, and releases
// the finalizer, in that order: the realm is tidied before anything that would
// let another plane into it goes. The Deployments, the Services, and the
// copies of referenced Secrets carry an owner reference, so Kubernetes
// collects them; what the management plane wrote on an orchestration cluster,
// what it wrote in the realm, the contract, and the realm claims are outside
// that chain.
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
	// The contract is the claim on the realm, so it is held until the realm is
	// tidy. Deleting it first would let a waiting management plane take the
	// name and register its own callbacks while this withdrawal is still in
	// flight, and this withdrawal would then take the new owner's away.
	if r.ownsContract(ctx, mc) {
		if err := r.stopIdentity(ctx, mc); err != nil {
			return err
		}
		r.withdrawOptimizeCallbacks(ctx, mc)
	}
	if err := r.withdrawContract(ctx, mc); err != nil {
		return err
	}
	// The realm claim goes last of all. Once it is gone a plane parked on the
	// realm claims it and runs its own Management Identity against clients
	// this withdrawal may still be writing.
	if err := r.releaseRealmClaims(ctx, mc, false); err != nil {
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

// stopIdentity deletes the Management Identity Deployment, so that no pod of
// it starts after this point and writes the Optimize client again.
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
func (r *Reconciler) stopIdentity(ctx context.Context, mc *v1.CamundaManagementCluster) error {
	key := client.ObjectKey{Namespace: mc.Namespace, Name: components.IdentityName(mc)}

	var identity appsv1.Deployment
	if err := r.APIReader.Get(ctx, key, &identity); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("reading Deployment %q: %w", key, err)
	}
	// The name is derived from the name of this management cluster, so another
	// owner can hold it: a plane whose components never converged leaves it
	// free for anybody. Only the owner reference tells the Management Identity
	// of this plane from a workload that somebody else runs.
	if !metav1.IsControlledBy(&identity, mc) {
		return nil
	}

	// The UID is a precondition, so a Deployment that took the name between
	// the read and this call is refused rather than deleted.
	err := r.Delete(ctx, &identity, client.Preconditions{UID: &identity.UID})
	if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
		return fmt.Errorf("deleting Deployment %q: %w", key, err)
	}

	return nil
}

// ownsContract reports whether a ManagementAuthConfig that this management
// plane holds exists, under the name it writes now or the name its status
// recorded. Only a plane that held one ever served an Optimize behind it, so
// only that plane has a login callback in the realm to remove.
//
// An absent contract answers no. A plane that was deleted before it ever wrote
// one registered nothing, and a read that fails answers no for the same
// reason: the deletion must never remove a callback of another owner on a
// guess.
func (r *Reconciler) ownsContract(ctx context.Context, mc *v1.CamundaManagementCluster) bool {
	names := []string{components.ContractName(mc)}
	if previous := mc.Status.ManagementAuthConfig; previous != "" {
		names = append(names, previous)
	}

	for _, name := range names {
		var existing v1.ManagementAuthConfig
		if err := r.APIReader.Get(ctx, client.ObjectKey{Name: name}, &existing); err != nil {
			continue
		}
		if ownedBy(&existing, mc) {
			return true
		}
	}

	return false
}

// withdrawOptimizeCallbacks removes the login callbacks that this management
// plane registered on the Optimize client of the realm.
//
// It is best effort, and it never stops the deletion. A Keycloak that does not
// answer must not keep the resource alive, because the orchestration clusters
// it holds would stay claimed with nothing left to free them. A realm that the
// operator could not reach keeps the callbacks, and the log line says so.
//
// A Keycloak that the operator ran goes with this resource, so only a Keycloak
// that you run keeps anything. The caller decides whether this plane owns the
// contract; without that, a plane parked on a name another owner holds would
// take the callbacks of the holder with it.
func (r *Reconciler) withdrawOptimizeCallbacks(ctx context.Context, mc *v1.CamundaManagementCluster) {
	for _, provider := range withdrawalRealms(ctx, mc) {
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
		default:
			return
		}
	}
}

// withdrawalRealms lists what to take the login callbacks out of, in the order
// to try it, and nothing for a management plane that registered none anywhere.
//
// status.callbackRealm is the realm they went into, which is the one to tidy
// even after the spec was pointed at another Keycloak. The spec goes first
// when it names that same realm, because a rotation of the Secrets changes no
// realm and the record can still name the Secret that was replaced. The
// record follows as a second try, for a spec whose Secrets are the broken
// ones. A plane that recorded no realm has the spec alone: the operator runs
// the Keycloak of the keycloak mode and records nothing for it, and a plane
// that serves no Optimize runs a Management Identity against the realm the
// spec names with no record of it. The provider of a Keycloak mode follows
// from the spec alone, so the deletion path needs none of the pre-checks.
func withdrawalRealms(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
) []components.IdentityProvider {
	recorded := mc.Status.CallbackRealm
	if components.Mode(mc) == components.ModeOIDC {
		if recorded == nil {
			return nil
		}

		return []components.IdentityProvider{components.RealmProvider(*recorded)}
	}

	provider, err := components.ResolveIdentityProvider(components.Input{Cluster: mc})
	if err != nil {
		if recorded != nil {
			return []components.IdentityProvider{components.RealmProvider(*recorded)}
		}
		logf.FromContext(ctx).Error(err, "Resolving Keycloak to withdraw the Optimize callbacks")

		return nil
	}
	if recorded == nil {
		return []components.IdentityProvider{provider}
	}
	target := components.RealmTarget(provider)
	if target == nil || !components.SameRealm(*recorded, *target) {
		return []components.IdentityProvider{components.RealmProvider(*recorded)}
	}
	if apiequality.Semantic.DeepEqual(*recorded, *target) {
		return []components.IdentityProvider{provider}
	}

	return []components.IdentityProvider{provider, components.RealmProvider(*recorded)}
}
