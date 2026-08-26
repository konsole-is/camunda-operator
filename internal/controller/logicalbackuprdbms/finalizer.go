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
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	camundacluster "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	components "github.com/konsole-is/camunda-operator/pkg/components/logicalbackuprdbms"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/mirror"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
)

// errRepairable marks a cleanup failure that the user can repair. Examples
// are a credentials Secret that exists but lost a key, or a failed cleanup
// Job. The finalizer holds and polls for the repair. It does not release, and
// it does not back off.
var errRepairable = errors.New("cleanup is waiting for a repair")

// errCleanupRunning means that the cleanup Job of the backup runs and that
// the object delete waits for its result.
var errCleanupRunning = errors.New("the cleanup Job is running")

// finalize removes the artifacts of a deleted backup: the Job and the dump
// object. The dump object is keyed strictly on the object key of this backup.
// It never touches Zeebe backups. They belong to the continuous range that a
// point-in-time restore consumes. The order matters. The Job goes first. The
// object is deleted only after the live API confirms that the Job and its
// pods are gone. Then an uploader that still terminates cannot recreate the
// object after the delete. A transient error is returned, so the deletion
// retries. The finalizer releases in two cases only: when the dependency
// chain is gone (the cluster, the pinned bucket, or its credentials no longer
// exist), or when the pinned bucket now points somewhere else. In both cases
// it records an event that names what it left behind. So a dead or
// retargeted contract never blocks the deletion, and the finalizer never
// deletes the object of a stranger.
func (r *LogicalBackupRDBMSReconciler) finalize(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) (hold, error) {
	if !controllerutil.ContainsFinalizer(backup, logicalbackup.Finalizer) {
		return settle, nil
	}

	gone, err := r.deleteJob(ctx, backup)
	if err != nil {
		return settle, err
	}
	if !gone {
		// The termination of the Job or its pods is not complete. The object
		// must wait. If it does not wait, an upload that is still in flight
		// can recreate it after the delete.
		return hold{after: shortly.after}, nil
	}

	if backup.Status.ObjectKey != "" {
		left, err := r.deleteObject(ctx, backup)
		if errors.Is(err, errCleanupRunning) {
			return hold{after: shortly.after}, nil
		}
		if errors.Is(err, errRepairable) {
			// The user must act. The finalizer polls for the repair instead of
			// an exponential backoff, so the deletion finishes soon after the
			// repair lands.
			return hold{after: r.opts.RetryInterval}, nil
		}
		if err != nil {
			return settle, err
		}
		if left != "" {
			r.EventRecorder.Eventf(
				backup,
				nil,
				corev1.EventTypeWarning,
				eventReasonCleanup,
				eventActionFinalize,
				"The dump object %q was left behind: %s",
				backup.Status.ObjectKey,
				left,
			)
		}
	}

	// The claim is released before the finalizer is removed. This also holds
	// for a backup that claimed the cluster but never flushed an id.
	if err := r.releaseClaim(ctx, backup); err != nil {
		return settle, err
	}

	return settle, r.releaseFinalizer(ctx, backup)
}

// deleteJob deletes the dump Job of the backup. It reports whether the Job
// and every pod of this backup are gone from the live API. The Job name is
// deterministic, so a crash that never recorded status.jobName still finds
// the Job. The UID label decides whether the Job belongs to this backup. A
// Job of another backup under the same name is left alone. The pods are
// always checked by the UID of this backup, whichever Job holds the name now.
// After a background delete, a foreign Job can take the name while the
// uploader of this backup still terminates. That uploader must not be able
// to recreate the object after the delete.
func (r *LogicalBackupRDBMSReconciler) deleteJob(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) (bool, error) {
	key := types.NamespacedName{Namespace: backup.Namespace, Name: components.JobName(backup)}

	var job batchv1.Job
	err := r.APIReader.Get(ctx, key, &job)
	switch {
	case err != nil && !apierrors.IsNotFound(err):
		return false, fmt.Errorf("reading the dump Job: %w", err)
	case err == nil && components.JobBelongsTo(&job, backup):
		if job.DeletionTimestamp.IsZero() {
			// The observed UID is the precondition of the delete, so the read
			// and the delete are one step. A same-named stranger that lands in
			// between answers Conflict and survives. The generated Secrets use
			// the same pattern through SSA metadata.uid. NotFound and Conflict
			// both mean that the object changed under us. The requeue re-reads.
			if err := r.Delete(
				ctx, &job,
				client.PropagationPolicy(metav1.DeletePropagationBackground),
				client.Preconditions{UID: &job.UID},
			); err != nil && !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
				return false, fmt.Errorf("deleting the dump Job: %w", err)
			}
		}

		return false, nil
	}

	return r.podsGone(ctx, backup)
}

