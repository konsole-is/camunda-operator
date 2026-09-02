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
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// claimRealm takes the claim on the Keycloak realm that the spec names, and
// gives back every realm claim of this management plane that nothing of it
// names any more, which releaseUnusedRealms decides.
//
// Only the externalKeycloak mode claims. The keycloak mode owns the Keycloak
// it runs, and the oidc mode administers no realm. A suspended plane touches
// no claim: the realm it holds stays held, and a realm the spec now names is
// claimed on resume.
//
// The failure it returns is the parked state: another management cluster
// holds the realm, and the message names it. The caller stages it on Ready,
// renders nothing, and comes back on the retry interval. Only a failure of
// the Kubernetes API comes back as an error.
//
// The bool is the one releaseUnusedRealms returns.
func (r *Reconciler) claimRealm(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	res resolved,
) (*conditions.PreCheckFailure, bool, error) {
	if res.Input.Suspended {
		return nil, false, nil
	}

	var current *v1.KeycloakRealmTarget
	if res.Input.Provider.Mode == components.ModeExternalKeycloak {
		current = components.RealmTarget(res.Input.Provider)
	}
	var parked *conditions.PreCheckFailure
	if current != nil {
		var err error
		parked, err = r.takeRealmClaim(ctx, mc, *current)
		if err != nil {
			return nil, false, err
		}
	}

	// The sweep runs on the parked path too: a claim kept there would block
	// every later claimant of a realm this plane already left.
	held, err := r.releaseUnusedRealms(ctx, mc)
	if err != nil {
		return nil, false, err
	}

	return parked, held, nil
}

// releaseUnusedRealms gives back every realm claim of mc that nothing of it
// names any more. Three things name a realm, and each of them keeps its
// claim:
//
// The realm of the spec, read from the spec alone, so a pass that resolved
// nothing sweeps the same way as one that resolved everything. Only the
// externalKeycloak mode names one here, because it is the only mode that
// claims; a plane that moves to another mode gives its realm back.
//
// status.callbackRealm, which still carries the login callbacks of this
// plane. Its claim goes once the withdrawal has cleared the record, which the
// next pass reads from the persisted status. Releasing it in the pass that
// cleared it would hand the realm on before the record of the withdrawal
// survives a crash.
//
// A Management Identity workload of the plane, because a start against that
// realm writes its clients again. A workload that writes a realm it does not
// name keeps every claim of the plane, because none of them can be shown to
// be unused.
//
// A suspended plane sweeps nothing. It touches no claim at all, so the realms
// it holds stay held until it resumes.
//
// It reports whether a workload alone kept a realm. Nothing watches the pods
// or the ReplicaSets of the plane, and Kubernetes collects them in the
// background, so the caller comes back on the retry interval to give that
// realm up once the workload is gone.
func (r *Reconciler) releaseUnusedRealms(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
) (bool, error) {
	if mc.Spec.Suspend {
		return false, nil
	}

	var current *v1.KeycloakRealmTarget
	if components.Mode(mc) == components.ModeExternalKeycloak {
		var err error
		if current, err = specRealmTarget(mc); err != nil {
			return false, err
		}
	}

	keep := []*v1.KeycloakRealmTarget{current, mc.Status.CallbackRealm}
	pointed, unknown, err := r.identityRealms(ctx, mc)
	if err != nil {
		return false, err
	}
	var held bool
	for i := range pointed {
		keep = append(keep, &pointed[i])
		held = held || (!namesRealm(current, pointed[i]) &&
			!namesRealm(mc.Status.CallbackRealm, pointed[i]))
	}

	return held, r.releaseRealmClaims(ctx, mc, unknown, keep...)
}

// namesRealm reports whether target names the realm of realm. A nil target
// names none.
func namesRealm(target *v1.KeycloakRealmTarget, realm v1.KeycloakRealmTarget) bool {
	return target != nil && components.SameRealm(*target, realm)
}

