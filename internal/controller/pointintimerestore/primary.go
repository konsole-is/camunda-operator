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

package pointintimerestore

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	ocfjob "github.com/sourcehawk/operator-component-framework/pkg/primitives/job"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/podstate"
	"github.com/konsole-is/camunda-operator/pkg/restore"
)

// errUnrecoverable reports a failure of the primary storage phase that
// nothing resolves on its own: the shared package cannot render a resource of
// this restore, or a Job under the name of this restore belongs to somebody
// else. The restore already emptied the broker volumes, so it must reach a
// terminal phase instead of waiting.
var errUnrecoverable = errors.New("the restore cannot continue")

// restorePrimaryStorage gives the brokers empty data volumes and runs the
// Camunda restore application on them, once per broker. It is the destructive
// phase, and every check that protects it already passed.
//
// The order is the safety property. status.recreatedClaims is durable before
// the first Job exists, so a reconcile that re-enters never deletes a volume
// that a Job already wrote to. Once the Jobs are recorded, the volumes are
// never touched again.
func (r *Reconciler) restorePrimaryStorage(
	ctx context.Context,
	pitr *v1.PointInTimeRestore,
) (hold, error) {
	resolved, failure, err := r.resolve(ctx, pitr)
	if err != nil {
		return r.resolveFailed(pitr, err)
	}
	if failure != nil {
		return r.holdStarted(pitr, failure), nil
	}

	// A chain that resolves is not progress. Only the work of the phase clears
	// the clock of the grace: a volume that came back empty, or a Job whose
	// pods run.
	// The restore acts on the brokers it pinned at its first look, not on the
	// brokers the cluster runs now. status.brokers, status.recreatedClaims,
	// and status.primaryJobNames are then one truth, whatever the cluster does
	// while the restore runs. The live count still decides whether a Job can
	// run at all, which ensureJob checks.
	pinned := *resolved.target
	if pitr.Status.Brokers > 0 {
		pinned.Brokers = pitr.Status.Brokers
	}

	if len(pitr.Status.PrimaryJobNames) == 0 {
		return r.recreateClaims(ctx, pitr, &pinned)
	}

	return r.runJobs(ctx, pitr, &pinned, resolved.target.Brokers)
}

// recreateClaims deletes the data volume of every broker and creates it again
// as an empty volume, because the restore application refuses a data directory
// that holds data. It records every volume whose old data is gone before it
// lets a Job start.
//
// The recreated volumes carry no owner reference to the restore. The
// StatefulSet owns them, and an owner reference here deletes a live broker
// volume as soon as somebody deletes the restore resource.
func (r *Reconciler) recreateClaims(
	ctx context.Context,
	pitr *v1.PointInTimeRestore,
	target *restore.Target,
) (hold, error) {
	progress, err := restore.RecreateClaims(ctx, r.Client, r.APIReader, restore.ClaimInput{
		Target: target,
		// A point-in-time restore reads no backup resource, so no recorded
		// size exists. The claim template of the StatefulSet is the size the
		// cluster runs with.
		Size:         target.ClaimSize(nil),
		Recreated:    pitr.Status.RecreatedClaims,
		FieldManager: restore.FieldManagerPointInTimeRestore,
	})
	if err != nil {
		// A malformed target is a render failure, not a wait. Nothing changes
		// on its own, and the restore already left the volumes behind.
		r.fail(pitr, fmt.Sprintf("the broker volumes cannot be recreated: %s", err))

		return settle, nil
	}

	recovered(pitr)
	recorded := !slices.Equal(progress.Recreated, pitr.Status.RecreatedClaims)
	pitr.Status.RecreatedClaims = progress.Recreated

	// The record of a deleted volume must be durable before any Job runs. Its
	// flush is this reconcile, so the Jobs wait for the next look.
	if !progress.Done || recorded {
		r.progressing(pitr, progress.Message)
		if progress.Done {
			return shortly, nil
		}

		return hold{after: r.opts.PollInterval}, nil
	}

	// The names are derived from the restore and the ordinal, so recording
	// them before the first Job exists claims exactly the Jobs that the next
	// look applies. From here the volumes are never touched again.
	pitr.Status.PrimaryJobNames = jobNames(pitr, target.Brokers)
	r.progressing(pitr, "the broker volumes are empty. The restore application starts")

	return shortly, nil
}

// jobNames returns the name of the restore Job of every broker, in broker
// order.
func jobNames(pitr *v1.PointInTimeRestore, brokers int32) []string {
	names := make([]string, 0, brokers)
	for ordinal := range brokers {
		names = append(names, restore.JobName(labels.PointInTimeRestore(pitr.Name), ordinal))
	}

	return names
}

