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
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
)

// finalize deletes the stored artifacts of the backup, strictly by its own
// backup id, and releases the finalizer. A non-terminal backup resumes
// exporting first: deleting the resource must never leave the cluster
// soft-paused with nothing left to resume it. When the cluster or its
// contracts are gone — or a client can no longer be built by construction —
// the artifacts are unreachable and the finalizer releases with an event
// instead of blocking the deletion forever. A cluster that merely has not
// published its binding yet keeps the deletion waiting: it still exists, so
// its artifacts remain deletable.
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

// deleteArtifacts resumes exporting when the backup left it paused, then
// removes the snapshots and the partition backup. It reports released=false
// when the cluster exists but is not addressable yet, or holds work that
// cannot be deleted yet, so the caller requeues instead of leaking artifacts
// that are still deletable — or leaving a cluster paused.
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

	binding := cluster.Status.Management
	if binding == nil || binding.Endpoint == "" {
		// The cluster exists, so the artifacts are still deletable once it
		// publishes its binding again (for example after a suspension).
		return false, nil
	}

	mgmt, failure, err := logicalbackup.ManagementClient(ctx, r.APIReader, &cluster)
	if err != nil {
		return false, fmt.Errorf("building the management client: %w", err)
	}
	if failure != nil {
		// A binding that is broken by construction — credentials gone, a
		// version this operator no longer drives — stays broken; holding the
		// deletion on it would pin the namespace forever.
		r.releaseWithoutCleanup(backup, failure.Message)
		return true, nil
	}

	// A non-terminal backup may have left exporting paused; the cluster must
	// run again before its artifacts go. Resume is idempotent, so a backup
	// that never paused resumes to a no-op.
	if !backup.Terminal() && backup.Status.Step != "" {
		if err := mgmt.ResumeExporting(ctx); err != nil {
			r.holdDeletion(backup, fmt.Sprintf("exporting is not resumed yet: %v", err))
			return false, nil
		}
	}

	storage, err := r.resolveStorage(ctx, &cluster)
	if err != nil {
		if apierrors.IsNotFound(errors.Unwrap(err)) || cluster.Spec.StorageRef == "" {
			r.releaseWithoutCleanup(backup, err.Error())
			return true, nil
		}
		return false, err
	}

	es, err := r.elasticsearchAdmin(ctx, storage)
	if err != nil {
		var failure *conditions.PreCheckFailure
		if errors.As(err, &failure) {
			r.releaseWithoutCleanup(backup, failure.Message)
			return true, nil
		}
		return false, fmt.Errorf("building the Elasticsearch client: %w", err)
	}

	// The status may miss snapshot names when the resource died between the
	// start of the history backup and the poll that records them; the
	// cluster's own report closes that window.
	if status, err := mgmt.HistoryBackupStatus(ctx, backup.Status.BackupID); err == nil {
		recordHistorySnapshots(backup, status)
	} else if errors.Is(err, camundaadmin.ErrUnreachable) {
		return false, err
	}

	repository := backup.Status.Repository
	if repository == "" {
		repository = binding.BackupRepository
	}

	for _, name := range backup.Status.HistorySnapshots {
		if err := es.DeleteSnapshot(ctx, repository, name); err != nil {
			return false, fmt.Errorf("deleting snapshot %q: %w", name, err)
		}
	}

	records := RecordsSnapshotName(backup.Status.BackupID)
	if err := es.DeleteSnapshot(ctx, repository, records); err != nil {
		return false, fmt.Errorf("deleting snapshot %q: %w", records, err)
	}

	if err := mgmt.DeleteRuntimeBackup(ctx, backup.Status.BackupID); err != nil {
		// A runtime backup that is still in progress cannot be deleted yet;
		// wait for it instead of hammering the endpoint on error backoff.
		if errors.Is(err, camundaadmin.ErrRejected) {
			r.holdDeletion(backup, fmt.Sprintf(
				"runtime backup %d cannot be deleted yet: %v", backup.Status.BackupID, err,
			))
			return false, nil
		}
		return false, fmt.Errorf("deleting runtime backup %d: %w", backup.Status.BackupID, err)
	}

	return true, nil
}

// holdDeletion records why the deletion waits, so the user sees the reason on
// the resource instead of a silently terminating object.
func (r *Reconciler) holdDeletion(backup *v1.LogicalBackupElasticsearch, reason string) {
	r.EventRecorder.Eventf(
		backup,
		nil,
		corev1.EventTypeWarning,
		eventReasonDeleteHeld,
		eventActionFinalize,
		"Deletion waits: %s",
		reason,
	)
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
