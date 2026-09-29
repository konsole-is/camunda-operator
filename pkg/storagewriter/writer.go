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

// Package storagewriter registers a resource that writes a storage backend
// without a pod of a CamundaCluster, such as a restore. A CamundaCluster that
// waits for the writers of its backend counts the registrations and needs no
// knowledge of their kinds.
//
// A registration is a Lease in the storage claim namespace. It holds the
// backend for as long as it exists: nothing renews it and it never expires.
// The writer releases it when it stops writing, from its finalizer when it is
// deleted. A registration that outlives its writer holds the backend until
// somebody deletes it.
package storagewriter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"slices"

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/konsole-is/camunda-operator/pkg/labels"
)

const (
	// Component is the component label value of a writer Lease.
	Component = "storage-writer"
	// KeyAnnotation holds the claim key of the backend that the writer writes.
	KeyAnnotation = "camunda.io/storage-writer-key"
	// WriterAnnotation holds the writer as "Kind namespace/name".
	WriterAnnotation = "camunda.io/storage-writer"
)

const leasePrefix = "camunda-writer-"

// Writer is a resource that writes a backend for the CamundaCluster with UID
// ClusterUID.
type Writer struct {
	Kind       string
	Namespace  string
	Name       string
	UID        types.UID
	ClusterUID types.UID
}

// String returns the writer as "Kind namespace/name", the form that the writer
// annotation holds and that Live returns.
func (w Writer) String() string {
	return w.Kind + " " + w.Namespace + "/" + w.Name
}

// Register creates the registration of w on the backend key. It restores the
// labels, the annotations and the holder of a registration that exists. claim
// is the name of the storage claim Lease of key. namespace is the storage claim
// namespace.
func Register(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	namespace, key, claim string,
	w Writer,
) error {
	want := newLease(namespace, key, claim, w)
	var lease coordinationv1.Lease
	err := reader.Get(ctx, client.ObjectKeyFromObject(want), &lease)
	if apierrors.IsNotFound(err) {
		err = c.Create(ctx, want)
		if err == nil {
			return nil
		}
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("registering %s as a writer of %q: %w", w, key, err)
		}
		err = reader.Get(ctx, client.ObjectKeyFromObject(want), &lease)
	}
	if err != nil {
		return fmt.Errorf("reading the writer Lease of %s: %w", w, err)
	}

	if matches(&lease, want) {
		return nil
	}

	lease.Labels = want.Labels
	lease.Annotations = want.Annotations
	lease.Spec.HolderIdentity = want.Spec.HolderIdentity
	if err := c.Update(ctx, &lease); err != nil {
		return fmt.Errorf("restoring the writer Lease of %s: %w", w, err)
	}

	return nil
}

func matches(lease, want *coordinationv1.Lease) bool {
	return maps.Equal(lease.Labels, want.Labels) &&
		maps.Equal(lease.Annotations, want.Annotations) &&
		ptr.Equal(lease.Spec.HolderIdentity, want.Spec.HolderIdentity)
}

// LeaseName returns the name of the registration of the writer with UID uid on
// the backend key.
func LeaseName(key string, uid types.UID) string {
	sum := sha256.Sum256([]byte(key + "|" + string(uid)))

	return leasePrefix + hex.EncodeToString(sum[:])[:40]
}

func newLease(namespace, key, claim string, w Writer) *coordinationv1.Lease {
	identity := labels.BoundedName(w.String(), 128)

	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      LeaseName(key, w.UID),
			Labels:    leaseLabels(claim, w),
			Annotations: map[string]string{
				KeyAnnotation:    key,
				WriterAnnotation: w.String(),
			},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity: &identity,
			AcquireTime:    new(metav1.NowMicro()),
		},
	}
}

func leaseLabels(claim string, w Writer) map[string]string {
	return map[string]string{
		labels.ManagedByKey:    labels.ManagedBy,
		labels.ComponentKey:    Component,
		labels.StorageClaimKey: labels.OwnerName(claim),
		labels.ClusterUIDKey:   string(w.ClusterUID),
		labels.WriterUIDKey:    string(w.UID),
	}
}

