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

package logicalbackuprdbms

import (
	"context"
	"fmt"

	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	ocfjob "github.com/sourcehawk/operator-component-framework/pkg/primitives/job"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/logicalbackuprdbms"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
)

// dump applies the Job once and tracks it to completion. A dependency that
// stopped resolving holds the backup for the mid-run grace, then fails it:
// a Running backup must either finish or terminalize, never park.
func (r *LogicalBackupRDBMSReconciler) dump(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) (hold, error) {
	key := types.NamespacedName{Namespace: backup.Namespace, Name: components.JobName(backup)}

	var current batchv1.Job
	if backup.Status.JobName != "" {
		// The Job was applied; the cache is enough to track it, and the
		// watch on owned Jobs wakes the backup on progress.
		if err := r.Get(ctx, key, &current); err != nil {
			if apierrors.IsNotFound(err) {
				// The cache has no read-your-writes guarantee: right after
				// the apply, the informer may not have seen the Job yet.
				// Only the live view may declare it gone — a lost race must
				// never terminally fail a valid dump.
				liveErr := r.APIReader.Get(ctx, key, &current)
				if liveErr == nil {
					return r.adopt(ctx, backup, &current)
				}
				if !apierrors.IsNotFound(liveErr) {
					return settle, liveErr
				}

				// The recorded Job is gone before it reported: deleted by
				// hand. The dump cannot be trusted to have uploaded.
				r.fail(backup, "the dump Job disappeared before it completed")

				return settle, nil
			}

			return settle, err
		}

		return r.adopt(ctx, backup, &current)
	}

	// The Job does not exist yet; the read must be live, because a stale
	// cache after the apply would re-apply against the server-stamped
	// immutable template and be rejected.
	err := r.APIReader.Get(ctx, key, &current)
	switch {
	case err == nil:
		return r.adopt(ctx, backup, &current)
	case !apierrors.IsNotFound(err):
		return settle, err
	}

	return r.createJob(ctx, backup)
}

// adopt tracks a Job found under the backup's Job name, after proving it is
// this backup's: a Job that carries another UID is a leftover of a
// same-named backup that was deleted and recreated, or foreign, and must
// never be tracked — its completion would let this backup advance without a
// dump of its own. That is a hard failure, not a wait: the other Job will not
// change identity.
func (r *LogicalBackupRDBMSReconciler) adopt(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
	job *batchv1.Job,
) (hold, error) {
	if !components.JobBelongsTo(job, backup) {
		r.fail(backup, fmt.Sprintf(
			"Job %s/%s exists but belongs to another backup (label %s=%q); this backup cannot "+
				"track it and will not create a second Job under the same name",
			job.Namespace, job.Name, components.BackupUIDLabel, job.Labels[components.BackupUIDLabel],
		))

		return settle, nil
	}
	backup.Status.JobName = job.Name

	return r.trackJob(ctx, backup, job)
}

// createJob re-resolves the dump dependencies and applies the Job. It runs
// once per backup; afterwards the recorded name is tracked, never re-applied.
func (r *LogicalBackupRDBMSReconciler) createJob(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) (hold, error) {
	res, failure, err := r.resolveRunning(ctx, backup)
	if err != nil {
		return settle, err
	}
	if failure != nil {
		return r.holdRunning(backup, failure)
	}
	r.recovered(backup)

	creds := res.dbSecret
	job, err := components.BuildJob(components.JobInput{
		Backup:             backup,
		ClusterName:        res.cluster.Name,
		Dump:               res.pod,
		PostgresImage:      res.image,
		Bucket:             res.bucket,
		BucketSecretName:   res.bucketSecret,
		DBSecretName:       creds.Name,
		DBUsernameKey:      creds.UsernameKey,
		DBPasswordKey:      creds.PasswordKey,
		ServiceAccountName: res.account,
		ServerVersion:      res.server.Status.ServerVersion,
		Host:               res.server.Spec.Host,
		Port:               res.server.Spec.Port,
		Database:           res.dbConfig.Spec.DatabaseName,
		ObjectKey:          backup.Status.ObjectKey,
		CLIImage:           r.opts.CLIImage,
	})
	if err != nil {
		return settle, err
	}

	if err := controllerutil.SetControllerReference(backup, job, r.Scheme); err != nil {
		return settle, fmt.Errorf("owning the dump Job: %w", err)
	}
	//nolint:staticcheck // the repo applies through the deprecated client.Apply patch
	if err := r.Patch(
		ctx, job, client.Apply, client.FieldOwner(fieldManager), client.ForceOwnership,
	); err != nil {
		return settle, fmt.Errorf("applying the dump Job: %w", err)
	}
	backup.Status.JobName = job.Name
	conditions.Stage(backup, progressing(backup, "the dump Job runs"))

	// The watch on owned Jobs wakes the backup on progress; the poll is the
	// safety net.
	return hold{after: r.opts.RetryInterval}, nil
}