// podsGone reports whether no pod of this backup is left in the live API.
// The pod template of the Job carries the backup UID, so the check follows
// the backup, not the Job name. A pod that still terminates can still
// upload. A foreign pod under the same Job name never holds the deletion.
func (r *LogicalBackupRDBMSReconciler) podsGone(ctx context.Context, backup *v1.LogicalBackupRDBMS) (bool, error) {
	pods, err := r.podsOf(ctx, backup)
	if err != nil {
		return false, err
	}

	return len(pods) == 0, nil
}

// deleteObject removes the dump from the pinned bucket, the one that the
// backup wrote through. It does so only while the bucket still points where
// it did at the start. It returns a non-empty reason when it leaves the
// object behind on purpose. That happens when the dependency chain is gone
// (the cluster, the contract, or the credentials Secret no longer exists), or
// when the contract now points somewhere else. In the second case a delete
// hits the object of a stranger at the same key. An error means that the
// failure is repairable or transient, for example a Secret that lacks a key
// or an API hiccup. Then the deletion retries until it succeeds.
func (r *LogicalBackupRDBMSReconciler) deleteObject(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) (string, error) {
	var cluster v1.CamundaCluster
	clusterKey := types.NamespacedName{Namespace: backup.Namespace, Name: backup.Spec.ClusterRef.Name}
	if err := r.APIReader.Get(ctx, clusterKey, &cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Sprintf("CamundaCluster %s is gone", clusterKey), nil
		}

		return "", fmt.Errorf("reading the cluster: %w", err)
	}

	var bucket v1.ObjectStorageConfig
	if err := r.APIReader.Get(
		ctx, types.NamespacedName{Namespace: backup.Namespace, Name: backup.Status.BucketRef}, &bucket,
	); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Sprintf(
				"the pinned ObjectStorageConfig %q is gone", backup.Status.BucketRef,
			), nil
		}

		return "", fmt.Errorf("reading ObjectStorageConfig %q: %w", backup.Status.BucketRef, err)
	}
	if location := bucket.Location(); location != backup.Status.BucketLocation {
		// The contract was retargeted under the object. The same key in the
		// new location belongs to a stranger. To leave the old object is the
		// safe failure.
		return fmt.Sprintf(
			"ObjectStorageConfig %q now points at %s, but the dump was written to %s; "+
				"deleting there could hit an unrelated object",
			bucket.Name, location, backup.Status.BucketLocation,
		), nil
	}

	// Cleanup runs where the identity lives. A workload-identity bucket
	// binds the CLUSTER ServiceAccount, which the manager does not have. So
	// the delete runs as a Job under that ServiceAccount, the same identity
	// surface that uploaded the object. The manager cleans a credentials-mode
	// bucket directly. It holds the Secret, and no pod identity is involved.
	credentials := bucket.CredentialsSecret()
	if credentials == nil {
		return r.cleanupWithJob(ctx, backup, &cluster, &bucket)
	}

	var creds *objectstore.Credentials
	{
		// The finalizer applies the same mirrored-Secret rule that the Job
		// applied, so it reads exactly the credentials that the upload used.
		local := types.NamespacedName{
			Namespace: cluster.Namespace,
			Name: mirror.LocalSecretName(
				&cluster,
				credentials.Namespace,
				credentials.Name,
				camundacluster.MirrorPurposeBackupCredentials,
			),
		}
		var secret corev1.Secret
		if err := r.APIReader.Get(ctx, local, &secret); err != nil {
			if apierrors.IsNotFound(err) {
				return fmt.Sprintf("the bucket credentials Secret %s is gone", local), nil
			}

			return "", fmt.Errorf("reading the bucket credentials %s: %w", local, err)
		}
		parsed, err := objectstore.CredentialsFrom(&bucket, secret.Data)
		if err != nil {
			// The Secret exists but a configured key is missing. That is
			// repairable, so the deletion is held and retried, not released.
			// A release orphans the dump during an edit that is in progress.
			// The event makes the hold visible, so the user knows what to fix.
			r.EventRecorder.Eventf(
				backup,
				nil,
				corev1.EventTypeWarning,
				eventReasonCleanup,
				eventActionFinalize,
				"Deletion is waiting for the bucket credentials Secret %s to be repaired: %v",
				local,
				err,
			)

			return "", fmt.Errorf("%w: the bucket credentials do not resolve: %w", errRepairable, err)
		}
		creds = parsed
	}

	store, err := r.opts.OpenBucket(ctx, &bucket, creds)
	if err != nil {
		return "", fmt.Errorf("opening the bucket: %w", err)
	}
	defer store.Close()

	if err := store.Delete(ctx, backup.Status.ObjectKey); err != nil {
		return "", fmt.Errorf("deleting %q: %w", backup.Status.ObjectKey, err)
	}

	return "", nil
}

