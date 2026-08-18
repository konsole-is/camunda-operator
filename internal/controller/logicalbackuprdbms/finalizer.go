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
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
)

// errRepairable marks a cleanup failure the user can fix — a credentials
// Secret that exists but lost a key. The finalizer holds and polls for the
// repair instead of releasing or backing off.
var errRepairable = errors.New("cleanup is waiting for a repair")

// finalize removes the artifacts of a deleted backup: the Job and the dump
// object, keyed strictly on this backup's own object key. Zeebe backups are
// never touched: they belong to the continuous range that a point-in-time
// restore consumes. The order matters: the Job goes first, and the object is
// deleted only once the live API confirms the Job and its pods are gone, so a
// terminating uploader cannot recreate the object after the delete. Transient
// errors are returned, so the deletion retries; only when the dependency
// chain is genuinely gone — the cluster, the pinned bucket, or its
// credentials no longer exist — or when the pinned bucket now points
// somewhere else does the finalizer release with an event that records what
// was left behind, so a dead or retargeted contract never blocks deletion
// forever and never deletes a stranger's object.
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
		// The Job or its pods are still terminating; the object must wait,
		// or an upload still in flight could recreate it after the delete.
		return hold{after: shortly.after}, nil
	}

	if backup.Status.ObjectKey != "" {
		left, err := r.deleteObject(ctx, backup)
		if errors.Is(err, errRepairable) {
			// The user has to act; poll for the repair instead of backing off
			// exponentially, so the deletion finishes soon after it lands.
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

	// The claim goes back before the finalizer does, also for a backup that
	// claimed the cluster but never flushed an id.
	if err := r.releaseClaim(ctx, backup); err != nil {
		return settle, err
	}

	return settle, r.releaseFinalizer(ctx, backup)
}

// deleteJob deletes the dump Job of the backup and reports whether it and
// every pod of this backup are gone from the live API. The name is
// deterministic, so a crash that never recorded status.jobName still finds
// it; the UID label decides whether it is this backup's — a Job of another
// backup under the same name is left alone. The pods are always checked by
// this backup's UID, whichever Job holds the name now: after a background
// delete a foreign Job can take the name while this backup's uploader still
// terminates, and that uploader must not be able to recreate the object after
// the delete.
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
			// The observed UID is the delete's precondition, so the read and
			// the delete are one step: a same-named stranger that lands in
			// between answers Conflict and survives — the same pattern the
			// generated Secrets use through SSA metadata.uid. NotFound and
			// Conflict both mean "changed under us"; the requeue re-reads.
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
// The Job's pod template carries the backup UID, so the check follows the
// backup, not the Job name: a pod still terminating may still be uploading,
// and a foreign pod under the same Job name never holds the deletion.
func (r *LogicalBackupRDBMSReconciler) podsGone(ctx context.Context, backup *v1.LogicalBackupRDBMS) (bool, error) {
	pods, err := r.podsOf(ctx, backup)
	if err != nil {
		return false, err
	}

	return len(pods) == 0, nil
}

// deleteObject removes the dump from the pinned bucket — the one the backup
// wrote through, and only while it still points where it did at the start.
// It returns a non-empty reason when the object is left behind on purpose:
// the dependency chain is genuinely gone (the cluster, the contract, or the
// credentials Secret no longer exists), or the contract now points somewhere
// else and a delete would hit a stranger's object at the same key. An error
// means the failure is repairable or transient — a Secret missing a key, an
// API hiccup — and the deletion retries until it is.
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
		ctx, types.NamespacedName{Name: backup.Status.BucketRef}, &bucket,
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
		// new location is a stranger's; leaving the old object is the safe
		// failure.
		return fmt.Sprintf(
			"ObjectStorageConfig %q now points at %s, but the dump was written to %s; "+
				"deleting there could hit an unrelated object",
			bucket.Name, location, backup.Status.BucketLocation,
		), nil
	}

	var creds *objectstore.Credentials
	if credentials := bucket.CredentialsSecret(); credentials != nil {
		// The same rule the Job used: the source Secret when it lives in the
		// cluster namespace, the local copy the CamundaCluster controller
		// keeps otherwise. Either way the finalizer reads exactly the
		// credentials the upload used.
		local := types.NamespacedName{
			Namespace: cluster.Namespace,
			Name: localSecretName(
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
			// The Secret exists but a configured key is missing: that is
			// repairable, so the deletion is held and retried, not released
			// — releasing would orphan the dump on an in-progress edit. The
			// event makes the hold visible, so the user knows what to fix.
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

// releaseFinalizer removes the finalizer against the live object, retrying a
// write conflict: the deletion itself updates the object concurrently, and
// cleanup must not re-run over a resolvable conflict.
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
