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
	"fmt"

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

// claimSizes are the capacities that the bound broker claims report.
type claimSizes struct{ claims []resource.Quantity }

// smallest returns the smallest capacity, or nil without a bound claim. It is
// the size that every broker has, so status reports it.
func (s claimSizes) smallest() *resource.Quantity {
	if len(s.claims) == 0 {
		return nil
	}
	smallest := s.claims[0]
	for _, size := range s.claims[1:] {
		if size.Cmp(smallest) < 0 {
			smallest = size
		}
	}
	return &smallest
}

// largest returns the largest capacity, or nil without a bound claim. It is
// the size that a rendered claim must not go below.
func (s claimSizes) largest() *resource.Quantity {
	if len(s.claims) == 0 {
		return nil
	}
	largest := s.claims[0]
	for _, size := range s.claims[1:] {
		if size.Cmp(largest) > 0 {
			largest = size
		}
	}
	return &largest
}

// keepAppliedStorageSize guards against the shrinks that admission cannot see:
// a preset baseline lowered under a cluster, or an inline storageSize set
// below a size that a preset provided before. When the effective size is
// below the largest bound broker claim, it keeps that claim size in the
// effective spec and records a Warning event, because volumes cannot be
// reduced in place. It returns the sizes of the bound claims.
func (r *CamundaClusterReconciler) keepAppliedStorageSize(
	ctx context.Context,
	cluster *v1.CamundaCluster,
	in *components.Input,
) (claimSizes, error) {
	claims, err := r.brokerClaims(ctx, cluster)
	if err != nil {
		return claimSizes{}, err
	}

	var sizes claimSizes
	for _, claim := range claims {
		if capacity, ok := claim.Status.Capacity[corev1.ResourceStorage]; ok {
			sizes.claims = append(sizes.claims, capacity)
		}
	}

	largest := sizes.largest()
	rendered := in.Effective.StorageSize()
	if largest == nil || rendered.Cmp(*largest) >= 0 {
		return sizes, nil
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

	return sizes, nil
}

// growBrokerClaims patches every bound broker claim that requests less than
// size up to it. The storage class must allow expansion; the API server
// rejects the patch otherwise.
func (r *CamundaClusterReconciler) growBrokerClaims(
	ctx context.Context,
	cluster *v1.CamundaCluster,
	size resource.Quantity,
) error {
	claims, err := r.brokerClaims(ctx, cluster)
	if err != nil {
		return err
	}

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
// update cannot change it. It records StatefulSetRecreated. A StatefulSet
// that is already being deleted is left alone.
func (r *CamundaClusterReconciler) recreateStatefulSetOnClaimChange(
	ctx context.Context,
	cluster *v1.CamundaCluster,
	in components.Input,
) error {
	var sts appsv1.StatefulSet
	key := client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      components.WorkloadName(cluster, components.ComponentZeebe),
	}
	if err := r.Get(ctx, key, &sts); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("reading StatefulSet %q: %w", key, err)
	}

	applied := appliedClaimSize(&sts)
	rendered := in.Effective.StorageSize()
	if sts.DeletionTimestamp != nil || applied == nil || applied.Cmp(rendered) == 0 {
		return nil
	}

	err := r.Delete(ctx, &sts, client.PropagationPolicy(metav1.DeletePropagationOrphan))
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting StatefulSet %q for recreation: %w", key, err)
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

	return nil
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
