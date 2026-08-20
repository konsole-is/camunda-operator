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

// Package clusterclaim holds the mutual exclusion of one CamundaCluster.
// An operation that takes the cluster for itself holds the claim while it
// runs, and no second operation of any kind starts in the meantime. Both
// backup kinds hold this claim. The two restore kinds must hold it too: a
// restore and a backup of one cluster must never run together.
//
// The claim on a cluster is a coordination.k8s.io Lease that the API server
// creates atomically: the first Create wins, every later Create answers
// AlreadyExists. That makes the claim correct under concurrent reconciles and
// across controllers, where a check-then-act on the status of the siblings is
// not. There is one Lease per cluster, so every claimant of that cluster
// meets on it, whatever kind it is. The Lease is authoritative. An ordering
// that a caller runs among its own pending resources stays a fairness
// pre-filter in front of it: it decides who tries the claim first, not who
// holds it.
//
// A holder that is gone, replaced, or terminal no longer needs the claim. A
// claimant that finds such a holder takes the Lease over, so a crash between
// the claim and the release never blocks the cluster forever.
//
// Every function here writes through a client.Client and reads through a
// separate client.Reader. The reader must read the API server directly.
// A controller passes its uncached API reader, never the cached manager
// client: a claim decided from a stale cache is no mutual exclusion.
package clusterclaim

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// claimLeasePrefix starts the name of every claim Lease.
const claimLeasePrefix = "camunda-cluster-"

// maxLeaseNameLength is the DNS subdomain bound of a Lease name.
const maxLeaseNameLength = 253

// maxHolderIdentityLength bounds the holderIdentity field of the Lease. The
// API server of Kubernetes 1.36 accepts longer values. The field is a
// display form here, and a documented bound keeps it readable and safe
// against a stricter server. The exact identity lives in the annotations.
const maxHolderIdentityLength = 128

// The annotations of the claim Lease carry the exact identity of the
// holder. holderIdentity is a bounded display form of the same, and every
// decision about ownership and takeover reads the annotations, never the
// display form.
const (
	// ClaimHolderKindAnnotation names the kind of the holder resource.
	ClaimHolderKindAnnotation = "camunda.io/claim-holder-kind"
	// ClaimHolderNameAnnotation names the holder resource.
	ClaimHolderNameAnnotation = "camunda.io/claim-holder-name"
	// ClaimHolderUIDAnnotation carries the UID of the holder resource.
	ClaimHolderUIDAnnotation = "camunda.io/claim-holder-uid"
)

// ClaimLeaseName returns the name of the Lease that claims the cluster:
// "camunda-cluster-<cluster>". The name is deterministic, so every claimant of
// one cluster meets on the same Lease. A cluster name that pushes the Lease
// name past the DNS subdomain bound is cut and suffixed with a hash of the
// full name. The result stays deterministic and unique.
func ClaimLeaseName(cluster string) string {
	return boundName(claimLeasePrefix+cluster, maxLeaseNameLength)
}

// boundName returns name when it fits limit. A longer name is cut and
// suffixed with "-" and a hash of the full name. The result stays
// deterministic and tells two long names apart. The cut can land after a
// "." or a "-". A separator before the suffix is not a valid DNS subdomain,
// so trailing separators are trimmed from the kept head.
func boundName(name string, limit int) string {
	if len(name) <= limit {
		return name
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(name))
	suffix := fmt.Sprintf("-%08x", hash.Sum32())
	head := strings.TrimRight(name[:limit-len(suffix)], "-.")
	return head + suffix
}

// Claimant identifies the resource that holds or wants the claim: its kind,
// its name, and its UID. The UID tells a resource apart from a later one
// with the same name.
type Claimant struct {
	// Kind is the kind of the resource, for example
	// LogicalBackupElasticsearch.
	Kind string
	// Name is the name of the resource. It lives in the namespace of the
	// cluster, like the Lease.
	Name string
	// UID is the UID of the resource.
	UID types.UID
}

// String returns the exact identity of the claimant: "<Kind>/<Name>/<UID>".
// Claim returns it for the holder that blocks, and the Lease carries its
// parts in the annotations. It is not the holderIdentity field, which is
// bounded.
func (c Claimant) String() string {
	return c.Kind + "/" + c.Name + "/" + string(c.UID)
}

// Display returns the short form for a message: "<Kind>/<Name>".
func (c Claimant) Display() string {
	return c.Kind + "/" + c.Name
}

