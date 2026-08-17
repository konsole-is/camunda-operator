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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

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
	namespace := backup.Spec.ClusterRef.EffectiveClusterNamespace(backup.Namespace)

	var cluster v1.CamundaCluster
	if err := r.APIReader.Get(
		ctx,
		types.NamespacedName{Namespace: namespace, Name: backup.Spec.ClusterRef.Name},
		&cluster,
	); err != nil {
		if apierrors.IsNotFound(err) {
			return r.holdRunning(backup, logicalbackup.InvalidReference(
				"CamundaCluster %s/%s does not exist", namespace, backup.Spec.ClusterRef.Name,
			))
		}

		return settle, err
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
