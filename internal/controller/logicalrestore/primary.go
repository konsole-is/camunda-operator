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

package logicalrestore

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	ocfjob "github.com/sourcehawk/operator-component-framework/pkg/primitives/job"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/podstate"
	restorepkg "github.com/konsole-is/camunda-operator/pkg/restore"
)

// whatPrimary names the work of the restore Jobs in the messages of a pod
// that cannot start.
const whatPrimary = "the restore Jobs"

// restorePrimaryStorage gives the brokers empty data volumes and runs the
// Camunda restore application on them, once per broker.
//
// The order is the safety property. The volumes are deleted and created again
// first, and the record of that reaches status before any Job starts writing.
// Once the Jobs exist, the volumes are never touched again: a second delete
// would erase what the Jobs restored.
func (r *Reconciler) restorePrimaryStorage(
	ctx context.Context,
	restore *v1.LogicalRestore,
) (hold, error) {
	resolved, failure, err := r.resolveTarget(ctx, restore)
	if err != nil {
		return settle, err
	}
	if failure != nil {
		return r.holdRunning(restore, failure), nil
	}
	recovered(restore)

	// The broker count is recorded before anything is deleted, so the status
	// says how many volumes and Jobs this restore covers even if it fails
	// halfway.
	restore.Status.Brokers = resolved.target.Brokers

	if len(restore.Status.PrimaryJobNames) == 0 {
		return r.recreateClaims(ctx, restore, resolved)
	}

	return r.trackRestoreJobs(ctx, restore, resolved.target.Brokers)
}

// recreateClaims deletes every broker data claim that still holds data and
// creates it again empty, because the restore application refuses a data
// directory that is not empty.
//
// The record of a deleted claim reaches status before the Jobs run. Until
// that write lands, a reconcile that re-enters deletes the empty volume again
// and creates it again, which costs a pass and loses nothing. After a Job
// started writing, the same pass would erase restored data.
func (r *Reconciler) recreateClaims(
	ctx context.Context,
	restore *v1.LogicalRestore,
	resolved *resolution,
) (hold, error) {
	progress, err := restorepkg.RecreateClaims(ctx, r.Client, r.APIReader, restorepkg.ClaimInput{
		Target:       resolved.target,
		Size:         resolved.target.ClaimSize(resolved.backup.ZeebeSize),
		Recreated:    restore.Status.RecreatedClaims,
		FieldManager: restorepkg.FieldManagerLogicalRestore,
	})
	if err != nil {
		return settle, err
	}

	recorded := slices.Equal(restore.Status.RecreatedClaims, progress.Recreated)
	restore.Status.RecreatedClaims = progress.Recreated

	if !progress.Done {
		conditions.Stage(restore, progressing(restore, progress.Message))

		return hold{after: r.opts.PollInterval}, nil
	}

	if !recorded {
		conditions.Stage(restore, progressing(
			restore, "the broker volumes are empty and the restore application starts",
		))

		return shortly, nil
	}

	return r.applyRestoreJobs(ctx, restore, resolved)
}

// applyRestoreJobs applies one Job per broker and records their names. A Job
// that already exists is left alone: its pod template is immutable, and the
// Job is the one this restore created under that name.
func (r *Reconciler) applyRestoreJobs(
	ctx context.Context,
	restore *v1.LogicalRestore,
	resolved *resolution,
) (hold, error) {
	owner := labels.LogicalRestore(restore.Name)
	names := make([]string, 0, resolved.target.Brokers)

	for ordinal := range resolved.target.Brokers {
		name := restorepkg.JobName(owner, ordinal)
		names = append(names, name)

		key := types.NamespacedName{Namespace: restore.Namespace, Name: name}

		var current batchv1.Job
		err := r.APIReader.Get(ctx, key, &current)
		if err == nil {
			continue
		}
		if !apierrors.IsNotFound(err) {
			return settle, fmt.Errorf("reading the restore Job %s: %w", key, err)
		}

		job, err := restorepkg.BuildJob(restorepkg.JobInput{
			Target:     resolved.target,
			Owner:      restore,
			OwnerLabel: owner,
			Ordinal:    ordinal,
			Args:       restoreArgs(restore),
		})
		if err != nil {
			// The builder answers a target or an owner that it cannot render
			// from. Every input here comes from ReadTarget and from the
			// restore itself, so no retry can change the answer.
			r.fail(restore, v1.ReasonFailed, err.Error())

			return settle, nil
		}
		// Without the controller reference, deleting the restore leaves its
		// Jobs behind.
		if err := controllerutil.SetControllerReference(restore, job, r.Scheme); err != nil {
			return settle, fmt.Errorf("owning the restore Job of broker %d: %w", ordinal, err)
		}
		if err := restorepkg.Apply(ctx, r.Client, job, restorepkg.FieldManagerLogicalRestore); err != nil {
			return settle, fmt.Errorf("applying the restore Job of broker %d: %w", ordinal, err)
		}
	}

	restore.Status.PrimaryJobNames = names
	conditions.Stage(restore, progressing(
		restore, "the restore application runs on every broker",
	))

	return hold{after: r.opts.PollInterval}, nil
}

