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
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin"
	"github.com/konsole-is/camunda-operator/pkg/esadmin"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/management"
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

	// The finalizer goes first, the claim after. Once the removal is
	// durable the object is gone for good. Its Get answers NotFound, and no
	// retry of this finalizer can ever resume exporting again. Releasing
	// first would open exactly that window. A crash between the release
	// and the removal would let a sibling claim and pause. The retried
	// finalizer would then resume exporting inside the sibling's run.
	controllerutil.RemoveFinalizer(backup, logicalbackup.Finalizer)
	if err := r.Update(ctx, backup); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("removing the finalizer: %w", err)
	}

	// The release is best effort. The object no longer exists, so an error
	// must not fail this reconcile. It is logged, and the stale-holder
	// takeover reclaims the Lease of a gone holder. For a backup that ended
	// as ResumeFailed this is the one release. deleteArtifacts resumed
	// exporting first, or held the deletion, so a sibling that starts after
	// this release never meets a paused cluster.
	if err := r.releaseClaim(ctx, backup); err != nil {
		log.FromContext(ctx).Error(
			err,
			"Could not release the claim Lease of a deleted backup; a later claimant takes it over",
			"backup", backup.Name,
			"cluster", backup.Spec.ClusterRef.Name,
		)
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
	gone := false
	if err := r.APIReader.Get(ctx, key, &cluster); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("reading CamundaCluster %s: %w", key, err)
		}
		gone = true
	}

	// A gone cluster and a replaced one are the same case: the cluster that
	// this backup paused and backed up no longer exists. No management call
	// is made — a resume or a runtime-backup delete would drive the
	// replacement, whose exporting this backup never paused. The
	// Elasticsearch cluster can outlive the CamundaCluster, so the
	// snapshots that this backup owns are swept through the pinned storage,
	// best effort, before the release.
	if gone || clusterReplaced(backup, &cluster) {
		reason := fmt.Sprintf("CamundaCluster %s is gone", key)
		if !gone {
			reason = fmt.Sprintf(
				"CamundaCluster %s was replaced (UID %s, the backup started against %s)",
				key, cluster.UID, backup.Status.ClusterUID,
			)
		}
		if err := r.sweepPinnedSnapshots(ctx, backup); err != nil {
			reason = fmt.Sprintf("%s; the snapshot sweep did not finish: %v", reason, err)
		}
		r.releaseWithoutCleanup(backup, reason)
		return true, nil
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

	mgmt, failure, err := management.NewClient(ctx, r.APIReader, &cluster)
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

	// A backup can have left exporting paused: one that is still running,
	// and one that gave up on resume with ResumeFailed. The cluster must run
	// again before the artifacts go, or the deletion removes the only thing
	// that resumes it. Resume is idempotent, so a backup that never paused
	// resumes to a no-op.
	if mayHoldExportingPaused(backup) {
		if err := mgmt.ResumeExporting(ctx); err != nil {
			r.holdDeletion(backup, fmt.Sprintf("exporting is not resumed yet: %v", err))
			return false, nil
		}
	}

	es, released, err := r.finalizerElasticsearch(ctx, backup, &cluster)
	if es == nil {
		return released, err
	}

	repository := backup.Status.Repository
	if repository == "" {
		repository = binding.BackupRepository
	}

	// Only a history backup whose request this backup saw accepted is its
	// own. Without the acceptance, a history backup under this ID and its
	// snapshots belong to another actor, or they are a lost-response orphan
	// without ownership evidence. They stay.
	if backup.Status.HistoryAcceptedTime != nil {
		// The status can miss snapshot names when the resource died between
		// the accepted start and the poll that records them. The cluster's
		// own report closes that window. Every failure of that query holds
		// the deletion: a release on an incomplete list leaks the snapshots
		// that the list does not name. A backup that does not exist is a
		// successful answer, so nothing legitimate is lost.
		status, err := mgmt.HistoryBackupStatus(ctx, backup.Status.BackupID)
		if err != nil {
			return false, fmt.Errorf("querying the history backup %d: %w", backup.Status.BackupID, err)
		}
		recordHistorySnapshots(backup, status)
		if status.State == camundaadmin.StateInProgress {
			// The web applications keep creating snapshots until the backup
			// is terminal, and the management API of Camunda 8.9 offers no
			// way to cancel it. A deletion that ran now would remove the
			// snapshots that exist and leave the ones still to come. The
			// hold has no bound, like the hold on an in-progress runtime
			// backup.
			r.holdDeletion(backup, fmt.Sprintf(
				"history backup %d is still in progress and cannot be cancelled", backup.Status.BackupID,
			))
			return false, nil
		}

		for _, name := range backup.Status.HistorySnapshots {
			if err := es.DeleteSnapshot(ctx, repository, name); err != nil {
				return false, fmt.Errorf("deleting snapshot %q: %w", name, err)
			}
		}
	}

	// The records snapshot is deleted only when its metadata carries the
	// UID of this backup. A snapshot under the deterministic name without
	// it was created by someone else, and it stays.
	records := RecordsSnapshotName(backup.Status.BackupID)
	snapshot, err := es.SnapshotStatus(ctx, repository, records)
	if err != nil {
		return false, fmt.Errorf("querying snapshot %q: %w", records, err)
	}
	if snapshot.State != esadmin.SnapshotMissing && snapshotOwnedBy(snapshot, backup) {
		if err := es.DeleteSnapshot(ctx, repository, records); err != nil {
			return false, fmt.Errorf("deleting snapshot %q: %w", records, err)
		}
	}

	// Only a runtime backup whose request this backup saw accepted is its
	// own. A backup that never recorded the acceptance never owned one. A
	// runtime backup under its ID, if any, belongs to another actor or is a
	// lost-response orphan without ownership evidence. It stays.
	if backup.Status.RuntimeAcceptedTime == nil {
		return true, nil
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
	storage, err := r.resolvePinnedStorage(ctx, backup, cluster)
	if err != nil {
		if storageMissing(err) {
			r.releaseWithoutCleanup(backup, err.Error())
			return nil, true, nil
		}
		return nil, false, err
	}
	if err := pinnedStorageMatches(backup, storage); err != nil {
		// The artifacts live on the pinned cluster. To delete against a
		// different one hits the wrong data. Hold until the contract points
		// back, or the operator removes the finalizer by hand.
		return nil, false, fmt.Errorf("the pinned storage no longer matches: %w", err)
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

// resolvePinnedStorage resolves the storage contract that the backup pinned
// when it started. A backup that never started has no pin, so it resolves the
// storage of the cluster.
func (r *Reconciler) resolvePinnedStorage(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
	cluster *v1.CamundaCluster,
) (*v1.SecondaryStorageConfig, error) {
	if backup.Status.Storage == nil {
		return r.resolveStorage(ctx, cluster)
	}

	storage := &v1.SecondaryStorageConfig{}
	key := types.NamespacedName{Namespace: backup.Namespace, Name: backup.Status.Storage.SecondaryStorageConfig}
	if err := r.APIReader.Get(ctx, key, storage); err != nil {
		return nil, fmt.Errorf("resolving the pinned SecondaryStorageConfig %q: %w", key.Name, err)
	}

	return storage, nil
}

// mayHoldExportingPaused reports whether the backup can have left exporting
// paused: it started and is not terminal, or it ended as ResumeFailed.
func mayHoldExportingPaused(backup *v1.LogicalBackupElasticsearch) bool {
	if backup.Status.Step == "" {
		return false
	}
	return !backup.Terminal() || backup.Status.TerminalReason == v1.ReasonResumeFailed
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
// sweepPinnedSnapshots deletes the snapshots that this backup owns from the
// pinned storage. Those are the recorded history snapshots when their
// backup was accepted, and the records snapshot when it carries the UID. It
// is best effort for a cluster that no longer exists. An unreachable or
// repointed storage abandons the sweep, and the caller releases anyway.
func (r *Reconciler) sweepPinnedSnapshots(ctx context.Context, backup *v1.LogicalBackupElasticsearch) error {
	pinned := backup.Status.Storage
	if pinned == nil {
		return nil
	}

	storage := &v1.SecondaryStorageConfig{}
	key := types.NamespacedName{Namespace: backup.Namespace, Name: pinned.SecondaryStorageConfig}
	if err := r.APIReader.Get(ctx, key, storage); err != nil {
		return fmt.Errorf("resolving the pinned SecondaryStorageConfig %q: %w", key.Name, err)
	}
	if err := pinnedStorageMatches(backup, storage); err != nil {
		return err
	}
	es, failure, err := secondarystorageconfig.ElasticsearchAdmin(ctx, r.APIReader, storage)
	if err != nil {
		return fmt.Errorf("building the Elasticsearch client: %w", err)
	}
	if failure != nil {
		return errors.New(failure.Message)
	}

	repository := backup.Status.Repository
	if backup.Status.HistoryAcceptedTime != nil {
		for _, name := range backup.Status.HistorySnapshots {
			if err := es.DeleteSnapshot(ctx, repository, name); err != nil {
				return fmt.Errorf("deleting snapshot %q: %w", name, err)
			}
		}
	}

	records := RecordsSnapshotName(backup.Status.BackupID)
	snapshot, err := es.SnapshotStatus(ctx, repository, records)
	if err != nil {
		return fmt.Errorf("querying snapshot %q: %w", records, err)
	}
	if snapshot.State != esadmin.SnapshotMissing && snapshotOwnedBy(snapshot, backup) {
		if err := es.DeleteSnapshot(ctx, repository, records); err != nil {
			return fmt.Errorf("deleting snapshot %q: %w", records, err)
		}
	}

	return nil
}

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
