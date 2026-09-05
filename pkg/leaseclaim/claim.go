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
// AlreadyExists. Of two claimants that reach one free key together, exactly
// one holds it. A rule that a caller runs before the claim stays a preference
// in front of the Lease. It decides who tries first, never who holds.
//
// Ownership lives in the holder annotations of the Lease, never in its
// holderIdentity, which is a bounded display form for a reader. The UID
// decides it, so a hand-edited name annotation never makes a holder disown
// its own claim. A Lease without all three holder annotations is not one of
// ours: it blocks the claim, and only its removal by hand frees the key. A
// holder that exists keeps its claim against every other claimant. A holder
// that is gone, or that a later resource replaced under its name, is taken
// over, so a crash between the claim and the release never blocks a key
// forever.
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

// OwnerExists is the HolderKeeps of a claim whose holder keeps it while the
// resource exists under the recorded UID. It reads the holder through reader,
// which must read the API server directly, and names the claim of schema in
// the error of a read that fails.
func OwnerExists[T any, PT interface {
	*T
	client.Object
}](reader client.Reader, schema Schema[PT]) HolderKeeps {
	return func(ctx context.Context, holder Holder) (bool, error) {
		var owner T
		if err := reader.Get(ctx, holder.NamespacedName, PT(&owner)); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}

			return false, fmt.Errorf(
				"reading the %s holder %s: %w", schema.Noun, holder.NamespacedName, err,
			)
		}

		return PT(&owner).GetUID() == holder.UID, nil
	}
}

// Unclaimed runs while nothing holds a key, before the claimant creates the
// Lease. A caller passes a rule that only orders claimants of a free key, and
// the rule never runs against a Lease that exists. An error stops the claim.
type Unclaimed func(ctx context.Context) error

// Blocker is what stops a claimant from taking a claim.
type Blocker struct {
	// Holder names the resource that holds the claim. It is the zero Holder
	// when the Lease records no holder.
	Holder Holder
	// Lease names the claim Lease that blocks the claim.
	Lease types.NamespacedName
}

