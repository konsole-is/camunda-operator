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

package logicalbackup

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

// The backups of one cluster run one at a time, across both backup kinds.
// The claim on a cluster is a coordination.k8s.io Lease that the API server
// creates atomically: the first Create wins, every later Create answers
// AlreadyExists. That makes the claim correct under concurrent reconciles and
// across the two controllers, where a check-then-act on the status of the
// siblings is not. The Lease is authoritative. The in-kind tie-break among
// pending backups stays a fairness pre-filter in front of it: it decides who
// tries the claim first, not who holds it.
//
// A holder that is gone, replaced, or terminal no longer needs the claim. A
// claimant that finds such a holder takes the Lease over, so a crash between
// the claim and the release never blocks the cluster forever.

// claimLeasePrefix starts the name of every claim Lease.
const claimLeasePrefix = "camunda-backup-"

// maxLeaseNameLength is the DNS subdomain bound of a Lease name.
const maxLeaseNameLength = 253

// ClaimLeaseName returns the name of the Lease that claims the cluster:
// "camunda-backup-<cluster>". The name is deterministic, so every claimant of
// one cluster meets on the same Lease. A cluster name that pushes the Lease
// name past the DNS subdomain bound is cut and suffixed with a hash of the
// full name. The result stays deterministic and unique.
func ClaimLeaseName(cluster string) string {
	name := claimLeasePrefix + cluster
	if len(name) <= maxLeaseNameLength {
		return name
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(cluster))
	suffix := fmt.Sprintf("-%08x", hash.Sum32())
	return name[:maxLeaseNameLength-len(suffix)] + suffix
}

// Claimant identifies the backup that holds or wants the claim: the kind of
// its resource, its name, and its UID. The UID tells a resource apart from a
// later one with the same name.
type Claimant struct {
	// Kind is the kind of the backup resource, LogicalBackupElasticsearch
	// or LogicalBackupRDBMS.
	Kind string
	// Name is the name of the backup resource. It lives in the namespace of
	// the cluster, like the Lease.
	Name string
	// UID is the UID of the backup resource.
	UID types.UID
}

// String returns the holder identity that the Lease records:
// "<Kind>/<Name>/<UID>".
func (c Claimant) String() string {
	return c.Kind + "/" + c.Name + "/" + string(c.UID)
}

// Display returns the short form for a message: "<Kind>/<Name>".
func (c Claimant) Display() string {
	return c.Kind + "/" + c.Name
}

// ParseClaimant reads a holder identity that String wrote. It rejects any
// other shape, so a Lease that something else wrote is never mistaken for a
// backup and never taken over.
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
// identity of the backup that holds the claim, and self must wait.
//
// A holder that HolderActive reports inactive is taken over. The Lease is
// deleted with its UID and resourceVersion as preconditions, and the Create
// is tried once more. A holder identity that ParseClaimant rejects blocks
// without a takeover.
//
// Reads go through c. A stale read fails closed: the claimant waits one more
// reconcile, it never takes a claim it does not hold.
func Claim(ctx context.Context, c client.Client, namespace, cluster string, self Claimant) (string, error) {
	holder, err := create(ctx, c, namespace, cluster, self)
	if err != nil || holder == "" || holder == self.String() {
		return holder, err
	}

	claimant, parseErr := ParseClaimant(holder)
	if parseErr != nil {
		return holder, nil
	}
	active, err := HolderActive(ctx, c, namespace, claimant)
	if err != nil {
		return "", err
	}
	if active {
		return holder, nil
	}

	if err := takeOver(ctx, c, namespace, cluster, holder); err != nil {
		return "", err
	}

	return create(ctx, c, namespace, cluster, self)
}

// create tries to create the Lease for self. It returns "" when self holds
// the Lease after the call, or the current holder identity.
func create(ctx context.Context, c client.Client, namespace, cluster string, self Claimant) (string, error) {
	for range 2 {
		lease := newLease(namespace, cluster, self)
		err := c.Create(ctx, lease)
		if err == nil {
			return "", nil
		}
		if !apierrors.IsAlreadyExists(err) {
			return "", fmt.Errorf("creating the claim Lease %s/%s: %w", namespace, lease.Name, err)
		}

		holder, found, err := currentHolder(ctx, c, namespace, cluster)
		if err != nil {
			return "", err
		}
		if !found {
			// The Lease went away between the Create and the Get: a release
			// or a takeover raced this claimant. Try the Create once more.
			continue
		}
		if holder == self.String() {
			return "", nil
		}
		return holder, nil
	}

	return "", fmt.Errorf("the claim Lease of cluster %s/%s exists but is not readable yet", namespace, cluster)
}

