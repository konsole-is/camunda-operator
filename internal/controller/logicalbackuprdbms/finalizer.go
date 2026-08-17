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

// finalize removes the artifacts of a deleted backup: the Job and the dump
// object, keyed strictly on this backup's own object key. Primary-storage
// backups are never touched: they belong to the continuous range that a
// point-in-time restore consumes. Transient errors are returned, so the
// deletion retries; only when the dependency chain is genuinely gone — the
// cluster, the pinned bucket, or its credentials no longer exist — does the
// finalizer release with an event that records what was left behind, so a
// dead cluster never blocks deletion forever.
func (r *LogicalBackupRDBMSReconciler) finalize(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) error {
	if !controllerutil.ContainsFinalizer(backup, logicalbackup.Finalizer) {
		return nil
	}

	namespace := backup.Spec.ClusterRef.EffectiveClusterNamespace(backup.Namespace)

	// The Job goes first so it is never left running for a backup that is
	// going away. The delete is not awaited, so an upload already in flight
	// can still finish after the object deletion below and leave the object
	// behind; the window is small and the leftover is only ever this
	// backup's own key, never another backup's data. The name is
	// deterministic, so a crash that never recorded status.jobName still
	// finds it.
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name:      components.JobName(backup),
		Namespace: namespace,
	}}
	propagation := metav1.DeletePropagationBackground
	if err := r.Delete(
		ctx, job, &client.DeleteOptions{PropagationPolicy: &propagation},
	); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting the dump Job: %w", err)
	}

	if backup.Status.ObjectKey != "" {
		gone, err := r.deleteObject(ctx, backup, namespace)
		if err != nil {
			return err
		}
		if gone != "" {
			r.EventRecorder.Eventf(
				backup,
				nil,
				corev1.EventTypeWarning,
				eventReasonCleanup,
				eventActionFinalize,
				"The dump object %q was left behind: %s",
				backup.Status.ObjectKey,
				gone,
			)
		}
	}

	return r.releaseFinalizer(ctx, backup)
}

// deleteObject removes the dump from the pinned bucket — the one the backup
// wrote through, not whatever the cluster references today. It returns a
// non-empty reason when the dependency chain is genuinely gone and the object
// cannot be reached anymore, and an error when the failure is transient and
// a retry can still clean up.
func (r *LogicalBackupRDBMSReconciler) deleteObject(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
	namespace string,
) (string, error) {
	var cluster v1.CamundaCluster
	clusterKey := types.NamespacedName{Namespace: namespace, Name: backup.Spec.ClusterRef.Name}
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
	if bucket.Generation != backup.Status.BucketGeneration {
		// The contract changed under the object. The delete still runs against
		// the current spec — the old one is not retrievable — but the change is
		// worth a trace when the key then turns out not to be there.
		r.EventRecorder.Eventf(
			backup,
			nil,
			corev1.EventTypeWarning,
			eventReasonCleanup,
			eventActionFinalize,
			"ObjectStorageConfig %q changed since the backup wrote through it "+
				"(generation %d, was %d); the cleanup uses the current spec",
			bucket.Name,
			bucket.Generation,
			backup.Status.BucketGeneration,
		)
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
