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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// HoldFinalizer keeps a deleted restore until it has removed its suspension
// hold from its cluster.
const HoldFinalizer = "core.camunda.io/suspension-hold"

// AddHoldFinalizer adds HoldFinalizer to the restore. The caller runs it on
// every look of a restore that is not being deleted, before Prepare can put
// a hold on the cluster.
func AddHoldFinalizer(ctx context.Context, c client.Client, owner client.Object) error {
	if controllerutil.ContainsFinalizer(owner, HoldFinalizer) {
		return nil
	}

	base, ok := owner.DeepCopyObject().(client.Object)
	if !ok {
		return fmt.Errorf("copying %s: not a client.Object", client.ObjectKeyFromObject(owner))
	}
	controllerutil.AddFinalizer(owner, HoldFinalizer)
	patch := client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{})
	if err := c.Patch(ctx, owner, patch); err != nil {
		return fmt.Errorf("adding the finalizer to %s: %w", client.ObjectKeyFromObject(owner), err)
	}

	return nil
}

// FinalizeHold removes the Jobs, the writer Leases, the suspension hold and
// HoldFinalizer of a deleted restore, in that order, so the backend stays held
// until no Job pod of the restore writes it. Outcome.Done reports that the
// finalizer is gone. claimNamespace holds the writer Leases. label is the
// owner label that the Job pods of the restore carry. cluster is the name of
// the target, which lives in the namespace of the restore. The suspension that
// the restore applied through spec.suspend stays. The reader must be uncached.
// A caller whose restore still writes the backend in another way calls it only
// after that work has stopped.
func FinalizeHold(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	claimNamespace string,
	owner client.Object,
	label labels.Owner,
	cluster string,
) (Outcome, error) {
	if !controllerutil.ContainsFinalizer(owner, HoldFinalizer) {
		return Outcome{Done: true}, nil
	}

	// The garbage collector removes the Jobs only after the restore is gone,
	// and a Job pod still writes the storage of the target until then.
	removed, err := removeJobs(ctx, c, reader, owner, label)
	if err != nil {
		return Outcome{}, err
	}
	if !removed {
		return Outcome{Wait: Shortly}, nil
	}

	if err := ReleaseWriters(ctx, c, reader, claimNamespace, owner); err != nil {
		return Outcome{}, err
	}

	key := types.NamespacedName{Namespace: owner.GetNamespace(), Name: cluster}
	if err := releaseHold(ctx, c, reader, owner, key); err != nil {
		return Outcome{}, err
	}

	base, ok := owner.DeepCopyObject().(client.Object)
	if !ok {
		return Outcome{}, fmt.Errorf("copying %s: not a client.Object", client.ObjectKeyFromObject(owner))
	}
	controllerutil.RemoveFinalizer(owner, HoldFinalizer)
	patch := client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{})
	if err := c.Patch(ctx, owner, patch); err != nil && !apierrors.IsNotFound(err) {
		return Outcome{}, fmt.Errorf("removing the finalizer from %s: %w", client.ObjectKeyFromObject(owner), err)
	}

	return Outcome{Done: true}, nil
}

// removeJobs reports whether no Job and no Job pod of the restore is left.
func removeJobs(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	owner client.Object,
	label labels.Owner,
) (bool, error) {
	var jobs batchv1.JobList
	if err := reader.List(ctx, &jobs, client.InNamespace(owner.GetNamespace())); err != nil {
		return false, fmt.Errorf("listing the Jobs of %s: %w", client.ObjectKeyFromObject(owner), err)
	}

	removed := true
	for i := range jobs.Items {
		job := &jobs.Items[i]
		if !ownedBy(job, owner) {
			continue
		}

		// Under foreground propagation the Job outlives its pods.
		removed = false
		if job.DeletionTimestamp != nil {
			continue
		}

		err := c.Delete(
			ctx,
			job,
			client.PropagationPolicy(metav1.DeletePropagationForeground),
			client.Preconditions{UID: &job.UID},
		)
		if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
			return false, fmt.Errorf("removing the Job %s: %w", client.ObjectKeyFromObject(job), err)
		}
	}

	if !removed {
		return false, nil
	}

	// A Job that somebody deleted with background propagation is gone before
	// its pods are.
	var pods corev1.PodList
	err := reader.List(
		ctx,
		&pods,
		client.InNamespace(owner.GetNamespace()),
		client.MatchingLabels{label.Key: label.Name},
	)
	if err != nil {
		return false, fmt.Errorf("listing the Job pods of %s: %w", client.ObjectKeyFromObject(owner), err)
	}

	return len(pods.Items) == 0, nil
}

// releaseHold removes the suspension hold of the restore from its cluster. A
// cluster that is gone, or that carries no hold of the restore, needs nothing.
func releaseHold(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	owner client.Object,
	cluster types.NamespacedName,
) error {
	var existing v1.CamundaCluster
	if err := reader.Get(ctx, cluster, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("reading CamundaCluster %s: %w", cluster, err)
	}
	if _, ok := existing.Annotations[holdKey(owner)]; !ok {
		return nil
	}

	patch := targetPatch(cluster, existing.UID, v1.CamundaClusterSpec{})
	if err := Apply(ctx, c, patch, holdManager(owner)); err != nil && !apierrors.IsConflict(err) {
		return fmt.Errorf("removing the suspension hold from CamundaCluster %s: %w", cluster, err)
	}

	return nil
}

// holdTarget puts the suspension hold of the restore on its cluster, unless
// the cluster carries it already.
func holdTarget(
	ctx context.Context,
	c client.Client,
	owner conditions.Owner,
	cluster *v1.CamundaCluster,
) error {
	key := holdKey(owner)
	if _, ok := cluster.Annotations[key]; ok {
		return nil
	}

	patch := targetPatch(client.ObjectKeyFromObject(cluster), cluster.UID, v1.CamundaClusterSpec{})
	patch.Annotations = map[string]string{
		key: fmt.Sprintf("%s %s/%s restores into this cluster", owner.GetKind(), owner.GetNamespace(), owner.GetName()),
	}
	if err := Apply(ctx, c, patch, holdManager(owner)); err != nil {
		return fmt.Errorf(
			"putting the suspension hold on CamundaCluster %s: %w",
			client.ObjectKeyFromObject(cluster),
			err,
		)
	}

	return nil
}

// holdKey is the suspension hold annotation key of one restore.
func holdKey(owner client.Object) string {
	return v1.SuspensionHoldPrefix + string(owner.GetUID())
}

// holdManager is the field manager of the hold of one restore. Each restore
// applies under its own, so its apply never removes the hold of another.
func holdManager(owner client.Object) client.FieldOwner {
	return client.FieldOwner("camunda-operator/suspension-hold-" + string(owner.GetUID()))
}