// Holds reports whether self holds the claim on the cluster now. A
// re-entering claimant asks this before it runs any ordering among the
// pending backups. A backup that holds the claim goes first, whatever the
// tie-break says. Otherwise the tie-break and the claim block each other.
func Holds(ctx context.Context, reader client.Reader, namespace, cluster string, self Claimant) (bool, error) {
	holder, found, err := currentHolder(ctx, reader, namespace, cluster)
	if err != nil {
		return false, err
	}
	return found && holder == self.String(), nil
}

// unidentifiedHolder stands in for the holder of a Lease that records no
// holder identity. Nothing of this operator writes such a Lease. It blocks
// the claim, and ParseClaimant rejects it, so it is never taken over.
const unidentifiedHolder = "unidentified holder"

// currentHolder returns the holder identity of the Lease and whether the
// Lease exists.
func currentHolder(ctx context.Context, c client.Reader, namespace, cluster string) (string, bool, error) {
	var lease coordinationv1.Lease
	key := client.ObjectKey{Namespace: namespace, Name: ClaimLeaseName(cluster)}
	if err := c.Get(ctx, key, &lease); err != nil {
		if apierrors.IsNotFound(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("reading the claim Lease %s: %w", key, err)
	}
	if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity == "" {
		return unidentifiedHolder, true, nil
	}
	return *lease.Spec.HolderIdentity, true, nil
}

// takeOver deletes the Lease while it still records holder. A Lease that
// changed hands in between is left alone. The Delete carries the UID and the
// resourceVersion of the Lease as preconditions. A conflict or a NotFound is
// not an error.
func takeOver(ctx context.Context, c client.Client, namespace, cluster, holder string) error {
	var lease coordinationv1.Lease
	key := client.ObjectKey{Namespace: namespace, Name: ClaimLeaseName(cluster)}
	if err := c.Get(ctx, key, &lease); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("reading the claim Lease %s: %w", key, err)
	}
	if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity != holder {
		return nil
	}

	err := c.Delete(ctx, &lease, client.Preconditions{
		UID:             &lease.UID,
		ResourceVersion: &lease.ResourceVersion,
	})
	if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
		return fmt.Errorf("taking over the claim Lease %s: %w", key, err)
	}

	return nil
}

// newLease builds the claim Lease of cluster for self.
func newLease(namespace, cluster string, self Claimant) *coordinationv1.Lease {
	holder := self.String()
	now := metav1.NowMicro()
	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      ClaimLeaseName(cluster),
			Labels:    labels.Managed(labels.Cluster(cluster), "backup-claim"),
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity: &holder,
			AcquireTime:    &now,
		},
	}
}

// HolderActive reports whether the backup that holder names still needs the
// claim. It reads the resource of holder.Kind in the namespace. Absent, a
// UID other than holder.UID (a later resource with the same name), or a
// terminal phase means inactive. A kind that the API server does not serve
// means inactive too: no resource of that kind can exist. The phase
// vocabulary is shared by both backup kinds. The check therefore reads the
// resource without a typed client, and it works whichever kind this
// operator has compiled in.
func HolderActive(ctx context.Context, reader client.Reader, namespace string, holder Claimant) (bool, error) {
	backup := &unstructured.Unstructured{}
	backup.SetGroupVersionKind(v1.GroupVersion.WithKind(holder.Kind))
	key := client.ObjectKey{Namespace: namespace, Name: holder.Name}
	if err := reader.Get(ctx, key, backup); err != nil {
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return false, nil
		}
		return false, fmt.Errorf("reading the claim holder %s %s: %w", holder.Kind, key, err)
	}
	if backup.GetUID() != holder.UID {
		return false, nil
	}

	phase, _, err := unstructured.NestedString(backup.Object, "status", "phase")
	if err != nil {
		return false, fmt.Errorf("reading the phase of the claim holder %s %s: %w", holder.Kind, key, err)
	}
	terminal := v1.LogicalBackupPhase(phase) == v1.LogicalBackupCompleted ||
		v1.LogicalBackupPhase(phase) == v1.LogicalBackupFailed

	return !terminal, nil
}

// Release gives the claim on the cluster back when self holds it. It is a
// no-op when the Lease does not exist or another claimant holds it. A caller
// can therefore release on every reconcile of a terminal backup and in its
// finalizer. The Delete carries the UID and the resourceVersion of the
// Lease as preconditions. A Lease that a takeover replaced in between is not
// deleted by the old holder.
func Release(ctx context.Context, c client.Client, namespace, cluster string, self Claimant) error {
	var lease coordinationv1.Lease
	key := client.ObjectKey{Namespace: namespace, Name: ClaimLeaseName(cluster)}
	if err := c.Get(ctx, key, &lease); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("reading the claim Lease %s: %w", key, err)
	}
	if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity != self.String() {
		return nil
	}

	err := c.Delete(ctx, &lease, client.Preconditions{
		UID:             &lease.UID,
		ResourceVersion: &lease.ResourceVersion,
	})
	if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
		return fmt.Errorf("releasing the claim Lease %s: %w", key, err)
	}

	return nil
}
