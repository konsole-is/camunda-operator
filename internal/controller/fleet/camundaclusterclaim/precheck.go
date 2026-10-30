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
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/claimpolicy"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundaclusterclaim"
	"github.com/konsole-is/camunda-operator/pkg/components/claimrequests"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// resolved is what the precheck read and decided: the input of the snapshot
// and of the components.
type resolved struct {
	// environment is nil when no Environment applies to the namespace.
	environment   *fleetv1alpha1.Environment
	resolution    *claimpolicy.Resolution
	order         []fleetv1alpha1.PluginSelection
	sections      []pluginsv1alpha1.PluginSection
	registrations []pluginsv1alpha1.PluginRegistration
	// requests holds only the requests whose claimRef names this claim.
	requests                []pluginsv1alpha1.PluginRequest
	release, platformConfig string
	// releaseVersion is spec.version of the release. Empty without a
	// release.
	releaseVersion string
	// live is nil before the CamundaCluster of the claim exists.
	live *v1.CamundaCluster
}

// preCheck reads what the components of claim need and refuses the claim, in
// this order, with NoEnvironment or UnknownEnvironment, InvalidReference,
// NotAllowed, SlotConflict, and then MissingInput, WiringCycle, or
// InvalidReference from the wiring. A refusal is a
// *conditions.PreCheckFailure. It writes nothing.
func (r *Reconciler) preCheck(ctx context.Context, claim *fleetv1alpha1.CamundaClusterClaim) (resolved, error) {
	var res resolved

	var namespace corev1.Namespace
	if err := r.APIReader.Get(ctx, client.ObjectKey{Name: claim.Namespace}, &namespace); err != nil {
		return res, fmt.Errorf("reading the namespace: %w", err)
	}
	var environments fleetv1alpha1.EnvironmentList
	if err := r.List(ctx, &environments); err != nil {
		return res, fmt.Errorf("listing the Environments: %w", err)
	}
	environment, err := claimpolicy.EnvironmentOf(&namespace, environments.Items)
	if err != nil {
		return res, err
	}
	res.environment = environment

	var sections pluginsv1alpha1.PluginSectionList
	if err := r.List(ctx, &sections); err != nil {
		return res, fmt.Errorf("listing the PluginSections: %w", err)
	}
	var registrations pluginsv1alpha1.PluginRegistrationList
	if err := r.List(ctx, &registrations); err != nil {
		return res, fmt.Errorf("listing the PluginRegistrations: %w", err)
	}
	res.sections, res.registrations = sections.Items, registrations.Items
	res.release, res.platformConfig = components.References(claim, environment)

	release, err := r.checkReferences(ctx, claim, res)
	if err != nil {
		return res, err
	}
	if release != nil {
		res.releaseVersion = release.Spec.Version
	}

	if err := claimrequests.CheckParameterObjects(claim.Spec.Plugins); err != nil {
		return res, err
	}

	if err := claimpolicy.CheckServed(claim.Spec.Plugins, res.registrations); err != nil {
		return res, err
	}

	if err := claimrequests.CheckNames(claim.Name, claim.Spec.Plugins, res.sections); err != nil {
		return res, err
	}

	res.resolution, err = claimpolicy.Resolve(claimpolicy.ResolveInput{
		Recorded:      claim.Status.ResolvedPolicy,
		Environment:   environment,
		Registrations: res.registrations,
		Selections:    claim.Spec.Plugins,
	})
	if err != nil {
		return res, err
	}

	if err := claimpolicy.CheckAllowed(claimpolicy.AllowedInput{
		Environment: environment,
		Kind:        pluginsv1alpha1.ClaimKindCamundaClusterClaim,
		Sections:    res.sections,
		Policy:      res.resolution.Policy,
		PresetRef:   claim.Spec.PresetRef,
		ReleaseRef:  claim.Spec.ReleaseRef,
		Selections:  claim.Spec.Plugins,
	}); err != nil {
		return res, err
	}

	if err := claimpolicy.CheckDeletionPolicy(
		claimpolicy.DeletionPolicy(claim.Spec.DeletionPolicy, &res.resolution.Policy),
		claim.Spec.Plugins,
		res.registrations,
	); err != nil {
		return res, err
	}

	overrides := claimpolicy.Overrides{Cluster: claim.Spec.Cluster}
	if claim.Spec.Attachments != nil {
		overrides.Optimize = claim.Spec.Attachments.Optimize
	}
	if err := claimpolicy.CheckSlotConflicts(res.sections, claim.Spec.Plugins, overrides); err != nil {
		return res, err
	}

	if res.order, err = claimpolicy.Order(claim.Spec.Plugins, res.registrations); err != nil {
		return res, err
	}

	var requests pluginsv1alpha1.PluginRequestList
	if err := r.List(
		ctx,
		&requests,
		client.InNamespace(claim.Namespace),
		client.MatchingLabels{fleetv1alpha1.LabelClaim: claim.Name},
	); err != nil {
		return res, fmt.Errorf("listing the PluginRequests of the claim: %w", err)
	}
	res.requests = claimrequests.Own(
		requests.Items,
		pluginsv1alpha1.ClaimRef{Kind: pluginsv1alpha1.ClaimKindCamundaClusterClaim, Name: claim.Name},
		claim.Spec.Plugins,
		res.sections,
	)

	var live v1.CamundaCluster
	err = r.Get(ctx, client.ObjectKeyFromObject(claim), &live)
	switch {
	case err == nil:
		res.live = &live
	case !apierrors.IsNotFound(err):
		return res, fmt.Errorf("reading the CamundaCluster of the claim: %w", err)
	}

	return res, nil
}

