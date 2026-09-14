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
	"strings"

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
// in.Storage.Holder and the controller renders this cluster suspended. Either
// way the cluster releases every other storage claim it holds, so a repoint
// frees the old backend. A cluster that holds the claim while pods of other
// clusters still carry it lands on in.Storage.Handover, and the controller
// renders it suspended too: those pods write the backend. It needs in.Storage
// from resolveStorage.
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

	blocker, err := res.claims.Take(ctx, res.cluster, key)
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
		// A parked cluster renders nothing, so it gives its previous backend
		// back here too. Two clusters that swap backends in one step meet
		// each other's claim, and each one keeps the other parked forever
		// while it holds a backend it no longer writes. Its pods still carry
		// the previous claim, so the next cluster on that backend waits for
		// them.
		if err := res.releaseOtherClaims(ctx, in.Storage.Claim); err != nil {
			return err
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

	if err := res.releaseOtherClaims(ctx, in.Storage.Claim); err != nil {
		return err
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

// releaseOtherClaims gives back every storage claim of the cluster except
// keep. A cluster that moved to another backend holds two claims until here,
// and the old one must go so the next cluster can take that backend.
func (res *resolver) releaseOtherClaims(ctx context.Context, keep string) error {
	leases, err := res.claims.Held(ctx, res.cluster)
	if err != nil {
		return err
	}

	for i := range leases {
		if leases[i].Name == keep {
			continue
		}
		if err := res.claims.Release(ctx, &leases[i]); err != nil {
			return err
		}
	}

	return nil
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
	message := fmt.Sprintf(
		"Pods of another cluster, or of its Optimize instance, still write the backend %q: %s. "+
			"This cluster starts when they are gone",
		handover.Backend, strings.Join(handover.Pods, ", "),
	)
	return conditions.Ready(
		metav1.ConditionFalse,
		v1.ReasonWaitingForHandover,
		appendApplyFailure(message, applyErr),
		cluster.Generation,
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
