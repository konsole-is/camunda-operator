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

package camundacluster

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
)

// The event reasons of the storage lifecycle.
const (
	// eventReasonStorageShrinkIgnored is the Warning event that the controller
	// records when the effective storageSize is below a bound broker claim.
	// It keeps the applied size, because volumes cannot be reduced in place.
	eventReasonStorageShrinkIgnored = "StorageShrinkIgnored"
	// eventReasonStatefulSetRecreated is recorded when the controller deletes
	// the broker StatefulSet with orphan propagation, so the volume claim
	// template can change. The pods and claims stay.
	eventReasonStatefulSetRecreated = "StatefulSetRecreated"
)

// errStatefulSetTerminating reports that the broker StatefulSet is being
// deleted for recreation. The reconcile stops without applying, because the
// apply of the new claim template onto the terminating object would fail; the
// owned-object delete event triggers the next reconcile.
var errStatefulSetTerminating = errors.New("broker StatefulSet is terminating")

// claimSizes are the sizes of the broker volumes: the capacities that the
// bound claims report, the storage they request, and the claim template size
// of the applied StatefulSet.
type claimSizes struct {
	capacities []resource.Quantity
	requests   []resource.Quantity
	applied    *resource.Quantity
}

// largest returns the largest of the capacities, the requests, and the
// applied template size, or nil when none exists. It is the size that a
// rendered claim must not go below: a claim that is still expanding requests
// more than it reports, and the applied template must not move backwards
// while no claim exists.
func (s claimSizes) largest() *resource.Quantity {
	largest := s.applied
	for _, sizes := range [][]resource.Quantity{s.capacities, s.requests} {
		for i := range sizes {
			if largest == nil || sizes[i].Cmp(*largest) > 0 {
				largest = &sizes[i]
			}
		}
	}
	return largest
}

// keepAppliedStorageSize guards against the shrinks that admission cannot see:
// a preset baseline lowered under a cluster, or an inline storageSize set
// below a size that a preset provided before. When the effective size is
// below the largest broker volume size (a bound claim's capacity or request,
// or the applied claim template), it keeps that size in the effective spec
// and records a Warning event, because volumes cannot be reduced in place. It
// returns the bound broker claims, so the caller lists them once.
func (r *CamundaClusterReconciler) keepAppliedStorageSize(
	ctx context.Context,
	cluster *v1.CamundaCluster,
	in *components.Input,
) ([]corev1.PersistentVolumeClaim, error) {
	claims, err := r.brokerClaims(ctx, cluster)
	if err != nil {
		return nil, err
	}

	var sizes claimSizes
	for _, claim := range claims {
		if capacity, ok := claim.Status.Capacity[corev1.ResourceStorage]; ok {
			sizes.capacities = append(sizes.capacities, capacity)
		}
		if request, ok := claim.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			sizes.requests = append(sizes.requests, request)
		}
	}

	sts, err := r.brokerStatefulSet(ctx, cluster)
	if err != nil {
		return nil, err
	}
	if sts != nil {
		sizes.applied = appliedClaimSize(sts)
	}

	largest := sizes.largest()
	rendered := in.Effective.StorageSize()
	if largest == nil || rendered.Cmp(*largest) >= 0 {
		return claims, nil
	}

	r.Recorder.Eventf(
		cluster,
		corev1.EventTypeWarning,
		eventReasonStorageShrinkIgnored,
		"storageSize %s is below the bound broker claim size %s; keeping %s, volumes cannot be reduced",
		&rendered,
		largest,
		largest,
	)
	if in.Effective.Zeebe == nil {
		in.Effective.Zeebe = &v1.ZeebeSpec{}
	}
	in.Effective.Zeebe.StorageSize = largest

	return claims, nil
}