// resolveRunning re-resolves what the Dumping step needs, reusing the
// admission resolution against the pinned bucket. It reports a failure the
// user must see; holdRunning bounds how long that failure may hold the
// backup.
func (r *LogicalBackupRDBMSReconciler) resolveRunning(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) (*dumpResolution, *conditions.PreCheckFailure, error) {
	namespace := backup.Namespace

	var cluster v1.CamundaCluster
	if err := r.APIReader.Get(
		ctx,
		types.NamespacedName{Namespace: namespace, Name: backup.Spec.ClusterRef.Name},
		&cluster,
	); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference(
				"CamundaCluster %s/%s does not exist", namespace, backup.Spec.ClusterRef.Name,
			), nil
		}

		return nil, nil, fmt.Errorf("reading the cluster: %w", err)
	}

	var storage v1.SecondaryStorageConfig
	if cluster.Spec.StorageRef == "" {
		return nil, logicalbackup.InvalidReference(
			"CamundaCluster %s/%s has no spec.storageRef", namespace, cluster.Name,
		), nil
	}
	storageKey := types.NamespacedName{Namespace: namespace, Name: cluster.Spec.StorageRef}
	if err := r.APIReader.Get(ctx, storageKey, &storage); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference(
				"SecondaryStorageConfig %s does not exist", storageKey,
			), nil
		}

		return nil, nil, fmt.Errorf("reading SecondaryStorageConfig %s: %w", storageKey, err)
	}
	if storage.Spec.RDBMS == nil {
		return nil, logicalbackup.InvalidReference(
			"SecondaryStorageConfig %s no longer describes a relational backend", storageKey,
		), nil
	}

	// The pinned bucket, not the cluster's current backupStorageRef: the
	// object key was written through the pinned one.
	var bucket v1.ObjectStorageConfig
	if err := r.APIReader.Get(
		ctx, types.NamespacedName{Name: backup.Status.BucketRef}, &bucket,
	); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference(
				"the pinned ObjectStorageConfig %q does not exist", backup.Status.BucketRef,
			), nil
		}

		return nil, nil, fmt.Errorf("reading ObjectStorageConfig %q: %w", backup.Status.BucketRef, err)
	}

	return r.resolveDump(ctx, backup, &logicalbackup.PreCheckResult{
		Cluster: &cluster,
		Storage: &storage,
		Bucket:  &bucket,
	})
}

// holdRunning stages a mid-run failure and decides its fate: within the
// grace it holds the backup on a timer, past it the backup fails. The grace
// is measured from when the dependency first stopped resolving — recorded in
// status and cleared by recovered — so an hours-old backup gets the same
// full grace as a fresh one.
func (r *LogicalBackupRDBMSReconciler) holdRunning(
	backup *v1.LogicalBackupRDBMS,
	failure *conditions.PreCheckFailure,
) (hold, error) {
	now := metav1.Now()
	if backup.Status.FirstFailedAt == nil {
		backup.Status.FirstFailedAt = &now
	}
	if now.Sub(backup.Status.FirstFailedAt.Time) > r.opts.MidRunGrace {
		r.fail(backup, fmt.Sprintf(
			"a dependency stopped resolving and did not recover: %s", failure.Message,
		))

		return settle, nil
	}

	conditions.Stage(backup, conditions.Failed(backup, failure))

	return hold{after: r.opts.RetryInterval}, nil
}

