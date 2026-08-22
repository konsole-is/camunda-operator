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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/secondarystorageconfig"
)

// resolveStorageHolder finds the CamundaCluster that holds the backend of
// the storage binding, when it is not this cluster, and records it on
// in.Storage.Holder. The oldest cluster holds the backend, with the name
// breaking a tie. It needs res.storage from resolveStorage. An own backend
// that does not resolve fails the pre-check with InvalidReference. One
// CamundaCluster uses one backend, see components.Storage.Holder.
func (res *resolver) resolveStorageHolder(ctx context.Context, in *components.Input) error {
	backend, failure, err := secondarystorageconfig.ResolveBackend(ctx, res.reader, res.storage)
	if err != nil {
		return fmt.Errorf(
			"resolving the backend of SecondaryStorageConfig %s: %w",
			client.ObjectKeyFromObject(res.storage), err,
		)
	}
	if failure != nil {
		return failure
	}

	holder, err := res.olderSibling(ctx, func(ctx context.Context, other *v1.CamundaCluster) (bool, error) {
		return res.usesBackend(ctx, other, backend)
	})
	if err != nil {
		return fmt.Errorf("finding the holder of %s: %w", backend, err)
	}
	if holder == nil {
		return nil
	}

	in.Storage.Holder = &components.StorageHolder{
		Cluster: client.ObjectKeyFromObject(holder),
		Backend: backend.String(),
	}

	return nil
}

// usesBackend reports whether other resolves to backend. A cluster whose
// contract or chain does not resolve uses nothing yet. When its chain
// resolves it holds the backend, because it is older, and this cluster yields
// on its next reconcile.
func (res *resolver) usesBackend(
	ctx context.Context,
	other *v1.CamundaCluster,
	backend secondarystorageconfig.Backend,
) (bool, error) {
	var binding v1.SecondaryStorageConfig
	key := client.ObjectKey{Namespace: other.Namespace, Name: other.Spec.StorageRef}
	if err := res.reader.Get(ctx, key, &binding); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, fmt.Errorf("reading SecondaryStorageConfig %s: %w", key, err)
	}

	theirs, failure, err := secondarystorageconfig.ResolveBackend(ctx, res.reader, &binding)
	if err != nil {
		return false, fmt.Errorf("resolving the backend of SecondaryStorageConfig %s: %w", key, err)
	}

	return failure == nil && theirs == backend, nil
}

// storageHeld builds the Ready condition of a cluster whose backend another
// cluster holds.
func storageHeld(cluster *v1.CamundaCluster, holder *components.StorageHolder) metav1.Condition {
	return conditions.Ready(
		metav1.ConditionFalse,
		v1.ReasonStorageAlreadyAttached,
		fmt.Sprintf(
			"CamundaCluster %q already uses %s. One CamundaCluster uses one backend, "+
				"so this cluster stays suspended until that one releases it",
			objectPath(holder.Cluster), holder.Backend,
		),
		cluster.Generation,
	)
}
