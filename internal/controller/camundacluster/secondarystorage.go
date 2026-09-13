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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/secondarystorageconfig"
)

// eventReasonStorageClaimed is recorded when the cluster takes the claim on
// its storage contract.
const eventReasonStorageClaimed = "StorageClaimed"

// claimStorage gives this cluster the storage contract, or records the
// cluster that holds it. The first CamundaCluster that claims a contract
// holds it. A claim whose holder is gone, or names another contract, is
// stale and is taken over once no pod of that holder runs on the contract.
// Until then the claim stays and the step fails with WaitingForHandover. A
// live holder lands on in.Storage.Holder and the controller renders this
// cluster suspended. It needs res.storage from resolveStorage. A conflict on
// the claim is a transient error.
func (res *resolver) claimStorage(ctx context.Context, in *components.Input) error {
	self := secondarystorageconfig.Holder{
		Cluster: client.ObjectKeyFromObject(res.cluster),
		UID:     res.cluster.UID,
	}

	holder, held := secondarystorageconfig.HolderOf(res.storage)
	if held && holder == self {
		return nil
	}

	if held {
		stale, err := res.staleHolder(ctx, holder)
		if err != nil {
			return err
		}
		if !stale {
			in.Storage.Holder = &components.StorageHolder{
				Cluster:  holder.Cluster,
				Contract: client.ObjectKeyFromObject(res.storage),
			}

			return nil
		}

		if err := res.waitForHandover(ctx, holder); err != nil {
			return err
		}
	}

	if err := secondarystorageconfig.Claim(ctx, res.writer, res.storage, self); err != nil {
		return err
	}

	res.recorder.Eventf(
		res.cluster,
		nil,
		corev1.EventTypeNormal,
		eventReasonStorageClaimed,
		eventActionReconcile,
		"Claimed SecondaryStorageConfig %q",
		res.storage.Name,
	)

	return nil
}

// staleHolder reports whether holder no longer uses the contract: it does
// not exist, it is a later cluster with the same name, or its storageRef
// names another contract. The read is live, so a holder that was just
// repointed is seen as it is. A paused holder keeps its claim. Its pods stay
// where they are while it is paused, so a repoint counts only after it is
// unpaused.
func (res *resolver) staleHolder(ctx context.Context, holder secondarystorageconfig.Holder) (bool, error) {
	var owner v1.CamundaCluster
	if err := res.reader.Get(ctx, holder.Cluster, &owner); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}

		return false, fmt.Errorf(
			"reading the holder %s of SecondaryStorageConfig %s: %w",
			holder.Cluster, client.ObjectKeyFromObject(res.storage), err,
		)
	}

	held := client.ObjectKey{Namespace: holder.Cluster.Namespace, Name: owner.Spec.StorageRef}
	repointed := !owner.Spec.Pause && held != client.ObjectKeyFromObject(res.storage)

	return owner.UID != holder.UID || repointed, nil
}

// waitForHandover fails with WaitingForHandover while a pod of the stale
// holder still runs on the contract: a deleted holder leaves its pods to
// the garbage collector, and a repointed one replaces them through a
// rollout.
// Nothing watches those pods for this cluster, so the failure is unwatched
// and the controller looks again on its timer.
//
// The list is read once, before the claim moves. A previous holder that is
// pointed back at the contract in that moment can start pods next to the
// taker, and it stops again as soon as it reads the new claim.
func (res *resolver) waitForHandover(ctx context.Context, holder secondarystorageconfig.Holder) error {
	pods, err := res.holderPods(ctx, holder)
	if err != nil {
		return err
	}
	if len(pods) == 0 {
		return nil
	}

	return conditions.NewUnwatchedFailure(
		v1.ReasonWaitingForHandover,
		fmt.Sprintf(
			"Pods of the previous holder CamundaCluster %q still run on SecondaryStorageConfig %q: %s. "+
				"This cluster starts when they are gone",
			objectPath(holder.Cluster), objectPath(client.ObjectKeyFromObject(res.storage)),
			strings.Join(pods, ", "),
		),
	)
}

// holderPods returns the sorted names of the pods of holder that carry the
// storage labels of the contract. A holder in another namespace names a
// contract of its own, so it has no pods on this one.
//
// The pods of a CamundaOptimize attached to the holder carry the same pair,
// see pkg/components/camundaoptimize. Its importer writes the analytics
// indices of this contract, so it holds the handover like a pod of the
// cluster itself.
func (res *resolver) holderPods(ctx context.Context, holder secondarystorageconfig.Holder) ([]string, error) {
	if holder.Cluster.Namespace != res.storage.Namespace {
		return nil, nil
	}

	var pods corev1.PodList
	if err := res.reader.List(
		ctx,
		&pods,
		client.InNamespace(holder.Cluster.Namespace),
		client.MatchingLabels(components.StoragePodLabels(holder.Cluster.Name, res.storage.Name)),
	); err != nil {
		return nil, fmt.Errorf(
			"listing the pods of the holder %s of SecondaryStorageConfig %s: %w",
			holder.Cluster, client.ObjectKeyFromObject(res.storage), err,
		)
	}

	var names []string
	for i := range pods.Items {
		names = append(names, pods.Items[i].Name)
	}
	slices.Sort(names)

	return names, nil
}

// storageHeld builds the Ready condition of a cluster whose storage contract
// another cluster holds. When applyErr is set, the message carries it as the
// error of the last apply.
func storageHeld(cluster *v1.CamundaCluster, holder *components.StorageHolder, applyErr error) metav1.Condition {
	message := fmt.Sprintf(
		"CamundaCluster %q already holds SecondaryStorageConfig %q. One CamundaCluster uses one "+
			"secondary storage contract, so this cluster stays suspended until that one releases it",
		objectPath(holder.Cluster), objectPath(holder.Contract),
	)
	if applyErr != nil {
		message += fmt.Sprintf(". The last apply of the suspended workloads failed: %s", applyErr)
	}

	return conditions.Ready(metav1.ConditionFalse, v1.ReasonStorageAlreadyAttached, message, cluster.Generation)
}
