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
// waits for the writers of its backend counts the live registrations and needs
// no knowledge of their kinds.
//
// A registration is a Lease in the storage claim namespace. It is live until
// Duration after its last renewal, or after this operator started to lead
// when that is later (see Clock). A writer renews it at least every
// RenewInterval while it writes, and releases it when it stops.
package storagewriter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"sync/atomic"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

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

const (
	// Duration is how long a registration stays live after its last renewal.
	// A writer that stops without a release holds the backend this long.
	Duration = 2 * time.Minute
	// RenewInterval is the longest time between two renewals of a writer.
	RenewInterval = 30 * time.Second
)

const leasePrefix = "camunda-writer-"

// Clock records when this operator started to lead. A registration counts as
// renewed at that time at the latest. A nil Clock never started.
type Clock struct {
	started atomic.Int64
}

// Start records the time, unless a Since call recorded it first. The manager
// runs it once this operator leads.
func (c *Clock) Start(ctx context.Context) error {
	c.started.CompareAndSwap(0, time.Now().UnixNano())
	<-ctx.Done()

	return nil
}

// NeedLeaderElection makes the manager run Start only on the leader.
func (c *Clock) NeedLeaderElection() bool { return true }

// Since returns the time this operator started to lead, and records the
// current time when Start has not run yet. Call it only while this operator
// leads. A nil Clock returns the zero time.
func (c *Clock) Since() time.Time {
	if c == nil {
		return time.Time{}
	}
	c.started.CompareAndSwap(0, time.Now().UnixNano())

	return time.Unix(0, c.started.Load())
}

// Writer is a resource that writes a backend for the CamundaCluster with UID
// ClusterUID.
type Writer struct {
	Kind       string
	Namespace  string
	Name       string
	UID        types.UID
	ClusterUID types.UID
}

func (w Writer) String() string {
	return w.Kind + " " + w.Namespace + "/" + w.Name
}

// Register creates the registration of w on the backend key, or renews it when
// its last renewal is RenewInterval old or older. claim is the name of the
// storage claim Lease of key. namespace is the storage claim namespace.
func Register(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	namespace, key, claim string,
	w Writer,
	now time.Time,
) error {
	var lease coordinationv1.Lease
	err := reader.Get(ctx, types.NamespacedName{Namespace: namespace, Name: LeaseName(key, w.UID)}, &lease)
	if apierrors.IsNotFound(err) {
		err = c.Create(ctx, newLease(namespace, key, claim, w, now))
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("registering %s as a writer of %q: %w", w, key, err)
		}

		return nil
	}
	if err != nil {
		return fmt.Errorf("reading the writer Lease of %s: %w", w, err)
	}
	if lease.Spec.RenewTime != nil && now.Sub(lease.Spec.RenewTime.Time) < RenewInterval {
		return nil
	}

	lease.Spec.RenewTime = &metav1.MicroTime{Time: now}
	if err := c.Update(ctx, &lease); err != nil {
		return fmt.Errorf("renewing the writer Lease of %s: %w", w, err)
	}

	return nil
}

// Renew renews the registration of w on the backend key when it exists. It
// never creates one, so a writer that a stale list shows as live cannot bring
// back a registration it released.
func Renew(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	namespace, key string,
	w Writer,
	now time.Time,
) error {
	var lease coordinationv1.Lease
	err := reader.Get(ctx, types.NamespacedName{Namespace: namespace, Name: LeaseName(key, w.UID)}, &lease)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading the writer Lease of %s: %w", w, err)
	}

	lease.Spec.RenewTime = &metav1.MicroTime{Time: now}
	err = c.Update(ctx, &lease)
	if apierrors.IsNotFound(err) || apierrors.IsConflict(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("renewing the writer Lease of %s: %w", w, err)
	}

	return nil
}

// LeaseName returns the name of the registration of the writer with UID uid on
// the backend key.
func LeaseName(key string, uid types.UID) string {
	sum := sha256.Sum256([]byte(key + "|" + string(uid)))

	return leasePrefix + hex.EncodeToString(sum[:])[:40]
}

func newLease(namespace, key, claim string, w Writer, now time.Time) *coordinationv1.Lease {
	identity := labels.BoundedName(w.String(), 128)
	renewed := metav1.MicroTime{Time: now}

	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      LeaseName(key, w.UID),
			Labels:    leaseLabels(claim, w.ClusterUID),
			Annotations: map[string]string{
				KeyAnnotation:    key,
				WriterAnnotation: w.String(),
			},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &identity,
			AcquireTime:          &renewed,
			RenewTime:            &renewed,
			LeaseDurationSeconds: new(int32(Duration / time.Second)),
		},
	}
}