// identityRealms returns the realm of every Management Identity workload of
// mc that can still start against a Keycloak: the pod template of the
// Deployment it owns, every ReplicaSet of it that can still create a pod, and
// every pod of it that is not done. It also reports whether one of them writes
// a realm that the workload does not name. The reads go through APIReader, for
// the reason startedInitialClaim gives.
//
// The three sources answer for one another. A parked plane renders nothing,
// so the Deployment it ran before keeps its old realm and starts a pod
// against that realm whenever one goes. A rollout leaves the old ReplicaSet
// behind, which keeps the template the Deployment has moved on from. A pod
// outlives the ReplicaSet that made it.
func (r *Reconciler) identityRealms(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
) ([]v1.KeycloakRealmTarget, bool, error) {
	var realms []v1.KeycloakRealmTarget
	var unknown bool

	key := client.ObjectKey{Namespace: mc.Namespace, Name: components.IdentityName(mc)}
	var identity appsv1.Deployment
	switch err := r.APIReader.Get(ctx, key, &identity); {
	case err == nil:
		if metav1.IsControlledBy(&identity, mc) {
			templateRealms, templateUnknown := components.IdentityTemplateRealms(&identity)
			realms = append(realms, templateRealms...)
			unknown = unknown || templateUnknown
		}
	case !apierrors.IsNotFound(err):
		return nil, false, fmt.Errorf("reading Deployment %q: %w", key, err)
	}

	// A ReplicaSet carries the labels of the pod template it holds, so one
	// selector reads both.
	var sets appsv1.ReplicaSetList
	if err := r.APIReader.List(
		ctx,
		&sets,
		client.InNamespace(mc.Namespace),
		client.MatchingLabels(components.IdentityPodLabels(mc)),
	); err != nil {
		return nil, false, fmt.Errorf("listing the Management Identity ReplicaSets: %w", err)
	}
	setRealms, setUnknown := components.IdentityReplicaSetRealms(sets.Items)
	realms = append(realms, setRealms...)
	unknown = unknown || setUnknown

	var pods corev1.PodList
	if err := r.APIReader.List(
		ctx,
		&pods,
		client.InNamespace(mc.Namespace),
		client.MatchingLabels(components.IdentityPodLabels(mc)),
	); err != nil {
		return nil, false, fmt.Errorf("listing the Management Identity pods: %w", err)
	}
	podRealms, podUnknown := components.IdentityRealms(pods.Items)

	return append(realms, podRealms...), unknown || podUnknown, nil
}

// takeRealmClaim creates the claim Lease of the realm that target names. The
// API server serializes the create, so of two management clusters that reach
// this together exactly one holds the realm.
//
// It returns nil when mc holds the claim after the call. A realm that another
// management cluster holds returns the parked failure. Only a holder that is
// gone, or that a later management cluster replaced under the same name, is
// taken over.
func (r *Reconciler) takeRealmClaim(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	target v1.KeycloakRealmTarget,
) (*conditions.PreCheckFailure, error) {
	holder, failure, err := r.createRealmClaim(ctx, mc, target)
	if err != nil || failure != nil || holder == nil {
		return failure, err
	}

	keeps, err := r.realmHolderKeeps(ctx, *holder)
	if err != nil {
		return nil, err
	}
	if !keeps {
		if err := r.dropRealmClaim(ctx, target, *holder); err != nil {
			return nil, err
		}
		holder, failure, err = r.createRealmClaim(ctx, mc, target)
		if err != nil || failure != nil || holder == nil {
			return failure, err
		}
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonRealmClaimedElsewhere,
		Message: fmt.Sprintf(
			"CamundaManagementCluster %s holds realm %q of Keycloak %q. One realm answers to one "+
				"management plane, so this one waits and starts nothing new until that claim is "+
				"released. Give it a realm of its own, or delete the holder",
			holder.NamespacedName, target.Realm, components.RealmURL(target),
		),
	}, nil
}

// createRealmClaim creates the claim Lease of target for mc. It returns no
// holder and no failure when mc holds the claim after the call, which covers
// the Lease it created and the one it held already. Otherwise it returns the
// holder that the Lease records.
//
// A Lease that carries no holder annotations is not one of ours. It blocks
// without a takeover, and the failure names it.
func (r *Reconciler) createRealmClaim(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	target v1.KeycloakRealmTarget,
) (*components.RealmClaimHolder, *conditions.PreCheckFailure, error) {
	// The Lease can go away between the create and the read, when a release
	// or a takeover races this claimant. The second pass then creates it.
	for range 2 {
		err := r.Create(ctx, components.NewRealmClaimLease(r.ClaimNamespace, target, mc))
		if err == nil {
			return nil, nil, nil
		}
		if !apierrors.IsAlreadyExists(err) {
			return nil, nil, fmt.Errorf(
				"creating the claim Lease of realm %q: %w", components.RealmIdentity(target), err,
			)
		}

		lease, found, err := r.readRealmClaim(ctx, target)
		if err != nil {
			return nil, nil, err
		}
		if !found {
			continue
		}

		holder, ours := components.RealmClaimHolderOf(lease)
		if !ours {
			return nil, &conditions.PreCheckFailure{
				Reason: v1.ReasonRealmClaimedElsewhere,
				Message: fmt.Sprintf(
					"Lease %s claims realm %q of Keycloak %q and names no CamundaManagementCluster. "+
						"Delete it if nothing else uses it",
					client.ObjectKeyFromObject(lease), target.Realm, components.RealmURL(target),
				),
			}, nil
		}
		if holder.UID == mc.UID {
			return nil, nil, nil
		}

		return &holder, nil, nil
	}

	return nil, nil, fmt.Errorf(
		"the claim Lease of realm %q exists but is not readable yet", components.RealmIdentity(target),
	)
}