// Foreign reports whether the Lease that blocks the claim records no holder.
// Such a Lease is not one of ours and is never taken over. A nil Blocker
// blocks nothing, so it is not foreign either.
func (b *Blocker) Foreign() bool {
	return b != nil && b.Holder == Holder{}
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
// A controller builds this on every pass, so New checks nothing. Its caller
// runs Validate on the schema once, where a failure stops the manager rather
// than one reconcile.
func New[T client.Object](
	schema Schema[T],
	c client.Client,
	reader client.Reader,
	namespace string,
	holderKeeps HolderKeeps,
) *Claim[T] {
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
// A holder that HolderKeeps answers for is taken over in the same call. A
// Lease that a live holder took between the read and the answer reads as
// blocked for this pass, and the caller looks again on its retry interval.
func (c *Claim[T]) Take(ctx context.Context, owner T, key string) (*Blocker, error) {
	return c.take(ctx, owner, key, nil)
}

// TakeUnclaimed takes the claim on key for owner and runs unclaimed first
// when no Lease holds the key. It is Take for a caller that orders the
// claimants of a free key, and the order never reaches a key that is held.
func (c *Claim[T]) TakeUnclaimed(
	ctx context.Context, owner T, key string, unclaimed Unclaimed,
) (*Blocker, error) {
	return c.take(ctx, owner, key, unclaimed)
}

// take reads the claim before it writes. A holder that reads its own claim
// back needs no write, and a key that a live holder owns is answered without
// one. The create still decides every free key, so two claimants that reach
// one together still leave one holder.
func (c *Claim[T]) take(
	ctx context.Context, owner T, key string, unclaimed Unclaimed,
) (*Blocker, error) {
	lease, found, err := c.Read(ctx, key)
	if err != nil {
		return nil, err
	}

	if !found {
		if unclaimed != nil {
			if err := unclaimed(ctx); err != nil {
				return nil, err
			}
		}

		lease, err = c.create(ctx, owner, key)
		if err != nil || lease == nil {
			return nil, err
		}
	}

	blocker := c.blockerFor(lease, owner)
	if blocker == nil || blocker.Foreign() {
		return blocker, nil
	}

	keeps, err := c.holderKeeps(ctx, blocker.Holder)
	if err != nil {
		return nil, err
	}
	if keeps {
		return blocker, nil
	}

	// One takeover per pass. A claimant that reaches the key between the
	// delete and the create holds it, and the next pass judges that one.
	if err := c.takeOver(ctx, key, lease); err != nil {
		return nil, err
	}
	lease, err = c.create(ctx, owner, key)
	if err != nil || lease == nil {
		return nil, err
	}

	return c.blockerFor(lease, owner), nil
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

// create creates the claim Lease of key for owner. It returns nil when owner
// created it, and the Lease that stands in the way otherwise. The caller
// judges that Lease: it is one owner holds already as often as one of another
// claimant.
func (c *Claim[T]) create(ctx context.Context, owner T, key string) (*coordinationv1.Lease, error) {
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
		if found {
			return lease, nil
		}
	}

	return nil, fmt.Errorf(
		"the %s Lease of %q went away twice during the claim: a release or a takeover "+
			"races this claimant",
		c.schema.Noun, key,
	)
}

// blockerFor judges lease for owner. It returns nil when lease records owner,
// and what blocks the claim otherwise.
func (c *Claim[T]) blockerFor(lease *coordinationv1.Lease, owner T) *Blocker {
	holder, ours := c.schema.HolderOf(lease)
	if ours && holder.UID == owner.GetUID() {
		return nil
	}

	blocker := &Blocker{Lease: client.ObjectKeyFromObject(lease)}
	if ours {
		blocker.Holder = holder
	}

	return blocker
}

// takeOver deletes lease, the claim Lease of key that its caller read and
// found held by a holder that keeps nothing, so the claimant can create it
// again. A Lease that is
// gone, and one that another holder took in between, is left alone.
//
// The delete goes under the resourceVersion of the read, with no read in
// between. A write that changes nothing but the resourceVersion, while the
// holder is being judged, refuses the delete for this pass, and the next pass
// takes the Lease over.
func (c *Claim[T]) takeOver(ctx context.Context, key string, lease *coordinationv1.Lease) error {
	err := c.Release(ctx, lease)
	if err == nil || !apierrors.IsConflict(err) {
		return err
	}

	// A refused precondition says the Lease changed, and nothing more. Only a
	// change of the holder decides the claim. Any other write leaves the
	// holder that keeps nothing in place, and reporting that as a takeover
	// names a dead holder to the caller as a live one.
	holder, _ := c.schema.HolderOf(lease)
	current, found, readErr := c.Read(ctx, key)
	if readErr != nil {
		return readErr
	}
	if !found || !c.schema.heldBy(current, holder.UID) {
		return nil
	}

	return err
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

// Holds reports whether the claim Lease of key records owner. A Lease that is
// gone, and one that records another resource, both answer no. A read that
// fails is an error and never a no.
func (c *Claim[T]) Holds(ctx context.Context, key string, owner T) (bool, error) {
	lease, found, err := c.Read(ctx, key)
	if err != nil || !found {
		return false, err
	}

	return c.schema.heldBy(lease, owner.GetUID()), nil
}

// Held returns every claim Lease of the namespace that records the UID of
// owner, which is every claim owner can give back.
//
// The label selector carries the name of the owner alone. The list therefore
// also holds the claims of a resource of another namespace with this name,
// and of a later resource. The recorded UID tells them apart.
func (c *Claim[T]) Held(ctx context.Context, owner T) ([]coordinationv1.Lease, error) {
	holder := holderOf(owner)

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
		if c.schema.heldBy(&leases.Items[i], holder.UID) {
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
