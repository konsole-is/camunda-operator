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
	"slices"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
)

// eventReasonStorageShrinkIgnored is the Warning event that the controller
// records once per requested size when the effective storageSize is below a
// bound broker claim. The claims keep their size, because volumes cannot be
// reduced in place.
const eventReasonStorageShrinkIgnored = "StorageShrinkIgnored"

// eventActionResize is the action of the events that the controller records
// about the size of the broker claims.
const eventActionResize = "Resize"

// brokerStorage is what the storage lifecycle reads before the components are
// built: the applied broker StatefulSet, or nil, and the bound broker claims.
type brokerStorage struct {
	statefulSet *appsv1.StatefulSet
	claims      []corev1.PersistentVolumeClaim
}

// readBrokerStorage reads the applied broker StatefulSet without the cache,
// so the requested size annotation is the one the last apply wrote, and lists
// the bound broker claims of cluster.
func (r *CamundaClusterReconciler) readBrokerStorage(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) (brokerStorage, error) {
	var storage brokerStorage

	key := client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      components.WorkloadName(cluster, components.ComponentZeebe),
	}
	var sts appsv1.StatefulSet
	switch err := r.APIReader.Get(ctx, key, &sts); {
	case err == nil:
		storage.statefulSet = &sts
	case !apierrors.IsNotFound(err):
		return brokerStorage{}, fmt.Errorf("reading StatefulSet %q: %w", key, err)
	}

	// The cache serves the claims while the StatefulSet exists: there they
	// only grow and report status. Without one they are the downgrade
	// baseline of a cluster recreated on retained volumes, and a stale
	// cache can miss the newest stamp, so they are read live.
	reader := client.Reader(r.Client)
	if storage.statefulSet == nil {
		reader = r.APIReader
	}

	var list corev1.PersistentVolumeClaimList
	if err := reader.List(
		ctx, &list,
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels(components.BrokerClaimSelector(cluster)),
	); err != nil {
		return brokerStorage{}, fmt.Errorf("listing broker claims of %q: %w", cluster.Name, err)
	}
	for _, claim := range list.Items {
		if claim.Status.Phase == corev1.ClaimBound {
			storage.claims = append(storage.claims, claim)
		}
	}

	return storage, nil
}

// volumeClaimSize returns the size that the rendered claim template must
// carry: the size of the applied template, because a StatefulSet cannot
// change its claim template, or nil before the first apply, so the renderer
// uses the effective size.
func (s brokerStorage) volumeClaimSize() *resource.Quantity {
	if s.statefulSet == nil {
		return nil
	}

	for _, claim := range s.statefulSet.Spec.VolumeClaimTemplates {
		if claim.Name != components.DataVolumeName {
			continue
		}
		if size, ok := claim.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			return &size
		}
	}

	return nil
}

// largestClaimSize returns the largest capacity or request of the bound
// claims, or nil without a bound claim. A claim that is still expanding
// requests more than it reports.
func (s brokerStorage) largestClaimSize() *resource.Quantity {
	var largest *resource.Quantity
	for i := range s.claims {
		claim := &s.claims[i]
		for _, size := range []resource.Quantity{
			claim.Status.Capacity[corev1.ResourceStorage],
			claim.Spec.Resources.Requests[corev1.ResourceStorage],
		} {
			if largest == nil || size.Cmp(*largest) > 0 {
				largest = &size
			}
		}
	}
	return largest
}

// requestedSizeApplied reports whether the applied StatefulSet already
// carries the requested storage size annotation for size. It is false before
// the first apply and after every change of the effective size, so an event
// that depends on it fires once per requested size.
func (s brokerStorage) requestedSizeApplied(size resource.Quantity) bool {
	if s.statefulSet == nil {
		return false
	}

	applied, err := resource.ParseQuantity(s.statefulSet.Annotations[components.RequestedStorageSizeAnnotation])
	return err == nil && applied.Cmp(size) == 0
}

