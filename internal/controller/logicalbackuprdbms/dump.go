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
	namespace := backup.Spec.ClusterRef.EffectiveClusterNamespace(backup.Namespace)
	key := types.NamespacedName{Namespace: namespace, Name: components.JobName(backup)}

	var current batchv1.Job
	if backup.Status.JobName != "" {
		// The Job was applied; the cache is enough to track it, and the
		// watch (same-namespace) or the poll below (cross-namespace) wakes
		// the backup on progress.
		if err := r.Get(ctx, key, &current); err != nil {
			if apierrors.IsNotFound(err) {
				// The cache has no read-your-writes guarantee: right after
				// the apply, the informer may not have seen the Job yet.
				// Only the live view may declare it gone — a lost race must
				// never terminally fail a valid dump.
				liveErr := r.APIReader.Get(ctx, key, &current)
				if liveErr == nil {
					return r.adopt(backup, &current)
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

		return r.adopt(backup, &current)
	}

	// The Job does not exist yet; the read must be live, because a stale
	// cache after the apply would re-apply against the server-stamped
	// immutable template and be rejected.
	err := r.APIReader.Get(ctx, key, &current)
	switch {
	case err == nil:
		return r.adopt(backup, &current)
	case !apierrors.IsNotFound(err):
		return settle, err
	}

	return r.createJob(ctx, backup)
}

// adopt tracks a Job found under the backup's Job name, after proving it is
// this backup's: the name derives from the backup namespace and name, but a
// Job that carries another UID belongs to another backup and must never be
// tracked — its completion would let this backup advance without a dump of
// its own. That is a hard failure, not a wait: the other Job will not change
// identity.
func (r *LogicalBackupRDBMSReconciler) adopt(
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

	return r.trackJob(backup, job)
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
		ClusterNamespace:   res.cluster.Namespace,
		Dump:               res.dump,
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

	if job.Namespace == backup.Namespace {
		if err := controllerutil.SetControllerReference(backup, job, r.Scheme); err != nil {
			return settle, fmt.Errorf("owning the dump Job: %w", err)
		}
	}
	//nolint:staticcheck // the repo applies through the deprecated client.Apply patch
	if err := r.Patch(
		ctx, job, client.Apply, client.FieldOwner(fieldManager), client.ForceOwnership,
	); err != nil {
		return settle, fmt.Errorf("applying the dump Job: %w", err)
	}
	backup.Status.JobName = job.Name
	conditions.Stage(backup, progressing(backup, "the dump Job runs"))

	// The watch only covers the backup's own namespace; a cross-namespace
	// cluster needs the poll.
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
	namespace := backup.Spec.ClusterRef.EffectiveClusterNamespace(backup.Namespace)

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
// handler the ocf job primitive uses.
func (r *LogicalBackupRDBMSReconciler) trackJob(
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

	conditions.Stage(backup, progressing(backup, status.Reason))

	// The watch wakes same-namespace backups instantly; the poll covers a
	// cross-namespace cluster, whose Job the watch cannot map back.
	return hold{after: r.opts.RetryInterval}, nil
}
