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
	"errors"
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
	"github.com/konsole-is/camunda-operator/pkg/podstate"
)

// dump applies the Job once and tracks it to completion. A dependency that
// stopped resolving holds the backup for the mid-run grace, then fails it.
// A Running backup must either finish or terminalize. It must never park.
func (r *LogicalBackupRDBMSReconciler) dump(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) (hold, error) {
	key := types.NamespacedName{Namespace: backup.Namespace, Name: components.JobName(backup)}

	var current batchv1.Job
	if backup.Status.JobName != "" {
		// The Job was applied. The cache is enough to track it, and the
		// watch on owned Jobs wakes the backup on progress.
		if err := r.Get(ctx, key, &current); err != nil {
			if apierrors.IsNotFound(err) {
				// The cache has no read-your-writes guarantee. Right after
				// the apply, the informer can still miss the Job. Only the
				// live view can declare it gone. A lost race must never
				// terminally fail a valid dump.
				liveErr := r.APIReader.Get(ctx, key, &current)
				if liveErr == nil {
					return r.adopt(ctx, backup, &current)
				}
				if !apierrors.IsNotFound(liveErr) {
					return settle, liveErr
				}

				// The recorded Job is gone before it reported, so someone
				// deleted it by hand. The controller cannot trust that the
				// dump uploaded.
				r.fail(backup, "the dump Job disappeared before it completed")

				return settle, nil
			}

			return settle, err
		}

		return r.adopt(ctx, backup, &current)
	}

	// The Job does not exist yet, so the read must be live. If the cache is
	// stale after the apply, a second apply hits the immutable template that
	// the server stamped, and the server rejects it.
	err := r.APIReader.Get(ctx, key, &current)
	switch {
	case err == nil:
		return r.adopt(ctx, backup, &current)
	case !apierrors.IsNotFound(err):
		return settle, err
	}

	return r.createJob(ctx, backup)
}

// adopt tracks a Job found under the Job name of the backup, after it proves
// that the Job belongs to this backup. A Job that carries another UID is a
// leftover of a same-named backup that was deleted and recreated, or a
// foreign Job. The controller must never track it, because its completion
// lets this backup advance without a dump of its own. That is a hard
// failure, not a wait. The other Job will not change identity.
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
// once per backup. Afterwards the controller tracks the recorded name and
// never re-applies it.
func (r *LogicalBackupRDBMSReconciler) createJob(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) (hold, error) {
	res, failure, err := r.resolveRunning(ctx, backup)
	if errors.Is(err, errClusterReplaced) {
		r.fail(backup, err.Error())

		return settle, nil
	}
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
		BackupOwnsDump:     res.podOwned,
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

	return r.claimJobName(ctx, backup, job)
}

// claimJobName creates the Job as an identity claim: create-only, never SSA.
// This deviates from the apply rule of the repo on purpose. A forced apply
// after a NotFound is not atomic. It overwrites the UID label and the owner
// reference of a same-named Job that lands in between, before adopt can
// check them. The API server makes a Create atomic, which is the same
// reasoning as the claim Lease of the cluster. On AlreadyExists, the
// controller reads the winner and adopts it only when it carries the UID of
// this backup. A foreign winner is the existing bounded failure.
func (r *LogicalBackupRDBMSReconciler) claimJobName(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
	job *batchv1.Job,
) (hold, error) {
	err := r.Create(ctx, job)
	switch {
	case apierrors.IsAlreadyExists(err):
		var winner batchv1.Job
		if err := r.APIReader.Get(
			ctx, types.NamespacedName{Namespace: job.Namespace, Name: job.Name}, &winner,
		); err != nil {
			if apierrors.IsNotFound(err) {
				// The Job was deleted again in between. The requeue re-enters
				// the claim.
				return hold{after: shortly.after}, nil
			}

			return settle, fmt.Errorf("reading the dump Job that won the name: %w", err)
		}

		return r.adopt(ctx, backup, &winner)
	case err != nil:
		return settle, fmt.Errorf("creating the dump Job: %w", err)
	}

	backup.Status.JobName = job.Name
	conditions.Stage(backup, progressing(backup, "the dump Job runs"))

	// The watch on owned Jobs wakes the backup on progress. The poll is the
	// safety net.
	return hold{after: r.opts.RetryInterval}, nil
}

// errClusterReplaced reports that the cluster of a started backup is not the
// cluster that admitted it. It is terminal: the old cluster is gone for good.
var errClusterReplaced = errors.New("the CamundaCluster was replaced")

