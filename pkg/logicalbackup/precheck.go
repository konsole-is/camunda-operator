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

package logicalbackup

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// InProgress reports the name of a backup of the same cluster that has not
// reached a terminal phase, or the empty string when none runs. Each backup
// kind supplies it, because only the kinds know how to list themselves; the
// pre-checks own when it is consulted.
type InProgress func(ctx context.Context) (string, error)

// PreCheckRequest is what the pre-checks of one backup need to resolve.
type PreCheckRequest struct {
	// Reader reads the referenced resources. Pass the uncached API reader:
	// a pre-check that decides a backup may start must not read a stale
	// suspend flag or storage reference.
	Reader client.Reader
	// Ref is the clusterRef of the backup.
	Ref v1.ClusterRef
	// Namespace is the namespace of the backup, which is the namespace of
	// the referenced cluster: a ClusterRef never crosses namespaces.
	Namespace string
	// StorageType is the secondary storage type that the backup kind backs
	// up. A cluster on any other type is not backed up by this kind.
	StorageType v1.SecondaryStorageType
	// InProgress serializes the backups of one cluster.
	InProgress InProgress
}

// PreCheckResult is everything the pre-checks resolved, for the reconcile to
// use without reading it again.
type PreCheckResult struct {
	// Cluster is the referenced CamundaCluster.
	Cluster *v1.CamundaCluster
	// Storage is the secondary storage contract of the cluster.
	Storage *v1.SecondaryStorageConfig
	// Bucket is the backup bucket contract of the cluster.
	Bucket *v1.ObjectStorageConfig
}

// PreCheck resolves everything a backup needs and reports the first failure,
// in the documented order: the cluster exists, it has backup storage, it is
// not suspended, its secondary storage matches the kind, and no other backup
// of it is running. The order is the contract — it decides which reason a
// user sees when more than one check would fail.
//
// A failed check returns a *conditions.PreCheckFailure carrying the Ready
// reason: InvalidReference for a dangling or empty reference,
// ClusterSuspended while the workloads are scaled down, StorageTypeMismatch
// for the wrong kind, and BackupInProgress while another backup of the
// cluster runs. Waiting reports which of those resolve on their own. Any
// other error is a transient API failure and carries no reason.
func PreCheck(ctx context.Context, req PreCheckRequest) (*PreCheckResult, error) {
	namespace := req.Namespace

	var cluster v1.CamundaCluster
	if err := req.Reader.Get(
		ctx,
		types.NamespacedName{Name: req.Ref.Name, Namespace: namespace},
		&cluster,
	); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, InvalidReference("CamundaCluster %s/%s does not exist", namespace, req.Ref.Name)
		}
		return nil, fmt.Errorf("reading CamundaCluster %s/%s: %w", namespace, req.Ref.Name, err)
	}

	if cluster.Spec.BackupStorageRef == "" {
		return nil, InvalidReference(
			"CamundaCluster %s/%s has no spec.backupStorageRef, so a backup has nowhere to write",
			namespace, req.Ref.Name,
		)
	}

	if cluster.Suspended() {
		return nil, &conditions.PreCheckFailure{
			Reason: v1.ReasonClusterSuspended,
			Message: fmt.Sprintf(
				"CamundaCluster %s/%s is suspended, so its management API is unreachable",
				namespace, req.Ref.Name,
			),
		}
	}

	var storage v1.SecondaryStorageConfig
	// A Get with an empty name is an invalid request, not a NotFound, so an
	// unset reference would loop as a transient error instead of reporting
	// itself.
	if cluster.Spec.StorageRef == "" {
		return nil, InvalidReference(
			"CamundaCluster %s/%s has no spec.storageRef", namespace, req.Ref.Name,
		)
	}

	storageName := types.NamespacedName{Name: cluster.Spec.StorageRef, Namespace: namespace}
	if err := req.Reader.Get(ctx, storageName, &storage); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, InvalidReference("SecondaryStorageConfig %s does not exist", storageName)
		}
		return nil, fmt.Errorf("reading SecondaryStorageConfig %s: %w", storageName, err)
	}

	if storage.Spec.Type != req.StorageType {
		return nil, &conditions.PreCheckFailure{
			Reason: v1.ReasonStorageTypeMismatch,
			Message: fmt.Sprintf(
				"CamundaCluster %s/%s stores its data in %s, which this backup kind does not back up",
				namespace, req.Ref.Name, storage.Spec.Type,
			),
		}
	}

	var bucket v1.ObjectStorageConfig
	bucketName := types.NamespacedName{Name: cluster.Spec.BackupStorageRef, Namespace: namespace}
	if err := req.Reader.Get(ctx, bucketName, &bucket); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, InvalidReference("ObjectStorageConfig %s does not exist", bucketName)
		}
		return nil, fmt.Errorf("reading ObjectStorageConfig %s: %w", bucketName, err)
	}

	running, err := req.InProgress(ctx)
	if err != nil {
		return nil, fmt.Errorf("checking for a running backup of %s/%s: %w", namespace, req.Ref.Name, err)
	}
	if running != "" {
		return nil, &conditions.PreCheckFailure{
			Reason: v1.ReasonBackupInProgress,
			Message: fmt.Sprintf(
				"backup %q of CamundaCluster %s/%s has not finished; backups of one cluster run one at a time",
				running, namespace, req.Ref.Name,
			),
		}
	}

	return &PreCheckResult{Cluster: &cluster, Storage: &storage, Bucket: &bucket}, nil
}

// Waiting reports whether err is a pre-check failure that resolves without a
// change to the backup: the cluster is suspended, or another backup of it is
// still running. The caller requeues on those instead of treating them as a
// configuration error.
func Waiting(err error) bool {
	var failure *conditions.PreCheckFailure
	if !errors.As(err, &failure) {
		return false
	}

	return failure.Reason == v1.ReasonClusterSuspended || failure.Reason == v1.ReasonBackupInProgress
}

// InvalidReference builds the pre-check failure of a reference that does not
// resolve.
func InvalidReference(format string, args ...any) *conditions.PreCheckFailure {
	return &conditions.PreCheckFailure{
		Reason:  v1.ReasonInvalidReference,
		Message: fmt.Sprintf(format, args...),
	}
}
