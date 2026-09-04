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

// Package leaseclaim holds the claim protocol that a resource uses to take a
// thing outside Kubernetes for itself. A Database claims a logical database
// on a PostgreSQL server with it, and a CamundaManagementCluster claims a
// Keycloak realm.
//
// The claim is a coordination.k8s.io Lease in the namespace of the operator,
// one per claim key, named from a hash of that key. The API server creates it
// atomically: the first Create wins and every later Create answers
// AlreadyExists. Of two claimants that reach one key together, exactly one
// holds it. A rule that a caller runs before the claim stays a preference in
// front of the Lease. It decides who tries first, never who holds.
//
// Ownership lives in the three holder annotations of the Lease, never in its
// holderIdentity, which is a bounded display form for a reader. A Lease
// without all three is not one of ours: it blocks the claim, and only its
// removal by hand frees the key. A holder that exists keeps its claim against
// every other claimant. A holder that is gone, or that a later resource
// replaced under its name, is taken over, so a crash between the claim and
// the release never blocks a key forever.
//
// Every read goes through the reader that the caller passes. That reader must
// read the API server directly, so a controller passes its uncached API
// reader and never the cached manager client. A claim decided from a stale
// cache is no mutual exclusion, and the resourceVersion that guards a
// takeover comes from the same read.
package leaseclaim

import (
	"context"
	"fmt"

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HolderKeeps reports whether the resource that a claim Lease names still
// owns the claim. A holder that is gone, and one that a later resource
// replaced under the same name, keeps nothing and is taken over. An error
// stops the claim, so a holder that cannot be read keeps what it holds.
type HolderKeeps func(ctx context.Context, holder Holder) (bool, error)

// Blocker is what stops a claimant from taking a claim.
type Blocker struct {
	// Holder names the resource that holds the claim. It is the zero Holder
	// when the Lease records no holder.
	Holder Holder
	// Lease names the claim Lease that blocks the claim.
	Lease types.NamespacedName
}

// Claim runs the claim protocol of one Schema over the claim Leases of one
// namespace. T is the kind that takes the claim.
type Claim[T client.Object] struct {
	schema      Schema[T]
	client      client.Client
	reader      client.Reader
	namespace   string
	holderKeeps HolderKeeps
}

// New returns the claim protocol of schema over the claim Leases of
// namespace, which is the namespace the operator runs in.
//
// Writes go through c. Every read goes through reader, which must read the
// API server directly and never a cache.
//
// New panics on a schema that Validate rejects, and on a nil c, reader or
// holderKeeps. A controller passes all four once at the top of a reconcile,
// so a wrong one is a programmer error that the first pass reports. A nil
// holderKeeps runs only where two claimants meet, which is the case the
// package exists for and the worst place to learn of it.
func New[T client.Object](
	schema Schema[T],
	c client.Client,
	reader client.Reader,
	namespace string,
	holderKeeps HolderKeeps,
) *Claim[T] {
	if err := schema.Validate(); err != nil {
		panic("leaseclaim: the claim Schema is not usable: " + err.Error())
	}
	if c == nil || reader == nil || holderKeeps == nil {
		panic("leaseclaim: a claim needs a client, a reader and a HolderKeeps")
	}

	return &Claim[T]{
		schema:      schema,
		client:      c,
		reader:      reader,
		namespace:   namespace,
		holderKeeps: holderKeeps,
	}
}

// Take takes the claim on key for owner. It returns nil when owner holds the
// claim after the call, which covers the Lease it created and the one it held
// already. Otherwise it returns what blocks the claim, and the caller reports
// it: only a failure of the Kubernetes API comes back as an error.
//
// A holder that HolderKeeps answers for is taken over: the Lease goes under
// its UID and its resourceVersion, and the create runs once more. A second
// claimant that took the same Lease over first leaves this one a Blocker that
// names it.
func (c *Claim[T]) Take(ctx context.Context, owner T, key string) (*Blocker, error) {
	blocker, err := c.create(ctx, owner, key)
	if err != nil || blocker == nil || blocker.Foreign() {
		return blocker, err
	}

	keeps, err := c.holderKeeps(ctx, blocker.Holder)
	if err != nil {
		return nil, err
	}
	if keeps {
		return blocker, nil
	}

	if err := c.Drop(ctx, key, blocker.Holder); err != nil {
		return nil, err
	}

	return c.create(ctx, owner, key)
}

// Foreign reports whether the Lease that blocks the claim records no holder.
// Such a Lease is not one of ours and is never taken over. A nil Blocker
// blocks nothing, so it is not foreign either.
func (b *Blocker) Foreign() bool {
	return b != nil && b.Holder == Holder{}
}

// Holds reports whether the claim Lease of key records holder. A Lease that
// is gone, and one that records another resource, both answer no. A read that
// fails is an error and never a no.
func (c *Claim[T]) Holds(ctx context.Context, key string, holder Holder) (bool, error) {
	lease, found, err := c.Read(ctx, key)
	if err != nil || !found {
		return false, err
	}

	recorded, ours := c.schema.HolderOf(lease)

	return ours && recorded == holder, nil
}

// create creates the claim Lease of key for owner. It returns nil when owner
// holds the claim after the call, and what blocks it otherwise.
func (c *Claim[T]) create(ctx context.Context, owner T, key string) (*Blocker, error) {
	// The Lease can go away between the create and the read, when a release
	// or a takeover races this claimant. The second pass then creates it.
	for range 2 {
		err := c.client.Create(ctx, c.schema.NewLease(c.namespace, key, owner))
		if err == nil {
			return nil, nil
		}
		if !apierrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("creating the %s Lease of %q: %w", c.schema.Noun, key, err)
		}

		lease, found, err := c.Read(ctx, key)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}

		holder, ours := c.schema.HolderOf(lease)
		if ours && holder == HolderOf(owner) {
			return nil, nil
		}

		blocker := &Blocker{Lease: client.ObjectKeyFromObject(lease)}
		if ours {
			blocker.Holder = holder
		}

		return blocker, nil
	}

	return nil, fmt.Errorf(
		"the %s Lease of %q exists but is not readable yet", c.schema.Noun, key,
	)
}

