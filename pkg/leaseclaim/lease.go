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

package leaseclaim

import (
	"crypto/sha256"
	"encoding/hex"

	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// MaxHolderIdentityLength bounds the holderIdentity field of a claim Lease.
// The API server of Kubernetes 1.36 accepts a longer value. The field is a
// display form for a reader, and a documented bound keeps it readable and
// safe against a stricter server. The exact identity lives in the
// annotations, which every decision about ownership reads.
const MaxHolderIdentityLength = 128

// Holder is the resource that a claim Lease records. The UID tells it apart
// from a later resource of the same name.
type Holder struct {
	types.NamespacedName
	UID types.UID
}

// Schema is the shape of the claim Leases of one kind of claim. A caller
// declares it once beside the names of its own resources, and the Lease
// shape, the sweep selector and the protocol all read it.
//
// Two Schema values must never share a Prefix or an annotation key. Each one
// names a separate claim, and a claimant of one must not read the Leases of
// the other as its own.
type Schema struct {
	// Prefix starts the name of every Lease of this claim. It ends with a
	// hyphen, and the hash of the claim key follows it.
	Prefix string
	// Noun names this claim in the errors that the protocol returns, for
	// example "claim" or "realm claim".
	Noun string
	// HolderNamespaceAnnotation carries the namespace of the holder.
	HolderNamespaceAnnotation string
	// HolderNameAnnotation carries the name of the holder.
	HolderNameAnnotation string
	// HolderUIDAnnotation carries the UID of the holder.
	HolderUIDAnnotation string
	// KeyAnnotation carries the claim key, for a reader of the Lease.
	KeyAnnotation string
	// Labels returns the labels of the claim Leases of the resources named
	// name. They carry the name alone, so two resources of two namespaces
	// that share a name share these labels.
	Labels func(name string) map[string]string
}

// NewLease builds the Lease that claims key for owner in namespace. The API
// server serializes the create, so exactly one claimant holds the key.
func (s Schema) NewLease(namespace, key string, owner client.Object) *coordinationv1.Lease {
	identity := labels.BoundedName(
		owner.GetNamespace()+"/"+owner.GetName(), MaxHolderIdentityLength,
	)
	now := metav1.NowMicro()

	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      s.LeaseName(key),
			Labels:    s.Labels(owner.GetName()),
			Annotations: map[string]string{
				s.HolderNamespaceAnnotation: owner.GetNamespace(),
				s.HolderNameAnnotation:      owner.GetName(),
				s.HolderUIDAnnotation:       string(owner.GetUID()),
				s.KeyAnnotation:             key,
			},
		},
		Spec: coordinationv1.LeaseSpec{HolderIdentity: &identity, AcquireTime: &now},
	}
}

// LeaseName returns the name of the Lease that claims key: the prefix of the
// Schema and 40 hex characters of the sha256 of the key. A claim key is no
// DNS subdomain, so the name is built from a hash of it. Every claimant of
// one key therefore meets on one Lease, and the key annotation says which key
// that is.
func (s Schema) LeaseName(key string) string {
	sum := sha256.Sum256([]byte(key))

	return s.Prefix + hex.EncodeToString(sum[:])[:40]
}

// HolderOf returns the resource that the annotations of the Lease name, and
// whether all three of them are there. Only the annotations carry ownership.
// The holderIdentity of the Lease is a display form for a reader.
func (s Schema) HolderOf(lease *coordinationv1.Lease) (Holder, bool) {
	annotations := lease.GetAnnotations()
	holder := Holder{
		NamespacedName: types.NamespacedName{
			Namespace: annotations[s.HolderNamespaceAnnotation],
			Name:      annotations[s.HolderNameAnnotation],
		},
		UID: types.UID(annotations[s.HolderUIDAnnotation]),
	}
	if holder.Namespace == "" || holder.Name == "" || holder.UID == "" {
		return Holder{}, false
	}

	return holder, true
}
