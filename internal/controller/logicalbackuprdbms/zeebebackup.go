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
	"time"

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/management"
)

// requestZeebeBackup is the step after the dump: it asks the cluster, through
// its management API, for one Zeebe backup — Camunda's own backup of the
// Zeebe log and snapshots (its "primary storage") to the backup bucket — and
// polls it to completion. The cluster generates the id; the backup records
// it before ever polling, so a re-entry polls and never requests a second
// one.
func (r *LogicalBackupRDBMSReconciler) requestZeebeBackup(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) (hold, error) {
	var cluster v1.CamundaCluster
	if err := r.APIReader.Get(
		ctx,
		types.NamespacedName{Namespace: backup.Namespace, Name: backup.Spec.ClusterRef.Name},
		&cluster,
	); err != nil {
		if apierrors.IsNotFound(err) {
			return r.holdRunning(backup, logicalbackup.InvalidReference(
				"CamundaCluster %s/%s does not exist", backup.Namespace, backup.Spec.ClusterRef.Name,
			))
		}

		return settle, err
	}

	// The dump is durably recorded once this step runs, so its Job — and
	// the scratch volume a PVC-backed dump holds — can go now instead of at
	// the end of the backup's life.
	if err := r.releaseJob(ctx, backup); err != nil {
		return settle, err
	}

	if backup.Status.ZeebeBackupID == nil {
		// The Zeebe backup goes to the cluster's current backup store; the
		// pair is one restore point only if that is still the bucket the
		// dump was written to — and only if Zeebe runs the spec that names
		// it, not a rollout still in progress.
		_, failure, err := r.pinnedBucket(ctx, backup, &cluster)
		if err != nil {
			return settle, err
		}
		if failure == nil {
			failure = clusterConverged(&cluster)
		}
		if failure != nil {
			return r.holdRunning(backup, failure)
		}
	}

	admin, failure, err := management.NewClient(ctx, r.APIReader, &cluster)
	if err != nil {
		return settle, err
	}
	if failure != nil {
		return r.holdRunning(backup, failure)
	}

	if backup.Status.ZeebeBackupID == nil {
		return r.startZeebeBackup(ctx, backup, &cluster, admin)
	}

	return r.pollZeebeBackup(ctx, backup, &cluster, admin)
}

// releaseJob deletes the dump Job once the backup has recorded its result,
// and clears status.jobName so the release runs once. The Job served its
// purpose; keeping it — and its retained pod with a PVC-backed scratch
// volume — for the life of the backup would hold storage for nothing. A
// failed Job is not released: it stays for inspection until the backup is
// deleted.
func (r *LogicalBackupRDBMSReconciler) releaseJob(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) error {
	if backup.Status.JobName == "" {
		return nil
	}
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: backup.Status.JobName, Namespace: backup.Namespace,
	}}
	propagation := metav1.DeletePropagationBackground
	if err := r.Delete(
		ctx, job, &client.DeleteOptions{PropagationPolicy: &propagation},
	); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("releasing the dump Job: %w", err)
	}
	backup.Status.JobName = ""

	return nil
}

// pinnedBucket verifies that the cluster's backup store is still the bucket
// the dump was, or will be, written through — the same contract, pointing at
// the same location — and returns it. A retarget after admission would send
// the dump or the Zeebe backup somewhere else, and the pair would not be one
// restore point; the Job and the Zeebe request both check it right before
// they act.
func (r *LogicalBackupRDBMSReconciler) pinnedBucket(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
	cluster *v1.CamundaCluster,
) (*v1.ObjectStorageConfig, *conditions.PreCheckFailure, error) {
	if cluster.Spec.BackupStorageRef != backup.Status.BucketRef {
		return nil, logicalbackup.InvalidReference(
			"CamundaCluster %s/%s now backs up through ObjectStorageConfig %q, but the backup was "+
				"pinned to %q; a dump or a Zeebe backup taken now would land in a different bucket",
			cluster.Namespace, cluster.Name, cluster.Spec.BackupStorageRef, backup.Status.BucketRef,
		), nil
	}

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
	if location := bucket.Location(); location != backup.Status.BucketLocation {
		return nil, logicalbackup.InvalidReference(
			"ObjectStorageConfig %q now points at %s, but the backup was pinned to %s; a dump or a "+
				"Zeebe backup taken now would land in a different bucket",
			bucket.Name, location, backup.Status.BucketLocation,
		), nil
	}

	return &bucket, nil, nil
}

