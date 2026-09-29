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
// somebody deletes it. A registration that names a contract also holds every
// other backend that the contract resolves to, so a move of the address frees
// nothing.
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
	// ContractAnnotation holds the contract of the writer, see Writer.Contract.
	ContractAnnotation = "camunda.io/storage-writer-contract"
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
	// Contract names the resource that holds the address of the backend, for
	// example a DatabaseServerConfig and a database name. A reader that passes
	// the same contract to Live finds the writer on any backend key. Empty
	// names no contract.
	Contract string
}

// String returns the writer as "Kind namespace/name", the form that the writer
// annotation holds and that Live returns.
func (w Writer) String() string {
	return w.Kind + " " + w.Namespace + "/" + w.Name
}

// Register creates the registration of w on the backend key. It restores the
// labels, the annotations and the holder of a registration that exists. A w
// with no Contract keeps the contract that an existing registration names.
// claim is the name of the storage claim Lease of key. namespace is the storage
// claim namespace.
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

	if w.Contract == "" && lease.Annotations[ContractAnnotation] != "" {
		w.Contract = lease.Annotations[ContractAnnotation]
		want = newLease(namespace, key, claim, w)
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
	annotations := map[string]string{
		KeyAnnotation:    key,
		WriterAnnotation: w.String(),
	}
	if w.Contract != "" {
		annotations[ContractAnnotation] = w.Contract
	}

	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        LeaseName(key, w.UID),
			Labels:      leaseLabels(claim, w),
			Annotations: annotations,
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity: &identity,
			AcquireTime:    new(metav1.NowMicro()),
		},
	}
}

func leaseLabels(claim string, w Writer) map[string]string {
	out := map[string]string{
		labels.ManagedByKey:    labels.ManagedBy,
		labels.ComponentKey:    Component,
		labels.StorageClaimKey: labels.OwnerName(claim),
		labels.ClusterUIDKey:   string(w.ClusterUID),
		labels.WriterUIDKey:    string(w.UID),
	}
	if w.Contract != "" {
		out[labels.StorageContractKey] = contractLabel(w.Contract)
	}

	return out
}

// contractLabel holds a hash because a contract has characters, such as "/",
// that a label value does not admit.
func contractLabel(contract string) string {
	sum := sha256.Sum256([]byte(contract))

	return hex.EncodeToString(sum[:])[:40]
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

// Live returns the writers of the backend key, and the writers that name
// contract on any key, as sorted "Kind namespace/name" entries. It leaves out
// the writers for the cluster with UID self. claim is the name of the storage
// claim Lease of key. An empty contract matches on the key alone. The reader
// must read the API server directly: a stale list lets a cluster start beside
// a writer.
func Live(
	ctx context.Context,
	reader client.Reader,
	namespace, key, claim, contract string,
	self types.UID,
) ([]string, error) {
	leases, err := registrations(ctx, reader, namespace, key, claim, contract)
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

// LiveExcept is Live on the contract of w, but it leaves out only the
// registrations of w, so the writers for every cluster count.
func LiveExcept(
	ctx context.Context,
	reader client.Reader,
	namespace, key, claim string,
	w Writer,
) ([]string, error) {
	leases, err := registrations(ctx, reader, namespace, key, claim, w.Contract)
	if err != nil {
		return nil, err
	}

	var writers []string
	for _, lease := range leases {
		if lease.Labels[labels.WriterUIDKey] == string(w.UID) {
			continue
		}
		writers = append(writers, lease.Annotations[WriterAnnotation])
	}
	slices.Sort(writers)

	return writers, nil
}

// registrations returns the writer Leases of the backend key and of contract.
func registrations(
	ctx context.Context,
	reader client.Reader,
	namespace, key, claim, contract string,
) ([]*coordinationv1.Lease, error) {
	// Two claim names can share a bounded label value, so the key decides.
	out, err := listMatching(
		ctx,
		reader,
		namespace,
		labels.StorageClaimKey,
		labels.OwnerName(claim),
		KeyAnnotation,
		key,
	)
	if err != nil || contract == "" {
		return out, err
	}

	byContract, err := listMatching(
		ctx, reader, namespace, labels.StorageContractKey, contractLabel(contract), ContractAnnotation, contract,
	)
	if err != nil {
		return nil, err
	}
	for _, lease := range byContract {
		if lease.Annotations[KeyAnnotation] != key {
			out = append(out, lease)
		}
	}

	return out, nil
}

// listMatching lists the writer Leases whose label is value and keeps those
// whose annotation is exactly want.
func listMatching(
	ctx context.Context,
	reader client.Reader,
	namespace, label, value, annotation, want string,
) ([]*coordinationv1.Lease, error) {
	var leases coordinationv1.LeaseList
	err := reader.List(
		ctx,
		&leases,
		client.InNamespace(namespace),
		client.MatchingLabels{
			labels.ManagedByKey: labels.ManagedBy,
			labels.ComponentKey: Component,
			label:               value,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("listing the writers of %q: %w", want, err)
	}

	var out []*coordinationv1.Lease
	for i := range leases.Items {
		if leases.Items[i].Annotations[annotation] == want {
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
