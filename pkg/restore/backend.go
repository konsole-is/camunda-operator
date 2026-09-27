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

// BackendCheck is what a logical restore needs to decide whether it may write
// the secondary storage of its target.
type BackendCheck struct {
	// ClaimNamespace holds the storage claim Leases, see
	// camundacluster.StorageClaimSchema.
	ClaimNamespace string
	// Cluster is the target cluster, as the restore resolved it.
	Cluster *v1.CamundaCluster
	// Pinned is the backend that the restore pinned in status.backend.
	Pinned string
	// OwnPod reports whether a pod that carries the storage claim belongs to
	// the restore or to its target. A nil OwnPod takes the pods of the target
	// only.
	OwnPod func(podLabels map[string]string) bool
}

// ResolveBackend returns the backend that the secondary storage of cluster
// resolves to, as the key that the storage claim of the cluster uses, see
// camundacluster.StorageClaimKey. A restore pins it before it writes, and the
// handover gate of every other cluster waits for a running restore on that
// key. A chain that does not resolve is a failure the user corrects.
func ResolveBackend(
	ctx context.Context,
	reader client.Reader,
	cluster *v1.CamundaCluster,
) (string, *conditions.PreCheckFailure, error) {
	storage, failure, err := ResolveStorage(ctx, reader, cluster)
	if err != nil || failure != nil {
		return "", failure, err
	}

	chain := clustercomponents.Storage{Type: storage.Spec.Type, Namespace: storage.Namespace}
	switch storage.Spec.Type {
	case v1.SecondaryStorageTypeElasticsearch:
		chain.Elasticsearch = storage.Spec.Elasticsearch
	case v1.SecondaryStorageTypeRDBMS:
		chain.RDBMS, failure, err = resolveDatabaseAddress(ctx, reader, storage)
		if err != nil || failure != nil {
			return "", failure, err
		}
	}

	key, err := clustercomponents.StorageClaimKey(chain)
	if err != nil {
		return "", clustercomponents.StorageClaimKeyFailure(client.ObjectKeyFromObject(storage), err), nil
	}

	return key, nil, nil
}

// resolveDatabaseAddress follows the rdbms block of storage to the host, the
// port, and the database name, which is all the claim key reads.
func resolveDatabaseAddress(
	ctx context.Context,
	reader client.Reader,
	storage *v1.SecondaryStorageConfig,
) (*clustercomponents.RDBMSStorage, *conditions.PreCheckFailure, error) {
	if storage.Spec.RDBMS == nil {
		return nil, logicalbackup.InvalidReference(
			"SecondaryStorageConfig %s/%s has type rdbms and no rdbms block", storage.Namespace, storage.Name,
		), nil
	}

	var config v1.DatabaseConfig
	configKey := types.NamespacedName{Namespace: storage.Namespace, Name: storage.Spec.RDBMS.DatabaseConfigRef}
	if failure, err := get(ctx, reader, configKey, &config, "DatabaseConfig"); err != nil || failure != nil {
		return nil, failure, err
	}

	var server v1.DatabaseServerConfig
	serverKey := types.NamespacedName{Namespace: config.Namespace, Name: config.Spec.ServerRef}
	if failure, err := get(ctx, reader, serverKey, &server, "DatabaseServerConfig"); err != nil || failure != nil {
		return nil, failure, err
	}

	return &clustercomponents.RDBMSStorage{
		Host:     server.Spec.Host,
		Port:     server.Spec.Port,
		Database: config.Spec.DatabaseName,
	}, nil, nil
}

// get reads one referenced object of the storage chain. A missing object is a
// failure the user corrects.
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

// CheckBackend reports why a restore must not write the backend it pinned,
// or nil when it may. It may when three things hold: the storage chain of the
// target still resolves to that backend, the target holds the storage claim
// of it, and no pod of another cluster carries that claim. Each read goes to
// the API server.
//
// The restore pins the backend and leaves Pending before its first check, and
// the handover gate of a cluster that takes the claim reads the running
// restores after it wrote the Lease. So of a restore and a cluster that race
// for one backend, at least one sees the other.
func CheckBackend(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	check BackendCheck,
) (*conditions.PreCheckFailure, error) {
	key, failure, err := ResolveBackend(ctx, reader, check.Cluster)
	if err != nil || failure != nil {
		return failure, err
	}
	if key != check.Pinned {
		return logicalbackup.InvalidReference(
			"CamundaCluster %s/%s now resolves to the backend %q, and the restore started against %q",
			check.Cluster.Namespace, check.Cluster.Name, key, check.Pinned,
		), nil
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
	if !ours || holder.UID != check.Cluster.UID {
		return notHeld(check.Cluster, key, holder.NamespacedName, ours), nil
	}

	own := clustercomponents.PodsOfCluster(check.Cluster.UID)
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
				"Pods of another cluster, or workloads that start one, still write the backend %q: %s. "+
					"The restore writes it when they are gone",
				key, strings.Join(pods, ", "),
			),
		}, nil
	}

	return nil, nil
}

// notHeld is the failure of a target that does not hold its backend. A
// suspended cluster takes a free backend on its own pass, so a free one is a
// wait for that pass. One that another cluster holds ends when that cluster
// leaves the backend.
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
