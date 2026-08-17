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

// jobNameLabel is the label the Job controller stamps on the pods of a Job.
const jobNameLabel = "batch.kubernetes.io/job-name"

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

	return settle, r.releaseFinalizer(ctx, backup)
}

// deleteJob deletes the dump Job of the backup and reports whether it and
// its pods are gone from the live API. The name is deterministic, so a crash
// that never recorded status.jobName still finds it; the UID label decides
// whether it is this backup's — a leftover Job of a same-named backup is left
// alone and counts as gone for this one.
func (r *LogicalBackupRDBMSReconciler) deleteJob(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) (bool, error) {
	key := types.NamespacedName{Namespace: backup.Namespace, Name: components.JobName(backup)}

	var job batchv1.Job
	err := r.APIReader.Get(ctx, key, &job)
	switch {
	case apierrors.IsNotFound(err):
		return r.podsGone(ctx, key)
	case err != nil:
		return false, fmt.Errorf("reading the dump Job: %w", err)
	case !components.JobBelongsTo(&job, backup):
		return true, nil
	}

	if job.DeletionTimestamp.IsZero() {
		propagation := metav1.DeletePropagationBackground
		if err := r.Delete(
			ctx, &job, &client.DeleteOptions{PropagationPolicy: &propagation},
		); err != nil && !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("deleting the dump Job: %w", err)
		}
	}

	return false, nil
}

// podsGone reports whether no pod of the Job is left in the live API. The
// Job controller labels its pods with the Job name; a pod still terminating
// may still be uploading.
func (r *LogicalBackupRDBMSReconciler) podsGone(ctx context.Context, job types.NamespacedName) (bool, error) {
	var pods corev1.PodList
	if err := r.APIReader.List(
		ctx, &pods, client.InNamespace(job.Namespace), client.MatchingLabels{jobNameLabel: job.Name},
	); err != nil {
		return false, fmt.Errorf("listing the pods of the dump Job: %w", err)
	}

	return len(pods.Items) == 0, nil
}

// deleteObject removes the dump from the pinned bucket — the one the backup
// wrote through, and only while it still points where it did at the start.
// It returns a non-empty reason when the object is left behind on purpose:
// the dependency chain is genuinely gone, or the contract now points
// somewhere else and a delete would hit a stranger's object at the same key.
// An error means the failure is transient and a retry can still clean up.
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
			// Broken credential data does not heal on retry; holding the
			// deletion on it would block forever.
			return fmt.Sprintf("the bucket credentials no longer parse: %v", err), nil
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
