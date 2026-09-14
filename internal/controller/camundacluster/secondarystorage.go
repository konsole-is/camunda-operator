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
	"errors"
	"fmt"
	"slices"
	"strings"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// eventReasonStorageClaimed is recorded when the cluster takes the storage
// claim of its backend.
const eventReasonStorageClaimed = "StorageClaimed"

// StorageClaimFinalizer keeps a deleted CamundaCluster alive until it gives
// back the backends it holds. Without it a deleted cluster leaves a claim that
// only the next claimant of that backend clears, and a cluster that is created
// and deleted on an endpoint of its own leaves one behind for good.
const StorageClaimFinalizer = "core.camunda.io/storage-claim"

// claimStorage takes the storage claim of the backend that resolveStorage
// resolved, or records the cluster that holds it. The claim key is the
// backend, so two contracts that name one address meet on one Lease. The
// first CamundaCluster that takes the claim holds it while it exists; a
// holder that is gone is taken over. A live holder lands on
// in.Storage.Holder and the controller renders this cluster suspended. A
// cluster whose backend pods of other clusters still carry lands on
// in.Storage.Handover, and the controller renders it suspended too: those pods
// write the backend. That holds whether this cluster took the claim or waits
// under those pods to take it. It needs in.Storage from resolveStorage.
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
	//
	// A suspended cluster is no claimant to order. It renders at zero and
	// writes nothing beside those pods, so it takes the backend and holds it
	// for when it resumes, and its Ready keeps the reason the user asked for.
	var waitingFor []string
	blocker, err := res.claims.TakeUnclaimed(ctx, res.cluster, key, func(ctx context.Context) error {
		if in.Effective.Suspend {
			return nil
		}

		waitingFor, err = res.podsUnderTheBackend(ctx, in.Storage.Claim)
		if err != nil {
			return err
		}
		if len(waitingFor) > 0 {
			return errPodsOnBackend
		}

		return nil
	})
	if err != nil && !errors.Is(err, errPodsOnBackend) {
		return err
	}
	// The wait is the one a cluster that holds the claim reports, and it reads
	// the same way: the cluster renders at zero and Ready says which pods it
	// waits for. A cluster that kept running here would write the backend
	// beside them, which is what the wait exists to prevent.
	if len(waitingFor) > 0 {
		in.Storage.Handover = &components.StorageHandover{Backend: key, Pods: waitingFor}

		return nil
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

	// The list costs a read of one namespace, so it is taken only where its
	// answer decides the gate: a suspended cluster waits for nothing, and a
	// cluster that took the claim on this pass meets the pods of whoever held
	// it before, whatever its own pods carry.
	var own components.PodClaims
	if !in.Effective.Suspend && held {
		if own, err = res.claimsOnOwnPods(ctx); err != nil {
			return err
		}
	}
	if !handoverPossible(in.Effective.Suspend, held, own.Carries(in.Storage.Claim)) {
		return nil
	}

	// Only a takeover leaves a pod of another cluster on this backend: no other
	// cluster holds it while this one does, and a cluster that holds no backend
	// renders nothing. A cluster with no pod of its own on it can be waiting
	// still, whatever it last reported, because the render at zero is what
	// keeps those pods away and a status write that never landed must not end
	// the wait. Every healthy pass would read the pods of the whole Kubernetes
	// cluster without that.
	//
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
// storage claim that this cluster now holds, which is what the wide list below
// answers. suspended is spec.suspend of the cluster, heldAtStart says whether
// it already held the claim when the pass began, and ownPodOnClaim whether a
// pod of it writes that backend.
func handoverPossible(suspended, heldAtStart, ownPodOnClaim bool) bool {
	if suspended {
		return false
	}

	return !heldAtStart || !ownPodOnClaim
}

// errPodsOnBackend refuses the claim on a free backend from inside the rule of
// TakeUnclaimed, which writes no Lease when the rule returns an error. It never
// leaves claimStorage: the pods it stands for become the handover that the
// caller reports.
var errPodsOnBackend = errors.New("pods of another cluster write the backend")

// podsUnderTheBackend returns the pods of other clusters that still write the
// backend of claim. Its own pods are no reason to wait: a holder whose Lease was
// deleted by hand meets them, writes the Lease again, and keeps running.
func (res *resolver) podsUnderTheBackend(ctx context.Context, claim string) ([]string, error) {
	return components.OtherPodsOnClaim(ctx, res.reader, claim, res.cluster.UID)
}

// addClaimFinalizer writes the finalizer that keeps a deleted cluster alive
// until it gives its backends back, and reports whether the reconcile must
// stop. A deletion between the claim and the next write would otherwise leave a
// backend claimed by a cluster that is gone, and every later claimant of it
// waiting for a holder that no longer exists.
func (r *CamundaClusterReconciler) addClaimFinalizer(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) (bool, error) {
	if !controllerutil.AddFinalizer(cluster, StorageClaimFinalizer) {
		return false, nil
	}
	if err := r.Update(ctx, cluster); err != nil {
		// A deletion that races this write is fine. The deletion path owns the
		// object from here.
		if apierrors.IsNotFound(err) {
			return true, nil
		}

		return false, fmt.Errorf("adding the storage claim finalizer: %w", err)
	}

	return false, nil
}

// finalizeStorageClaims gives back every storage claim of a deleted cluster and
// removes the finalizer. The pods of a deleting cluster hold no claim back: the
// next claimant of that backend meets them through its own handover gate, and
// waits for them there.
func (r *CamundaClusterReconciler) finalizeStorageClaims(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) error {
	if !controllerutil.ContainsFinalizer(cluster, StorageClaimFinalizer) {
		return nil
	}

	claims := components.StorageClaimSchema().NewClaim(r.Client, r.APIReader, r.ClaimNamespace)
	leases, err := claims.Held(ctx, cluster)
	if err != nil {
		return err
	}
	for i := range leases {
		if err := claims.Release(ctx, &leases[i]); err != nil {
			return err
		}
	}

	controllerutil.RemoveFinalizer(cluster, StorageClaimFinalizer)
	if err := r.Update(ctx, cluster); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("removing the storage claim finalizer: %w", err)
	}

	return nil
}

// claimsOnOwnPods reads the storage claims that the pods of this cluster carry.
// The handover gate reads it to learn whether a takeover of the backend it
// holds is over. The release of the backends it left takes a list of its own,
// after the apply, because a pod created in between carries the claim this one
// would not show.
func (res *resolver) claimsOnOwnPods(ctx context.Context) (components.PodClaims, error) {
	return components.ClaimsOnOwnPods(ctx, res.reader, res.cluster.Namespace, res.cluster.UID, nil)
}

// releaseLeftBackends gives back every storage claim of the cluster except
// keep, and returns the claims it held back. A cluster that moved to another
// backend holds two claims until here, and the old one must go so the next
// cluster can take that backend.
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

	own, err := components.ClaimsOnOwnPods(ctx, r.APIReader, cluster.Namespace, cluster.UID, nil)
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
