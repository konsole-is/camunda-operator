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
// stale and is taken over. Otherwise the holder lands on in.Storage.Holder
// and the controller renders this cluster suspended. It needs res.storage
// from resolveStorage. A conflict on the claim is a transient error.
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