// startZeebeBackup requests the backup without an id and records the one the
// cluster generated.
func (r *LogicalBackupRDBMSReconciler) startZeebeBackup(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
	cluster *v1.CamundaCluster,
	admin *camundaadmin.Client,
) (hold, error) {
	id, err := admin.StartRuntimeBackup(ctx, nil)
	switch {
	case errors.Is(err, camundaadmin.ErrConflict):
		// A conflict on the id the cluster just generated means a higher id
		// landed in between; the next request generates a fresh one, so this
		// is retried with backoff — and never resolved by adopting the backup
		// that holds the id.
		return settle, fmt.Errorf("requesting the Zeebe backup: %w", err)
	case err != nil:
		// Unreachable or rejected (a 503 through a restarting gateway, a
		// 401): both run through the same bounded grace as any other mid-run
		// failure. The dump already succeeded, so one bad answer must not
		// discard it — but an API that never answers well again must
		// terminalize the backup, not park it forever.
		return r.holdRunning(backup, managementFailure(cluster, err))
	}
	r.recovered(backup)

	now := metav1.Now()
	backup.Status.ZeebeBackupID = &id
	backup.Status.ZeebeBackupRequestedAt = &now
	conditions.Stage(backup, progressing(backup, "the Zeebe backup runs"))

	// Persist the generated id before polling it: a crash here must re-enter
	// polling, never request a second backup.
	return shortly, nil
}

// pollZeebeBackup reads the state of the recorded backup and terminalizes on
// a final answer. Right after the request the partitions register their
// parts asynchronously, so a backup the cluster does not report yet is
// normal within the registration grace — and fatal past it.
func (r *LogicalBackupRDBMSReconciler) pollZeebeBackup(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
	cluster *v1.CamundaCluster,
	admin *camundaadmin.Client,
) (hold, error) {
	status, err := admin.RuntimeBackupStatus(ctx, *backup.Status.ZeebeBackupID)
	if err != nil {
		// Unreachable or rejected alike: bounded by the mid-run grace.
		return r.holdRunning(backup, managementFailure(cluster, err))
	}
	r.recovered(backup)

	switch status.State {
	case camundaadmin.StateCompleted:
		r.complete(backup)

		return settle, nil
	case camundaadmin.StateInProgress:
		conditions.Stage(backup, progressing(backup, "the Zeebe backup runs"))

		return hold{after: r.opts.RetryInterval}, nil
	case camundaadmin.StateDoesNotExist, camundaadmin.StateIncomplete:
		if requested := backup.Status.ZeebeBackupRequestedAt; requested != nil &&
			time.Since(requested.Time) < r.opts.RegistrationGrace {
			conditions.Stage(backup, progressing(
				backup, "the Zeebe backup is registering its partitions",
			))

			return hold{after: r.opts.RetryInterval}, nil
		}
	}

	r.fail(backup, fmt.Sprintf(
		"Zeebe backup %d reports %s: %s",
		*backup.Status.ZeebeBackupID, status.State, status.FailureReason,
	))

	return settle, nil
}

// managementFailure is the mid-run failure of a management API that does
// not answer, or answers with an error, naming the endpoint so the terminal
// message points somewhere. Both share ConnectionFailed: from the backup's
// side the API is not usable, whichever way it fails.
func managementFailure(cluster *v1.CamundaCluster, err error) *conditions.PreCheckFailure {
	verb := "rejected the call"
	if errors.Is(err, camundaadmin.ErrUnreachable) {
		verb = "is not reachable"
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonConnectionFailed,
		Message: fmt.Sprintf(
			"the management API at %s %s: %v", cluster.Status.Management.Endpoint, verb, err,
		),
	}
}
