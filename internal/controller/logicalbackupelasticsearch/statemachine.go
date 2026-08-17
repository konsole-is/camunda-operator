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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/esadmin"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
)

// zeebeRecordIndices is the index pattern of the exported Zeebe record
// indices: the exporter's default prefix. The operator configures no other
// prefix, so the default is the pattern.
const zeebeRecordIndices = "zeebe-record*"

// RecordsSnapshotName returns the name of the Elasticsearch snapshot that
// holds the exported Zeebe record indices of the backup id. LogicalRestore
// locates the snapshot by the same rule.
func RecordsSnapshotName(id int64) string {
	return "camunda_zeebe_records_backup_" + strconv.FormatInt(id, 10)
}

// runStep executes the step that status.step records and advances at most one
// step, so every transition is persisted before the next side effect. Every
// step queries the current state before it acts; a crash re-enters without
// repeating a call. Each step builds only the clients it uses, and a
// dependency that broke mid-run fails the step through resume — the machine
// owns every exit.
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
// step. A binding that broke mid-run — the credentials Secret gone, an
// unsupported version — fails the step through resume; only a transient read
// error comes back as an error.
func (r *Reconciler) management(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
	cluster *v1.CamundaCluster,
	step string,
	part *v1.BackupPart,
) (*camundaadmin.Client, ctrl.Result, error) {
	mgmt, failure, err := logicalbackup.ManagementClient(ctx, r.APIReader, cluster)
	if err != nil {
		return nil, ctrl.Result{}, err
	}
	if failure != nil {
		r.failStep(backup, step, part, errors.New(failure.Message))
		return nil, ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	return mgmt, ctrl.Result{}, nil
}

// pauseExporting soft-pauses exporting: records keep flowing, log compaction
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
		// not. Resume reverts it either way.
		r.failStep(backup, "PauseExporting", nil, err)
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	backup.Status.Step = v1.StepBackupHistory
	r.stageProgress(backup, "Exporting is soft-paused; backing up the web-application indices")

	return ctrl.Result{RequeueAfter: r.poll()}, nil
}

// backupHistory drives the backup of the web-application indices: start it if
// the cluster holds none under the id, then poll it to completion. The
// snapshot names are recorded as soon as the cluster reports them, so the
// finalizer and a restore can locate them after the cluster is gone.
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
		reason := status.FailureReason
		if reason == "" {
			reason = string(status.State)
		}
		r.failStep(backup, "BackupHistory", &backup.Status.History, errors.New(reason))
	}

	return ctrl.Result{RequeueAfter: r.poll()}, nil
}

