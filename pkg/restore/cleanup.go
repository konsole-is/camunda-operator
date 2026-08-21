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

// CollectJobs removes the per-broker restore Jobs of a restore that completed.
//
// A Job that completed keeps its pod. A pod that mounts a broker data volume
// counts as a user of it, under the kubernetes.io/pvc-protection finalizer.
// The volume therefore never terminates while the pod exists. The next
// operation that needs the volume then waits without end: a second restore of
// the cluster, or the deletion of the cluster itself.
//
// The recorded terminal reason decides. A completed restore gives its Jobs up,
// and a failed restore keeps them, because the logs of a failed Job are the
// diagnosis. A failed restore therefore holds the broker data volumes until
// somebody deletes the restore, which takes its Jobs with it.
//
// The delete carries foreground propagation. Background propagation returns
// before the pods are gone, and the pods are what hold the volume.
//
// The caller runs this from the terminal branch of its reconcile, before it
// gives the cluster claim back. The claim is what tells the next operation
// that the cluster is free, so the volumes are asked for first. An error keeps
// the claim for one more look, and the terminal branch runs again on the next
// one. The Jobs belong to the restore, so the delete itself wakes the
// controller again.
//
// It is safe to call again on every look. A Job that is gone, a Job that
// another writer owns now, and a Job that already terminates are all left as
// they are. A name that another writer takes between the read and the delete
// answers the same way, because the delete carries a UID precondition.
//
// The reader must be uncached. Only the controller reference proves that a Job
// under a recorded name belongs to this restore, and a stale read of that
// reference removes the Job of somebody else.
func CollectJobs(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	owner client.Object,
	p *v1.RestoreProgress,
) error {
	if p.TerminalReason != v1.ReasonCompleted {
		return nil
	}

	for _, name := range p.PrimaryJobNames {
		key := types.NamespacedName{Namespace: owner.GetNamespace(), Name: name}

		var job batchv1.Job
		err := reader.Get(ctx, key, &job)
		switch {
		case apierrors.IsNotFound(err):
			continue
		case err != nil:
			return fmt.Errorf("reading the restore Job %s: %w", key, err)
		}

		if !ownedBy(&job, owner) || job.DeletionTimestamp != nil {
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
		// is the same answer as a Job that is already gone.
		if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
			return fmt.Errorf("removing the restore Job %s: %w", key, err)
		}
	}

	return nil
}
