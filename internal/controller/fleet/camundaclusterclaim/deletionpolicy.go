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

package camundaclusterclaim

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/pkg/claimpolicy"
	"github.com/konsole-is/camunda-operator/pkg/components/claimrequests"
)

// syncDeletionPolicy writes the effective deletion policy of claim on the
// requests that claim controls and that have no deletion timestamp, unless a
// selected registration does not support the policy.
func (r *Reconciler) syncDeletionPolicy(ctx context.Context, claim *fleetv1alpha1.CamundaClusterClaim) error {
	requests, err := r.ownedRequests(ctx, claim)
	if err != nil {
		return err
	}

	return r.writeDeletionPolicy(ctx, claim, requests)
}

// ownedRequests reads the PluginRequests in the namespace of claim that
// claim controls, without the cache.
func (r *Reconciler) ownedRequests(
	ctx context.Context,
	claim *fleetv1alpha1.CamundaClusterClaim,
) ([]client.Object, error) {
	// Not by the claim label: a user can remove a label, and the claim must
	// still find the request.
	var list pluginsv1alpha1.PluginRequestList
	if err := r.APIReader.List(ctx, &list, client.InNamespace(claim.Namespace)); err != nil {
		return nil, fmt.Errorf("listing the PluginRequests of the claim: %w", err)
	}

	var owned []client.Object
	for i := range list.Items {
		if metav1.IsControlledBy(&list.Items[i], claim) {
			owned = append(owned, &list.Items[i])
		}
	}

	return owned, nil
}

// writeDeletionPolicy applies the effective deletion policy of claim to each
// of requests that has no deletion timestamp and another policy. It writes
// nothing when a selected registration does not support the policy, so the
// requests keep the policy that they have.
func (r *Reconciler) writeDeletionPolicy(
	ctx context.Context,
	claim *fleetv1alpha1.CamundaClusterClaim,
	requests []client.Object,
) error {
	policy := claimpolicy.DeletionPolicy(claim.Spec.DeletionPolicy, claim.Status.ResolvedPolicy)
	var registrations pluginsv1alpha1.PluginRegistrationList
	if err := r.List(ctx, &registrations); err != nil {
		return fmt.Errorf("listing the PluginRegistrations: %w", err)
	}
	// The check of the claim refuses this policy and reports it on Ready.
	if claimpolicy.CheckDeletionPolicy(policy, claim.Spec.Plugins, registrations.Items) != nil {
		return nil
	}

	owner := client.FieldOwner(
		claimrequests.DeletionPolicyFieldManager(fleetv1alpha1.FieldManagerPrefixClusterClaim, claim.UID),
	)
	for _, obj := range requests {
		request, ok := obj.(*pluginsv1alpha1.PluginRequest)
		if !ok || !request.DeletionTimestamp.IsZero() || request.Spec.DeletionPolicy == policy {
			continue
		}

		apply := claimrequests.DeletionPolicyApply(request, policy)
		//nolint:staticcheck // the repository applies through the deprecated client.Apply patch
		if err := r.Patch(ctx, apply, client.Apply, owner, client.ForceOwnership); err != nil {
			return fmt.Errorf("writing the deletion policy on PluginRequest %s: %w", request.Name, err)
		}
	}

	return nil
}