// checkReferences refuses with InvalidReference a claim that names an
// object that does not exist. The message names each missing object: the
// preset, the release, the platform config, then each selected section and
// registration. An empty release or platform config is not checked. It
// returns the release that it read, or nil when the claim has none.
func (r *Reconciler) checkReferences(
	ctx context.Context,
	claim *fleetv1alpha1.CamundaClusterClaim,
	res resolved,
) (*v1.CamundaRelease, error) {
	var release v1.CamundaRelease
	var missing []string
	for _, ref := range []struct {
		kind, name string
		obj        client.Object
	}{
		{"CamundaClusterPreset", claim.Spec.PresetRef, &v1.CamundaClusterPreset{}},
		{"CamundaRelease", res.release, &release},
		{"CamundaPlatformConfig", res.platformConfig, &v1.CamundaPlatformConfig{}},
	} {
		if ref.name == "" {
			continue
		}

		err := r.Get(ctx, client.ObjectKey{Name: ref.name}, ref.obj)
		if apierrors.IsNotFound(err) {
			missing = append(missing, fmt.Sprintf("%s %q", ref.kind, ref.name))
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("reading %s %s: %w", ref.kind, ref.name, err)
		}
	}

	for _, s := range claim.Spec.Plugins {
		if !slices.ContainsFunc(res.sections, named[pluginsv1alpha1.PluginSection](s.Section)) {
			missing = append(missing, fmt.Sprintf("PluginSection %q", s.Section))
		}
	}
	for _, s := range claim.Spec.Plugins {
		if !slices.ContainsFunc(res.registrations, named[pluginsv1alpha1.PluginRegistration](s.Registration)) {
			missing = append(missing, fmt.Sprintf("PluginRegistration %q", s.Registration))
		}
	}

	if len(missing) > 0 {
		return nil, &conditions.PreCheckFailure{
			Reason:  v1.ReasonInvalidReference,
			Message: "the claim names objects that do not exist: " + strings.Join(missing, ", "),
		}
	}
	if res.release == "" {
		return nil, nil
	}

	return &release, nil
}

func named[T any, P interface {
	*T
	GetName() string
}](name string) func(T) bool {
	return func(item T) bool { return P(&item).GetName() == name }
}