// releaseFinalizer removes the finalizer against the live object and retries
// a write conflict. The deletion itself updates the object concurrently, and
// cleanup must not run again over a resolvable conflict.
func (r *LogicalBackupRDBMSReconciler) releaseFinalizer(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) error {
	key := client.ObjectKeyFromObject(backup)

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var current v1.LogicalBackupRDBMS
		if err := r.Get(ctx, key, &current); err != nil {
			return client.IgnoreNotFound(err)
		}
		if !controllerutil.RemoveFinalizer(&current, logicalbackup.Finalizer) {
			return nil
		}

		return r.Update(ctx, &current)
	})
}

// cleanupWithJob removes the dump object through a cleanup Job in the
// namespace of the backup. The Job runs under the cluster ServiceAccount that
// the workload identity of the bucket is bound to. The Job name is
// deterministic, and its creation is a create-only identity claim, like the
// dump Job. A running Job holds the delete (errCleanupRunning). A completed
// Job is removed, and the cleanup counts as done. A failed Job holds the
// deletion visibly (errRepairable) with an event that names the Job. The
// user inspects the Job. When the user deletes it, the cleanup retries with
// a fresh Job.
func (r *LogicalBackupRDBMSReconciler) cleanupWithJob(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
	cluster *v1.CamundaCluster,
	bucket *v1.ObjectStorageConfig,
) (string, error) {
	key := types.NamespacedName{Namespace: backup.Namespace, Name: components.CleanupJobName(backup)}

	var job batchv1.Job
	err := r.APIReader.Get(ctx, key, &job)
	switch {
	case err != nil && !apierrors.IsNotFound(err):
		return "", fmt.Errorf("reading the cleanup Job: %w", err)
	case err == nil && !components.JobBelongsTo(&job, backup):
		r.cleanupEvent(backup, "Deletion is waiting: Job %s belongs to another backup", key)

		return "", fmt.Errorf("%w: the cleanup Job name %s is taken by another backup", errRepairable, key)
	case err == nil:
		return r.watchCleanupJob(ctx, backup, &job)
	}

	pod, failure, err := r.resolvePod(ctx, cluster, backup)
	if err != nil {
		return "", err
	}
	if failure != nil {
		r.cleanupEvent(
			backup, "Deletion is waiting: the cleanup Job cannot be rendered: %s", failure.Message,
		)

		return "", fmt.Errorf("%w: %s", errRepairable, failure.Message)
	}

	cleanup, err := components.BuildCleanupJob(components.CleanupJobInput{
		Backup:             backup,
		ClusterName:        cluster.Name,
		Dump:               pod.settings,
		BackupOwnsDump:     pod.owned,
		Bucket:             bucket,
		ServiceAccountName: pod.account,
		ObjectKey:          backup.Status.ObjectKey,
		CLIImage:           r.opts.CLIImage,
	})
	if err != nil {
		return "", err
	}
	if err := controllerutil.SetControllerReference(backup, cleanup, r.Scheme); err != nil {
		return "", fmt.Errorf("owning the cleanup Job: %w", err)
	}
	// Create-only, like every identity claim on a deterministic name. An
	// AlreadyExists lost the race to a concurrent reconcile, and the requeue
	// re-reads.
	if err := r.Create(ctx, cleanup); err != nil && !apierrors.IsAlreadyExists(err) {
		return "", fmt.Errorf("creating the cleanup Job: %w", err)
	}

	return "", errCleanupRunning
}

// watchCleanupJob maps the observed cleanup Job onto the deletion: done,
// still running, or held on a failure that the user must see.
func (r *LogicalBackupRDBMSReconciler) watchCleanupJob(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
	job *batchv1.Job,
) (string, error) {
	status, err := ocfjob.DefaultConvergingStatusHandler(concepts.ConvergingOperationNone, job)
	if err != nil {
		return "", err
	}

	switch status.Status {
	case concepts.CompletionStatusCompleted:
		// The object is gone. The Job served its purpose.
		if err := r.Delete(
			ctx, job,
			client.PropagationPolicy(metav1.DeletePropagationBackground),
			client.Preconditions{UID: &job.UID},
		); err != nil && !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
			return "", fmt.Errorf("removing the finished cleanup Job: %w", err)
		}

		return "", nil
	case concepts.CompletionStatusFailing:
		r.cleanupEvent(
			backup,
			"Deletion is waiting: the cleanup Job %s/%s failed (%s); inspect it, fix the cause, "+
				"and delete the Job to retry",
			job.Namespace, job.Name, status.Reason,
		)

		return "", fmt.Errorf(
			"%w: the cleanup Job %s/%s failed", errRepairable, job.Namespace, job.Name,
		)
	}

	return "", errCleanupRunning
}

// cleanupEvent records why the deletion is held, so the hold is visible.
func (r *LogicalBackupRDBMSReconciler) cleanupEvent(
	backup *v1.LogicalBackupRDBMS, format string, args ...any,
) {
	r.EventRecorder.Eventf(
		backup, nil, corev1.EventTypeWarning, eventReasonCleanup, eventActionFinalize, format, args...,
	)
}
