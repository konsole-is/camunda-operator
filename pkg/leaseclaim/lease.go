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
	"errors"
	"fmt"
	"strings"

	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// MaxHolderIdentityLength bounds the holderIdentity field of a claim Lease.
// The API server of Kubernetes 1.36 accepts a longer value. The field is a
// display form for a reader, and a documented bound keeps it readable and
// safe against a stricter server. The exact identity lives in the
// annotations, which every decision about ownership reads.
const MaxHolderIdentityLength = 128

// Holder is the resource that a claim Lease records. The UID decides
// ownership on its own. The namespace and the name name the holder in a
// message: a claimant that read them as ownership disowns its own claim over
// one hand-edited annotation.
type Holder struct {
	types.NamespacedName
	UID types.UID
}

// holderOf is the holder identity of obj.
func holderOf(obj client.Object) Holder {
	return Holder{NamespacedName: client.ObjectKeyFromObject(obj), UID: obj.GetUID()}
}

// Schema is the shape of the claim Leases of one kind of claim. A caller
// declares it once beside the names of its own resources, and the Lease
// shape, the sweep selector and the protocol all read it. T is the kind that
// takes this claim, so a Schema of one claim never renders a Lease for the
// holder of another.
//
// Two Schema values must never share a Prefix, an annotation key or a label
// set. Each one names a separate claim, and a claimant of one that reads the
// Leases of the other as its own hands a key to two holders.
type Schema[T client.Object] struct {
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

// NewClaim returns the claim protocol of the Schema over the claim Leases of
// namespace, with OwnerExists as its HolderKeeps. It is New for a controller,
// which wires its client, its uncached API reader and the namespace of the
// operator and runs no policy of its own.
func (s Schema[T]) NewClaim(c client.Client, reader client.Reader, namespace string) *Claim[T] {
	return New(s, c, reader, namespace, nil)
}

// NewLease builds the Lease that claims key for owner in namespace. The API
// server serializes the create, so exactly one claimant holds the key.
func (s Schema[T]) NewLease(namespace, key string, owner T) *coordinationv1.Lease {
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
func (s Schema[T]) LeaseName(key string) string {
	sum := sha256.Sum256([]byte(key))

	return s.Prefix + hex.EncodeToString(sum[:])[:40]
}

// HolderOf returns the resource that the annotations of the Lease name, and
// whether all three of them are there. Only the annotations carry ownership.
// The holderIdentity of the Lease is a display form for a reader.
func (s Schema[T]) HolderOf(lease *coordinationv1.Lease) (Holder, bool) {
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

// heldBy reports whether the annotations of the Lease record the holder with
// this UID.
func (s Schema[T]) heldBy(lease *coordinationv1.Lease, uid types.UID) bool {
	holder, ours := s.HolderOf(lease)

	return ours && holder.UID == uid
}

// Validate reports whether the Schema names a claim that a claimant reads
// back, and whether T is a pointer to a struct, which OwnerExists allocates.
// A missing prefix, a missing annotation key and a Labels function that gives
// no label each write a Lease that no other claimant recognises as a claim.
// A label or an annotation key the API server refuses writes no Lease at all.
// Two annotation keys that are equal drop one of the values the ownership
// decision reads.
//
// A caller runs it once where a failure stops the operator, and never on a
// reconcile path.
func (s Schema[T]) Validate() error {
	if _, err := newOwner[T](); err != nil {
		return err
	}
	if s.Prefix == "" {
		return errors.New("the Lease name prefix is empty")
	}
	// The hyphen keeps the prefix off the hash that follows it, so no prefix
	// that starts another one can name the Lease of that other claim.
	if !strings.HasSuffix(s.Prefix, "-") {
		return fmt.Errorf("the Lease name prefix %q does not end with a hyphen", s.Prefix)
	}
	if problems := validation.IsDNS1123Subdomain(s.LeaseName("probe")); len(problems) > 0 {
		return fmt.Errorf(
			"the Lease name prefix %q names no Lease: %s", s.Prefix, strings.Join(problems, "; "),
		)
	}
	if s.Noun == "" {
		return errors.New("the noun of the claim is empty")
	}
	if s.Labels == nil {
		return errors.New("the Labels function is nil")
	}
	// Held lists on the selector alone, so a claim with no label reads every
	// Lease of the namespace as its own.
	selector := s.Labels("any-holder")
	if len(selector) == 0 {
		return errors.New("the Labels function returns no label")
	}
	// The API server refuses a Lease with a malformed key or value on every
	// create, which a claimant meets on its first reconcile and not here.
	for key, value := range selector {
		if problems := validation.IsQualifiedName(key); len(problems) > 0 {
			return fmt.Errorf("the label key %q is no qualified name: %s", key, strings.Join(problems, "; "))
		}
		if problems := validation.IsValidLabelValue(value); len(problems) > 0 {
			return fmt.Errorf("the label value %q of %q is invalid: %s", value, key, strings.Join(problems, "; "))
		}
	}

	seen := make(map[string]string, 4)
	for _, annotation := range s.annotations() {
		if annotation.key == "" {
			return fmt.Errorf("the %s is empty", annotation.field)
		}
		if problems := validation.IsQualifiedName(annotation.key); len(problems) > 0 {
			return fmt.Errorf(
				"the %s %q is no qualified name: %s",
				annotation.field, annotation.key, strings.Join(problems, "; "),
			)
		}
		if first, taken := seen[annotation.key]; taken {
			return fmt.Errorf(
				"the %s and the %s are both %q", first, annotation.field, annotation.key,
			)
		}
		seen[annotation.key] = annotation.field
	}

	return nil
}

// schemaAnnotation is one annotation key of a Schema beside the name of the
// field that holds it, so Validate names the field a caller has to fix.
type schemaAnnotation struct {
	field string
	key   string
}

// annotations lists the annotation keys of the Schema in a fixed order.
func (s Schema[T]) annotations() []schemaAnnotation {
	return []schemaAnnotation{
		{"HolderNamespaceAnnotation", s.HolderNamespaceAnnotation},
		{"HolderNameAnnotation", s.HolderNameAnnotation},
		{"HolderUIDAnnotation", s.HolderUIDAnnotation},
		{"KeyAnnotation", s.KeyAnnotation},
	}
}
