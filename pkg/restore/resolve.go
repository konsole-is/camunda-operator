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

// Each read of this file returns a value, or a *conditions.PreCheckFailure
// that the user must see, or a transient error that the caller retries. Pass
// the uncached API reader: a phase that deletes an index or a broker volume
// must not act on a stale suspend flag or a stale storage reference.

// ResolveCluster reads the target cluster of a restore. A cluster that does
// not exist, or whose UID differs from pinned, is a failure: a cluster created
// again under the same name is another cluster. Pass the empty pinned UID
// while the restore has pinned none.
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

// ResolveStorage reads the SecondaryStorageConfig of the target cluster. An
// unset spec.storageRef, or a config that does not exist, is a failure.
func ResolveStorage(
	ctx context.Context,
	reader client.Reader,
	cluster *v1.CamundaCluster,
) (*v1.SecondaryStorageConfig, *conditions.PreCheckFailure, error) {
	// A Get with an empty name is an invalid request, not a NotFound, so an
	// unset reference loops as a transient error without this check.
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

// ResolveTarget reads the facts a restore needs off the live broker
// StatefulSet of the cluster. It works on a suspended cluster, whose
// management binding is unset. A StatefulSet that cannot answer one of the
// facts is a failure. A transport error is one the caller retries.
func ResolveTarget(
	ctx context.Context,
	reader client.Reader,
	cluster *v1.CamundaCluster,
) (*Target, *conditions.PreCheckFailure, error) {
	target, err := readTarget(ctx, reader, cluster)
	if err != nil {
		var failure *conditions.PreCheckFailure
		if errors.As(err, &failure) {
			return nil, failure, nil
		}

		return nil, nil, err
	}

	return target, nil, nil
}
