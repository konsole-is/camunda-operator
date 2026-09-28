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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
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

// FinalizeHold removes the suspension hold of a deleted restore from its
// cluster, and then HoldFinalizer. cluster is the name of the target, which
// lives in the namespace of the restore. The suspension that the restore
// applied through spec.suspend stays.
func FinalizeHold(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	owner client.Object,
	cluster string,
) error {
	if !controllerutil.ContainsFinalizer(owner, HoldFinalizer) {
		return nil
	}

	key := types.NamespacedName{Namespace: owner.GetNamespace(), Name: cluster}
	if err := releaseHold(ctx, c, reader, owner, key); err != nil {
		return err
	}

	base, ok := owner.DeepCopyObject().(client.Object)
	if !ok {
		return fmt.Errorf("copying %s: not a client.Object", client.ObjectKeyFromObject(owner))
	}
	controllerutil.RemoveFinalizer(owner, HoldFinalizer)
	patch := client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{})
	if err := c.Patch(ctx, owner, patch); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("removing the finalizer from %s: %w", client.ObjectKeyFromObject(owner), err)
	}

	return nil
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
