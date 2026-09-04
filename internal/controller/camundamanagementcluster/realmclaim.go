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
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/leaseclaim"
)

// claimRealm takes the claim on the Keycloak realm that the spec names, and
// gives back every realm claim of this management plane that nothing of it
// names any more, which releaseUnusedRealms decides.
//
// Both Keycloak modes claim. The oidc mode administers no realm and claims
// nothing. The Keycloak that the operator runs is claimed like any other: an
// externalKeycloak plane can name the Service URL of that Keycloak, reach the
// same realm through it, and run a second Management Identity over the clients
// of the plane that owns it. A suspended plane touches no claim: the realm it
// holds stays held, and a realm the spec now names is claimed on resume.
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

	current := components.RealmTarget(res.Input.Provider)
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

// realmClaims is the claim protocol on the Keycloak realms, over the Leases
// of the namespace of the operator. Its reads go through the uncached
// APIReader, because a claim decided from a cache is no serialization.
func (r *Reconciler) realmClaims() *leaseclaim.Claim {
	return leaseclaim.New(
		components.RealmClaimSchema(), r.Client, r.APIReader, r.ClaimNamespace, r.realmHolderKeeps,
	)
}

// releaseUnusedRealms gives back every realm claim of mc that nothing of it
// names any more. Three things name a realm, and each of them keeps its
// claim:
//
// The realm of the spec, read from the spec alone, so a pass that resolved
// nothing sweeps the same way as one that resolved everything. The oidc mode
// names none, so a plane that moves to it gives its realm back.
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
// It reports whether a claim of this plane stayed only because a workload
// names its realm. Nothing watches the pods or the ReplicaSets of the plane,
// and Kubernetes collects them in the background, so the caller comes back on
// the retry interval to give that claim up once the workload is gone.
func (r *Reconciler) releaseUnusedRealms(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
) (bool, error) {
	if mc.Spec.Suspend {
		return false, nil
	}

	// The claims are read first. A plane that holds none has nothing to give
	// back, and the workloads below only say which of them to keep, so their
	// three reads would answer a question nobody asked.
	leases, err := r.realmClaimLeases(ctx, mc)
	if err != nil || len(leases) == 0 {
		return false, err
	}

	current, err := specRealmTarget(mc)
	if err != nil {
		return false, err
	}

	keep := []*v1.KeycloakRealmTarget{current, mc.Status.CallbackRealm}
	pointed, unknown, err := r.identityRealms(ctx, mc)
	if err != nil {
		return false, err
	}
	var workload []*v1.KeycloakRealmTarget
	for i := range pointed {
		keep = append(keep, &pointed[i])
		if !namesRealm(current, pointed[i]) && !namesRealm(mc.Status.CallbackRealm, pointed[i]) {
			workload = append(workload, &pointed[i])
		}
	}

	return r.releaseRealmClaims(ctx, leases, keep, workload, unknown)
}

// realmClaimLeases reads every realm claim Lease of the namespace of the
// operator that names mc as its holder.
//
// The label selector is the one that NewRealmClaimLease writes. It carries the
// name of the management cluster alone, so the list also holds the claims of a
// management cluster of another namespace with this name, and of a later one.
// The holder annotations tell them apart.
func (r *Reconciler) realmClaimLeases(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
) ([]coordinationv1.Lease, error) {
	return r.realmClaims().Held(ctx, realmClaimSelf(mc))
}

// namesRealm reports whether target names the realm of realm. A nil target
// names none.
func namesRealm(target *v1.KeycloakRealmTarget, realm v1.KeycloakRealmTarget) bool {
	return target != nil && components.SameRealm(*target, realm)
}

// identityRealms returns the realm of every Management Identity workload of
// mc that can still start against a Keycloak: the pod template of the
// Deployment at its derived name, every ReplicaSet of it that can still
// create a pod, and every pod of it that is not done. It also reports whether
// one of them writes a realm that the workload does not name. The reads go
// through APIReader, for the reason startedInitialClaim gives.
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
		// The owner is not read here, for the reason the ReplicaSets below
		// give: a Deployment at this name starts a pod against the realm its
		// template names whoever owns it.
		templateRealms, templateUnknown := components.IdentityTemplateRealms(&identity)
		realms = append(realms, templateRealms...)
		unknown = unknown || templateUnknown
	case !apierrors.IsNotFound(err):
		return nil, false, fmt.Errorf("reading Deployment %q: %w", key, err)
	}

	// A ReplicaSet carries the labels of the pod template it holds, so one
	// selector reads both. The labels alone are enough to keep a claim, where
	// the finalizer reads the owner reference before it deletes: a workload
	// that runs Management Identity against a realm writes its clients
	// whoever owns it, and keeping the claim of that realm is the safe answer
	// either way.
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

