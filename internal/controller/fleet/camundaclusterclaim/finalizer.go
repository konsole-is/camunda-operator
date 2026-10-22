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

package camundaclusterclaim

import (
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundaclusterclaim"
)

// The event of a delete call that the deletion of a claim sends, in the
// vocabulary of the ocf component deletion.
const (
	eventReasonResourceDeleted = "ResourceDeleted"
	eventActionDelete          = "Delete"
)

// finalize deletes what the claim generated, one step per pass: the
// attachments, then the CamundaCluster, then the PluginRequests. Before any
// step it writes the effective deletion policy on the requests, unless a
// selected registration does not support it. It returns the objects that the
// claim still waits for, as "<Kind> <name>", and removes the finalizer of the
// claim when nothing is left. An object that the claim does not control counts as
// gone. A claim without the finalizer returns nil and writes nothing.
func (r *Reconciler) finalize(ctx context.Context, claim *fleetv1alpha1.CamundaClusterClaim) ([]string, error) {
	if !controllerutil.ContainsFinalizer(claim, fleetv1alpha1.FinalizerCleanup) {
		return nil, nil
	}

	requests, err := r.ownedRequests(ctx, claim)
	if err != nil {
		return nil, err
	}

	// A plugin reads the policy when its request gets a deletion timestamp,
	// so the policy goes on before any delete of this pass.
	if err := r.writeDeletionPolicy(ctx, claim, requests); err != nil {
		return nil, err
	}

	attachments, err := r.ownedObjects(
		ctx,
		claim,
		&v1.CamundaOptimize{ObjectMeta: objectKey(claim, components.OptimizeName(claim.Name))},
		&v1.BackupSchedule{ObjectMeta: objectKey(claim, components.BackupScheduleName(claim.Name))},
	)
	if err != nil {
		return nil, err
	}
	cluster, err := r.ownedObjects(ctx, claim, &v1.CamundaCluster{ObjectMeta: objectKey(claim, claim.Name)})
	if err != nil {
		return nil, err
	}

	remaining := slices.Concat(attachments, cluster, requests)
	var step []client.Object
	switch {
	case len(attachments) > 0:
		step = attachments
	case len(cluster) > 0:
		step = cluster
	case len(requests) > 0:
		step = requests
	default:
		controllerutil.RemoveFinalizer(claim, fleetv1alpha1.FinalizerCleanup)
		if err := r.Update(ctx, claim); err != nil && !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("removing the finalizer: %w", err)
		}
		return nil, nil
	}

	if err := r.deleteAll(ctx, claim, step); err != nil {
		return nil, err
	}

	return r.describe(remaining)
}

// objectKey returns the metadata of the object name in the namespace of
// claim.
func objectKey(claim *fleetv1alpha1.CamundaClusterClaim, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: name, Namespace: claim.Namespace}
}

// ownedObjects reads each of objs by its name without the cache and returns
// the ones that exist and that claim controls.
func (r *Reconciler) ownedObjects(
	ctx context.Context,
	claim *fleetv1alpha1.CamundaClusterClaim,
	objs ...client.Object,
) ([]client.Object, error) {
	var owned []client.Object
	for _, obj := range objs {
		err := r.APIReader.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		switch {
		case apierrors.IsNotFound(err):
			continue
		case err != nil:
			return nil, fmt.Errorf("reading %T %s: %w", obj, obj.GetName(), err)
		}

		if metav1.IsControlledBy(obj, claim) {
			owned = append(owned, obj)
		}
	}

	return owned, nil
}

// deleteAll deletes each of objs that has no deletion timestamp yet, and
// records an event for each delete that it sends.
func (r *Reconciler) deleteAll(
	ctx context.Context,
	claim *fleetv1alpha1.CamundaClusterClaim,
	objs []client.Object,
) error {
	for _, obj := range objs {
		if !obj.GetDeletionTimestamp().IsZero() {
			continue
		}

		kind, err := r.kindOf(obj)
		if err != nil {
			return err
		}

		// The UID keeps the delete off an object that replaced this one
		// under its name since the read.
		err = r.Delete(ctx, obj, client.Preconditions{UID: new(obj.GetUID())})
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("deleting %s %s: %w", kind, obj.GetName(), err)
		}

		r.EventRecorder.Eventf(
			claim,
			obj,
			corev1.EventTypeNormal,
			eventReasonResourceDeleted,
			eventActionDelete,
			"Deleted %s %s",
			kind,
			obj.GetName(),
		)
	}

	return nil
}

// describe returns "<Kind> <name>" for each of objs.
func (r *Reconciler) describe(objs []client.Object) ([]string, error) {
	names := make([]string, 0, len(objs))
	for _, obj := range objs {
		kind, err := r.kindOf(obj)
		if err != nil {
			return nil, err
		}
		names = append(names, kind+" "+obj.GetName())
	}

	return names, nil
}

func (r *Reconciler) kindOf(obj client.Object) (string, error) {
	gvk, err := apiutil.GVKForObject(obj, r.Scheme)
	if err != nil {
		return "", fmt.Errorf("finding the kind of %T: %w", obj, err)
	}

	return gvk.Kind, nil
}
