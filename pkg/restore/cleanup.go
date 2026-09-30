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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// CollectJobs removes the per-broker restore Jobs of a completed restore. It
// reports Outcome.Done when no recorded Job of the restore and no pod with
// the JobSelector labels of label is left, and only then are the broker
// volumes free. Until then it reports Outcome.Wait. Call it again on each
// look. A Job that another writer owns now counts as gone. label is the owner
// label of the restore.
//
// A restore that did not complete keeps its Jobs. Its Jobs hold the broker
// volumes until somebody deletes the restore, and CollectJobs reports Done at
// once.
//
// reader must be uncached. A stale read reports the volumes free too early.
func CollectJobs(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	owner client.Object,
	label labels.Owner,
	p *v1.RestoreProgress,
) (Outcome, error) {
	// The logs of a failed Job are the diagnosis.
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

		// A pod of the Job holds the broker volume. Under foreground propagation
		// the Job outlives its pods, so a deletion timestamp is not the end.
		collected = false

		if job.DeletionTimestamp != nil {
			continue
		}

		// Another writer can claim the name between the read and the delete.
		err = c.Delete(
			ctx,
			&job,
			client.PropagationPolicy(metav1.DeletePropagationForeground),
			client.Preconditions{UID: &job.UID},
		)
		// A Conflict means that another writer owns the name now. The next look
		// reads that Job and skips it.
		if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
			return Outcome{}, fmt.Errorf("removing the restore Job %s: %w", key, err)
		}
	}

	if !collected {
		return Outcome{Wait: Shortly}, nil
	}

	// A Job that somebody deleted with background propagation is gone before
	// its pods are.
	gone, err := podsGone(ctx, reader, owner, JobSelector(label))
	if err != nil || !gone {
		return Outcome{Wait: Shortly}, err
	}

	return Outcome{Done: true}, nil
}

// podsGone reports whether no pod in the namespace of owner matches selector.
func podsGone(
	ctx context.Context,
	reader client.Reader,
	owner client.Object,
	selector map[string]string,
) (bool, error) {
	var pods corev1.PodList
	err := reader.List(ctx, &pods, client.InNamespace(owner.GetNamespace()), client.MatchingLabels(selector))
	if err != nil {
		return false, fmt.Errorf("listing the Job pods of %s: %w", client.ObjectKeyFromObject(owner), err)
	}

	return len(pods.Items) == 0, nil
}
