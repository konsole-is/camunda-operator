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

package restore

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	ocfjob "github.com/sourcehawk/operator-component-framework/pkg/primitives/job"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/podstate"
)

// The lifecycle events that every restore kind publishes on its own resource.
const (
	// EventReasonStarted marks the restore application starting for one
	// broker.
	EventReasonStarted = "RestoreStarted"
	// EventReasonCompleted marks the restore reaching Completed.
	EventReasonCompleted = "RestoreCompleted"
	// EventReasonFailed marks the restore reaching Failed.
	EventReasonFailed = "RestoreFailed"
	// EventActionRestore is the action verb of all three.
	EventActionRestore = "Restore"
)

// whatPrimary names the work of the restore Jobs in the message of a pod that
// cannot start.
const whatPrimary = "the restore Jobs"

// PrimaryInput is what the primary-storage phase renders and applies from.
type PrimaryInput struct {
	// Owner is the restore resource. Every Job gets a controller reference to
	// it, and the phase stages its Ready condition on it.
	Owner conditions.Owner
	// OwnerLabel is the owner label of the restore kind, from pkg/labels.
	OwnerLabel labels.Owner
	// Target is the live broker StatefulSet and the facts read off it.
	Target *Target
	// Size is the requested size of each recreated claim.
	Size resource.Quantity
	// FieldManager is the field manager of the calling restore kind.
	FieldManager client.FieldOwner
	// Recorder emits one event for each broker whose Job starts.
	Recorder events.EventRecorder
	// Args are the arguments of the restore application, for example
	// --backupId=42 or --to=2026-07-30T14:30:00Z.
	Args []string
	// Poll paces a running phase.
	Poll time.Duration
	// Grace bounds how long the phase waits on a dependency that stopped
	// resolving before the restore fails.
	Grace time.Duration
}

// Primary drives the whole primary-storage phase: it records the broker
// count, deletes and creates the broker data volumes, and runs the restore
// application once per broker. It reports Done when every Job completed, and
// a Failure that nothing resolves on its own when the restore cannot go on.
//
// It never writes status.phase. The caller persists the phase before it calls
// Primary the first time, because the phase is the resume marker of the
// destructive step.
//
// The order is the safety property. The broker count is durable before the
// first volume is deleted. The record of a deleted volume is durable before
// the first Job exists. A reconcile that re-enters therefore never deletes a
// volume that a Job already wrote to. Once the Jobs are recorded, the volumes
// are never touched again.
//
// The reader must be uncached. Every decision here reads live state: a stale
// claim or a stale Job lets a restore act twice on one volume.
func Primary(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	scheme *runtime.Scheme,
	p *v1.RestoreProgress,
	in PrimaryInput,
) (Outcome, error) {
	// Every entry point that renders from a Target rejects an incomplete one.
	// The check comes first here, because the phase reads the broker count off
	// the target before it renders anything. A nil target takes the manager
	// down inside a reconcile, and a target without brokers pins a count of
	// zero that no later look ever replaces.
	if err := in.Target.complete(); err != nil {
		return failure(fmt.Sprintf(
			"the restore cannot run the primary-storage phase: %s", err,
		)), nil
	}

	// The count is pinned at the first look. The restore recreates the volumes
	// of the brokers it read and runs a Job for each of them, and a cluster
	// that is scaled while it runs changes neither. The live count still
	// decides whether a Job can run at all, which runJobs checks.
	if p.Brokers == 0 {
		p.Brokers = in.Target.Brokers

		// Every volume this restore deletes is counted against the pinned
		// count, so the count is durable first. A restore that re-enters from
		// a count it never persisted reads the live one again. A cluster that
		// was scaled in between then loses volumes that this restore never
		// read.
		progressing(in.Owner, fmt.Sprintf(
			"the restore covers %d brokers. Their volumes are emptied next", p.Brokers,
		))

		return Outcome{Wait: Shortly}, nil
	}
	pinned := *in.Target
	pinned.Brokers = p.Brokers

	if len(p.PrimaryJobNames) == 0 {
		return recreateClaims(ctx, c, reader, p, in, &pinned), nil
	}

	return runJobs(ctx, c, reader, scheme, p, in, &pinned)
}