// restoreArgs are the arguments of the restore application. The Elasticsearch
// path names the backup id, because the id keys the snapshots of the set. The
// relational path names nothing: the application reads the exporter position
// from the restored database and picks the backups itself (Camunda 8.9 "Restore
// a backup (RDBMS)", default restore).
func restoreArgs(restore *v1.LogicalRestore) []string {
	if restore.Status.StorageType != v1.SecondaryStorageTypeElasticsearch {
		return nil
	}

	return []string{"--backupId=" + strconv.FormatInt(restore.Status.BackupID, 10)}
}

// trackRestoreJobs follows the recorded Jobs to their end. The restore
// finishes when every Job completed. One failing Job fails the restore and
// names its broker: the partitions of that broker are not restored, and a
// second attempt needs empty volumes again, which only a new restore resource
// arranges.
func (r *Reconciler) trackRestoreJobs(
	ctx context.Context,
	restore *v1.LogicalRestore,
	brokers int32,
) (hold, error) {
	completed := 0

	for ordinal, name := range restore.Status.PrimaryJobNames {
		job, gone, err := r.restoreJob(ctx, restore.Namespace, name)
		if err != nil {
			return settle, err
		}
		if gone {
			r.fail(restore, v1.ReasonFailed, fmt.Sprintf(
				"the restore Job %s of broker %d disappeared before it completed", name, ordinal,
			))

			return settle, nil
		}

		status, err := ocfjob.DefaultConvergingStatusHandler(concepts.ConvergingOperationNone, job)
		if err != nil {
			return settle, err
		}

		switch status.Status {
		case concepts.CompletionStatusCompleted:
			completed++
		case concepts.CompletionStatusFailing:
			r.fail(restore, v1.ReasonFailed, fmt.Sprintf(
				"the restore Job of broker %d failed: %s", ordinal, status.Reason,
			))

			return settle, nil
		}
	}

	if completed == int(brokers) {
		r.complete(restore)

		return settle, nil
	}

	stuck, err := podstate.Stuck(
		ctx,
		r.APIReader,
		restore.Namespace,
		restorepkg.JobSelector(labels.LogicalRestore(restore.Name)),
		whatPrimary,
	)
	if err != nil {
		return settle, err
	}
	if stuck != nil {
		return r.holdRunning(restore, stuck), nil
	}
	recovered(restore)

	conditions.Stage(restore, progressing(restore, fmt.Sprintf(
		"the restore application finished on %d of %d brokers", completed, brokers,
	)))

	// The watch on the owned Jobs wakes the restore on progress. The poll also
	// re-checks the pods, because a Job does not report their waiting states.
	return hold{after: r.opts.PollInterval}, nil
}

// restoreJob reads one Job of the restore. gone reports that the live API has
// no such Job: somebody deleted it, and the restore cannot trust that the
// broker was restored. The cache has no read-your-writes guarantee, so only
// the live view may declare a Job gone.
func (r *Reconciler) restoreJob(
	ctx context.Context,
	namespace, name string,
) (job *batchv1.Job, gone bool, err error) {
	key := types.NamespacedName{Namespace: namespace, Name: name}

	var cached batchv1.Job
	err = r.Get(ctx, key, &cached)
	if err == nil {
		return &cached, false, nil
	}
	if !apierrors.IsNotFound(err) {
		return nil, false, fmt.Errorf("reading the restore Job %s: %w", key, err)
	}

	var live batchv1.Job
	err = r.APIReader.Get(ctx, key, &live)
	switch {
	case err == nil:
		return &live, false, nil
	case apierrors.IsNotFound(err):
		return nil, true, nil
	default:
		return nil, false, fmt.Errorf("reading the restore Job %s: %w", key, err)
	}
}
