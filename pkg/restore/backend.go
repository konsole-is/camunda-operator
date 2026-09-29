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
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	clustercomponents "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/leaseclaim"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
)

// BackendCheck is what a logical restore needs to decide whether it can write
// the secondary storage of its target.
type BackendCheck struct {
	// ClaimNamespace holds the storage claim Leases, see
	// camundacluster.StorageClaimSchema.
	ClaimNamespace string
	// Cluster is the target cluster, as the restore resolved it.
	Cluster *v1.CamundaCluster
	// Storage is the SecondaryStorageConfig that the restore writes through,
	// as the restore resolved it.
	Storage *v1.SecondaryStorageConfig
	// Pinned is the backend that the restore pinned in status.backend.
	Pinned string
	// OwnPod reports whether a pod that carries the storage claim belongs to
	// the restore itself. A nil OwnPod takes no pod as its own. A pod of the
	// target is not the restore's own.
	OwnPod func(podLabels map[string]string) bool
}

// BackendOf returns the claim key of the backend that storage describes. A
// chain that does not resolve is a failure the user corrects.
func BackendOf(
	ctx context.Context,
	reader client.Reader,
	storage *v1.SecondaryStorageConfig,
) (string, *conditions.PreCheckFailure, error) {
	if storage.Spec.Type != v1.SecondaryStorageTypeRDBMS {
		key, failure := claimKey(storage, nil)

		return key, failure, nil
	}
	if storage.Spec.RDBMS == nil {
		return "", logicalbackup.InvalidReference(
			"SecondaryStorageConfig %s/%s has type rdbms and no rdbms block", storage.Namespace, storage.Name,
		), nil
	}

	var config v1.DatabaseConfig
	configKey := types.NamespacedName{Namespace: storage.Namespace, Name: storage.Spec.RDBMS.DatabaseConfigRef}
	if failure, err := get(ctx, reader, configKey, &config, "DatabaseConfig"); err != nil || failure != nil {
		return "", failure, err
	}

	var server v1.DatabaseServerConfig
	serverKey := types.NamespacedName{Namespace: config.Namespace, Name: config.Spec.ServerRef}
	if failure, err := get(ctx, reader, serverKey, &server, "DatabaseServerConfig"); err != nil || failure != nil {
		return "", failure, err
	}

	key, failure := DatabaseBackend(storage, &config, &server)

	return key, failure, nil
}

// DatabaseBackend returns the claim key of the logical database that config
// and server describe, reached through storage.
func DatabaseBackend(
	storage *v1.SecondaryStorageConfig,
	config *v1.DatabaseConfig,
	server *v1.DatabaseServerConfig,
) (string, *conditions.PreCheckFailure) {
	return claimKey(storage, &clustercomponents.RDBMSStorage{
		Host:     server.Spec.Host,
		Port:     server.Spec.Port,
		Database: config.Spec.DatabaseName,
	})
}

func claimKey(
	storage *v1.SecondaryStorageConfig,
	database *clustercomponents.RDBMSStorage,
) (string, *conditions.PreCheckFailure) {
	key, err := clustercomponents.StorageClaimKey(clustercomponents.Storage{
		Type:          storage.Spec.Type,
		Namespace:     storage.Namespace,
		Elasticsearch: storage.Spec.Elasticsearch,
		RDBMS:         database,
	})
	if err != nil {
		return "", clustercomponents.StorageClaimKeyFailure(client.ObjectKeyFromObject(storage), err)
	}

	return key, nil
}

// get reports a missing object as a failure the user corrects.
func get(
	ctx context.Context,
	reader client.Reader,
	key types.NamespacedName,
	obj client.Object,
	kind string,
) (*conditions.PreCheckFailure, error) {
	if err := reader.Get(ctx, key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return logicalbackup.InvalidReference("%s %s does not exist", kind, key), nil
		}

		return nil, fmt.Errorf("reading %s %s: %w", kind, key, err)
	}

	return nil, nil
}

// CheckBackend reports why a restore must not write the backend it pinned.
// It returns nil when check.Storage still resolves to that backend, the target
// holds its storage claim, and nothing but the restore writes it. reader must
// read the API server directly.
func CheckBackend(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	check BackendCheck,
) (*conditions.PreCheckFailure, error) {
	key, failure, err := BackendOf(ctx, reader, check.Storage)
	if err != nil || failure != nil {
		return failure, err
	}
	if failure := MovedBackend(check.Cluster, key, check.Pinned); failure != nil {
		return failure, nil
	}

	schema := clustercomponents.StorageClaimSchema()
	lease, found, err := schema.NewClaim(c, reader, check.ClaimNamespace).Read(ctx, key)
	if err != nil {
		return nil, err
	}
	var holder leaseclaim.Holder
	ours := false
	if found {
		holder, ours = schema.HolderOf(lease)
	}
	// The target meets the same Lease and reports it the same way. It cannot
	// take the backend while that Lease exists, so waiting for it never ends.
	if found && !ours {
		return logicalbackup.InvalidReference(
			"Lease %s/%s claims the backend %q and names no CamundaCluster. Delete it if nothing uses it",
			lease.Namespace, lease.Name, key,
		), nil
	}
	if !ours || holder.UID != check.Cluster.UID {
		return notHeld(check.Cluster, key, holder.NamespacedName, ours), nil
	}

	own := func(map[string]string) bool { return false }
	if check.OwnPod != nil {
		own = check.OwnPod
	}
	pods, err := clustercomponents.OtherPodsOnClaim(ctx, reader, schema.LeaseName(key), own)
	if err != nil {
		return nil, err
	}
	if len(pods) > 0 {
		return &conditions.PreCheckFailure{
			Reason: v1.ReasonWaitingForHandover,
			Message: fmt.Sprintf(
				"Pods, or workloads that start one, still write the backend %q: %s. "+
					"The restore writes it when they are gone",
				key, strings.Join(pods, ", "),
			),
		}, nil
	}

	return nil, nil
}

// MovedBackend returns a failure when key, the backend that cluster resolves
// to now, is not pinned. It returns nil when they match.
func MovedBackend(cluster *v1.CamundaCluster, key, pinned string) *conditions.PreCheckFailure {
	if key == pinned {
		return nil
	}

	return logicalbackup.InvalidReference(
		"CamundaCluster %s/%s now resolves to the backend %q, and the restore started against %q",
		cluster.Namespace, cluster.Name, key, pinned,
	)
}

// A suspended cluster takes a free backend on its own pass, so a free backend
// is a wait and not a failure.
func notHeld(
	cluster *v1.CamundaCluster,
	key string,
	holder types.NamespacedName,
	held bool,
) *conditions.PreCheckFailure {
	if !held {
		return &conditions.PreCheckFailure{
			Reason: v1.ReasonWaitingForHandover,
			Message: fmt.Sprintf(
				"CamundaCluster %s/%s does not hold the backend %q yet. The restore writes it "+
					"once the cluster holds it",
				cluster.Namespace, cluster.Name, key,
			),
		}
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonStorageAlreadyAttached,
		Message: fmt.Sprintf(
			"CamundaCluster %s holds the backend %q, and CamundaCluster %s/%s does not. "+
				"The restore writes nothing into a backend that another cluster holds",
			holder, key, cluster.Namespace, cluster.Name,
		),
	}
}