// Release removes the registration of w on the backend key. A registration that
// is gone already is fine.
func Release(ctx context.Context, c client.Client, namespace, key string, w Writer) error {
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: LeaseName(key, w.UID)},
	}
	if err := c.Delete(ctx, lease); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("releasing the writer Lease of %s: %w", w, err)
	}

	return nil
}

// ReleaseAll removes every registration of the writer with UID uid in
// namespace, whatever backend each one names. It finds a registration whose
// key the writer never recorded. The reader must read the API server
// directly, or a registration that a stale list misses stays.
func ReleaseAll(ctx context.Context, c client.Client, reader client.Reader, namespace string, uid types.UID) error {
	var leases coordinationv1.LeaseList
	err := reader.List(
		ctx,
		&leases,
		client.InNamespace(namespace),
		client.MatchingLabels{
			labels.ManagedByKey: labels.ManagedBy,
			labels.ComponentKey: Component,
			labels.WriterUIDKey: string(uid),
		},
	)
	if err != nil {
		return fmt.Errorf("listing the Leases of the writer with UID %s: %w", uid, err)
	}

	for i := range leases.Items {
		lease := &leases.Items[i]
		if err := c.Delete(ctx, lease); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("releasing the writer Lease %s: %w", lease.Name, err)
		}
	}

	return nil
}

// Live returns the writers of the backend key, as sorted "Kind namespace/name"
// entries, leaving out the writers for the cluster with UID self. claim is the
// name of the storage claim Lease of key. The reader must read the API server
// directly: a stale list lets a cluster start beside a writer.
func Live(
	ctx context.Context,
	reader client.Reader,
	namespace, key, claim string,
	self types.UID,
) ([]string, error) {
	leases, err := registrations(ctx, reader, namespace, key, claim)
	if err != nil {
		return nil, err
	}

	var writers []string
	for _, lease := range leases {
		if lease.Labels[labels.ClusterUIDKey] == string(self) {
			continue
		}
		writers = append(writers, lease.Annotations[WriterAnnotation])
	}
	slices.Sort(writers)

	return writers, nil
}

// LiveExcept is Live, but it leaves out only the registration of w, so the
// writers for every cluster count.
func LiveExcept(
	ctx context.Context,
	reader client.Reader,
	namespace, key, claim string,
	w Writer,
) ([]string, error) {
	leases, err := registrations(ctx, reader, namespace, key, claim)
	if err != nil {
		return nil, err
	}

	own := LeaseName(key, w.UID)
	var writers []string
	for _, lease := range leases {
		if lease.Name == own {
			continue
		}
		writers = append(writers, lease.Annotations[WriterAnnotation])
	}
	slices.Sort(writers)

	return writers, nil
}

// registrations returns the writer Leases of the backend key.
func registrations(
	ctx context.Context,
	reader client.Reader,
	namespace, key, claim string,
) ([]*coordinationv1.Lease, error) {
	var leases coordinationv1.LeaseList
	err := reader.List(
		ctx,
		&leases,
		client.InNamespace(namespace),
		client.MatchingLabels{
			labels.ManagedByKey:    labels.ManagedBy,
			labels.ComponentKey:    Component,
			labels.StorageClaimKey: labels.OwnerName(claim),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("listing the writers of the backend %q: %w", key, err)
	}

	var out []*coordinationv1.Lease
	for i := range leases.Items {
		// Two claim names can share a bounded label value, so the key decides.
		if leases.Items[i].Annotations[KeyAnnotation] == key {
			out = append(out, &leases.Items[i])
		}
	}

	return out, nil
}

// IsWriterLease reports whether obj is a writer Lease.
func IsWriterLease(obj client.Object) bool {
	return obj.GetLabels()[labels.ComponentKey] == Component &&
		obj.GetLabels()[labels.ManagedByKey] == labels.ManagedBy
}
