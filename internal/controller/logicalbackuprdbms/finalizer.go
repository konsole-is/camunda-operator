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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	camundacluster "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	components "github.com/konsole-is/camunda-operator/pkg/components/logicalbackuprdbms"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
)

// finalize removes the artifacts of a deleted backup: the Job, the copied
// credentials, and the dump object — keyed strictly on this backup's own
// object key. Primary-storage backups are never touched: they belong to the
// continuous range that a point-in-time restore consumes. Cleanup is best
// effort: when the cluster or the bucket is gone, an event records what was
// left behind and the finalizer releases, so a backup never blocks deletion
// forever.
func (r *LogicalBackupRDBMSReconciler) finalize(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) error {
	if !controllerutil.ContainsFinalizer(backup, logicalbackup.Finalizer) {
		return nil
	}

	namespace := backup.Spec.ClusterRef.Namespace
	if namespace == "" {
		namespace = backup.Namespace
	}

	// A still-running Job is deleted first, so no upload races the object
	// deletion below. The name is deterministic, so a crash that never
	// recorded status.jobName still finds it.
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

	credentials := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name:      dbSecretName(backup),
		Namespace: namespace,
	}}
	if err := r.Delete(ctx, credentials); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting the credentials copy: %w", err)
	}

	if backup.Status.ObjectKey != "" {
		if err := r.deleteObject(ctx, backup, namespace); err != nil {
			r.EventRecorder.Eventf(
				backup,
				nil,
				corev1.EventTypeWarning,
				eventReasonCleanup,
				eventActionFinalize,
				"The dump object %q was not deleted: %v",
				backup.Status.ObjectKey,
				err,
			)
		}
	}

	controllerutil.RemoveFinalizer(backup, logicalbackup.Finalizer)

	return r.Update(ctx, backup)
}

// deleteObject removes the dump from the backup bucket, resolving the bucket
// through the cluster the way the backup itself did.
func (r *LogicalBackupRDBMSReconciler) deleteObject(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
	namespace string,
) error {
	var cluster v1.CamundaCluster
	if err := r.APIReader.Get(
		ctx,
		types.NamespacedName{Namespace: namespace, Name: backup.Spec.ClusterRef.Name},
		&cluster,
	); err != nil {
		return fmt.Errorf("reading the cluster: %w", err)
	}

	var bucket v1.ObjectStorageConfig
	if err := r.APIReader.Get(
		ctx, types.NamespacedName{Name: cluster.Spec.BackupStorageRef}, &bucket,
	); err != nil {
		return fmt.Errorf("reading the bucket contract: %w", err)
	}

	var creds *objectstore.Credentials
	if bucket.CredentialsSecret() != nil {
		// The local copy in the cluster namespace is what the Job consumed;
		// the finalizer reads the same one, so it works with exactly the
		// credentials the upload used.
		var secret corev1.Secret
		mirror := types.NamespacedName{
			Namespace: namespace,
			Name: camundacluster.MirroredSecretName(
				&cluster, camundacluster.MirrorPurposeBackupCredentials,
			),
		}
		if err := r.APIReader.Get(ctx, mirror, &secret); err != nil {
			return fmt.Errorf("reading the bucket credentials copy %s: %w", mirror, err)
		}
		parsed, err := objectstore.CredentialsFrom(&bucket, secret.Data)
		if err != nil {
			return err
		}
		creds = parsed
	}

	store, err := r.OpenBucket(ctx, &bucket, creds)
	if err != nil {
		return fmt.Errorf("opening the bucket: %w", err)
	}
	defer store.Close()

	if err := store.Delete(ctx, backup.Status.ObjectKey); err != nil {
		return fmt.Errorf("deleting %q: %w", backup.Status.ObjectKey, err)
	}

	return nil
}