// runningCluster reads the cluster of a started backup and checks that it is
// the cluster that admitted the backup, by UID. A cluster that was deleted
// and created again under the same name is another cluster with other
// primary storage, whatever its config hash says. A gone cluster is a
// mid-run failure that the grace bounds. A replaced cluster returns
// errClusterReplaced, and the caller fails the backup at once.
func (r *LogicalBackupRDBMSReconciler) runningCluster(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) (*v1.CamundaCluster, *conditions.PreCheckFailure, error) {
	var cluster v1.CamundaCluster
	if err := r.APIReader.Get(
		ctx,
		types.NamespacedName{Namespace: backup.Namespace, Name: backup.Spec.ClusterRef.Name},
		&cluster,
	); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference(
				"CamundaCluster %s/%s does not exist", backup.Namespace, backup.Spec.ClusterRef.Name,
			), nil
		}

		return nil, nil, fmt.Errorf("reading the cluster: %w", err)
	}
	if backup.Status.ClusterUID != "" && cluster.UID != backup.Status.ClusterUID {
		return nil, nil, fmt.Errorf(
			"%w: CamundaCluster %s/%s admitted the backup with UID %s and now has UID %s, "+
				"so the dump and the Zeebe backup would not pair",
			errClusterReplaced, cluster.Namespace, cluster.Name, backup.Status.ClusterUID, cluster.UID,
		)
	}

	return &cluster, nil, nil
}

// resolveRunning re-resolves what the Dumping step needs. It reuses the
// admission resolution against the pinned bucket. It reports a failure that
// the user must see. holdRunning bounds how long that failure can hold the
// backup.
func (r *LogicalBackupRDBMSReconciler) resolveRunning(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) (*dumpResolution, *conditions.PreCheckFailure, error) {
	namespace := backup.Namespace

	cluster, failure, err := r.runningCluster(ctx, backup)
	if err != nil || failure != nil {
		return nil, failure, err
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

	// The same invariants that admission checked, right before createJob
	// renders the Job. Between the flush of admission and this reconcile, the
	// cluster can start a rollout, or its backup store can move. A Job
	// rendered then dumps the wrong database or uploads elsewhere, and the
	// Zeebe step finds out only afterwards. The object key was written for
	// the pinned bucket, not for the current backupStorageRef of the
	// cluster.
	if failure := clusterConverged(cluster); failure != nil {
		return nil, failure, nil
	}
	if failure, err := r.workloadUnchanged(ctx, backup, cluster); err != nil || failure != nil {
		return nil, failure, err
	}
	bucket, failure, err := r.pinnedBucket(ctx, backup, cluster)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	return r.resolveDump(ctx, backup, &logicalbackup.PreCheckResult{
		Cluster: cluster,
		Storage: &storage,
		Bucket:  bucket,
	})
}

// holdRunning stages a mid-run failure and decides its fate. Within the
// grace, it holds the backup on a timer. Past the grace, the backup fails.
// The grace counts from when the dependency first stopped resolving.
// holdRunning records that time in status, and recovered clears it. So an
// hours-old backup gets the same full grace as a fresh one.
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

// recovered clears the mid-run failure clock. The step just succeeded at
// what it needed, so the next failure gets the full grace again.
func (r *LogicalBackupRDBMSReconciler) recovered(backup *v1.LogicalBackupRDBMS) {
	backup.Status.FirstFailedAt = nil
}

// trackJob maps the observed Job onto the backup through the same status
// handler that the ocf job primitive uses. For a Job that is neither done
// nor failing, trackJob checks for a pod that cannot start. Such a pod
// consumes no retry and never fails the Job. So it runs through the bounded
// mid-run grace instead of holding the backup, and the queue behind it,
// without a bound.
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

	stuck, err := r.stuckPod(ctx, backup)
	if err != nil {
		return settle, err
	}
	if stuck != nil {
		return r.holdRunning(backup, stuck)
	}
	r.recovered(backup)
	conditions.Stage(backup, progressing(backup, status.Reason))

	// The watch on owned Jobs wakes the backup on Job progress. The poll
	// also re-checks the pods, because the Job does not report their waiting
	// states.
	return hold{after: r.opts.RetryInterval}, nil
}

// podsOf lists the pods of this backup's dump Job in the live API, by the
// backup UID that the pod template carries.
func (r *LogicalBackupRDBMSReconciler) podsOf(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) ([]corev1.Pod, error) {
	var pods corev1.PodList
	if err := r.APIReader.List(
		ctx, &pods, client.InNamespace(backup.Namespace),
		client.MatchingLabels{components.BackupUIDLabel: string(backup.UID)},
	); err != nil {
		return nil, fmt.Errorf("listing the pods of the dump Job: %w", err)
	}

	return pods.Items, nil
}

// stuckPod reports the first pod of the backup's Job that cannot start as a
// mid-run failure that names the pod and the reason. It selects the pods by
// the backup UID that the pod template carries, so a leftover pod of another
// backup of the same name never holds this one.
func (r *LogicalBackupRDBMSReconciler) stuckPod(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) (*conditions.PreCheckFailure, error) {
	return podstate.Stuck(
		ctx,
		r.APIReader,
		backup.Namespace,
		map[string]string{components.BackupUIDLabel: string(backup.UID)},
		"the dump Job",
	)
}