// recovered clears the mid-run failure clock: the step just succeeded at
// what it needed, so the next failure gets the full grace again.
func (r *LogicalBackupRDBMSReconciler) recovered(backup *v1.LogicalBackupRDBMS) {
	backup.Status.FirstFailedAt = nil
}

// trackJob maps the observed Job onto the backup through the same status
// handler the ocf job primitive uses. A Job that is neither done nor failing
// is checked for a pod that cannot start: such a pod consumes no retry and
// never fails the Job, so it runs through the bounded mid-run grace instead
// of holding the backup — and the queue behind it — forever.
func (r *LogicalBackupRDBMSReconciler) trackJob(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
	job *batchv1.Job,
) (hold, error) {
	status, err := ocfjob.DefaultConvergingStatusHandler(concepts.ConvergingOperationNone, job)
	if err != nil {
		return settle, err
	}

	switch status.Status {
	case concepts.CompletionStatusCompleted:
		backup.Status.Step = v1.StepZeebeBackup
		conditions.Stage(backup, progressing(
			backup, "the dump uploaded; the Zeebe backup starts",
		))

		return shortly, nil
	case concepts.CompletionStatusFailing:
		r.fail(backup, fmt.Sprintf("the dump Job failed: %s", status.Reason))

		return settle, nil
	}

	stuck, err := r.stuckPod(ctx, job)
	if err != nil {
		return settle, err
	}
	if stuck != nil {
		return r.holdRunning(backup, stuck)
	}
	r.recovered(backup)
	conditions.Stage(backup, progressing(backup, status.Reason))

	// The watch on owned Jobs wakes the backup on Job progress; the poll
	// also re-checks the pods, whose waiting states the Job does not report.
	return hold{after: r.opts.RetryInterval}, nil
}

// stuckWaitingReasons are the container waiting states that mean the pod
// will not start on its own: the configuration it mounts does not resolve, or
// its image does not pull. The kubelet keeps retrying them, the Job stays
// active, and no backoff is consumed.
var stuckWaitingReasons = map[string]string{
	"CreateContainerConfigError": v1.ReasonMissingSecret,
	"CreateContainerError":       v1.ReasonMissingSecret,
	"ErrImagePull":               v1.ReasonInvalidReference,
	"ImagePullBackOff":           v1.ReasonInvalidReference,
	"InvalidImageName":           v1.ReasonInvalidReference,
}

// stuckPod reports the first pod of job that cannot start — a container in a
// non-progressing waiting state, or a pod the scheduler cannot place, for
// example on a volume that never binds — as a mid-run failure naming the pod
// and the reason, or nil when every pod progresses.
func (r *LogicalBackupRDBMSReconciler) stuckPod(
	ctx context.Context,
	job *batchv1.Job,
) (*conditions.PreCheckFailure, error) {
	var pods corev1.PodList
	if err := r.APIReader.List(
		ctx, &pods, client.InNamespace(job.Namespace), client.MatchingLabels{jobNameLabel: job.Name},
	); err != nil {
		return nil, fmt.Errorf("listing the pods of the dump Job: %w", err)
	}

	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Status.Phase != corev1.PodPending && pod.Status.Phase != corev1.PodRunning {
			continue
		}
		statuses := append(
			append([]corev1.ContainerStatus{}, pod.Status.InitContainerStatuses...),
			pod.Status.ContainerStatuses...,
		)
		for _, cs := range statuses {
			waiting := cs.State.Waiting
			if waiting == nil {
				continue
			}
			if reason, stuck := stuckWaitingReasons[waiting.Reason]; stuck {
				return &conditions.PreCheckFailure{
					Reason: reason,
					Message: fmt.Sprintf(
						"pod %s of the dump Job cannot start: container %s reports %s: %s",
						pod.Name, cs.Name, waiting.Reason, waiting.Message,
					),
				}, nil
			}
		}
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse &&
				cond.Reason == corev1.PodReasonUnschedulable {
				return &conditions.PreCheckFailure{
					Reason: v1.ReasonProgressing,
					Message: fmt.Sprintf(
						"pod %s of the dump Job cannot be scheduled: %s", pod.Name, cond.Message,
					),
				}, nil
			}
		}
	}

	return nil, nil
}
