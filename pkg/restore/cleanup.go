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

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// CollectJobs removes the per-broker restore Jobs of a restore that completed,
// and reports whether they are gone.
//
// The recorded terminal reason decides what happens. A completed restore gives
// its Jobs up, and a failed restore keeps them, because the logs of a failed
// Job are the diagnosis. A failed restore therefore holds the broker data
// volumes until somebody deletes the restore, which takes its Jobs with it.
//
// Outcome.Done reports that no recorded Job is left, and only then are the
// broker volumes free. Outcome.Wait paces the look that follows.
//
// It is safe to call again on every look, and the answer converges. A Job that
// is gone, a Job that another writer owns now, and a restore that recorded no
// Job at all are all complete.
//
// The reader must be uncached. Only the controller reference proves that a Job
// under a recorded name belongs to this restore, and a stale read of that
// reference removes the Job of somebody else. A cached read also reports a Job
// that is already gone, so it reports the volumes free too early.
func CollectJobs(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	owner client.Object,
	p *v1.RestoreProgress,
) (Outcome, error) {
	if p.TerminalReason != v1.ReasonCompleted {
		return Outcome{Done: true}, nil
	}

	collected := true

	for _, name := range p.PrimaryJobNames {
		key := types.NamespacedName{Namespace: owner.GetNamespace(), Name: name}

		var job batchv1.Job
		err := reader.Get(ctx, key, &job)
		switch {
		case apierrors.IsNotFound(err):
			continue
		case err != nil:
			return Outcome{}, fmt.Errorf("reading the restore Job %s: %w", key, err)
		}

		if !ownedBy(&job, owner) {
			continue
		}

		// The Job of this restore is still here, so its pods can still be here,
		// and a pod is what holds the broker volume. Under foreground
		// propagation the Job outlives its pods by design, so a deletion
		// timestamp says the delete was asked for, never that it finished.
		collected = false

		if job.DeletionTimestamp != nil {
			continue
		}

		// The precondition closes the gap between the read and the delete.
		// Another writer can claim the name in between, and its Job then
		// carries another UID.
		err = c.Delete(
			ctx,
			&job,
			client.PropagationPolicy(metav1.DeletePropagationForeground),
			client.Preconditions{UID: &job.UID},
		)
		// A Conflict says that the precondition did not hold, so another writer
		// owns the name now. That Job is not this restore's to remove, and it
		// is the same answer as a Job that is already gone. The next look reads
		// the winner and reports the name complete.
		if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
			return Outcome{}, fmt.Errorf("removing the restore Job %s: %w", key, err)
		}
	}

	if !collected {
		return Outcome{Wait: Shortly}, nil
	}

	return Outcome{Done: true}, nil
}