// growBrokerClaims patches every claim of claims that requests less than size
// up to it. The storage class must allow expansion; the API server rejects
// the patch otherwise.
func (r *CamundaClusterReconciler) growBrokerClaims(
	ctx context.Context,
	claims []corev1.PersistentVolumeClaim,
	size resource.Quantity,
) error {
	for i := range claims {
		claim := &claims[i]
		requested, ok := claim.Spec.Resources.Requests[corev1.ResourceStorage]
		if ok && requested.Cmp(size) >= 0 {
			continue
		}

		patch := client.MergeFrom(claim.DeepCopy())
		claim.Spec.Resources.Requests[corev1.ResourceStorage] = size
		if err := r.Patch(ctx, claim, patch); err != nil {
			return fmt.Errorf("growing broker claim %q to %s: %w", claim.Name, &size, err)
		}
	}

	return nil
}

// recreateStatefulSetOnClaimChange deletes the broker StatefulSet with orphan
// propagation when the size of its applied volume claim template differs
// from the rendered one, so the component re-applies it and the new
// StatefulSet adopts the pods and claims. The template is immutable, so an
// update cannot change it. It records StatefulSetRecreated on the reconcile
// that issues the delete, and returns errStatefulSetTerminating from then on
// until the StatefulSet is gone.
func (r *CamundaClusterReconciler) recreateStatefulSetOnClaimChange(
	ctx context.Context,
	cluster *v1.CamundaCluster,
	in components.Input,
) error {
	sts, err := r.brokerStatefulSet(ctx, cluster)
	if err != nil || sts == nil {
		return err
	}
	if sts.DeletionTimestamp != nil {
		return errStatefulSetTerminating
	}

	applied := appliedClaimSize(sts)
	rendered := in.Effective.StorageSize()
	if applied == nil || applied.Cmp(rendered) == 0 {
		return nil
	}

	err = r.Delete(ctx, sts, client.PropagationPolicy(metav1.DeletePropagationOrphan))
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting StatefulSet %q for recreation: %w", sts.Name, err)
	}

	r.Recorder.Eventf(
		cluster,
		corev1.EventTypeNormal,
		eventReasonStatefulSetRecreated,
		"StatefulSet %s is recreated: the volume claim template changes from %s to %s; pods and claims stay",
		sts.Name,
		applied,
		&rendered,
	)

	return errStatefulSetTerminating
}

// brokerStatefulSet reads the applied broker StatefulSet, or nil when it does
// not exist.
func (r *CamundaClusterReconciler) brokerStatefulSet(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) (*appsv1.StatefulSet, error) {
	var sts appsv1.StatefulSet
	key := client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      components.WorkloadName(cluster, components.ComponentZeebe),
	}
	if err := r.Get(ctx, key, &sts); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading StatefulSet %q: %w", key, err)
	}

	return &sts, nil
}

// appliedClaimSize returns the storage request of the data volume claim
// template of sts, or nil when the template is absent.
func appliedClaimSize(sts *appsv1.StatefulSet) *resource.Quantity {
	for _, claim := range sts.Spec.VolumeClaimTemplates {
		if claim.Name != components.DataVolumeName {
			continue
		}
		if size, ok := claim.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			return &size
		}
	}

	return nil
}

// volumeStatus returns one status entry per claim, sorted by name.
func volumeStatus(claims []corev1.PersistentVolumeClaim) []v1.VolumeStatus {
	volumes := make([]v1.VolumeStatus, 0, len(claims))
	for _, claim := range claims {
		volumes = append(volumes, v1.VolumeStatus{
			Name:     claim.Name,
			Capacity: claim.Status.Capacity[corev1.ResourceStorage],
		})
	}
	slices.SortFunc(volumes, func(a, b v1.VolumeStatus) int { return strings.Compare(a.Name, b.Name) })
	return volumes
}

// brokerClaims lists the bound broker claims of cluster.
func (r *CamundaClusterReconciler) brokerClaims(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) ([]corev1.PersistentVolumeClaim, error) {
	var list corev1.PersistentVolumeClaimList
	if err := r.List(
		ctx, &list,
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels(components.BrokerClaimSelector(cluster)),
	); err != nil {
		return nil, fmt.Errorf("listing broker claims of %q: %w", cluster.Name, err)
	}

	var bound []corev1.PersistentVolumeClaim
	for _, claim := range list.Items {
		if claim.Status.Phase == corev1.ClaimBound {
			bound = append(bound, claim)
		}
	}

	return bound, nil
}