// HolderIdentity returns the bounded display form that the Lease records in
// spec.holderIdentity: "<Kind>/<Name>", cut with a hash suffix when it does
// not fit maxHolderIdentityLength. It carries no UID and decides nothing.
func (c Claimant) HolderIdentity() string {
	return boundName(c.Display(), maxHolderIdentityLength)
}

// ParseClaimant reads an identity that String wrote. It rejects any other
// shape, so a Lease that something else wrote is never mistaken for one of
// ours and never taken over.
func ParseClaimant(holder string) (Claimant, error) {
	parts := strings.Split(holder, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return Claimant{}, fmt.Errorf("holder identity %q is not <Kind>/<Name>/<UID>", holder)
	}
	return Claimant{Kind: parts[0], Name: parts[1], UID: types.UID(parts[2])}, nil
}

// Claim takes the claim on the cluster for self, or reports who holds it.
// It returns "" when self holds the claim after the call. That is the case
// when the Lease was created, and when self held it already, so a re-entry
// after a failed status flush proceeds. Otherwise it returns the holder
// identity of the claimant that holds the claim, and self must wait.
//
// A holder that HolderActive reports inactive is taken over. The Lease is
// deleted with its UID and resourceVersion as preconditions, and the Create
// is tried once more. A Lease without the holder annotations is not one of
// ours: it blocks without a takeover, and Claim returns its holderIdentity
// as it is.
//
// Writes go through c. Every read goes through reader, which must read the
// API server directly and never a cache: a mutual exclusion that decides
// "held by me" or a takeover from a stale cache is no exclusion. The
// resourceVersion that guards a takeover comes from that same read.
func Claim(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	namespace, cluster string,
	self Claimant,
) (string, error) {
	holder, err := create(ctx, c, reader, namespace, cluster, self)
	if err != nil || holder == "" || holder == self.String() {
		return holder, err
	}

	claimant, parseErr := ParseClaimant(holder)
	if parseErr != nil {
		return holder, nil
	}
	active, err := HolderActive(ctx, reader, namespace, claimant)
	if err != nil {
		return "", err
	}
	if active {
		return holder, nil
	}

	if err := takeOver(ctx, c, reader, namespace, cluster, claimant); err != nil {
		return "", err
	}

	return create(ctx, c, reader, namespace, cluster, self)
}

// create tries to create the Lease for self. It returns "" when self holds
// the Lease after the call, or the current holder identity.
func create(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	namespace, cluster string,
	self Claimant,
) (string, error) {
	for range 2 {
		lease := newLease(namespace, cluster, self)
		err := c.Create(ctx, lease)
		if err == nil {
			return "", nil
		}
		if !apierrors.IsAlreadyExists(err) {
			return "", fmt.Errorf("creating the claim Lease %s/%s: %w", namespace, lease.Name, err)
		}

		holder, ours, found, err := currentHolder(ctx, reader, namespace, cluster)
		if err != nil {
			return "", err
		}
		if !found {
			// The Lease went away between the Create and the Get: a release
			// or a takeover raced this claimant. Try the Create once more.
			continue
		}
		// Only an annotated Lease can be self. A foreign Lease whose
		// holderIdentity happens to spell the same identity has no
		// provenance. It blocks: a Lease without the holder annotations
		// is not ours.
		if ours && holder == self.String() {
			return "", nil
		}
		return holder, nil
	}

	return "", fmt.Errorf("the claim Lease of cluster %s/%s exists but is not readable yet", namespace, cluster)
}

// Holds reports whether self holds the claim on the cluster now. A
// re-entering claimant asks this before it runs any ordering among its own
// pending resources. A resource that holds the claim goes first, whatever
// the tie-break says. Otherwise the tie-break and the claim block each
// other. reader must read the API server directly, like every read of the
// claim.
func Holds(ctx context.Context, reader client.Reader, namespace, cluster string, self Claimant) (bool, error) {
	holder, ours, found, err := currentHolder(ctx, reader, namespace, cluster)
	if err != nil {
		return false, err
	}
	return found && ours && holder == self.String(), nil
}

// unidentifiedHolder stands in for the holder of a Lease that carries no
// holder identity at all. Nothing of this operator writes such a Lease. It
// blocks the claim, and ParseClaimant rejects it, so it is never taken over.
const unidentifiedHolder = "unidentified holder"

