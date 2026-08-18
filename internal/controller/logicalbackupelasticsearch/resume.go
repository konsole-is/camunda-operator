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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/management"
)

// resumeExporting always runs, after success and after any failure. A cluster
// that stays soft-paused cannot compact its log and fills its disks. The
// terminal phase is written only here, after resume succeeded. If the
// deadline of accumulated active attempts passed instead, the cluster needs a
// human and the phase goes Failed with reason ResumeFailed.
func (r *Reconciler) resumeExporting(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
	cluster *v1.CamundaCluster,
) (ctrl.Result, error) {
	now := metav1.Now()
	r.chargeResumeAttempt(backup, now)

	if err := resumeOnce(ctx, r, cluster); err != nil {
		if now.Sub(backup.Status.ResumeStartedTime.Time) > r.resumeDeadline() {
			return r.giveUpOnResume(backup, now, err)
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

	r.finish(backup, now)

	return ctrl.Result{}, nil
}

// chargeResumeAttempt accounts one resume attempt against the deadline. The
// deadline bounds attempting, not wall-clock time. Every gap between attempts
// counts as at most one poll interval. A parked procedure or a slow reconcile
// slides the anchor forward and does not consume the budget.
func (r *Reconciler) chargeResumeAttempt(backup *v1.LogicalBackupElasticsearch, now metav1.Time) {
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
}

// resumeOnce makes one resume call against the cluster.
func resumeOnce(ctx context.Context, r *Reconciler, cluster *v1.CamundaCluster) error {
	mgmt, failure, err := management.NewClient(ctx, r.APIReader, cluster)
	if err != nil {
		return err
	}
	if failure != nil {
		return errors.New(failure.Message)
	}
	return mgmt.ResumeExporting(ctx)
}

// giveUpOnResume writes the ResumeFailed terminal phase and warns.
func (r *Reconciler) giveUpOnResume(
	backup *v1.LogicalBackupElasticsearch,
	now metav1.Time,
	err error,
) (ctrl.Result, error) {
	backup.Status.Phase = v1.LogicalBackupFailed
	backup.Status.TerminalReason = v1.ReasonResumeFailed
	backup.Status.ResumeFailureMessage = conditions.BoundMessage(fmt.Sprintf(
		"Exporting could not be resumed before the deadline: %v", err,
	))
	if backup.Status.FailureMessage == "" {
		backup.Status.FailureMessage = backup.Status.ResumeFailureMessage
	}
	backup.Status.CompletionTime = &now
	conditions.Stage(backup, terminalReady(backup))
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

// finish writes the terminal phase after a successful resume. A recorded step
// failure ends in Failed. A clean walk ends in Completed.
func (r *Reconciler) finish(backup *v1.LogicalBackupElasticsearch, now metav1.Time) {
	backup.Status.CompletionTime = &now

	if backup.Status.FailureMessage != "" {
		backup.Status.Phase = v1.LogicalBackupFailed
		backup.Status.TerminalReason = v1.ReasonFailed
		conditions.Stage(backup, terminalReady(backup))
		return
	}

	backup.Status.Phase = v1.LogicalBackupCompleted
	backup.Status.TerminalReason = v1.ReasonCompleted
	conditions.Stage(backup, terminalReady(backup))
	r.EventRecorder.Eventf(
		backup,
		nil,
		corev1.EventTypeNormal,
		eventReasonCompleted,
		eventActionBackup,
		"Backup %d completed",
		backup.Status.BackupID,
	)
}

// failStep records the failing step, marks the affected part, and routes to
// resume. The terminal phase waits until exporting runs again.
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
		*part = v1.BackupPart{State: v1.BackupPartFailed, FailureReason: conditions.BoundMessage(reason)}
	}

	// The clients bound the body they carry, and every condition is bounded
	// centrally. The status strings that hold the same text are bounded here
	// too, so one oversized error can never make the status unwritable.
	backup.Status.FailureMessage = conditions.BoundMessage(fmt.Sprintf("Step %s failed: %v", step, err))
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

// partOf returns the backup part that the step drives, or nil for a step that
// owns no part.
func partOf(backup *v1.LogicalBackupElasticsearch, step v1.LogicalBackupElasticsearchStep) *v1.BackupPart {
	switch step {
	case v1.StepBackupHistory:
		return &backup.Status.History
	case v1.StepSnapshotRecords:
		return &backup.Status.Records
	case v1.StepBackupRuntime:
		return &backup.Status.Runtime
	default:
		return nil
	}
}
