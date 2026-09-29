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

package restore

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
)

// Progress is how far a step of a restore got. A step that is not done is a
// wait, not a failure: the caller polls until Done.
type Progress struct {
	// Done reports that the step finished.
	Done bool
	// Message says what the step waits for. It reaches the Ready condition.
	Message string
	// Recreated names every claim whose old volume is gone: the input list
	// and the names that this call added. The caller writes it to status
	// before it acts again.
	Recreated []string
}

// ClaimInput is everything RecreateClaims needs.
type ClaimInput struct {
	// Target holds the broker StatefulSet the claims belong to.
	Target *Target
	// Size is the storage request of every recreated claim.
	Size resource.Quantity
	// Recreated names the claims whose old volume is already gone, as status
	// records them. A recorded name is never deleted, so the empty volume
	// that took its place survives a reconcile that re-enters.
	Recreated []string
	// FieldManager applies the claims: the field manager of the calling
	// restore kind.
	FieldManager client.FieldOwner
}

// RecreateClaims gives the brokers the empty data volumes that the restore
// application needs. It deletes each claim that in.Recreated does not name,
// and it reports Done only when every claim exists again as an empty volume.
//
// Persist Progress.Recreated before you apply any restore Job. After a Job
// starts to write, a call without that record erases restored data.
//
// reader must be uncached, because the next decision reads a claim that was
// just deleted or applied. reader also lists the pods of the namespace.
func RecreateClaims(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	in ClaimInput,
) (Progress, error) {
	if err := in.Target.complete(); err != nil {
		return Progress{}, fmt.Errorf("recreating the broker volumes: %w", err)
	}

	progress := Progress{Done: true, Recreated: slices.Clone(in.Recreated)}

	for ordinal, name := range in.Target.ClaimNames() {
		key := types.NamespacedName{Namespace: in.Target.StatefulSet.Namespace, Name: name}

		var current corev1.PersistentVolumeClaim
		err := reader.Get(ctx, key, &current)
		if err != nil && !apierrors.IsNotFound(err) {
			return Progress{}, fmt.Errorf("reading the broker volume %s: %w", name, err)
		}

		switch {
		case apierrors.IsNotFound(err):
			if !slices.Contains(progress.Recreated, name) {
				progress.Recreated = append(progress.Recreated, name)
			}

			claim := in.Target.BuildClaim(int32(ordinal), in.Size)
			if err := Apply(ctx, c, claim, in.FieldManager); err != nil {
				return Progress{}, fmt.Errorf("applying the broker volume %s: %w", name, err)
			}
		case !slices.Contains(progress.Recreated, name):
			// A claim that is not recorded yet still holds the data of the cluster.
			if err := c.Delete(ctx, &current); err != nil && !apierrors.IsNotFound(err) {
				return Progress{}, fmt.Errorf("deleting the broker volume %s: %w", name, err)
			}
			progress.Recreated = append(progress.Recreated, name)
			progress.hold(fmt.Sprintf("the broker volume %s is deleted and comes back empty", name))
		case current.DeletionTimestamp != nil:
			// A pod still holds the volume. Naming the pod lists the pods of the
			// namespace, and only the first hold reaches the caller.
			if progress.Done {
				progress.hold(terminatingMessage(ctx, reader, in.Target, name))
			}
		default:
			claim := in.Target.BuildClaim(int32(ordinal), in.Size)
			if err := Apply(ctx, c, claim, in.FieldManager); err != nil {
				return Progress{}, fmt.Errorf("applying the broker volume %s: %w", name, err)
			}
		}
	}

	return progress, nil
}

// ClaimNames returns the names of the broker data claims, in broker order.
// The name is what the StatefulSet expects: data-<cluster>-zeebe-<ordinal>.
func (t *Target) ClaimNames() []string {
	names := make([]string, 0, t.Brokers)
	for ordinal := range t.Brokers {
		names = append(names, t.claimName(ordinal))
	}

	return names
}

func (t *Target) claimName(ordinal int32) string {
	return components.DataVolumeName + "-" + t.StatefulSet.Name + "-" + strconv.FormatInt(int64(ordinal), 10)
}

// BuildClaim renders the broker data claim of one ordinal at the given size.
// Everything but the size comes from the claim template, so the discovery
// labels keep working. The claim carries no owner
// reference: one to a restore deletes a live broker volume with the restore.
func (t *Target) BuildClaim(ordinal int32, size resource.Quantity) *corev1.PersistentVolumeClaim {
	spec := *t.ClaimTemplate.Spec.DeepCopy()
	spec.Resources.Requests = corev1.ResourceList{corev1.ResourceStorage: size}

	return &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "PersistentVolumeClaim"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        t.claimName(ordinal),
			Namespace:   t.StatefulSet.Namespace,
			Labels:      maps.Clone(t.ClaimTemplate.Labels),
			Annotations: maps.Clone(t.ClaimTemplate.Annotations),
		},
		Spec: spec,
	}
}

// hold records that the step is not done. The first claim that holds it names
// the message, so the message names the lowest broker that waits.
func (p *Progress) hold(message string) {
	if !p.Done {
		return
	}
	p.Done = false
	p.Message = message
}

// ClaimSize returns the storage request of a recreated broker volume. Pass
// the effective restore size that the backup recorded as recorded, or nil to
// get the request of the claim template, which can be too small for the data
// of the backup.
func (t *Target) ClaimSize(recorded *resource.Quantity) resource.Quantity {
	if recorded != nil {
		return *recorded
	}

	return t.ClaimTemplate.Spec.Resources.Requests[corev1.ResourceStorage]
}