// recreateClaims gives every broker an empty data volume, because the restore
// application refuses a data directory that holds data. It records every
// volume whose old data is gone before it lets a Job start.
func recreateClaims(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	p *v1.RestoreProgress,
	in PrimaryInput,
	pinned *Target,
) Outcome {
	progress, err := RecreateClaims(ctx, c, reader, ClaimInput{
		Target:       pinned,
		Size:         in.Size,
		Recreated:    p.RecreatedClaims,
		FieldManager: in.FieldManager,
	})
	if err != nil {
		// A malformed target is a render failure, not a wait. Nothing changes
		// on its own, and the restore already left the volumes behind.
		return failure(fmt.Sprintf("the broker volumes cannot be recreated: %s", err))
	}

	Recovered(p)
	recorded := !slices.Equal(progress.Recreated, p.RecreatedClaims)
	p.RecreatedClaims = progress.Recreated

	// The record of a deleted volume must be durable before any Job runs. Its
	// flush is this reconcile, so the Jobs wait for the next look.
	if !progress.Done || recorded {
		progressing(in.Owner, progress.Message)
		if progress.Done {
			return Outcome{Wait: Shortly}
		}

		return Outcome{Wait: in.Poll}
	}

	// The names are derived from the restore and the ordinal, so recording
	// them before the first Job exists claims exactly the Jobs that the next
	// look applies. From here the volumes are never touched again.
	p.PrimaryJobNames = jobNames(in.OwnerLabel, pinned.Brokers)
	progressing(in.Owner, "the broker volumes are empty. The restore application starts")

	return Outcome{Wait: Shortly}
}

// jobNames returns the name of the restore Job of every broker, in broker
// order.
func jobNames(owner labels.Owner, brokers int32) []string {
	names := make([]string, 0, brokers)
	for ordinal := range brokers {
		names = append(names, JobName(owner, ordinal))
	}

	return names
}

// runJobs applies the restore Job of every broker that has none yet, and
// tracks the Jobs to a terminal outcome.
//
// The recorded names are the work of this restore. The live broker count can
// change while the restore runs. A Job derived from the new count writes to a
// volume that this restore never emptied, or it leaves a recorded broker
// without a Job.
func runJobs(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	scheme *runtime.Scheme,
	p *v1.RestoreProgress,
	in PrimaryInput,
	pinned *Target,
) (Outcome, error) {
	existing, outcome, err := readJobs(ctx, reader, p, in)
	if err != nil || outcome != nil {
		return outcomeOrZero(outcome), err
	}

	if outcome, err := applyMissingJobs(
		ctx, c, reader, scheme, p, in, pinned, existing,
	); err != nil || outcome != nil {
		return outcomeOrZero(outcome), err
	}

	return trackJobs(ctx, reader, p, in, existing)
}

// readJobs reads every recorded Job live and proves that it belongs to this
// restore. A Job that is absent comes back nil, and applyMissingJobs decides
// what that means.
//
// A restore that somebody deleted and created again under one name derives
// the names of its predecessor's Jobs. Tracking such a Job counts work that
// this restore never did, on volumes that it already emptied, so only the
// controller reference proves ownership.
func readJobs(
	ctx context.Context,
	reader client.Reader,
	p *v1.RestoreProgress,
	in PrimaryInput,
) ([]*batchv1.Job, *Outcome, error) {
	jobs := make([]*batchv1.Job, len(p.PrimaryJobNames))

	for index, name := range p.PrimaryJobNames {
		ordinal := int32(index)

		// The recorded name and the derived name are one truth. A restore that
		// polls for a Job whose name it never derives waits for ever, so a
		// mismatch ends it here instead.
		if derived := JobName(in.OwnerLabel, ordinal); derived != name {
			return nil, new(failure(fmt.Sprintf(
				"the restore recorded the Job %q for broker %d, but the Job of that broker is "+
					"named %q", name, ordinal, derived,
			))), nil
		}

		key := types.NamespacedName{Namespace: in.Owner.GetNamespace(), Name: name}

		var current batchv1.Job
		err := reader.Get(ctx, key, &current)
		switch {
		case apierrors.IsNotFound(err):
			continue
		case err != nil:
			return nil, nil, fmt.Errorf("reading the restore Job %s: %w", key, err)
		}
		if !ownedBy(&current, in.Owner) {
			return nil, new(foreignJob(key)), nil
		}
		jobs[index] = &current
	}

	return jobs, nil, nil
}

