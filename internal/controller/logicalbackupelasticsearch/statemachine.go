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
	"slices"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/esadmin"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/management"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/secondarystorageconfig"
)

// snapshotOwnerUIDKey is the metadata key under which every snapshot that
// this controller creates carries the UID of its backup resource.
const snapshotOwnerUIDKey = "camunda-operator/backup-uid"

// runStep executes the step that status.step records. It advances at most one
// step, so every transition is persisted before the next side effect. Every
// step queries the current state before it acts. A crash re-enters without a
// repeated call. Each step builds only the clients that it uses. A dependency
// that broke mid-run fails the step through resume. The machine owns every
// exit.
func (r *Reconciler) runStep(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
	cluster *v1.CamundaCluster,
) (ctrl.Result, error) {
	switch backup.Status.Step {
	case v1.StepPauseExporting:
		return r.pauseExporting(ctx, backup, cluster)
	case v1.StepBackupHistory:
		return r.backupHistory(ctx, backup, cluster)
	case v1.StepSnapshotRecords:
		return r.snapshotRecords(ctx, backup, cluster)
	case v1.StepBackupRuntime:
		return r.backupRuntime(ctx, backup, cluster)
	case v1.StepResumeExporting:
		return r.resumeExporting(ctx, backup, cluster)
	default:
		return ctrl.Result{}, fmt.Errorf("unknown step %q", backup.Status.Step)
	}
}

// pauseExporting soft-pauses exporting. Records keep flowing, log compaction
// stops, and the backup is hot. The call is idempotent on the cluster side,
// so re-entry after a crash is safe.
func (r *Reconciler) pauseExporting(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
	cluster *v1.CamundaCluster,
) (ctrl.Result, error) {
	mgmt, result, err := r.management(ctx, backup, cluster, "PauseExporting", nil)
	if mgmt == nil {
		return result, err
	}

	if err := mgmt.PauseExporting(ctx, true); err != nil {
		if errors.Is(err, camundaadmin.ErrUnreachable) {
			// A lost answer can be a partial pause, so the retry is bounded
			// here too.
			return r.stageUnreachable(backup, "PauseExporting", nil, err)
		}

		// A rejection can be a partial pause: some partitions paused, some
		// did not. Resume reverts it either way.
		r.failStep(backup, "PauseExporting", nil, err)
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}
	answered(backup)

	backup.Status.Step = v1.StepBackupHistory
	r.stageProgress(backup, "Exporting is soft-paused; backing up the web-application indices")

	return ctrl.Result{RequeueAfter: r.poll()}, nil
}

