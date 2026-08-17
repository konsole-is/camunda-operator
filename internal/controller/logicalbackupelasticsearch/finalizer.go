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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin"
	"github.com/konsole-is/camunda-operator/pkg/esadmin"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/secondarystorageconfig"
)

// finalize deletes the stored artifacts of the backup and releases the
// finalizer. It deletes strictly by the backup's own ID. A non-terminal backup
// resumes exporting first. A deleted resource must never leave the cluster
// soft-paused with nothing left to resume it. When the artifacts are not
// reachable anymore, the finalizer releases with an event instead of a
// deletion that blocks forever. That is the case when the cluster or its
// contracts are gone, or when a client can no longer be built by
// construction. A cluster that only has not published its binding yet keeps
// the deletion waiting. It still exists, so its artifacts remain deletable.
func (r *Reconciler) finalize(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(backup, logicalbackup.Finalizer) {
		return ctrl.Result{}, nil
	}

	// A backup that never allocated an ID wrote nothing anywhere.
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

// deleteArtifacts resumes exporting when the backup left it paused. Then it
// removes the snapshots and the partition backup. It reports released=false
// when the deletion must wait: the cluster exists but is not addressable yet,
// or it holds work that cannot be deleted yet. The caller then requeues
// instead of leaking artifacts that are still deletable, or leaving a cluster
// paused.
func (r *Reconciler) deleteArtifacts(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
) (released bool, err error) {
	var cluster v1.CamundaCluster
	key := clusterKey(backup)
	if err := r.APIReader.Get(ctx, key, &cluster); err != nil {
		if apierrors.IsNotFound(err) {
			r.releaseWithoutCleanup(backup, fmt.Sprintf("CamundaCluster %s is gone", key))
			return true, nil
		}
		return false, fmt.Errorf("reading CamundaCluster %s: %w", key, err)
	}

	binding := cluster.Status.Management
	if binding == nil || binding.Endpoint == "" {
		// The cluster exists, so the artifacts are still deletable when it
		// publishes its binding again, for example after a suspension.
		r.holdDeletion(backup, fmt.Sprintf(
			"CamundaCluster %s publishes no management binding (suspended?)", key,
		))
		return false, nil
	}

	mgmt, failure, err := logicalbackup.ManagementClient(ctx, r.APIReader, &cluster)
	if err != nil {
		return false, fmt.Errorf("building the management client: %w", err)
	}
	if failure != nil {
		// A binding that is broken by construction stays broken: the
		// credentials are gone, or the version is one this operator no
		// longer drives. A deletion that holds on it pins the namespace
		// forever.
		r.releaseWithoutCleanup(backup, failure.Message)
		return true, nil
	}

	// A non-terminal backup can have left exporting paused. The cluster must
	// run again before its artifacts go. Resume is idempotent, so a backup
	// that never paused resumes to a no-op.
	if !backup.Terminal() && backup.Status.Step != "" {
		if err := mgmt.ResumeExporting(ctx); err != nil {
			r.holdDeletion(backup, fmt.Sprintf("exporting is not resumed yet: %v", err))
			return false, nil
		}
	}

	es, released, err := r.finalizerElasticsearch(ctx, backup, &cluster)
	if es == nil {
		return released, err
	}

	// The status can miss snapshot names when the resource died between the
	// start of the history backup and the poll that records them. The
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
		// Any rejected delete holds the deletion. A backup that is still in
		// progress is the documented case. Every rejection means that the
		// cluster refuses for a reason worth waiting out, rather than
		// hammering the endpoint on error backoff.
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

// finalizerElasticsearch builds the Elasticsearch client for the finalizer. A
// storage contract or a Secret that is gone releases the finalizer with an
// event, because the artifacts are not reachable by construction. Every
// failure to build the client either releases or returns an error. A nil
// client with released=false and no error does not occur.
func (r *Reconciler) finalizerElasticsearch(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
	cluster *v1.CamundaCluster,
) (es *esadmin.Client, released bool, err error) {
	storage, err := r.resolveStorage(ctx, cluster)
	if err != nil {
		if apierrors.IsNotFound(errors.Unwrap(err)) || cluster.Spec.StorageRef == "" {
			r.releaseWithoutCleanup(backup, err.Error())
			return nil, true, nil
		}
		return nil, false, err
	}

	es, failure, err := secondarystorageconfig.ElasticsearchAdmin(ctx, r.APIReader, storage)
	if err != nil {
		return nil, false, fmt.Errorf("building the Elasticsearch client: %w", err)
	}
	if failure != nil {
		r.releaseWithoutCleanup(backup, failure.Message)
		return nil, true, nil
	}

	return es, false, nil
}

// holdDeletion records why the deletion waits. The user then sees the reason
// on the resource instead of an object that terminates in silence.
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

// releaseWithoutCleanup records that the artifacts are not reachable. The
// finalizer releases anyway. A deleted cluster must not pin its backups
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