// applyMissingJobs creates the Jobs that do not exist yet, in broker order.
//
// The Jobs are created in that order, so the Jobs of a restore that is part
// way through applying them are a prefix of the recorded names. A gap in that
// prefix is therefore a Job that was created and then removed, and the
// restore fails: a second Job would run the restore application on a volume
// that the first one already wrote.
func applyMissingJobs(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	scheme *runtime.Scheme,
	p *v1.RestoreProgress,
	in PrimaryInput,
	pinned *Target,
	jobs []*batchv1.Job,
) (*Outcome, error) {
	for index, job := range jobs {
		if job != nil {
			continue
		}
		if slices.ContainsFunc(jobs[index+1:], func(later *batchv1.Job) bool { return later != nil }) {
			return new(failure(fmt.Sprintf(
				"the restore Job %s of broker %d was removed before it completed. The volume of that "+
					"broker holds what the Job wrote, so only a new restore can restore it again",
				p.PrimaryJobNames[index], index,
			))), nil
		}

		outcome, err := applyJob(ctx, c, reader, scheme, p, in, pinned, int32(index))
		if err != nil || outcome != nil {
			return outcome, err
		}
	}

	return nil, nil
}

// applyJob creates the restore Job of one broker as an identity claim:
// create-only, never a forced apply. The create carries the field manager of
// the calling kind, so every resource of a restore names the same owner of its
// fields.
//
// A forced apply after a NotFound is not atomic. Between the read and the
// apply, another writer can create a Job under this name, and the apply then
// overwrites its owner reference and its fields before anything looked at
// them. A Create is atomic, so the API server decides who owns the name. On
// AlreadyExists the winner is read and rejected unless this restore owns it.
// The pg_restore Job of a LogicalRestoreRDBMS claims its name the same way.
func applyJob(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	scheme *runtime.Scheme,
	p *v1.RestoreProgress,
	in PrimaryInput,
	pinned *Target,
	ordinal int32,
) (*Outcome, error) {
	// The cluster lost brokers while the restore ran. The restore cannot run
	// the work it recorded, and the render error underneath names only the
	// counts, so the real cause is reported here.
	if ordinal >= in.Target.Brokers {
		return new(failure(fmt.Sprintf(
			"the cluster was resized during the restore. It runs %d brokers now, and the restore "+
				"recorded %d. Create a new restore for the cluster as it is",
			in.Target.Brokers, len(p.PrimaryJobNames),
		))), nil
	}

	job, err := BuildJob(JobInput{
		Target:     pinned,
		Owner:      in.Owner,
		OwnerLabel: in.OwnerLabel,
		Ordinal:    ordinal,
		Args:       in.Args,
	})
	if err != nil {
		return new(failure(fmt.Sprintf(
			"the restore Job of broker %d cannot be rendered: %s", ordinal, err,
		))), nil
	}

	// Without the controller reference, deleting the restore leaves its Jobs
	// behind, and the next restore of the cluster finds pods that write to the
	// broker volumes.
	if err := controllerutil.SetControllerReference(in.Owner, job, scheme); err != nil {
		return nil, fmt.Errorf("owning the restore Job of broker %d: %w", ordinal, err)
	}

	key := types.NamespacedName{Namespace: job.Namespace, Name: job.Name}
	switch err := c.Create(ctx, job, in.FieldManager); {
	case apierrors.IsAlreadyExists(err):
		return claimedByAnother(ctx, reader, in, key)
	case err != nil:
		return nil, fmt.Errorf("creating the restore Job %s: %w", key, err)
	}

	in.Recorder.Eventf(
		in.Owner,
		nil,
		corev1.EventTypeNormal,
		EventReasonStarted,
		EventActionRestore,
		"The restore application runs for broker %d",
		ordinal,
	)

	return nil, nil
}

