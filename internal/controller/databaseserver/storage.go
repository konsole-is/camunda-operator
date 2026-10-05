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

package databaseserver

import (
	"context"
	"fmt"
	"slices"
	"strings"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
)

// eventReasonStorageShrinkIgnored is the Warning event that the controller
// records when a merged volume size is below the size that is already there
// and the applied cluster does not carry that request yet, except while the
// server is held for suspension. It keeps the size that is there, because
// PostgreSQL volumes cannot be reduced in place.
const eventReasonStorageShrinkIgnored = "StorageShrinkIgnored"

// eventReasonWALStorageKept is the Warning event that the controller records
// when a merged spec asks for no write-ahead log volume under a server that
// has one, on the same terms as eventReasonStorageShrinkIgnored. It keeps the
// volume, because CloudNativePG refuses a cluster that gives one up.
const eventReasonWALStorageKept = "WALStorageKept"

// eventActionResize is the action of the events that the controller records
// about the size of the volumes.
const eventActionResize = "Resize"

// serverVolumes are the volumes of the current cluster: one entry per
// PersistentVolumeClaim that reports a capacity, split by what the claim
// holds, and the sizes the applied CloudNativePG cluster asks for. The applied
// sizes matter on their own while the server is suspended, when the claims are
// there and report their capacity before any instance comes back. requested
// holds the annotations of the applied cluster.
type serverVolumes struct {
	data        []v1.VolumeStatus
	wal         []v1.VolumeStatus
	appliedData *resource.Quantity
	appliedWAL  *resource.Quantity
	requested   map[string]string
}

// all returns every claim of the cluster, sorted by name.
func (v serverVolumes) all() []v1.VolumeStatus {
	volumes := append(append([]v1.VolumeStatus{}, v.data...), v.wal...)
	slices.SortFunc(volumes, func(a, b v1.VolumeStatus) int { return strings.Compare(a.Name, b.Name) })

	return volumes
}

// requestApplied reports whether the applied cluster carries size in the
// annotation key. The empty value stands for no size.
func (v serverVolumes) requestApplied(key string, size *resource.Quantity) bool {
	value, ok := v.requested[key]
	if !ok {
		return false
	}
	if size == nil {
		return value == ""
	}

	applied, err := resource.ParseQuantity(value)
	return err == nil && applied.Cmp(*size) == 0
}

// volumeClaims reads the volumes of the current cluster. CloudNativePG labels
// every claim of a cluster with the cluster name, and it names the write-ahead
// log claim of an instance after the data claim of that instance, with the
// suffix -wal.
//
// It returns nothing at all for a cluster of that name this server does not
// own. status.Volumes and the clamp of keepAppliedStorageSize are the two
// readers, and neither has anything to say about the volumes of somebody
// else's database.
func (r *DatabaseServerReconciler) volumeClaims(
	ctx context.Context,
	server *v1.DatabaseServer,
) (serverVolumes, error) {
	key := types.NamespacedName{Namespace: server.Namespace, Name: components.ClusterName(server)}

	// Read live: a cache that missed the last apply holds an old request,
	// and the clamp reports the same kept volume again.
	var cluster cnpgv1.Cluster
	applied := true
	if err := r.APIReader.Get(ctx, key, &cluster); err != nil {
		if !apierrors.IsNotFound(err) {
			return serverVolumes{}, fmt.Errorf("reading the applied cluster %s: %w", key, err)
		}

		applied = false
	}

	// The name is derived, so a cluster under it can hold the database of
	// somebody else. Its claims carry the label this list selects by, so both
	// the claims and the sizes it applies belong to that database, and the
	// clamp would render this server on them.
	//
	// A name with no cluster is read as before. The claims of the cluster that
	// was there outlive it under a retain policy, and a server that must not
	// come back smaller than them is what the clamp exists for.
	if applied && !ownedByServer(server, &cluster) {
		return serverVolumes{}, nil
	}

	var claims corev1.PersistentVolumeClaimList
	if err := r.List(
		ctx, &claims,
		client.InNamespace(server.Namespace),
		client.MatchingLabels{components.CNPGClusterNameLabel: key.Name},
	); err != nil {
		return serverVolumes{}, fmt.Errorf("listing the volume claims of %q: %w", key.Name, err)
	}

	var volumes serverVolumes
	for i := range claims.Items {
		claim := &claims.Items[i]
		capacity, ok := claim.Status.Capacity[corev1.ResourceStorage]
		if !ok {
			continue
		}
		status := v1.VolumeStatus{Name: claim.Name, Capacity: capacity}
		if strings.HasSuffix(claim.Name, cnpgv1.WalArchiveVolumeSuffix) {
			volumes.wal = append(volumes.wal, status)
			continue
		}
		volumes.data = append(volumes.data, status)
	}

	if !applied {
		return volumes, nil
	}

	volumes.appliedData = parsedSize(cluster.Spec.StorageConfiguration.Size)
	volumes.requested = cluster.Annotations
	if cluster.Spec.WalStorage != nil {
		volumes.appliedWAL = parsedSize(cluster.Spec.WalStorage.Size)
	}

	return volumes, nil
}

