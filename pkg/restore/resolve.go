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

package restore

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
)

// The reads of this file are what a logical restore resolves on every look,
// from admission to the last phase. Each one answers a value, or the
// *conditions.PreCheckFailure that the user must see, or a transient error
// that the caller retries. Pass the uncached API reader: a phase that deletes
// an index or a broker volume must not act on a stale suspend flag or a stale
// storage reference.
//
// A PointInTimeRestore reads its own cluster and its own storage. It pins the
// cluster UID lazily and ends terminally when the cluster was replaced, and
// its storage read also refuses a backend that is not relational.

// ResolveCluster reads the target cluster of a restore. A cluster that does
// not exist is a failure the user corrects. After admission the pinned UID
// must match too: a cluster that was deleted and created again under the same
// name is another cluster, and this restore is not its restore. Pass the empty
// UID while the restore has pinned none.
func ResolveCluster(
	ctx context.Context,
	reader client.Reader,
	key types.NamespacedName,
	pinned types.UID,
) (*v1.CamundaCluster, *conditions.PreCheckFailure, error) {
	var cluster v1.CamundaCluster
	if err := reader.Get(ctx, key, &cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference("CamundaCluster %s does not exist", key), nil
		}

		return nil, nil, fmt.Errorf("reading CamundaCluster %s: %w", key, err)
	}

	if pinned != "" && cluster.UID != pinned {
		return nil, logicalbackup.InvalidReference(
			"CamundaCluster %s has UID %s and the restore started against %s, so it is another cluster",
			key, cluster.UID, pinned,
		), nil
	}

	return &cluster, nil, nil
}

// ResolveStorage reads the secondary storage contract of the target cluster.
// A restore reaches the secondary storage of the target through it, with the
// credentials the target's own workloads use.
func ResolveStorage(
	ctx context.Context,
	reader client.Reader,
	cluster *v1.CamundaCluster,
) (*v1.SecondaryStorageConfig, *conditions.PreCheckFailure, error) {
	// A Get with an empty name is an invalid request, not a NotFound, so an
	// unset reference would loop as a transient error instead of reporting
	// itself.
	if cluster.Spec.StorageRef == "" {
		return nil, logicalbackup.InvalidReference(
			"CamundaCluster %s/%s has no spec.storageRef", cluster.Namespace, cluster.Name,
		), nil
	}

	key := types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Spec.StorageRef}

	var storage v1.SecondaryStorageConfig
	if err := reader.Get(ctx, key, &storage); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference(
				"SecondaryStorageConfig %s does not exist", key,
			), nil
		}

		return nil, nil, fmt.Errorf("reading SecondaryStorageConfig %s: %w", key, err)
	}

	return &storage, nil, nil
}

// ResolveTarget is ReadTarget with the pre-check failure separated from the
// transient error, which is the shape a phase reports in. Every phase after
// admission needs the target: the broker count, the partition count, the
// Camunda version, and the claim template all come from the live broker
// StatefulSet, because the management binding of a suspended cluster is unset.
func ResolveTarget(
	ctx context.Context,
	reader client.Reader,
	cluster *v1.CamundaCluster,
) (*Target, *conditions.PreCheckFailure, error) {
	target, err := ReadTarget(ctx, reader, cluster)
	if err != nil {
		var failure *conditions.PreCheckFailure
		if errors.As(err, &failure) {
			return nil, failure, nil
		}

		return nil, nil, err
	}

	return target, nil, nil
}