// Read reads the claim Lease of key live, and reports whether it is there.
func (c *Claim[T]) Read(ctx context.Context, key string) (*coordinationv1.Lease, bool, error) {
	name := types.NamespacedName{Namespace: c.namespace, Name: c.schema.LeaseName(key)}

	var lease coordinationv1.Lease
	if err := c.reader.Get(ctx, name, &lease); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}

		return nil, false, fmt.Errorf("reading the %s Lease %s: %w", c.schema.Noun, name, err)
	}

	return &lease, true, nil
}

// Drop deletes the claim Lease of key while its annotations still name
// holder. A Lease that is gone, and one that another resource holds, is left
// alone.
//
// A second claimant that took the same stale holder over between the read and
// the delete leaves a Lease the preconditions refuse. That is the claim
// decided rather than a failure, so Drop reports no error and the create that
// follows it names the winner.

func (c *Claim[T]) Drop(ctx context.Context, key string, holder Holder) error {
	lease, found, err := c.Read(ctx, key)
	if err != nil || !found {
		return err
	}

	if recorded, ours := c.schema.HolderOf(lease); !ours || recorded != holder {
		return nil
	}

	if err := c.Release(ctx, lease); err != nil && !apierrors.IsConflict(err) {
		return err
	}

	return nil
}

// Release deletes lease under the UID and the resourceVersion that it was
// read with. A Lease that is gone is left alone.
//
// A Lease that changed in between fails the preconditions, which means it was
// not deleted. The error goes back to the caller, so a release keeps its
// finalizer and reads the Lease again on the next look. To report a conflict
// as a release lets the holder go while its claim stays.
func (c *Claim[T]) Release(ctx context.Context, lease *coordinationv1.Lease) error {
	err := c.client.Delete(ctx, lease, client.Preconditions{
		UID: &lease.UID, ResourceVersion: &lease.ResourceVersion,
	})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf(
			"deleting the %s Lease %s: %w",
			c.schema.Noun, client.ObjectKeyFromObject(lease), err,
		)
	}

	return nil
}

// Held returns every claim Lease of the namespace that still names holder.
//
// The label selector carries the name of the holder alone, so the list also
// holds the claims of a resource of another namespace with this name, and of
// a later resource. The holder annotations tell them apart.
func (c *Claim[T]) Held(ctx context.Context, holder Holder) ([]coordinationv1.Lease, error) {
	var leases coordinationv1.LeaseList
	err := c.reader.List(
		ctx, &leases,
		client.InNamespace(c.namespace),
		client.MatchingLabels(c.schema.Labels(holder.Name)),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"listing the %s Leases of %s: %w", c.schema.Noun, holder.NamespacedName, err,
		)
	}

	var held []coordinationv1.Lease
	for i := range leases.Items {
		if recorded, ours := c.schema.HolderOf(&leases.Items[i]); ours && recorded == holder {
			held = append(held, leases.Items[i])
		}
	}

	return held, nil
}

// LeaseName returns the name of the Lease that claims key, so a caller that
// holds a Claim names a Lease without the Schema behind it.
func (c *Claim[T]) LeaseName(key string) string {
	return c.schema.LeaseName(key)
}