// parsedSize reads one storage size of the applied cluster, or nil when the
// field is empty or holds something that is not a quantity. Nothing of this
// operator writes such a value, and a cluster that carries one is answered by
// leaving the size out of the comparison rather than by failing the reconcile.
func parsedSize(size string) *resource.Quantity {
	quantity, err := resource.ParseQuantity(size)
	if err != nil {
		return nil
	}

	return &quantity
}

// keepAppliedStorageSize raises each merged volume size back to the size that
// is already there, and keeps a write-ahead log volume that the merged spec no
// longer asks for.
//
// It guards against the edits that admission cannot see: a preset baseline
// lowered or cleared under a server, or an inline size set below a size that a
// preset provided before. The CEL transition rules of storageSize and
// walStorageSize bind the spec of the DatabaseServer, and none of those edits
// touches it.
// CloudNativePG refuses a cluster whose storage is smaller than the one it
// applied, so a size that reaches it stops the server from converging.
//
// It sets resolved.requested to the sizes that the merged spec asked for. It
// records a Warning event only for a request that the applied cluster does not
// carry yet.
func (r *DatabaseServerReconciler) keepAppliedStorageSize(
	server *v1.DatabaseServer,
	resolved *resolvedSpec,
	volumes serverVolumes,
) {
	merged := &resolved.merged
	requested := components.RequestedStorage{Data: merged.StorageSize, WAL: merged.WALStorageSize}
	resolved.requested = requested

	// A held server applies no cluster to carry the request, so the event
	// waits for the apply after the hold ends.
	reported := func(key string, size *resource.Quantity) bool {
		return resolved.holdForSuspension || volumes.requestApplied(key, size)
	}

	merged.StorageSize = r.keepAppliedSize(
		server, "storageSize", requested.Data, largestVolume(volumes.data, volumes.appliedData),
		reported(components.RequestedStorageSizeAnnotation, requested.Data),
	)
	merged.WALStorageSize = r.keepAppliedWALSize(
		server, requested.WAL, largestVolume(volumes.wal, volumes.appliedWAL),
		reported(components.RequestedWALStorageSizeAnnotation, requested.WAL),
	)
}

// keepAppliedSize returns the size to render for one volume of the server:
// requested, or existing when requested is below it. It records the Warning
// event when it keeps existing, unless reported is true.
func (r *DatabaseServerReconciler) keepAppliedSize(
	server *v1.DatabaseServer,
	field string,
	requested, existing *resource.Quantity,
	reported bool,
) *resource.Quantity {
	if requested == nil || existing == nil || requested.Cmp(*existing) >= 0 {
		return requested
	}
	if reported {
		return existing
	}

	r.EventRecorder.Eventf(
		server,
		nil,
		corev1.EventTypeWarning,
		eventReasonStorageShrinkIgnored,
		eventActionResize,
		"%s %s is below the existing volume size %s. Keeping %s, because PostgreSQL volumes "+
			"cannot be reduced in place",
		field,
		requested,
		existing,
		existing,
	)

	return existing
}

// keepAppliedWALSize returns the size to render for the write-ahead log volume
// of the server: the clamp of keepAppliedSize, and the size that is there when
// the merged spec asks for no such volume at all. reported applies to both
// events as it does in keepAppliedSize.
//
// CloudNativePG refuses a cluster that gives up the write-ahead log volume it
// applied, with "walStorage cannot be disabled once configured". It accepts
// one that adds it, so the field itself stays free to set.
func (r *DatabaseServerReconciler) keepAppliedWALSize(
	server *v1.DatabaseServer,
	requested, existing *resource.Quantity,
	reported bool,
) *resource.Quantity {
	if requested != nil || existing == nil {
		return r.keepAppliedSize(server, "walStorageSize", requested, existing, reported)
	}
	if reported {
		return existing
	}

	r.EventRecorder.Eventf(
		server,
		nil,
		corev1.EventTypeWarning,
		eventReasonWALStorageKept,
		eventActionResize,
		"walStorageSize is no longer set, and the write-ahead log volume of %s is already there. "+
			"Keeping it, because CloudNativePG does not take a write-ahead log volume away from "+
			"a server that has one",
		existing,
	)

	return existing
}

// largestVolume returns the largest of the claim capacities and applied, or
// nil when neither exists. It is the size that a rendered volume must not go
// below.
func largestVolume(volumes []v1.VolumeStatus, applied *resource.Quantity) *resource.Quantity {
	largest := applied
	for i := range volumes {
		if largest == nil || volumes[i].Capacity.Cmp(*largest) > 0 {
			largest = &volumes[i].Capacity
		}
	}

	return largest
}