// holderOf returns the identity of the holder that the Lease records and
// whether it came from the holder annotations. A Lease with the three
// annotations is one of ours, and the identity is the String of that
// Claimant. Any other Lease returns its holderIdentity as it is, or
// unidentifiedHolder when it has none. Such a Lease is foreign, whatever
// its holderIdentity spells. It blocks, it is never self, and takeOver
// never deletes it: only annotations carry provenance.
func holderOf(lease *coordinationv1.Lease) (string, bool) {
	if claimant, ok := annotatedHolder(lease); ok {
		return claimant.String(), true
	}
	if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity == "" {
		return unidentifiedHolder, false
	}
	return *lease.Spec.HolderIdentity, false
}

// annotatedHolder reads the exact holder identity from the annotations of
// the Lease. ok is false when any part is absent.
func annotatedHolder(lease *coordinationv1.Lease) (Claimant, bool) {
	annotations := lease.GetAnnotations()
	claimant := Claimant{
		Kind: annotations[ClaimHolderKindAnnotation],
		Name: annotations[ClaimHolderNameAnnotation],
		UID:  types.UID(annotations[ClaimHolderUIDAnnotation]),
	}
	if claimant.Kind == "" || claimant.Name == "" || claimant.UID == "" {
		return Claimant{}, false
	}
	return claimant, true
}

// currentHolder returns the identity of the holder of the Lease, whether it
// came from the holder annotations, and whether the Lease exists.
func currentHolder(
	ctx context.Context,
	reader client.Reader,
	namespace, cluster string,
) (holder string, ours bool, found bool, err error) {
	var lease coordinationv1.Lease
	key := client.ObjectKey{Namespace: namespace, Name: ClaimLeaseName(cluster)}
	if err := reader.Get(ctx, key, &lease); err != nil {
		if apierrors.IsNotFound(err) {
			return "", false, false, nil
		}
		return "", false, false, fmt.Errorf("reading the claim Lease %s: %w", key, err)
	}
	holder, ours = holderOf(&lease)
	return holder, ours, true, nil
}

// takeOver deletes the Lease while it still records holder. A Lease that
// changed hands in between is left alone.
func takeOver(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	namespace, cluster string,
	holder Claimant,
) error {
	if err := deleteHeldBy(ctx, c, reader, namespace, cluster, holder); err != nil {
		return fmt.Errorf("taking over the claim Lease of cluster %s/%s: %w", namespace, cluster, err)
	}
	return nil
}

// deleteHeldBy deletes the claim Lease of the cluster while its annotations
// still name holder. The Delete carries the UID and the resourceVersion of
// the Lease as preconditions, both from the direct read through reader. A
// Lease that is gone, that another holder has, or that changed in between
// (a conflict on the preconditions) is left alone without an error.
func deleteHeldBy(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	namespace, cluster string,
	holder Claimant,
) error {
	var lease coordinationv1.Lease
	key := client.ObjectKey{Namespace: namespace, Name: ClaimLeaseName(cluster)}
	if err := reader.Get(ctx, key, &lease); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("reading the claim Lease %s: %w", key, err)
	}
	if recorded, ok := annotatedHolder(&lease); !ok || recorded != holder {
		return nil
	}

	err := c.Delete(ctx, &lease, client.Preconditions{
		UID:             &lease.UID,
		ResourceVersion: &lease.ResourceVersion,
	})
	if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
		return fmt.Errorf("deleting the claim Lease %s: %w", key, err)
	}

	return nil
}

// newLease builds the claim Lease of cluster for self. The annotations
// carry the exact identity; holderIdentity carries the bounded display form.
func newLease(namespace, cluster string, self Claimant) *coordinationv1.Lease {
	holder := self.HolderIdentity()
	now := metav1.NowMicro()
	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      ClaimLeaseName(cluster),
			Labels:    labels.Managed(labels.Cluster(cluster), "cluster-claim"),
			Annotations: map[string]string{
				ClaimHolderKindAnnotation: self.Kind,
				ClaimHolderNameAnnotation: self.Name,
				ClaimHolderUIDAnnotation:  string(self.UID),
			},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity: &holder,
			AcquireTime:    &now,
		},
	}
}

// readyReasonResumeFailed is the Ready condition reason of a backup that
// ended with exporting still paused. It is declared here, not taken from
// the kind's own constants. The claim reads every holder kind, and each
// branch of the module compiles with only its own kind.
const readyReasonResumeFailed = "ResumeFailed"