func leaseLabels(claim string, clusterUID types.UID) map[string]string {
	return map[string]string{
		labels.ManagedByKey:    labels.ManagedBy,
		labels.ComponentKey:    Component,
		labels.StorageClaimKey: labels.OwnerName(claim),
		labels.ClusterUIDKey:   string(clusterUID),
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

// Live returns the live writers of the backend key, as sorted "Kind
// namespace/name" entries, leaving out the writers for the cluster with UID
// self. claim is the name of the storage claim Lease of key. The reader must
// read the API server directly: a stale list lets a cluster start beside a
// writer. since is Clock.Since.
func Live(
	ctx context.Context,
	reader client.Reader,
	namespace, key, claim string,
	self types.UID,
	now, since time.Time,
) ([]string, error) {
	leases, err := registrations(ctx, reader, namespace, key, claim)
	if err != nil {
		return nil, err
	}

	var writers []string
	for _, lease := range leases {
		if lease.Labels[labels.ClusterUIDKey] == string(self) || expired(lease, now, since) {
			continue
		}
		writers = append(writers, lease.Annotations[WriterAnnotation])
	}
	slices.Sort(writers)

	return writers, nil
}

// PruneExpired deletes the expired registrations on the backend key. The
// delete carries the resource version that was read, so a registration that
// its writer renewed in between stays. since is Clock.Since.
func PruneExpired(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	namespace, key, claim string,
	now, since time.Time,
) error {
	leases, err := registrations(ctx, reader, namespace, key, claim)
	if err != nil {
		return err
	}

	return prune(ctx, c, leases, now, since)
}

// PruneAllExpired deletes the expired registrations of every backend in
// namespace, the same way PruneExpired does for one.
func PruneAllExpired(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	namespace string,
	now, since time.Time,
) error {
	var leases coordinationv1.LeaseList
	err := reader.List(
		ctx,
		&leases,
		client.InNamespace(namespace),
		client.MatchingLabels{labels.ManagedByKey: labels.ManagedBy, labels.ComponentKey: Component},
	)
	if err != nil {
		return fmt.Errorf("listing the writer Leases: %w", err)
	}

	all := make([]*coordinationv1.Lease, 0, len(leases.Items))
	for i := range leases.Items {
		all = append(all, &leases.Items[i])
	}

	return prune(ctx, c, all, now, since)
}

func prune(ctx context.Context, c client.Client, leases []*coordinationv1.Lease, now, since time.Time) error {
	for _, lease := range leases {
		if !expired(lease, now, since) {
			continue
		}
		err := c.Delete(ctx, lease, client.Preconditions{ResourceVersion: &lease.ResourceVersion})
		if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
			return fmt.Errorf("pruning the expired writer Lease %s: %w", lease.Name, err)
		}
	}

	return nil
}

// Janitor prunes the expired registrations in Namespace every Duration while
// this operator leads.
type Janitor struct {
	Client    client.Client
	Namespace string
	Clock     *Clock
}

// NeedLeaderElection makes the manager run Start only on the leader.
func (j *Janitor) NeedLeaderElection() bool { return true }

// Start prunes until ctx ends. The manager runs it once this operator leads.
func (j *Janitor) Start(ctx context.Context) error {
	ticker := time.NewTicker(Duration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := PruneAllExpired(ctx, j.Client, j.Client, j.Namespace, time.Now(), j.Clock.Since()); err != nil {
				log.FromContext(ctx).Error(err, "Could not prune the expired writer Leases")
			}
		}
	}
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

func expired(lease *coordinationv1.Lease, now, since time.Time) bool {
	if lease.Spec.RenewTime == nil || lease.Spec.LeaseDurationSeconds == nil {
		return true
	}
	duration := time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second
	renewed := lease.Spec.RenewTime.Time
	if since.After(renewed) {
		renewed = since
	}

	return !now.Before(renewed.Add(duration))
}

// IsWriterLease reports whether obj is a writer Lease.
func IsWriterLease(obj client.Object) bool {
	return obj.GetLabels()[labels.ComponentKey] == Component &&
		obj.GetLabels()[labels.ManagedByKey] == labels.ManagedBy
}