// snapshotRecords snapshots the exported Zeebe record indices directly in
// Elasticsearch: Camunda exposes no management endpoint for them. The
// snapshot goes to the repository pinned at start; a repository that was
// repointed mid-run fails the step rather than splitting the set.
func (r *Reconciler) snapshotRecords(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
	cluster *v1.CamundaCluster,
) (ctrl.Result, error) {
	if repointed := cluster.Status.Management.BackupRepository; repointed != backup.Status.Repository {
		r.failStep(backup, "SnapshotRecords", &backup.Status.Records, fmt.Errorf(
			"the cluster's snapshot repository changed from %q to %q mid-run; the set must stay in one repository",
			backup.Status.Repository, repointed,
		))
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	storage, err := r.resolveStorage(ctx, cluster)
	if err != nil {
		r.failStep(backup, "SnapshotRecords", &backup.Status.Records, err)
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	es, err := r.elasticsearchAdmin(ctx, storage)
	if err != nil {
		var failure *conditions.PreCheckFailure
		if errors.As(err, &failure) {
			r.failStep(backup, "SnapshotRecords", &backup.Status.Records, errors.New(failure.Message))
			return ctrl.Result{RequeueAfter: r.poll()}, nil
		}
		return ctrl.Result{}, err
	}

	name := RecordsSnapshotName(backup.Status.BackupID)
	state, err := es.SnapshotStatus(ctx, backup.Status.Repository, name)
	if err != nil {
		if errors.Is(err, esadmin.ErrUnreachable) {
			return r.stageUnreachable(backup, err)
		}

		r.failStep(backup, "SnapshotRecords", &backup.Status.Records, err)
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	switch state {
	case esadmin.SnapshotMissing:
		if err := es.CreateSnapshot(
			ctx, backup.Status.Repository, name, []string{zeebeRecordIndices},
		); err != nil {
			if errors.Is(err, esadmin.ErrUnreachable) {
				return r.stageUnreachable(backup, err)
			}

			r.failStep(backup, "SnapshotRecords", &backup.Status.Records, err)
			return ctrl.Result{RequeueAfter: r.poll()}, nil
		}
		backup.Status.Records = v1.BackupPart{State: v1.BackupPartInProgress}
		r.stageProgress(backup, "The snapshot of the exported record indices started")

	case esadmin.SnapshotInProgress:
		backup.Status.Records = v1.BackupPart{State: v1.BackupPartInProgress}
		r.stageProgress(backup, "The snapshot of the exported record indices is in progress")

	case esadmin.SnapshotSuccess:
		backup.Status.Records = v1.BackupPart{State: v1.BackupPartCompleted}
		backup.Status.Step = v1.StepBackupRuntime
		r.stageProgress(backup, "Backing up the Zeebe partitions")

	default:
		r.failStep(
			backup,
			"SnapshotRecords",
			&backup.Status.Records,
			fmt.Errorf("snapshot %q ended in state %s", name, state),
		)
	}

	return ctrl.Result{RequeueAfter: r.poll()}, nil
}

// backupRuntime drives the backup of the Zeebe partitions under the same id
// as every other part of the set.
func (r *Reconciler) backupRuntime(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
	cluster *v1.CamundaCluster,
) (ctrl.Result, error) {
	mgmt, result, err := r.management(ctx, backup, cluster, "BackupRuntime", &backup.Status.Runtime)
	if mgmt == nil {
		return result, err
	}

	status, err := mgmt.RuntimeBackupStatus(ctx, backup.Status.BackupID)
	if err != nil {
		if errors.Is(err, camundaadmin.ErrUnreachable) {
			return r.stageUnreachable(backup, err)
		}

		r.failStep(backup, "BackupRuntime", &backup.Status.Runtime, err)
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	switch status.State {
	case camundaadmin.StateDoesNotExist:
		id := backup.Status.BackupID
		if _, err := mgmt.StartRuntimeBackup(ctx, &id); err != nil {
			if errors.Is(err, camundaadmin.ErrUnreachable) {
				return r.stageUnreachable(backup, err)
			}

			// A conflict on an id the cluster does not hold means another
			// actor took a same-or-higher id. Adopting that backup would
			// point this resource at someone else's artifacts.
			r.failStep(backup, "BackupRuntime", &backup.Status.Runtime, err)
			return ctrl.Result{RequeueAfter: r.poll()}, nil
		}
		backup.Status.Runtime = v1.BackupPart{State: v1.BackupPartInProgress}
		r.stageProgress(backup, "The backup of the Zeebe partitions started")

	case camundaadmin.StateInProgress:
		backup.Status.Runtime = v1.BackupPart{State: v1.BackupPartInProgress}
		r.stageProgress(backup, "The backup of the Zeebe partitions is in progress")

	case camundaadmin.StateCompleted:
		backup.Status.Runtime = v1.BackupPart{State: v1.BackupPartCompleted}
		backup.Status.Step = v1.StepResumeExporting
		r.stageProgress(backup, "Resuming exporting")

	default:
		reason := status.FailureReason
		if reason == "" {
			reason = string(status.State)
		}
		r.failStep(backup, "BackupRuntime", &backup.Status.Runtime, errors.New(reason))
	}

	return ctrl.Result{RequeueAfter: r.poll()}, nil
}

// resumeExporting always runs, after success and after any failure: a cluster
// left soft-paused cannot compact its log and fills its disks. The terminal
// phase is written only here, once resume succeeded — or once the deadline
// of accumulated active attempts passed and the cluster needs a human. Time
// in which the procedure was parked elsewhere slides the deadline anchor
// forward, so a long suspension cannot consume the budget of the attempts.
func (r *Reconciler) resumeExporting(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
	cluster *v1.CamundaCluster,
) (ctrl.Result, error) {
	// The deadline bounds attempting, not wall-clock time: every gap between
	// attempts counts as at most one poll interval, so a parked procedure —
	// or a slow reconcile — slides the anchor forward instead of consuming
	// the budget.
	now := metav1.Now()
	if backup.Status.ResumeStartedTime == nil {
		backup.Status.ResumeStartedTime = &now
	}
	if last := backup.Status.LastResumeAttemptTime; last != nil {
		if gap := now.Sub(last.Time); gap > r.poll() {
			slid := metav1.NewTime(backup.Status.ResumeStartedTime.Add(gap - r.poll()))
			backup.Status.ResumeStartedTime = &slid
		}
	}
	backup.Status.LastResumeAttemptTime = &now

	attempt := func() error {
		mgmt, failure, err := logicalbackup.ManagementClient(ctx, r.APIReader, cluster)
		if err != nil {
			return err
		}
		if failure != nil {
			return errors.New(failure.Message)
		}
		return mgmt.ResumeExporting(ctx)
	}

	if err := attempt(); err != nil {
		if now.Sub(backup.Status.ResumeStartedTime.Time) > r.resumeDeadline() {
			backup.Status.Phase = v1.LogicalBackupFailed
			backup.Status.TerminalReason = v1.ReasonResumeFailed
			if backup.Status.FailureMessage == "" {
				backup.Status.FailureMessage = fmt.Sprintf(
					"Exporting could not be resumed before the deadline: %v", err,
				)
			}
			backup.Status.CompletionTime = &now
			conditions.Stage(backup, r.terminalReady(backup))
			r.EventRecorder.Eventf(
				backup,
				nil,
				corev1.EventTypeWarning,
				eventReasonResumeFailed,
				eventActionBackup,
				"Exporting could not be resumed before the deadline; the cluster cannot compact its log: %v",
				err,
			)

			return ctrl.Result{}, nil
		}

		reason := v1.ReasonFailed
		if errors.Is(err, camundaadmin.ErrUnreachable) {
			reason = v1.ReasonConnectionFailed
		}
		conditions.Stage(backup, conditions.Ready(
			metav1.ConditionFalse,
			reason,
			fmt.Sprintf("Resuming exporting failed and is retried: %v", err),
			backup.Generation,
		))

		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	backup.Status.CompletionTime = &now

	if backup.Status.FailureMessage != "" {
		backup.Status.Phase = v1.LogicalBackupFailed
		backup.Status.TerminalReason = v1.ReasonFailed
		conditions.Stage(backup, r.terminalReady(backup))

		return ctrl.Result{}, nil
	}

	backup.Status.Phase = v1.LogicalBackupCompleted
	backup.Status.TerminalReason = v1.ReasonCompleted
	conditions.Stage(backup, r.terminalReady(backup))
	r.EventRecorder.Eventf(
		backup,
		nil,
		corev1.EventTypeNormal,
		eventReasonCompleted,
		eventActionBackup,
		"Backup %d completed",
		backup.Status.BackupID,
	)

	return ctrl.Result{}, nil
}

// failStep records the failing step, marks the affected part, and routes to
// resume: the terminal phase waits until exporting runs again.
func (r *Reconciler) failStep(
	backup *v1.LogicalBackupElasticsearch,
	step string,
	part *v1.BackupPart,
	err error,
) {
	if part != nil {
		reason := err.Error()
		if part.FailureReason != "" {
			reason = part.FailureReason
		}
		*part = v1.BackupPart{State: v1.BackupPartFailed, FailureReason: reason}
	}

	backup.Status.FailureMessage = fmt.Sprintf("Step %s failed: %v", step, err)
	backup.Status.Step = v1.StepResumeExporting
	backup.Status.ResumeStartedTime = nil
	backup.Status.LastResumeAttemptTime = nil
	conditions.Stage(backup, conditions.Ready(
		metav1.ConditionFalse,
		v1.ReasonProgressing,
		backup.Status.FailureMessage+"; resuming exporting",
		backup.Generation,
	))
	r.EventRecorder.Eventf(
		backup,
		nil,
		corev1.EventTypeWarning,
		eventReasonStepFailed,
		eventActionBackup,
		"%s",
		backup.Status.FailureMessage,
	)
}

// stageProgress stages the Ready condition of a healthy running procedure.
func (r *Reconciler) stageProgress(backup *v1.LogicalBackupElasticsearch, message string) {
	conditions.Stage(backup, conditions.Ready(
		metav1.ConditionFalse, v1.ReasonProgressing, message, backup.Generation,
	))
}

// stageUnreachable keeps the step and retries: an unreachable endpoint is
// transient and nothing was started.
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

// recordHistorySnapshots merges the snapshot names the cluster reports into
// status, so they survive the cluster.
func recordHistorySnapshots(backup *v1.LogicalBackupElasticsearch, status camundaadmin.BackupStatus) {
	for _, detail := range status.Details {
		if detail.Name == "" || slices.Contains(backup.Status.HistorySnapshots, detail.Name) {
			continue
		}
		backup.Status.HistorySnapshots = append(backup.Status.HistorySnapshots, detail.Name)
	}
}