// volumes returns one status entry per bound claim that reports a capacity,
// sorted by name.
func (s brokerStorage) volumes() []v1.VolumeStatus {
	volumes := make([]v1.VolumeStatus, 0, len(s.claims))
	for _, claim := range s.claims {
		capacity, ok := claim.Status.Capacity[corev1.ResourceStorage]
		if !ok {
			continue
		}
		volumes = append(volumes, v1.VolumeStatus{Name: claim.Name, Capacity: capacity})
	}
	slices.SortFunc(volumes, func(a, b v1.VolumeStatus) int { return strings.Compare(a.Name, b.Name) })
	return volumes
}

// runningVersion returns the version that the next broker start runs,
// whatever spec.version says: the version on the applied StatefulSet, or,
// without one, the version stamped on the bound claims, which a cluster
// recreated on retained volumes carries. The empty string means that
// nothing constrains the version.
func (s brokerStorage) runningVersion() string {
	if version := s.appliedVersion(); version != "" {
		return version
	}

	return components.RetainedVersion(s.claims)
}

// appliedVersion returns the tag of the broker image on the applied
// StatefulSet, or the empty string before the first apply or when the
// container carries no tag.
func (s brokerStorage) appliedVersion() string {
	if s.statefulSet == nil {
		return ""
	}

	for _, container := range s.statefulSet.Spec.Template.Spec.Containers {
		if container.Name == components.ContainerCamunda {
			return components.ImageTag(container.Image)
		}
	}

	return ""
}

// stampBrokerVersion patches the broker version annotation onto every bound
// broker claim that does not carry the version of the applied StatefulSet.
// The stamp survives a delete of the cluster with retained volumes, so the
// downgrade rule holds for the cluster that is recreated on them.
func (r *CamundaClusterReconciler) stampBrokerVersion(ctx context.Context, storage brokerStorage) error {
	version := storage.appliedVersion()
	if version == "" {
		return nil
	}

	for i := range storage.claims {
		claim := &storage.claims[i]
		if claim.Annotations[components.BrokerVersionAnnotation] == version {
			continue
		}

		patch := client.MergeFrom(claim.DeepCopy())
		if claim.Annotations == nil {
			claim.Annotations = map[string]string{}
		}
		claim.Annotations[components.BrokerVersionAnnotation] = version
		if err := r.Patch(ctx, claim, patch); err != nil {
			return fmt.Errorf("stamping the broker version on claim %q: %w", claim.Name, err)
		}
	}

	return nil
}

// growBrokerClaims patches every bound broker claim that requests less than
// size up to it. A claim of a new replica is grown once it binds, because
// the claim watch triggers a reconcile. The storage class must allow
// expansion; the API server rejects the patch otherwise.
func (r *CamundaClusterReconciler) growBrokerClaims(
	ctx context.Context,
	storage brokerStorage,
	size resource.Quantity,
) error {
	for i := range storage.claims {
		claim := &storage.claims[i]
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

// recordIgnoredShrink records StorageShrinkIgnored when size is below the
// largest bound broker claim, once per requested size: the event fires until
// the StatefulSet carries the requested size annotation, which the apply of
// this reconcile writes. Admission rejects an inline shrink, so only a
// preset-driven decrease reaches this point.
func (r *CamundaClusterReconciler) recordIgnoredShrink(
	cluster *v1.CamundaCluster,
	storage brokerStorage,
	size resource.Quantity,
) {
	largest := storage.largestClaimSize()
	if largest == nil || size.Cmp(*largest) >= 0 || storage.requestedSizeApplied(size) {
		return
	}

	r.EventRecorder.Eventf(
		cluster,
		nil,
		corev1.EventTypeWarning,
		eventReasonStorageShrinkIgnored,
		eventActionResize,
		"requested storageSize %s is below the bound broker claim size %s; the claims keep %s, volumes cannot be reduced",
		&size,
		largest,
		largest,
	)
}
