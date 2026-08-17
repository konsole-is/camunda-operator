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
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/esadmin"
	"github.com/konsole-is/camunda-operator/pkg/management"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/secondarystorageconfig"
)

// zeebeRecordIndices is the index pattern of the exported Zeebe record
// indices. It is the default prefix of the exporter. The operator configures
// no other prefix, so the default is the pattern.
const zeebeRecordIndices = "zeebe-record*"

// RecordsSnapshotName returns the name of the Elasticsearch snapshot that
// holds the exported Zeebe record indices of the backup id. LogicalRestore
// locates the snapshot by the same rule.
func RecordsSnapshotName(id int64) string {
	return "camunda_zeebe_records_backup_" + strconv.FormatInt(id, 10)
}

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
			return r.stageUnreachable(backup, err)
		}

		// A rejection can be a partial pause: some partitions paused, some
		// did not. Resume reverts it either way.
		r.failStep(backup, "PauseExporting", nil, err)
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	backup.Status.Step = v1.StepBackupHistory
	r.stageProgress(backup, "Exporting is soft-paused; backing up the web-application indices")

	return ctrl.Result{RequeueAfter: r.poll()}, nil
}

// backupHistory drives the backup of the web-application indices. It starts
// the backup when the cluster holds none under the ID, then it polls the
// backup to completion. The snapshot names are recorded as soon as the
// cluster reports them. The finalizer and a restore can then locate the
// snapshots after the cluster is gone.
func (r *Reconciler) backupHistory(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
	cluster *v1.CamundaCluster,
) (ctrl.Result, error) {
	mgmt, result, err := r.management(ctx, backup, cluster, "BackupHistory", &backup.Status.History)
	if mgmt == nil {
		return result, err
	}

	status, err := mgmt.HistoryBackupStatus(ctx, backup.Status.BackupID)
	if err != nil {
		if errors.Is(err, camundaadmin.ErrUnreachable) {
			return r.stageUnreachable(backup, err)
		}

		r.failStep(backup, "BackupHistory", &backup.Status.History, err)
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	recordHistorySnapshots(backup, status)

	switch status.State {
	case camundaadmin.StateDoesNotExist:
		if err := mgmt.StartHistoryBackup(ctx, backup.Status.BackupID); err != nil {
			if errors.Is(err, camundaadmin.ErrUnreachable) {
				return r.stageUnreachable(backup, err)
			}

			r.failStep(backup, "BackupHistory", &backup.Status.History, err)
			return ctrl.Result{RequeueAfter: r.poll()}, nil
		}
		backup.Status.History = v1.BackupPart{State: v1.BackupPartInProgress}
		r.stageProgress(backup, "The backup of the web-application indices started")

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
	if repointed := cluster.Status.Management.BackupRepository; repointed != backup.Status.Repository {
		r.failStep(backup, "SnapshotRecords", part, fmt.Errorf(
			"the cluster's snapshot repository changed from %q to %q mid-run; the set must stay in one repository",
			backup.Status.Repository, repointed,
		))
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	es, result, err := r.elasticsearch(ctx, backup, cluster, "SnapshotRecords", part)
	if es == nil {
		return result, err
	}

	name := RecordsSnapshotName(backup.Status.BackupID)
	state, err := es.SnapshotStatus(ctx, backup.Status.Repository, name)
	if err != nil {
		if errors.Is(err, esadmin.ErrUnreachable) {
			return r.stageElasticsearchUnreachable(backup, "SnapshotRecords", part, err)
		}

		r.failStep(backup, "SnapshotRecords", part, err)
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}
	backup.Status.ElasticsearchUnreachableSince = nil

	switch state {
	case esadmin.SnapshotMissing:
		if err := es.CreateSnapshot(
			ctx, backup.Status.Repository, name, []string{zeebeRecordIndices},
		); err != nil {
			if errors.Is(err, esadmin.ErrUnreachable) {
				return r.stageElasticsearchUnreachable(backup, "SnapshotRecords", part, err)
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
		r.failStep(backup, "SnapshotRecords", part, fmt.Errorf("snapshot %q ended in state %s", name, state))
	}

	return ctrl.Result{RequeueAfter: r.poll()}, nil
}

// elasticsearch builds the Elasticsearch client for one running step. A
// storage contract or a Secret that is gone mid-run fails the step through
// resume. Only a transient read error comes back as an error.
func (r *Reconciler) elasticsearch(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
	cluster *v1.CamundaCluster,
	step string,
	part *v1.BackupPart,
) (*esadmin.Client, ctrl.Result, error) {
	storage, err := r.resolveStorage(ctx, cluster)
	if err != nil {
		r.failStep(backup, step, part, err)
		return nil, ctrl.Result{RequeueAfter: r.poll()}, nil
	}
	if err := pinnedStorageMatches(backup, storage); err != nil {
		r.failStep(backup, step, part, err)
		return nil, ctrl.Result{RequeueAfter: r.poll()}, nil
	}

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

	status, err := mgmt.RuntimeBackupStatus(ctx, backup.Status.BackupID)
	if err != nil {
		if errors.Is(err, camundaadmin.ErrUnreachable) {
			return r.stageUnreachable(backup, err)
		}

		r.failStep(backup, "BackupRuntime", part, err)
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	switch status.State {
	case camundaadmin.StateDoesNotExist:
		// The cluster registers the backup asynchronously. After it accepted
		// the request it can report the backup absent for a moment. A second
		// POST for the same ID answers 409 then, and the valid backup would
		// be marked failed. Within the registration grace, an absent backup
		// is polled, not started again.
		if requested := backup.Status.RuntimeRequestedTime; requested != nil {
			if metav1.Now().Sub(requested.Time) < runtimeRegistrationGrace {
				r.stageProgress(backup, "The backup of the Zeebe partitions is registering")
				return ctrl.Result{RequeueAfter: r.poll()}, nil
			}
		}

		id := backup.Status.BackupID
		if _, err := mgmt.StartRuntimeBackup(ctx, &id); err != nil {
			if errors.Is(err, camundaadmin.ErrUnreachable) {
				return r.stageUnreachable(backup, err)
			}

			// A conflict on an ID that the cluster does not hold means that
			// another actor took a same-or-higher ID. To adopt that backup
			// points this resource at the artifacts of someone else.
			r.failStep(backup, "BackupRuntime", part, err)
			return ctrl.Result{RequeueAfter: r.poll()}, nil
		}
		now := metav1.Now()
		backup.Status.RuntimeRequestedTime = &now
		*part = v1.BackupPart{State: v1.BackupPartInProgress}
		r.stageProgress(backup, "The backup of the Zeebe partitions started")

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

// stageUnreachable keeps the step and retries. An unreachable endpoint is
// transient, and nothing was started.
func (r *Reconciler) stageUnreachable(
	backup *v1.LogicalBackupElasticsearch,
	err error,
) (ctrl.Result, error) {
	conditions.Stage(backup, conditions.Ready(
		metav1.ConditionFalse,
		v1.ReasonConnectionFailed,
		"The endpoint is unreachable and the step is retried: "+err.Error(),
		backup.Generation,
	))

	return ctrl.Result{RequeueAfter: r.poll()}, nil
}

// stageElasticsearchUnreachable retries an unreachable Elasticsearch endpoint
// for a bounded time. Exporting is paused at the steps that call
// Elasticsearch, so the retry cannot be unbounded: a healthy management API
// must get to resume the cluster. After the bound, the step fails through
// resume.
func (r *Reconciler) stageElasticsearchUnreachable(
	backup *v1.LogicalBackupElasticsearch,
	step string,
	part *v1.BackupPart,
	err error,
) (ctrl.Result, error) {
	now := metav1.Now()
	if backup.Status.ElasticsearchUnreachableSince == nil {
		backup.Status.ElasticsearchUnreachableSince = &now
	}
	if now.Sub(backup.Status.ElasticsearchUnreachableSince.Time) > r.elasticsearchUnreachableBound() {
		r.failStep(backup, step, part, fmt.Errorf("the Elasticsearch endpoint stayed unreachable: %w", err))
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	return r.stageUnreachable(backup, err)
}

// pinnedStorageMatches reports an error when the storage contract no longer
// matches the destination that start pinned. The repository name alone does
// not identify a cluster. A repointed contract or endpoint with the same
// repository name splits the set across two clusters.
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
	if es := storage.Spec.Elasticsearch; es == nil || es.Endpoint != pinned.Endpoint {
		return fmt.Errorf(
			"the Elasticsearch endpoint of %q changed from %q mid-run; the set must stay on one cluster",
			pinned.SecondaryStorageConfig, pinned.Endpoint,
		)
	}
	return nil
}

// recordHistorySnapshots merges the snapshot names that the cluster reports
// into status, so they survive the cluster.
func recordHistorySnapshots(backup *v1.LogicalBackupElasticsearch, status camundaadmin.BackupStatus) {
	for _, detail := range status.Details {
		if detail.Name == "" || slices.Contains(backup.Status.HistorySnapshots, detail.Name) {
			continue
		}
		backup.Status.HistorySnapshots = append(backup.Status.HistorySnapshots, detail.Name)
	}
}