// runJobs applies the restore Job of every broker that has none yet, and
// tracks the Jobs to a terminal outcome. The restore application reads the
// exporter position of the restored database itself and restores the newest
// checkpoint at or before the requested point.
func (r *Reconciler) runJobs(
	ctx context.Context,
	pitr *v1.PointInTimeRestore,
	target *restore.Target,
	liveBrokers int32,
) (hold, error) {
	// The recorded names are the work of this restore. The live broker count
	// can change while the restore runs. A Job derived from the new count
	// writes to a volume that this restore never emptied, or it leaves a
	// recorded broker without a Job.
	done := 0
	for index, name := range pitr.Status.PrimaryJobNames {
		ordinal := int32(index)
		job, err := r.ensureJob(ctx, pitr, target, ordinal, name, liveBrokers)
		if errors.Is(err, errUnrecoverable) {
			r.fail(pitr, err.Error())

			return settle, nil
		}
		if err != nil {
			return settle, err
		}
		if job == nil {
			// The Job was applied in this pass. It reports nothing yet.
			continue
		}

		status, err := ocfjob.DefaultConvergingStatusHandler(concepts.ConvergingOperationNone, job)
		if err != nil {
			return settle, err
		}
		switch status.Status {
		case concepts.CompletionStatusCompleted:
			done++
		case concepts.CompletionStatusFailing:
			r.fail(pitr, fmt.Sprintf(
				"the restore of broker %d failed: %s", ordinal, status.Reason,
			))

			return settle, nil
		}
	}

	if done == len(pitr.Status.PrimaryJobNames) {
		r.complete(pitr)

		return settle, nil
	}

	// A pod that cannot start consumes no retry of its Job, so the Job stays
	// active and reports nothing. Without this look the restore waits without a
	// bound on a missing Secret or an image that does not pull.
	stuck, err := podstate.Stuck(
		ctx,
		r.APIReader,
		pitr.Namespace,
		restore.JobSelector(labels.PointInTimeRestore(pitr.Name)),
		"the restore Jobs",
	)
	if err != nil {
		return settle, err
	}
	if stuck != nil {
		return r.holdStarted(pitr, stuck), nil
	}
	recovered(pitr)

	r.progressing(pitr, fmt.Sprintf(
		"the restore application runs: %d of %d brokers restored",
		done, len(pitr.Status.PrimaryJobNames),
	))

	// The watch on the owned Jobs carries the progress. The poll also looks at
	// the pods again, which no Job reports.
	return hold{after: r.opts.PollInterval}, nil
}

// ensureJob returns the restore Job of one broker, and applies it when none
// exists yet. It returns a nil Job when it just applied one: a fresh Job
// reports nothing that the caller can track.
//
// The read is live and comes first, because spec.template of a Job is
// immutable. A second apply of a Job that the API server already stamped is a
// validation error, not an adoption.
func (r *Reconciler) ensureJob(
	ctx context.Context,
	pitr *v1.PointInTimeRestore,
	target *restore.Target,
	ordinal int32,
	name string,
	liveBrokers int32,
) (*batchv1.Job, error) {
	owner := labels.PointInTimeRestore(pitr.Name)

	// The recorded name and the derived name are one truth. A restore that
	// polls for a Job whose name it never derives waits for ever, so a
	// mismatch ends it here instead.
	if derived := restore.JobName(owner, ordinal); derived != name {
		return nil, fmt.Errorf(
			"%w: the restore recorded the Job %q for broker %d, but the Job of that broker is named %q",
			errUnrecoverable, name, ordinal, derived,
		)
	}

	key := types.NamespacedName{Namespace: pitr.Namespace, Name: name}

	var current batchv1.Job
	err := r.APIReader.Get(ctx, key, &current)
	switch {
	case err == nil:
		// The name of a Job comes from the name of the restore and the
		// ordinal, so a restore that somebody deleted and created again under
		// one name finds the Jobs of its predecessor. Tracking such a Job
		// counts work that this restore never did, on volumes that it already
		// emptied. Only the controller reference proves that the Job is this
		// restore's own.
		if !ownedBy(&current, pitr) {
			return nil, fmt.Errorf(
				"%w: Job %s exists, but no controller reference of this restore owns it. "+
					"Remove the Job of the earlier restore, then create a new restore",
				errUnrecoverable, key,
			)
		}

		return &current, nil
	case !apierrors.IsNotFound(err):
		return nil, fmt.Errorf("reading the restore Job %s: %w", key, err)
	}

	// The cluster lost brokers while the restore ran. The restore cannot run
	// the work it recorded, and the render error underneath names only the
	// counts, so the real cause is reported here.
	if ordinal >= liveBrokers {
		return nil, fmt.Errorf(
			"%w: the cluster was resized during the restore. It runs %d brokers now, and the restore "+
				"recorded %d. Create a new restore for the cluster as it is",
			errUnrecoverable, liveBrokers, len(pitr.Status.PrimaryJobNames),
		)
	}

	job, err := restore.BuildJob(restore.JobInput{
		Target:     target,
		Owner:      pitr,
		OwnerLabel: owner,
		Ordinal:    ordinal,
		// The restore application takes the point as an argument. It finds the
		// newest checkpoint at or before it, and it refuses a point that lies
		// before the state of the restored database.
		Args: []string{"--to=" + pitr.Spec.Timestamp.UTC().Format(time.RFC3339)},
	})
	if err != nil {
		return nil, fmt.Errorf(
			"%w: the restore Job of broker %d cannot be rendered: %s", errUnrecoverable, ordinal, err,
		)
	}

	// Without the controller reference, deleting the restore leaves its Jobs
	// behind, and the next restore of the cluster finds pods that write to the
	// broker volumes.
	if err := controllerutil.SetControllerReference(pitr, job, r.Scheme); err != nil {
		return nil, fmt.Errorf("owning the restore Job %s: %w", key, err)
	}

	if err := restore.Apply(ctx, r.Client, job, restore.FieldManagerPointInTimeRestore); err != nil {
		return nil, fmt.Errorf("applying the restore Job %s: %w", key, err)
	}
	r.EventRecorder.Eventf(
		pitr,
		nil,
		corev1.EventTypeNormal,
		eventReasonStarted,
		eventActionRestore,
		"The restore application runs for broker %d",
		ordinal,
	)

	return nil, nil
}

// ownedBy reports whether job is a Job that this restore created. The UID
// answers it, not the name: a restore that somebody deleted and created again
// carries the same name and another UID.
func ownedBy(job *batchv1.Job, pitr *v1.PointInTimeRestore) bool {
	controller := metav1.GetControllerOf(job)

	return controller != nil && controller.UID == pitr.UID
}