// claimedByAnother answers a Create that lost the name. The winner is read
// live: a Job that this restore owns is the one an earlier look created, and
// any other Job ends the restore. A name that is free again is claimed by the
// next look.
//
// The read goes through the uncached reader. A cache that still holds a Job of
// this restore under a name that another writer now owns reports the name as
// claimed by self, and the phase counts a foreign Job as its own.
func claimedByAnother(
	ctx context.Context,
	reader client.Reader,
	in PrimaryInput,
	key types.NamespacedName,
) (*Outcome, error) {
	var winner batchv1.Job
	if err := reader.Get(ctx, key, &winner); err != nil {
		if apierrors.IsNotFound(err) {
			return new(Outcome{Wait: Shortly}), nil
		}

		return nil, fmt.Errorf("reading the restore Job that won the name %s: %w", key, err)
	}
	if !ownedBy(&winner, in.Owner) {
		return new(foreignJob(key)), nil
	}

	return nil, nil
}

// trackJobs follows the Jobs that already existed at the start of this look to
// their end. The restore finishes when every one of them completed. One
// failing Job fails the restore and names its broker: the partitions of that
// broker are not restored, and a second attempt needs empty volumes again,
// which only a new restore arranges.
func trackJobs(
	ctx context.Context,
	reader client.Reader,
	p *v1.RestoreProgress,
	in PrimaryInput,
	jobs []*batchv1.Job,
) (Outcome, error) {
	done := 0
	for ordinal, job := range jobs {
		if job == nil {
			// The Job was applied in this pass. It reports nothing yet.
			continue
		}

		status, err := ocfjob.DefaultConvergingStatusHandler(concepts.ConvergingOperationNone, job)
		if err != nil {
			return Outcome{}, err
		}
		switch status.Status {
		case concepts.CompletionStatusCompleted:
			done++
		case concepts.CompletionStatusFailing:
			return failure(fmt.Sprintf(
				"the restore of broker %d failed: %s", ordinal, status.Reason,
			)), nil
		}
	}

	if done == len(p.PrimaryJobNames) {
		return Outcome{Done: true}, nil
	}

	// A pod that cannot start consumes no retry of its Job, so the Job stays
	// active and reports nothing. Without this look the restore waits without a
	// bound on a missing Secret or an image that does not pull.
	stuck, err := podstate.Stuck(
		ctx, reader, in.Owner.GetNamespace(), JobSelector(in.OwnerLabel), whatPrimary,
	)
	if err != nil {
		return Outcome{}, err
	}
	if stuck != nil {
		return HoldRunning(
			in.Owner, p, stuck, metav1.Now(), in.Grace, in.Poll,
		), nil
	}

	Recovered(p)
	progressing(in.Owner, fmt.Sprintf(
		"the restore application runs: %d of %d brokers restored", done, len(p.PrimaryJobNames),
	))

	// The watch on the owned Jobs carries the progress. The poll also looks at
	// the pods again, which no Job reports.
	return Outcome{Wait: in.Poll}, nil
}

// ownedBy reports whether job is a Job that this restore created. The UID
// answers it, not the name: a restore that somebody deleted and created again
// carries the same name and another UID.
func ownedBy(job *batchv1.Job, owner client.Object) bool {
	controller := metav1.GetControllerOf(job)

	return controller != nil && controller.UID == owner.GetUID()
}

// failure ends the restore with message. Every failure of the primary-storage
// phase reports the reason Failed: the phase runs behind every rule that
// carries a reason of its own.
func failure(message string) Outcome {
	return Outcome{Failure: &conditions.PreCheckFailure{
		Reason: v1.ReasonFailed, Message: message,
	}}
}

// foreignJob ends the restore because a Job under the name of one of its Jobs
// belongs to somebody else.
func foreignJob(key types.NamespacedName) Outcome {
	return failure(fmt.Sprintf(
		"Job %s exists, but no controller reference of this restore owns it. Remove the Job of the "+
			"earlier restore, then create a new restore", key,
	))
}

// progressing stages that the primary-storage phase runs.
func progressing(owner conditions.Owner, message string) {
	conditions.Stage(owner, conditions.Ready(
		metav1.ConditionFalse, v1.ReasonProgressing, message, owner.GetGeneration(),
	))
}

// outcomeOrZero unwraps an outcome that a step decided, or the zero outcome
// when it decided none. A caller that returns an error ignores it.
func outcomeOrZero(outcome *Outcome) Outcome {
	if outcome == nil {
		return Outcome{}
	}

	return *outcome
}
