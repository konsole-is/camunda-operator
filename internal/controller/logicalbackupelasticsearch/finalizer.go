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

package logicalbackupelasticsearch

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
)

// finalize deletes the stored artifacts of the backup, strictly by its own
// backup id, and releases the finalizer. When the cluster or its contracts
// are gone the artifacts are unreachable; the finalizer releases with an
// event instead of blocking the deletion forever. A cluster that merely has
// not published its binding yet keeps the deletion waiting: it still exists,
// so its artifacts remain deletable.
func (r *Reconciler) finalize(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(backup, logicalbackup.Finalizer) {
		return ctrl.Result{}, nil
	}

	// A backup that never allocated an id wrote nothing anywhere.
	if backup.Status.BackupID != 0 {
		released, err := r.deleteArtifacts(ctx, backup)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !released {
			return ctrl.Result{RequeueAfter: r.poll()}, nil
		}
	}

	controllerutil.RemoveFinalizer(backup, logicalbackup.Finalizer)
	if err := r.Update(ctx, backup); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("removing the finalizer: %w", err)
	}

	return ctrl.Result{}, nil
}

// deleteArtifacts removes the snapshots and the partition backup. It reports
// released=false when the cluster exists but is not addressable yet, so the
// caller requeues instead of leaking artifacts that are still deletable.
func (r *Reconciler) deleteArtifacts(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
) (released bool, err error) {
	var cluster v1.CamundaCluster
	key := types.NamespacedName{
		Namespace: backup.EffectiveClusterNamespace(),
		Name:      backup.Spec.ClusterRef.Name,
	}
	if err := r.APIReader.Get(ctx, key, &cluster); err != nil {
		if apierrors.IsNotFound(err) {
			r.releaseWithoutCleanup(backup, fmt.Sprintf("CamundaCluster %s is gone", key))
			return true, nil
		}
		return false, fmt.Errorf("reading CamundaCluster %s: %w", key, err)
	}

	var storage v1.SecondaryStorageConfig
	storageKey := types.NamespacedName{Namespace: key.Namespace, Name: cluster.Spec.StorageRef}
	if cluster.Spec.StorageRef == "" {
		r.releaseWithoutCleanup(backup, fmt.Sprintf("CamundaCluster %s no longer names a storage contract", key))
		return true, nil
	}
	if err := r.APIReader.Get(ctx, storageKey, &storage); err != nil {
		if apierrors.IsNotFound(err) {
			r.releaseWithoutCleanup(backup, fmt.Sprintf("SecondaryStorageConfig %s is gone", storageKey))
			return true, nil
		}
		return false, fmt.Errorf("reading SecondaryStorageConfig %s: %w", storageKey, err)
	}

	binding := cluster.Status.Management
	if binding == nil || binding.Endpoint == "" || binding.BackupRepository == "" {
		// The cluster exists, so the artifacts are still deletable once it
		// publishes its binding again (for example after a suspension).
		return false, nil
	}

	es, err := r.elasticsearchAdmin(ctx, &storage)
	if err != nil {
		return false, fmt.Errorf("building the Elasticsearch client: %w", err)
	}

	for _, name := range backup.Status.HistorySnapshots {
		if err := es.DeleteSnapshot(ctx, binding.BackupRepository, name); err != nil {
			return false, fmt.Errorf("deleting snapshot %q: %w", name, err)
		}
	}

	records := RecordsSnapshotName(backup.Status.BackupID)
	if err := es.DeleteSnapshot(ctx, binding.BackupRepository, records); err != nil {
		return false, fmt.Errorf("deleting snapshot %q: %w", records, err)
	}

	mgmt, err := r.managementClient(ctx, binding)
	if err != nil {
		return false, fmt.Errorf("building the management client: %w", err)
	}
	if err := mgmt.DeleteRuntimeBackup(ctx, backup.Status.BackupID); err != nil {
		return false, fmt.Errorf("deleting runtime backup %d: %w", backup.Status.BackupID, err)
	}

	return true, nil
}

// releaseWithoutCleanup records that the artifacts could not be reached and
// the finalizer releases anyway: a deleted cluster must not pin its backups
// forever.
func (r *Reconciler) releaseWithoutCleanup(backup *v1.LogicalBackupElasticsearch, reason string) {
	r.EventRecorder.Eventf(
		backup,
		nil,
		corev1.EventTypeWarning,
		eventReasonReleased,
		eventActionFinalize,
		"Releasing without deleting the stored artifacts: %s",
		reason,
	)
}