// takeRealmClaim takes the claim Lease of the realm that target names, and
// turns a claim this plane does not hold into the parked failure. The API
// server serializes the create, so of two management clusters that reach this
// together exactly one holds the realm.
//
// It returns nil when mc holds the claim after the call. A realm that another
// management cluster holds returns the parked failure. A Lease that carries
// no holder annotations is not one of ours: it blocks without a takeover, and
// the failure names it.
func (r *Reconciler) takeRealmClaim(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	target v1.KeycloakRealmTarget,
) (*conditions.PreCheckFailure, error) {
	blocker, err := r.realmClaims().Take(ctx, mc, components.RealmIdentity(target))
	if err != nil || blocker == nil {
		return nil, err
	}

	if blocker.Foreign() {
		return &conditions.PreCheckFailure{
			Reason: v1.ReasonRealmClaimedElsewhere,
			Message: fmt.Sprintf(
				"Lease %s claims realm %q of Keycloak %q and names no CamundaManagementCluster. "+
					"Delete it if nothing else uses it",
				blocker.Lease, target.Realm, components.RealmURL(target),
			),
		}, nil
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonRealmClaimedElsewhere,
		Message: fmt.Sprintf(
			"CamundaManagementCluster %s holds realm %q of Keycloak %q. One realm answers to one "+
				"management plane, so this one waits and starts nothing new until that claim is "+
				"released. Give it a realm of its own, or delete the holder",
			blocker.Holder.NamespacedName, target.Realm, components.RealmURL(target),
		),
	}, nil
}

// realmHolderKeeps reports whether the management cluster that the Lease
// names still owns the claim. It does while it exists under the recorded UID.
func (r *Reconciler) realmHolderKeeps(
	ctx context.Context,
	holder leaseclaim.Holder,
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

// releaseRealmClaims deletes every Lease of leases, which realmClaimLeases
// read, except the ones of the realms that keep names. A nil entry of keep
// names no realm. The finalizer passes none, so a deleted management cluster
// gives back every realm it holds.
//
// workload names the realms that only a Management Identity workload of the
// plane keeps, and it is a part of keep. The bool reports that a Lease of one
// of them is there and stayed, which is the state that no watch ends.
//
// unknown says that a Management Identity workload of the plane writes a realm
// that the workload does not name, so no claim of this plane can be shown to
// be unused. Every one of them is kept then, the way
// IdentityTemplatePointsAtRealm counts such a workload as a writer of every
// realm, and each one reads as held by that workload.
func (r *Reconciler) releaseRealmClaims(
	ctx context.Context,
	leases []coordinationv1.Lease,
	keep []*v1.KeycloakRealmTarget,
	workload []*v1.KeycloakRealmTarget,
	unknown bool,
) (bool, error) {
	claims := r.realmClaims()
	kept := leaseNames(keep)
	held := leaseNames(workload)

	var workloadHolds bool
	for i := range leases {
		lease := &leases[i]
		if kept[lease.Name] {
			workloadHolds = workloadHolds || held[lease.Name]

			continue
		}
		// A workload that writes a realm it does not name can write this one,
		// so the claim stays and the caller comes back for it.
		if unknown {
			workloadHolds = true

			continue
		}
		if err := claims.Release(ctx, lease); err != nil {
			return false, err
		}
	}

	return workloadHolds, nil
}

// leaseNames is the set of claim Lease names of targets. A nil entry names no
// realm.
func leaseNames(targets []*v1.KeycloakRealmTarget) map[string]bool {
	names := make(map[string]bool, len(targets))
	for _, target := range targets {
		if target != nil {
			names[components.RealmClaimLeaseName(components.RealmIdentity(*target))] = true
		}
	}

	return names
}

// readRealmClaim reads the claim Lease of target live, and reports whether it
// is there.
func (r *Reconciler) readRealmClaim(
	ctx context.Context,
	target v1.KeycloakRealmTarget,
) (*coordinationv1.Lease, bool, error) {
	return r.realmClaims().Read(ctx, components.RealmIdentity(target))
}

// realmClaimSelf is the holder identity of mc.
func realmClaimSelf(mc *v1.CamundaManagementCluster) components.RealmClaimHolder {
	return components.RealmClaimHolder{
		NamespacedName: client.ObjectKeyFromObject(mc),
		UID:            mc.UID,
	}
}
