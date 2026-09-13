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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/labels"
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
// cluster that holds the claim releases every other storage claim it holds,
// so a repoint frees the old backend, and then waits for the pods of other
// clusters that still write this one. It needs in.Storage from
// resolveStorage.
func (res *resolver) claimStorage(ctx context.Context, in *components.Input) error {
	key, err := components.StorageClaimKey(in.Storage)
	if err != nil {
		return &conditions.PreCheckFailure{
			Reason: v1.ReasonInvalidReference,
			Message: fmt.Sprintf(
				"SecondaryStorageConfig %q: %s",
				objectPath(client.ObjectKeyFromObject(res.storage)), err,
			),
		}
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
			return &conditions.PreCheckFailure{
				Reason: v1.ReasonInvalidReference,
				Message: fmt.Sprintf(
					"Lease %s claims the backend %q and names no CamundaCluster. "+
						"Delete it if nothing uses it",
					blocker.Lease, key,
				),
			}
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

	return res.waitForHandover(ctx, in.Storage.Claim, key)
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

// waitForHandover fails with WaitingForHandover while a pod of another
// cluster still carries the storage claim: a deleted holder leaves its pods
// to the garbage collector, and a repointed one replaces them through a
// rollout. Nothing watches those pods for this cluster, so the failure is
// unwatched and the controller looks again on its timer.
//
// The list is read once, before the render. A previous holder that is pointed
// back at the backend in that moment can start pods next to this cluster, and
// it stops again as soon as it meets the claim.
func (res *resolver) waitForHandover(ctx context.Context, claim, key string) error {
	pods, err := res.otherPodsOnClaim(ctx, claim)
	if err != nil {
		return err
	}
	if len(pods) == 0 {
		return nil
	}

	return conditions.NewUnwatchedFailure(
		v1.ReasonWaitingForHandover,
		fmt.Sprintf(
			"Pods of another CamundaCluster still write the backend %q: %s. "+
				"This cluster starts when they are gone",
			key, strings.Join(pods, ", "),
		),
	)
}

// otherPodsOnClaim returns the pods that carry the storage claim and another
// cluster UID, as sorted "namespace/name" paths. The list covers every
// namespace: two clusters of two namespaces meet on one Lease, so a previous
// holder elsewhere leaves pods that write this backend.
//
// The pods of a CamundaOptimize attached to another cluster carry the same
// labels, see pkg/components/camundaoptimize, and its importer writes the
// backend like a pod of that cluster.
func (res *resolver) otherPodsOnClaim(ctx context.Context, claim string) ([]string, error) {
	var pods corev1.PodList
	if err := res.reader.List(
		ctx,
		&pods,
		client.MatchingLabels(map[string]string{labels.StorageClaimKey: labels.OwnerName(claim)}),
	); err != nil {
		return nil, fmt.Errorf("listing the pods on storage claim %q: %w", claim, err)
	}

	var names []string
	for i := range pods.Items {
		if pods.Items[i].Labels[labels.ClusterUIDKey] == string(res.cluster.UID) {
			continue
		}
		names = append(names, objectPath(client.ObjectKeyFromObject(&pods.Items[i])))
	}
	slices.Sort(names)

	return names, nil
}

// storageHeld builds the Ready condition of a cluster whose backend another
// cluster holds. When applyErr is set, the message carries it as the error of
// the last apply.
func storageHeld(cluster *v1.CamundaCluster, holder *components.StorageHolder, applyErr error) metav1.Condition {
	message := fmt.Sprintf(
		"CamundaCluster %q already writes the backend %q. One CamundaCluster writes one backend, "+
			"so this cluster stays suspended until that one releases it",
		objectPath(holder.Cluster), holder.Backend,
	)
	if applyErr != nil {
		message += fmt.Sprintf(". The last apply of the suspended workloads failed: %s", applyErr)
	}

	return conditions.Ready(metav1.ConditionFalse, v1.ReasonStorageAlreadyAttached, message, cluster.Generation)
}