// HolderActive reports whether the backup that holder names still needs the
// claim. It reads the resource of holder.Kind in the namespace. Absent, a
// UID other than holder.UID (a later resource with the same name), or a
// terminal phase means inactive. A kind that the API server does not serve
// means inactive too: no resource of that kind can exist.
//
// One terminal holder stays active: a backup whose Ready condition carries
// the reason ResumeFailed. Such a backup ended with the cluster's exporting
// still paused, and its claim is what keeps a sibling from backing up a
// paused cluster. The claim follows the pause, not the phase. It goes back
// when the holder's deletion resumes exporting, or when the holder is gone.
//
// The phase and condition vocabulary is shared by both backup kinds. The
// check therefore reads the resource without a typed client, and it works
// whichever kind this operator has compiled in. reader must read the API
// server directly: a cached read of the holder can be behind, and a stale
// "gone" or a stale phase would take a live claim over.
func HolderActive(ctx context.Context, reader client.Reader, namespace string, holder Claimant) (bool, error) {
	backup, err := holderResource(ctx, reader, namespace, holder)
	if err != nil || backup == nil {
		return false, err
	}

	phase, _, err := unstructured.NestedString(backup.Object, "status", "phase")
	if err != nil {
		return false, fmt.Errorf("reading the phase of the claim holder %s: %w", holder.Display(), err)
	}
	terminal := v1.LogicalBackupPhase(phase) == v1.LogicalBackupCompleted ||
		v1.LogicalBackupPhase(phase) == v1.LogicalBackupFailed
	if !terminal {
		return true, nil
	}

	resumeFailed, err := holderResumeFailed(backup, holder)
	if err != nil {
		return false, err
	}

	return resumeFailed, nil
}

// HolderKeepsClusterPaused reports whether the holder is a terminal backup
// that left the cluster's exporting paused: its Ready condition reason is
// ResumeFailed. A claimant blocked by such a holder tells the user more
// than "a backup runs". The cluster is paused, and it needs the holder's
// deletion or repair.
func HolderKeepsClusterPaused(
	ctx context.Context,
	reader client.Reader,
	namespace string,
	holder Claimant,
) (bool, error) {
	backup, err := holderResource(ctx, reader, namespace, holder)
	if err != nil || backup == nil {
		return false, err
	}
	return holderResumeFailed(backup, holder)
}

// holderResource reads the resource of the holder. It returns nil when the
// resource is gone or replaced under the same name. A kind that the API
// server does not serve is nil too.
func holderResource(
	ctx context.Context,
	reader client.Reader,
	namespace string,
	holder Claimant,
) (*unstructured.Unstructured, error) {
	backup := &unstructured.Unstructured{}
	backup.SetGroupVersionKind(v1.GroupVersion.WithKind(holder.Kind))
	key := client.ObjectKey{Namespace: namespace, Name: holder.Name}
	if err := reader.Get(ctx, key, backup); err != nil {
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading the claim holder %s %s: %w", holder.Kind, key, err)
	}
	if backup.GetUID() != holder.UID {
		return nil, nil
	}
	return backup, nil
}

// holderResumeFailed reports whether the holder ended as ResumeFailed. The
// recorded status.terminalReason decides first. A write conflict at the
// terminal transition can persist the phase and the terminal reason while
// it restores an older Ready condition. A decision from that stale
// condition would take a live paused-cluster claim over. A kind without the
// field falls back to the reason of the Ready condition.
func holderResumeFailed(backup *unstructured.Unstructured, holder Claimant) (bool, error) {
	terminalReason, found, err := unstructured.NestedString(backup.Object, "status", "terminalReason")
	if err != nil {
		return false, fmt.Errorf("reading the terminal reason of the claim holder %s: %w", holder.Display(), err)
	}
	if found && terminalReason != "" {
		return terminalReason == readyReasonResumeFailed, nil
	}

	items, _, err := unstructured.NestedSlice(backup.Object, "status", "conditions")
	if err != nil {
		return false, fmt.Errorf("reading the conditions of the claim holder %s: %w", holder.Display(), err)
	}
	for _, item := range items {
		condition, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if condition["type"] == v1.ConditionReady {
			return condition["reason"] == readyReasonResumeFailed, nil
		}
	}
	return false, nil
}

// Release gives the claim on the cluster back when self holds it. It is a
// no-op when the Lease does not exist or another claimant holds it. A caller
// can therefore release on every reconcile of a terminal resource and in its
// finalizer. The Delete carries the UID and the resourceVersion of the
// Lease as preconditions, both from the direct read through reader. A Lease
// that a takeover replaced in between is not deleted by the old holder.
func Release(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	namespace, cluster string,
	self Claimant,
) error {
	if err := deleteHeldBy(ctx, c, reader, namespace, cluster, self); err != nil {
		return fmt.Errorf("releasing the claim Lease of cluster %s/%s: %w", namespace, cluster, err)
	}
	return nil
}
