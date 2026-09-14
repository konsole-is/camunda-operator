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

package camundacluster

import (
	"context"
	"fmt"
	"slices"
	"strings"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// eventReasonStorageClaimed is recorded when the cluster takes the storage
// claim of its backend.
const eventReasonStorageClaimed = "StorageClaimed"

// claimStorage takes the storage claim of the backend that resolveStorage
// resolved, or records the cluster that holds it. The claim key is the
// backend, so two contracts that name one address meet on one Lease. The
// first CamundaCluster that takes the claim holds it while it exists; a
// holder that is gone is taken over. A live holder lands on
// in.Storage.Holder and the controller renders this cluster suspended. A
// cluster that holds the claim while pods of other clusters still carry it
// lands on in.Storage.Handover, and the controller renders it suspended too:
// those pods write the backend. It needs in.Storage from resolveStorage.
//
// It takes claims and never gives one back. A backend that this cluster left
// goes back after the render that stops writing it was applied, see
// releaseLeftBackends.
func (res *resolver) claimStorage(ctx context.Context, in *components.Input) error {
	key, err := components.StorageClaimKey(in.Storage)
	if err != nil {
		return components.StorageClaimKeyFailure(client.ObjectKeyFromObject(res.storage), err)
	}
	in.Storage.Claim = res.claims.LeaseName(key)

	held, err := res.claims.Holds(ctx, key, res.cluster)
	if err != nil {
		return err
	}

	// A free backend is not free while pods of another cluster write it. The
	// rule runs only while no Lease holds the key, so it orders the claimants
	// of a backend whose Lease is gone: the cluster whose own pods write it
	// creates it again, and every other cluster waits.
	blocker, err := res.claims.TakeUnclaimed(ctx, res.cluster, key, func(ctx context.Context) error {
		return res.refuseUnderOtherPods(ctx, in.Storage.Claim, key)
	})
	if err != nil {
		return err
	}
	if blocker != nil {
		if blocker.Foreign() {
			// Nothing watches that Lease for this cluster, so its deletion
			// arrives on the retry timer and not through an event.
			return conditions.NewUnwatchedFailure(
				v1.ReasonInvalidReference,
				fmt.Sprintf(
					"Lease %s claims the backend %q and names no CamundaCluster. "+
						"Delete it if nothing uses it",
					blocker.Lease, key,
				),
			)
		}
		in.Storage.Holder = &components.StorageHolder{
			Cluster: blocker.Holder.NamespacedName,
			Backend: key,
		}

		return nil
	}

	if !held {
		res.recorder.Eventf(
			res.cluster,
			nil,
			corev1.EventTypeNormal,
			eventReasonStorageClaimed,
			eventActionReconcile,
			"Claimed the backend %q through SecondaryStorageConfig %q",
			key,
			res.storage.Name,
		)
	}

	own, err := res.claimsOnOwnPods(ctx)
	if err != nil {
		return err
	}
	if !handoverPossible(held, own.Carries(in.Storage.Claim)) {
		return nil
	}

	// The list is read once, before the render. It covers the pods that the
	// previous holder started before it lost the claim. A holder that is
	// pointed back at the backend meets the claim this cluster holds and
	// parks, so it starts nothing beside it.
	pods, err := components.OtherPodsOnClaim(ctx, res.reader, in.Storage.Claim, res.cluster.UID)
	if err != nil {
		return err
	}
	if len(pods) > 0 {
		in.Storage.Handover = &components.StorageHandover{Backend: key, Pods: pods}
	}

	return nil
}

// handoverPossible reports whether a pod of another cluster can carry the
// storage claim that this cluster now holds. Only a takeover leaves such a pod
// behind: no other cluster holds the backend while this one does, and a cluster
// that holds no backend renders nothing.
//
// heldAtStart says whether the cluster already held the claim when the pass
// began, and ownPodOnClaim whether a pod of this cluster writes that backend.
// Both are needed. A pass that took the claim can meet the pods of the cluster
// it took it from. A cluster with no pod of its own on the backend can be
// waiting still, whatever it last reported: the render at zero is what keeps
// the pods away, and a status write that never landed must not end the wait.
// Every healthy pass of every cluster would list the pods of the whole
// Kubernetes cluster without this.
func handoverPossible(heldAtStart, ownPodOnClaim bool) bool {
	return !heldAtStart || !ownPodOnClaim
}

// refuseUnderOtherPods stops this cluster from taking a backend that pods of
// another cluster still write. Its own pods are no reason to wait: a holder
// whose Lease was deleted by hand meets them, writes the Lease again, and keeps
// running.
//
// The failure is the one the handover gate reports, and it is unwatched for the
// same reason: nothing tells this cluster when those pods go.
func (res *resolver) refuseUnderOtherPods(ctx context.Context, claim, key string) error {
	pods, err := components.OtherPodsOnClaim(ctx, res.reader, claim, res.cluster.UID)
	if err != nil {
		return err
	}
	if len(pods) == 0 {
		return nil
	}

	return conditions.NewUnwatchedFailure(v1.ReasonWaitingForHandover, handoverMessage(key, pods))
}

// claimsOnOwnPods reads the storage claims that the pods of this cluster carry.
// One list serves the handover gate of the backend it holds and, after the
// apply, the release of the backends it left.
func (res *resolver) claimsOnOwnPods(ctx context.Context) (components.PodClaims, error) {
	return components.ClaimsOnOwnPods(ctx, res.reader, res.cluster.Namespace, res.cluster.UID)
}

// releaseLeftBackends gives back every storage claim of the cluster except
// keep, and returns the claims it held back. A cluster that moved to another
// backend holds two claims until here, and the old one must go so the next
// cluster can take that backend. An empty keep releases every claim it holds,
// which is what a cluster that resolves no backend at all asks for.
//
// It runs after the render was applied, never before: a pass that released and
// then failed to apply would leave the old StatefulSet recreating pods into a
// backend that another cluster may take.
//
// A cluster gives a backend back only when none of its own pods writes it any
// more. The next claimant reads the pods of that backend once, before it
// renders, so a release under running pods can let it start beside them.
// Nothing reports the end of a pod drain, so the caller looks again on its
// timer while a claim is held back.
func (r *CamundaClusterReconciler) releaseLeftBackends(
	ctx context.Context,
	cluster *v1.CamundaCluster,
	keep string,
) ([]string, error) {
	claims := components.StorageClaimSchema().NewClaim(r.Client, r.APIReader, r.ClaimNamespace)
	leases, err := claims.Held(ctx, cluster)
	if err != nil {
		return nil, err
	}

	candidates := make([]*coordinationv1.Lease, 0, len(leases))
	for i := range leases {
		if leases[i].Name != keep {
			candidates = append(candidates, &leases[i])
		}
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	own, err := components.ClaimsOnOwnPods(ctx, r.APIReader, cluster.Namespace, cluster.UID)
	if err != nil {
		return nil, err
	}

	var heldBack []string
	for _, lease := range candidates {
		if own.Carries(lease.Name) {
			heldBack = append(heldBack, lease.Name)

			continue
		}
		if err := claims.Release(ctx, lease); err != nil {
			return nil, err
		}
	}
	slices.Sort(heldBack)

	return heldBack, nil
}

// claimSuspends reports whether the storage claim keeps this cluster at zero:
// another cluster holds the backend, or pods of another cluster still write
// the one this cluster took over. Nothing watches either of them for this
// cluster, so the controller looks again on its timer while it holds.
func claimSuspends(storage components.Storage) bool {
	return storage.Holder != nil || storage.Handover != nil
}

// storageHeld builds the Ready condition of a cluster whose backend another
// cluster holds. When applyErr is set, the message carries it as the error of
// the last apply.
func storageHeld(cluster *v1.CamundaCluster, holder *components.StorageHolder, applyErr error) metav1.Condition {
	message := fmt.Sprintf(
		"CamundaCluster %q already holds the backend %q. One CamundaCluster holds one backend, "+
			"so this cluster stays suspended until that cluster moves to another backend or is deleted",
		objectPath(holder.Cluster), holder.Backend,
	)
	return conditions.Ready(
		metav1.ConditionFalse,
		v1.ReasonStorageAlreadyAttached,
		appendApplyFailure(message, applyErr),
		cluster.Generation,
	)
}

// storageHandover builds the Ready condition of a cluster that holds the
// storage claim and waits for the pods of other clusters on that backend. A
// deleted holder leaves its pods to the garbage collector, and a holder that
// moved replaces them through a rollout. When applyErr is set, the message
// carries it as the error of the last apply.
func storageHandover(
	cluster *v1.CamundaCluster,
	handover *components.StorageHandover,
	applyErr error,
) metav1.Condition {
	return conditions.Ready(
		metav1.ConditionFalse,
		v1.ReasonWaitingForHandover,
		appendApplyFailure(handoverMessage(handover.Backend, handover.Pods), applyErr),
		cluster.Generation,
	)
}

// handoverMessage names the backend and the pods of other clusters that still
// write it. The claim rule and the handover gate report the same wait, so they
// read the same to a user.
func handoverMessage(backend string, pods []string) string {
	return fmt.Sprintf(
		"Pods of another cluster, or of its Optimize instance, still write the backend %q: %s. "+
			"This cluster starts when they are gone",
		backend, strings.Join(pods, ", "),
	)
}

// appendApplyFailure adds the error of the last apply to a claim message. The
// claim reason stays on Ready while an apply fails, so the message carries
// what went wrong with it.
func appendApplyFailure(message string, applyErr error) string {
	if applyErr == nil {
		return message
	}

	return message + fmt.Sprintf(". The last apply of the suspended workloads failed: %s", applyErr)
}