// realmHolderKeeps reports whether the management cluster that the Lease
// names still owns the claim. It does while it exists under the recorded UID.
// A holder that is gone, or that a later management cluster replaced under
// the same name, keeps nothing, so a crash between the claim and the release
// never blocks the realm forever.
func (r *Reconciler) realmHolderKeeps(
	ctx context.Context,
	holder components.RealmClaimHolder,
) (bool, error) {
	var other v1.CamundaManagementCluster
	if err := r.APIReader.Get(ctx, holder.NamespacedName, &other); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, fmt.Errorf("reading the realm claim holder %s: %w", holder.NamespacedName, err)
	}

	return other.UID == holder.UID, nil
}

// releaseRealmClaims deletes every realm claim Lease of the namespace of the
// operator that still names mc, except the ones of the realms that keep name.
// A nil entry of keep names no realm. The finalizer passes none, so a deleted
// management cluster gives back every realm it holds.
//
// unknown says that a Management Identity workload of mc writes a realm that
// the workload does not name, so no claim of this plane can be shown to be
// unused. Every one of them is kept then, the way IdentityTemplatePointsAtRealm
// counts such a workload as a writer of every realm.
//
// The label selector is the one that NewRealmClaimLease writes. It carries the
// name of the management cluster alone, so the list also holds the claims of
// a management cluster of another namespace with this name, and of a later
// one. The holder annotations tell them apart.
func (r *Reconciler) releaseRealmClaims(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	unknown bool,
	keep ...*v1.KeycloakRealmTarget,
) error {
	var leases coordinationv1.LeaseList
	err := r.APIReader.List(
		ctx, &leases,
		client.InNamespace(r.ClaimNamespace),
		client.MatchingLabels(components.RealmClaimLeaseLabels(mc.Name)),
	)
	if err != nil {
		return fmt.Errorf("listing the realm claim Leases of %s: %w", client.ObjectKeyFromObject(mc), err)
	}

	kept := make(map[string]bool, len(keep))
	for _, target := range keep {
		if target != nil {
			kept[components.RealmClaimLeaseName(components.RealmIdentity(*target))] = true
		}
	}

	self := realmClaimSelf(mc)
	for i := range leases.Items {
		lease := &leases.Items[i]
		if kept[lease.Name] || unknown {
			continue
		}
		if recorded, ours := components.RealmClaimHolderOf(lease); !ours || recorded != self {
			continue
		}
		if err := r.deleteRealmClaim(ctx, lease); err != nil {
			return err
		}
	}

	return nil
}

// dropRealmClaim deletes the claim Lease of target while its annotations
// still name holder. A Lease that is gone, or that another management cluster
// holds, is left alone.
func (r *Reconciler) dropRealmClaim(
	ctx context.Context,
	target v1.KeycloakRealmTarget,
	holder components.RealmClaimHolder,
) error {
	lease, found, err := r.readRealmClaim(ctx, target)
	if err != nil || !found {
		return err
	}

	if recorded, ours := components.RealmClaimHolderOf(lease); !ours || recorded != holder {
		return nil
	}

	return r.deleteRealmClaim(ctx, lease)
}

// deleteRealmClaim deletes lease under the UID and the resourceVersion that it
// was read with. A Lease that is gone is left alone, and one that changed in
// between fails the preconditions and returns the error, so the caller reads
// it again on its next look.
func (r *Reconciler) deleteRealmClaim(ctx context.Context, lease *coordinationv1.Lease) error {
	err := r.Delete(ctx, lease, client.Preconditions{
		UID: &lease.UID, ResourceVersion: &lease.ResourceVersion,
	})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf(
			"deleting the realm claim Lease %s: %w", client.ObjectKeyFromObject(lease), err,
		)
	}

	return nil
}

// readRealmClaim reads the claim Lease of target live. Every claim decision
// reads the API server. A claim decided from a cache is no serialization.
func (r *Reconciler) readRealmClaim(
	ctx context.Context,
	target v1.KeycloakRealmTarget,
) (*coordinationv1.Lease, bool, error) {
	name := types.NamespacedName{
		Namespace: r.ClaimNamespace,
		Name:      components.RealmClaimLeaseName(components.RealmIdentity(target)),
	}

	var lease coordinationv1.Lease
	if err := r.APIReader.Get(ctx, name, &lease); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}

		return nil, false, fmt.Errorf("reading the realm claim Lease %s: %w", name, err)
	}

	return &lease, true, nil
}

// realmClaimSelf is the holder identity of mc.
func realmClaimSelf(mc *v1.CamundaManagementCluster) components.RealmClaimHolder {
	return components.RealmClaimHolder{
		NamespacedName: client.ObjectKeyFromObject(mc),
		UID:            mc.UID,
	}
}
