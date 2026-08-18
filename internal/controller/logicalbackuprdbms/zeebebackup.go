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
	components "github.com/konsole-is/camunda-operator/pkg/components/logicalbackuprdbms"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/management"
)

// requestZeebeBackup is the step after the dump. It asks the cluster, through
// its management API, for one Zeebe backup, and polls it to completion. A
// Zeebe backup is Camunda's own backup of the Zeebe log and snapshots (its
// primary storage) to the backup bucket. The cluster generates the id. The
// backup records the id before it polls, so a re-entry polls and never
// requests a second one.
func (r *LogicalBackupRDBMSReconciler) requestZeebeBackup(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) (hold, error) {
	cluster, failure, err := r.runningCluster(ctx, backup)
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

	// The dump is durably recorded once this step runs. So its Job can go
	// now instead of at the end of the backup's life, together with the
	// scratch volume that a PVC-backed dump holds.
	if err := r.releaseJob(ctx, backup); err != nil {
		return settle, err
	}

	if backup.Status.ZeebeBackupID == nil {
		// The Zeebe backup goes to the current backup store of the cluster.
		// The pair is one restore point only if that store is still the
		// bucket that the dump was written to. It also requires that Zeebe
		// runs the spec that names the bucket, not a rollout still in
		// progress.
		_, failure, err := r.pinnedBucket(ctx, backup, cluster)
		if err != nil {
			return settle, err
		}
		if failure == nil {
			failure = clusterConverged(cluster)
		}
		if failure == nil {
			// The generation alone cannot tell a database swap. Mutable
			// referents enter the workload config hash without a bump of
			// the generation.
			failure, err = r.workloadUnchanged(ctx, backup, cluster)
			if err != nil {
				return settle, err
			}
		}
		if failure != nil {
			return r.holdRunning(backup, failure)
		}
	}

	admin, failure, err := management.NewClient(ctx, r.APIReader, cluster)
	if err != nil {
		return settle, err
	}
	if failure != nil {
		return r.holdRunning(backup, failure)
	}

	if backup.Status.ZeebeBackupID == nil {
		return r.startZeebeBackup(ctx, backup, cluster, admin)
	}

	return r.pollZeebeBackup(ctx, backup, cluster, admin)
}

// releaseJob deletes the dump Job once the backup recorded its result, and
// clears status.jobName so that the release runs once. The Job served its
// purpose. A Job that stays for the life of the backup holds storage for
// nothing, because its retained pod keeps a PVC-backed scratch volume.
// releaseJob does not release a failed Job. That Job stays for inspection
// until the backup is deleted. releaseJob deletes only the own Job of this
// backup. The live Job must carry the UID label of the backup, and the
// delete carries the UID of that Job as a precondition. So a same-named
// stranger that appears between the read and the delete survives. The
// conflict of the precondition, like a missing or foreign Job, only clears
// the name.
func (r *LogicalBackupRDBMSReconciler) releaseJob(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) error {
	if backup.Status.JobName == "" {
		return nil
	}
	key := types.NamespacedName{Namespace: backup.Namespace, Name: backup.Status.JobName}

	var job batchv1.Job
	err := r.APIReader.Get(ctx, key, &job)
	switch {
	case apierrors.IsNotFound(err):
		// The Job is gone already. There is nothing to release.
	case err != nil:
		return fmt.Errorf("reading the dump Job to release it: %w", err)
	case !components.JobBelongsTo(&job, backup):
		// A stranger took the name. It is not ours to delete.
	default:
		if err := r.Delete(
			ctx, &job,
			client.PropagationPolicy(metav1.DeletePropagationBackground),
			client.Preconditions{UID: &job.UID},
		); err != nil && !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
			return fmt.Errorf("releasing the dump Job: %w", err)
		}
	}
	backup.Status.JobName = ""

	return nil
}

// pinnedBucket checks that the backup store of the cluster is still the
// bucket that the dump was, or will be, written through, and returns it.
// Still the same bucket means the same contract that points at the same
// location. A retarget after admission sends the dump or the Zeebe backup
// somewhere else, and then the pair is not one restore point. The Job and
// the Zeebe request both check the bucket right before they act.
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
		// A conflict on the id that the cluster just generated means that a
		// higher id landed in between. The next request generates a fresh
		// one, so the controller retries with backoff. It never adopts the
		// backup that holds the id. The API answered, so the failure clock
		// of an earlier outage must not carry into the next unreachable
		// answer.
		r.recovered(backup)

		return settle, fmt.Errorf("requesting the Zeebe backup: %w", err)
	case err != nil:
		// Unreachable or rejected, for example a 503 through a gateway that
		// restarts, or a 401. Both run through the same bounded grace as any
		// other mid-run failure. The dump already succeeded, so one bad
		// answer must not discard it. But an API that never answers well
		// again must terminalize the backup, not park it without a bound.
		return r.holdRunning(backup, managementFailure(cluster, err))
	}
	r.recovered(backup)

	now := metav1.Now()
	backup.Status.ZeebeBackupID = &id
	backup.Status.ZeebeBackupRequestedAt = &now
	conditions.Stage(backup, progressing(backup, "the Zeebe backup runs"))

	// The generated id must be persisted before the poll. A crash here must
	// re-enter the poll, never request a second backup.
	return shortly, nil
}

// pollZeebeBackup reads the state of the recorded backup and terminalizes on
// a final answer. Right after the request, the partitions register their
// parts asynchronously. So a backup that the cluster does not report yet is
// normal within the registration grace, and fatal past it.
func (r *LogicalBackupRDBMSReconciler) pollZeebeBackup(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
	cluster *v1.CamundaCluster,
	admin *camundaadmin.Client,
) (hold, error) {
	status, err := admin.RuntimeBackupStatus(ctx, *backup.Status.ZeebeBackupID)
	if err != nil {
		// Unreachable or rejected alike. The mid-run grace bounds both.
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
// not answer, or answers with an error. It names the endpoint so that the
// terminal message points somewhere. Both cases share ConnectionFailed.
// From the side of the backup, the API is not usable, whichever way it
// fails.
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