// backupHistory drives the backup of the web-application indices. It starts
// the backup when the cluster holds none under the ID, then it polls the
// backup to completion. The snapshot names are recorded as soon as the
// cluster names them. The start answer names the scheduled snapshots, and
// every status report names them again. The finalizer and a restore can
// then locate the snapshots after the cluster is gone.
func (r *Reconciler) backupHistory(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
	cluster *v1.CamundaCluster,
) (ctrl.Result, error) {
	mgmt, result, err := r.management(ctx, backup, cluster, "BackupHistory", &backup.Status.History)
	if mgmt == nil {
		return result, err
	}

	// The web applications write into the repository that the cluster is
	// configured with now. A destination that moved since the start puts
	// the history snapshots outside the pinned set. The finalizer then
	// deletes their names from the pinned repository and leaks them. The
	// check runs before the status of an existing backup is trusted and
	// before a start.
	if storage, result, err := r.destination(
		ctx,
		backup,
		cluster,
		"BackupHistory",
		&backup.Status.History,
	); storage == nil {
		return result, err
	}

	status, err := mgmt.HistoryBackupStatus(ctx, backup.Status.BackupID)
	if err != nil {
		if errors.Is(err, camundaadmin.ErrUnreachable) {
			return r.stageUnreachable(backup, "BackupHistory", &backup.Status.History, err)
		}

		r.failStep(backup, "BackupHistory", &backup.Status.History, err)
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	if status.State == camundaadmin.StateDoesNotExist {
		return r.requestHistoryBackup(ctx, mgmt, backup)
	}
	answered(backup)

	// A history backup under this ID exists. Only an accepted request of
	// this backup, a 200 that this controller observed, proves that it is
	// ours. Without that evidence the backup is not adopted. Its snapshot
	// names are not recorded, and the finalizer will not delete them.
	if backup.Status.HistoryAcceptedTime == nil {
		r.failStep(backup, "BackupHistory", &backup.Status.History, unownedHistoryBackup(backup))
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	recordHistorySnapshots(backup, status)

	switch status.State {
	case camundaadmin.StateInProgress:
		backup.Status.History = v1.BackupPart{State: v1.BackupPartInProgress}
		r.stageProgress(backup, "The backup of the web-application indices is in progress")

	case camundaadmin.StateCompleted:
		backup.Status.History = v1.BackupPart{State: v1.BackupPartCompleted}
		backup.Status.Step = v1.StepSnapshotRecords
		r.stageProgress(backup, "Snapshotting the exported Zeebe record indices")

	default:
		r.failStep(backup, "BackupHistory", &backup.Status.History, errors.New(failureReason(status)))
	}

	return ctrl.Result{RequeueAfter: r.poll()}, nil
}

// requestHistoryBackup drives an absent history backup to a request, the
// same way requestRuntimeBackup drives the runtime one. The intent is
// written one reconcile before the request. Only an observed 200 records
// the acceptance, stamped after the call returned. A conflict is never an
// acceptance: the cluster gives no token that tells this backup's
// lost-response request from another actor's backup.
func (r *Reconciler) requestHistoryBackup(
	ctx context.Context,
	mgmt *camundaadmin.Client,
	backup *v1.LogicalBackupElasticsearch,
) (ctrl.Result, error) {
	part := &backup.Status.History
	now := metav1.Now()
	// The status query that led here answered. On the branches that make
	// no further call, every call of this reconcile answered, and the
	// unreachable clock clears. After the start call, only an answer of
	// that call clears it. A stale clock from an old outage must not fail
	// the step on the first isolated unreachable answer after it.
	if backup.Status.HistoryRequestedTime == nil {
		backup.Status.HistoryRequestedTime = &now
		answered(backup)
		r.stageProgress(backup, "The backup of the web-application indices is requested")
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	if accepted := backup.Status.HistoryAcceptedTime; accepted != nil {
		answered(backup)
		if now.Sub(accepted.Time) < r.runtimeRegistrationGrace() {
			r.stageProgress(backup, "The backup of the web-application indices is registering")
			return ctrl.Result{RequeueAfter: r.poll()}, nil
		}
		r.failStep(backup, "BackupHistory", part, fmt.Errorf(
			"the cluster holds no history backup %d within %s of the accepted request",
			backup.Status.BackupID, r.runtimeRegistrationGrace(),
		))
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	scheduled, err := mgmt.StartHistoryBackup(ctx, backup.Status.BackupID)
	switch {
	case err == nil:
		// The names are recorded in the reconcile that saw the acceptance.
		// The status flush at its end makes the acceptance and the names
		// durable together. If the cluster is gone before the first poll,
		// the finalizer then still knows which snapshots to sweep.
		accepted := metav1.Now()
		backup.Status.HistoryAcceptedTime = &accepted
		answered(backup)
		recordHistorySnapshotNames(backup, scheduled)
		*part = v1.BackupPart{State: v1.BackupPartInProgress}
		r.stageProgress(backup, "The backup of the web-application indices started")

	case errors.Is(err, camundaadmin.ErrUnreachable):
		return r.stageUnreachable(backup, "BackupHistory", part, err)

	case errors.Is(err, camundaadmin.ErrConflict):
		answered(backup)
		r.failStep(backup, "BackupHistory", part, fmt.Errorf("%w: %v", err, unownedHistoryBackup(backup)))

	default:
		answered(backup)
		r.failStep(backup, "BackupHistory", part, err)
	}

	return ctrl.Result{RequeueAfter: r.poll()}, nil
}

// recordHistorySnapshots merges the snapshot names that the cluster reports
// into status, so they survive the cluster.
func recordHistorySnapshots(backup *v1.LogicalBackupElasticsearch, status camundaadmin.BackupStatus) (added bool) {
	names := make([]string, 0, len(status.Details))
	for _, detail := range status.Details {
		names = append(names, detail.Name)
	}

	return recordHistorySnapshotNames(backup, names)
}

// snapshotRecords snapshots the exported Zeebe record indices directly in
// Elasticsearch. Camunda exposes no management endpoint for them. The snapshot
// goes to the repository that start pinned. A repository that changed mid-run
// fails the step, so the set is never split.
func (r *Reconciler) snapshotRecords(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
	cluster *v1.CamundaCluster,
) (ctrl.Result, error) {
	part := &backup.Status.Records
	storage, result, err := r.destination(ctx, backup, cluster, "SnapshotRecords", part)
	if storage == nil {
		return result, err
	}

	es, result, err := r.elasticsearch(ctx, backup, storage, "SnapshotRecords", part)
	if es == nil {
		return result, err
	}

	name := logicalbackup.RecordsSnapshotName(backup.Status.BackupID)
	snapshot, err := es.SnapshotStatus(ctx, backup.Status.Repository, name)
	if err != nil {
		if errors.Is(err, esadmin.ErrUnreachable) {
			return r.stageUnreachable(backup, "SnapshotRecords", part, err)
		}

		r.failStep(backup, "SnapshotRecords", part, err)
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	// The name is deterministic per backup ID, and an ID can be reused. An
	// existing snapshot is this backup's only when it carries the UID that
	// this backup writes into the snapshot metadata. A name match with
	// missing or foreign metadata is never adopted, and the finalizer will
	// not delete it.
	if snapshot.State != esadmin.SnapshotMissing && !snapshotOwnedBy(snapshot, backup) {
		r.failStep(backup, "SnapshotRecords", part, unownedSnapshot(name))
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	switch snapshot.State {
	case esadmin.SnapshotMissing:
		if err := es.CreateSnapshot(
			ctx, backup.Status.Repository, name, []string{logicalbackup.ZeebeRecordIndices}, snapshotMetadata(backup),
		); err != nil {
			if errors.Is(err, esadmin.ErrUnreachable) {
				return r.stageUnreachable(backup, "SnapshotRecords", part, err)
			}

			r.failStep(backup, "SnapshotRecords", part, err)
			return ctrl.Result{RequeueAfter: r.poll()}, nil
		}
		*part = v1.BackupPart{State: v1.BackupPartInProgress}
		r.stageProgress(backup, "The snapshot of the exported record indices started")

	case esadmin.SnapshotInProgress:
		*part = v1.BackupPart{State: v1.BackupPartInProgress}
		r.stageProgress(backup, "The snapshot of the exported record indices is in progress")

	case esadmin.SnapshotSuccess:
		*part = v1.BackupPart{State: v1.BackupPartCompleted}
		backup.Status.Step = v1.StepBackupRuntime
		r.stageProgress(backup, "Backing up the Zeebe partitions")

	default:
		r.failStep(backup, "SnapshotRecords", part, fmt.Errorf(
			"snapshot %q ended in state %s", name, snapshot.State,
		))
	}

	answered(backup)

	return ctrl.Result{RequeueAfter: r.poll()}, nil
}

// elasticsearch builds the Elasticsearch client of a verified storage
// contract for one running step. A Secret that is gone or unusable mid-run
// fails the step through resume. Only a transient read error comes back as
// an error.
func (r *Reconciler) elasticsearch(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
	storage *v1.SecondaryStorageConfig,
	step string,
	part *v1.BackupPart,
) (*esadmin.Client, ctrl.Result, error) {
	es, failure, err := secondarystorageconfig.ElasticsearchAdmin(ctx, r.APIReader, storage)
	if err != nil {
		return nil, ctrl.Result{}, err
	}
	if failure != nil {
		r.failStep(backup, step, part, errors.New(failure.Message))
		return nil, ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	return es, ctrl.Result{}, nil
}

// snapshotOwnedBy reports whether the snapshot carries the UID of backup. Only
// the one entry decides. A foreign snapshot can carry any other value under
// the name, of any JSON type. No such value makes the snapshot the property
// of this backup, and no such value keeps the finalizer from telling it
// apart.
func snapshotOwnedBy(snapshot esadmin.Snapshot, backup *v1.LogicalBackupElasticsearch) bool {
	uid, ok := snapshot.MetadataString(snapshotOwnerUIDKey)
	return ok && uid == string(backup.UID)
}

// unownedSnapshot is the failure of a snapshot that exists under the name
// of this backup without its UID in the metadata.
func unownedSnapshot(name string) error {
	return fmt.Errorf(
		"a snapshot %q exists that this backup did not create: "+
			"it is not adopted, and the finalizer will not delete it; remove it by hand if it is not wanted",
		name,
	)
}

// snapshotMetadata returns the user metadata of a snapshot of backup: the
// UID that proves ownership when the deterministic name is met again.
func snapshotMetadata(backup *v1.LogicalBackupElasticsearch) map[string]string {
	return map[string]string{snapshotOwnerUIDKey: string(backup.UID)}
}

// backupRuntime drives the backup of the Zeebe partitions under the same ID
// as every other part of the set.
func (r *Reconciler) backupRuntime(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
	cluster *v1.CamundaCluster,
) (ctrl.Result, error) {
	part := &backup.Status.Runtime
	mgmt, result, err := r.management(ctx, backup, cluster, "BackupRuntime", part)
	if mgmt == nil {
		return result, err
	}

	// The partitions write into the backup store that the cluster is
	// configured with now. A store that moved since the start puts the
	// runtime backup outside the pinned set. The delete of the finalizer
	// through the new store then misses it. The check runs before the
	// status of an existing backup is trusted and before a start.
	if storage, result, err := r.destination(ctx, backup, cluster, "BackupRuntime", part); storage == nil {
		return result, err
	}

	status, err := mgmt.RuntimeBackupStatus(ctx, backup.Status.BackupID)
	if err != nil {
		if errors.Is(err, camundaadmin.ErrUnreachable) {
			return r.stageUnreachable(backup, "BackupRuntime", part, err)
		}

		r.failStep(backup, "BackupRuntime", part, err)
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	if status.State == camundaadmin.StateDoesNotExist {
		return r.requestRuntimeBackup(ctx, mgmt, backup, part)
	}
	answered(backup)

	// A runtime backup under this ID exists. Only an accepted request of
	// this backup, a 202 that this controller observed, proves that it is
	// ours. Without that evidence the backup is not adopted. It belongs to
	// another actor, or it is ours with a lost response, and the cluster
	// gives no token to tell. The safe failure is to fail, never to adopt
	// and later delete a backup that is not ours.
	if backup.Status.RuntimeAcceptedTime == nil {
		r.failStep(backup, "BackupRuntime", part, unownedRuntimeBackup(backup))
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	switch status.State {
	case camundaadmin.StateInProgress:
		*part = v1.BackupPart{State: v1.BackupPartInProgress}
		r.stageProgress(backup, "The backup of the Zeebe partitions is in progress")

	case camundaadmin.StateCompleted:
		*part = v1.BackupPart{State: v1.BackupPartCompleted}
		backup.Status.Step = v1.StepResumeExporting
		r.stageProgress(backup, "Resuming exporting")

	default:
		r.failStep(backup, "BackupRuntime", part, errors.New(failureReason(status)))
	}

	return ctrl.Result{RequeueAfter: r.poll()}, nil
}

// requestRuntimeBackup drives an absent runtime backup to a request. The
// start is not idempotent, so the intent is written to the status one
// reconcile before the request. A crash or a lost response after the cluster
// accepted the request then finds the intent, not a fresh ID. It fails
// safely instead of adopting.
//
// The cluster registers the backup asynchronously and can report it absent
// for a moment after it accepted the request. Only a 202 that this
// controller observes records the acceptance time. An absent backup with
// the acceptance recorded is registration lag, and the step polls through
// the registration grace. The grace starts at the acceptance, so downtime
// before the request cannot consume it. Absent after the grace, the step
// fails.
//
// A 409 says that the cluster holds the same or a higher ID. It does not
// say whose backup that is: this backup's with a lost response, or another
// actor's that won the ID. The cluster gives no token to tell them apart,
// so a 409 is never an acceptance and the step fails without adopting. A
// crash between the request and the flush of the acceptance therefore
// fails the backup safely. It can leave a runtime backup under this ID in
// the cluster, for the user to remove by hand.
func (r *Reconciler) requestRuntimeBackup(
	ctx context.Context,
	mgmt *camundaadmin.Client,
	backup *v1.LogicalBackupElasticsearch,
	part *v1.BackupPart,
) (ctrl.Result, error) {
	now := metav1.Now()
	// The unreachable clock clears on the same terms as in
	// requestHistoryBackup. It clears on every branch after which every
	// call of this reconcile answered. It never clears before an
	// unreachable start call.
	if backup.Status.RuntimeRequestedTime == nil {
		backup.Status.RuntimeRequestedTime = &now
		answered(backup)
		r.stageProgress(backup, "The backup of the Zeebe partitions is requested")
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	if accepted := backup.Status.RuntimeAcceptedTime; accepted != nil {
		answered(backup)
		if now.Sub(accepted.Time) < r.runtimeRegistrationGrace() {
			r.stageProgress(backup, "The backup of the Zeebe partitions is registering")
			return ctrl.Result{RequeueAfter: r.poll()}, nil
		}
		r.failStep(backup, "BackupRuntime", part, fmt.Errorf(
			"the cluster holds no runtime backup %d within %s of the accepted request",
			backup.Status.BackupID, r.runtimeRegistrationGrace(),
		))
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	id := backup.Status.BackupID
	_, err := mgmt.StartRuntimeBackup(ctx, &id)
	switch {
	case err == nil:
		// The acceptance is stamped after the call returned, not with the
		// time before it. The registration grace starts here, and a slow
		// request must not silently shorten it.
		accepted := metav1.Now()
		backup.Status.RuntimeAcceptedTime = &accepted
		answered(backup)
		*part = v1.BackupPart{State: v1.BackupPartInProgress}
		r.stageProgress(backup, "The backup of the Zeebe partitions started")

	case errors.Is(err, camundaadmin.ErrUnreachable):
		return r.stageUnreachable(backup, "BackupRuntime", part, err)

	case errors.Is(err, camundaadmin.ErrConflict):
		answered(backup)
		r.failStep(backup, "BackupRuntime", part, fmt.Errorf("%w: %v", err, unownedRuntimeBackup(backup)))

	default:
		answered(backup)
		r.failStep(backup, "BackupRuntime", part, err)
	}

	return ctrl.Result{RequeueAfter: r.poll()}, nil
}

// management builds the management client of the cluster for one running
// step. A binding that broke mid-run fails the step through resume: the
// credentials Secret is gone, or the version is not supported. Only a
// transient read error comes back as an error.
func (r *Reconciler) management(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
	cluster *v1.CamundaCluster,
	step string,
	part *v1.BackupPart,
) (*camundaadmin.Client, ctrl.Result, error) {
	mgmt, failure, err := management.NewClient(ctx, r.APIReader, cluster)
	if err != nil {
		return nil, ctrl.Result{}, err
	}
	if failure != nil {
		r.failStep(backup, step, part, errors.New(failure.Message))
		return nil, ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	return mgmt, ctrl.Result{}, nil
}

// unownedHistoryBackup is the failure of a history backup that exists under
// the ID of this backup without an accepted request of this backup.
func unownedHistoryBackup(backup *v1.LogicalBackupElasticsearch) error {
	whose := "it belongs to another actor"
	if backup.Status.HistoryRequestedTime != nil {
		whose = "it can be the backup that this resource requested with a lost response, or one of another actor"
	}
	return fmt.Errorf(
		"a history backup %d exists that this backup did not see accepted: %s; "+
			"it is not adopted, and the finalizer will not delete its snapshots; remove it by hand if it is not wanted",
		backup.Status.BackupID, whose,
	)
}

// destination verifies that the cluster still writes to the destination
// that the backup pinned at its start. That is the snapshot repository of
// the binding, the storage contract, and its Elasticsearch endpoint. It returns
// the resolved contract when everything matches. A destination that moved,
// or a contract that is gone, fails the step through resume and returns a
// nil contract with the result to return. Only a transient read of the
// contract comes back as an error.
func (r *Reconciler) destination(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
	cluster *v1.CamundaCluster,
	step string,
	part *v1.BackupPart,
) (*v1.SecondaryStorageConfig, ctrl.Result, error) {
	if repointed := cluster.Status.Management.BackupRepository; repointed != backup.Status.Repository {
		r.failStep(backup, step, part, fmt.Errorf(
			"the snapshot repository of the cluster changed from %q to %q mid-run; the set must stay in one repository",
			backup.Status.Repository, repointed,
		))
		return nil, ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	storage, err := r.resolveStorage(ctx, cluster)
	if err != nil {
		if !storageMissing(err) {
			// A transient read of the contract is not a state of the
			// backup. Controller-runtime retries it with backoff.
			return nil, ctrl.Result{}, err
		}
		r.failStep(backup, step, part, err)
		return nil, ctrl.Result{RequeueAfter: r.poll()}, nil
	}
	if err := pinnedStorageMatches(backup, storage); err != nil {
		r.failStep(backup, step, part, err)
		return nil, ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	if err := r.pinnedBucketCurrent(ctx, backup, cluster); err != nil {
		if !errors.Is(err, errBucketMoved) && !apierrors.IsNotFound(err) {
			return nil, ctrl.Result{}, err
		}
		r.failStep(backup, step, part, err)
		return nil, ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	return storage, ctrl.Result{}, nil
}

// pinnedStorageMatches reports an error when the storage contract no longer
// matches the destination that start pinned. The repository name alone does
// not identify a cluster. A repointed contract or endpoint with the same
// repository name splits the set across two clusters. Endpoints compare in
// the form the client reaches. esadmin.New trims trailing slashes. An
// endpoint that differs by one slash is therefore the same cluster, not a
// retarget.
func pinnedStorageMatches(backup *v1.LogicalBackupElasticsearch, storage *v1.SecondaryStorageConfig) error {
	pinned := backup.Status.Storage
	if pinned == nil {
		return nil
	}
	if storage.Name != pinned.SecondaryStorageConfig {
		return fmt.Errorf(
			"the storage contract of the cluster changed from %q to %q mid-run; the set must stay on one cluster",
			pinned.SecondaryStorageConfig, storage.Name,
		)
	}
	if es := storage.Spec.Elasticsearch; es == nil ||
		strings.TrimRight(es.Endpoint, "/") != strings.TrimRight(pinned.Endpoint, "/") {
		return fmt.Errorf(
			"the Elasticsearch endpoint of %q changed from %q mid-run; the set must stay on one cluster",
			pinned.SecondaryStorageConfig, pinned.Endpoint,
		)
	}
	return nil
}

// unownedRuntimeBackup is the failure of a runtime backup that exists under
// the ID of this backup without an accepted request of this backup. The
// message says what the user needs: whose it can be, that it is not
// adopted, and that the finalizer leaves it alone.
func unownedRuntimeBackup(backup *v1.LogicalBackupElasticsearch) error {
	whose := "it belongs to another actor"
	if backup.Status.RuntimeRequestedTime != nil {
		whose = "it can be the backup that this resource requested with a lost response, or one of another actor"
	}
	return fmt.Errorf(
		"a runtime backup %d exists that this backup did not see accepted: %s; "+
			"it is not adopted, and the finalizer will not delete it; remove it by hand if it is not wanted",
		backup.Status.BackupID, whose,
	)
}

// failureReason returns the failure reason of a backup status. It joins the
// aggregate reason with every per-part reason that names its part, so the
// message says which snapshot or partition failed and why. When the endpoint
// gave no reason at all, it returns the state.
func failureReason(status camundaadmin.BackupStatus) string {
	var reasons []string
	if status.FailureReason != "" {
		reasons = append(reasons, status.FailureReason)
	}
	for _, detail := range status.Details {
		if detail.Reason == "" {
			continue
		}
		reasons = append(reasons, detail.Name+": "+detail.Reason)
	}
	if len(reasons) == 0 {
		return string(status.State)
	}

	return strings.Join(reasons, "; ")
}

// stageProgress stages the Ready condition of a healthy running procedure.
func (r *Reconciler) stageProgress(backup *v1.LogicalBackupElasticsearch, message string) {
	conditions.Stage(backup, conditions.Ready(
		metav1.ConditionFalse, v1.ReasonProgressing, message, backup.Generation,
	))
}

// stageUnreachable retries an unreachable endpoint for a bounded time and
// keeps the step. The endpoint is the management API or Elasticsearch,
// whichever the step calls. Exporting can be paused at every working step,
// so the retry cannot be unbounded. A route that black-holes only the
// backup endpoint while resume stays healthy leaves the cluster paused for
// good. After the bound, the step fails through resume. The clock starts on
// the first unreachable answer. It clears through answered, once every call
// of a reconcile answered.
func (r *Reconciler) stageUnreachable(
	backup *v1.LogicalBackupElasticsearch,
	step string,
	part *v1.BackupPart,
	err error,
) (ctrl.Result, error) {
	now := metav1.Now()
	if backup.Status.UnreachableSince == nil {
		backup.Status.UnreachableSince = &now
	}
	if now.Sub(backup.Status.UnreachableSince.Time) > r.unreachableBound() {
		r.failStep(
			backup,
			step,
			part,
			fmt.Errorf("the endpoint stayed unreachable for %s: %w", r.unreachableBound(), err),
		)
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	conditions.Stage(backup, conditions.Ready(
		metav1.ConditionFalse,
		v1.ReasonConnectionFailed,
		"The endpoint is unreachable and the step is retried: "+err.Error(),
		backup.Generation,
	))

	return ctrl.Result{RequeueAfter: r.poll()}, nil
}

// answered clears the unreachable clock. A step calls it only after every
// call of the reconcile answered. An answer before a call that stays
// unreachable must not reset the bound. If it does, the step retries
// forever with exporting paused.
func answered(backup *v1.LogicalBackupElasticsearch) {
	backup.Status.UnreachableSince = nil
}

// recordHistorySnapshotNames merges names into status.historySnapshots. It
// skips empty names and names that the status already holds.
func recordHistorySnapshotNames(backup *v1.LogicalBackupElasticsearch, names []string) (added bool) {
	for _, name := range names {
		if name == "" || slices.Contains(backup.Status.HistorySnapshots, name) {
			continue
		}
		backup.Status.HistorySnapshots = append(backup.Status.HistorySnapshots, name)
		added = true
	}

	return added
}
