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

package camundamanagementclusterclaim

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
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementclusterclaim"
	"github.com/konsole-is/camunda-operator/pkg/components/claimrequests"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// resolved is what the precheck read and decided: the input of the snapshot
// and of the components.
type resolved struct {
	// environment is nil when no Environment applies to the namespace.
	environment *fleetv1alpha1.Environment
	resolution  *claimpolicy.Resolution
	// parameters are resolution.Requests with the databases parameter.
	parameters    []claimpolicy.RequestParameters
	order         []fleetv1alpha1.PluginSelection
	sections      []pluginsv1alpha1.PluginSection
	registrations []pluginsv1alpha1.PluginRegistration
	// requests holds only the requests whose claimRef names this claim.
	requests       []pluginsv1alpha1.PluginRequest
	platformConfig string
	// live is nil before the CamundaManagementCluster of the claim exists.
	live *v1.CamundaManagementCluster
}

// preCheck reads what the components of claim need and refuses the claim, in
// this order, with NoEnvironment or UnknownEnvironment, InvalidReference,
// NotAllowed, SlotConflict, and then MissingInput, WiringCycle, or
// InvalidReference from the wiring. A refusal is a
// *conditions.PreCheckFailure. It writes nothing.
func (r *Reconciler) preCheck(
	ctx context.Context,
	claim *fleetv1alpha1.CamundaManagementClusterClaim,
) (resolved, error) {
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
	res.platformConfig = components.PlatformConfigRef(claim, environment)

	if err := r.checkReferences(ctx, claim, environments.Items, res); err != nil {
		return res, err
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
		Kind:        pluginsv1alpha1.ClaimKindCamundaManagementClusterClaim,
		Sections:    res.sections,
		Policy:      res.resolution.Policy,
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

	if err := claimpolicy.CheckSlotConflicts(
		res.sections,
		claim.Spec.Plugins,
		claimpolicy.Overrides{Management: &claim.Spec.Management},
	); err != nil {
		return res, err
	}

	if err := checkPlaneInputs(claim, res); err != nil {
		return res, err
	}

	order := components.WriteOrder(claim.Spec.Plugins, res.sections)
	if res.order, err = claimpolicy.Order(order, res.registrations); err != nil {
		return res, err
	}

	res.parameters, err = components.WithDatabases(
		res.resolution.Requests,
		res.sections,
		components.Databases(claim.Spec.Management),
	)
	if err != nil {
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
		pluginsv1alpha1.ClaimRef{Kind: pluginsv1alpha1.ClaimKindCamundaManagementClusterClaim, Name: claim.Name},
		claim.Spec.Plugins,
		res.sections,
	)

	var live v1.CamundaManagementCluster
	err = r.Get(ctx, client.ObjectKeyFromObject(claim), &live)
	switch {
	case err == nil:
		res.live = &live
	case !apierrors.IsNotFound(err):
		return res, fmt.Errorf("reading the CamundaManagementCluster of the claim: %w", err)
	}

	return res, nil
}

// checkReferences refuses with InvalidReference a claim that names an
// object that does not exist. The message names each missing object: each
// served Environment, the platform config, then each selected section and
// registration. An empty platform config is not checked.
func (r *Reconciler) checkReferences(
	ctx context.Context,
	claim *fleetv1alpha1.CamundaManagementClusterClaim,
	environments []fleetv1alpha1.Environment,
	res resolved,
) error {
	var missing []string
	for _, name := range claim.Spec.Environments {
		if !slices.ContainsFunc(environments, named[fleetv1alpha1.Environment](name)) {
			missing = append(missing, fmt.Sprintf("Environment %q", name))
		}
	}

	if res.platformConfig != "" {
		err := r.Get(ctx, client.ObjectKey{Name: res.platformConfig}, &v1.CamundaPlatformConfig{})
		if apierrors.IsNotFound(err) {
			missing = append(missing, fmt.Sprintf("CamundaPlatformConfig %q", res.platformConfig))
		} else if err != nil {
			return fmt.Errorf("reading CamundaPlatformConfig %s: %w", res.platformConfig, err)
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

	if len(missing) == 0 {
		return nil
	}

	return &conditions.PreCheckFailure{
		Reason:  v1.ReasonInvalidReference,
		Message: "the claim names objects that do not exist: " + strings.Join(missing, ", "),
	}
}

func named[T any, P interface {
	*T
	GetName() string
}](name string) func(T) bool {
	return func(item T) bool { return P(&item).GetName() == name }
}

// checkPlaneInputs refuses with MissingInput a claim from which no valid
// plane can be written: one without a selection in a section with the slot
// managementDatabases, one whose plane gets no platform config, and one
// without an access selection that leaves a URL of the plane empty.
func checkPlaneInputs(claim *fleetv1alpha1.CamundaManagementClusterClaim, res resolved) error {
	refuse := func(message string) error {
		return &conditions.PreCheckFailure{Reason: fleetv1alpha1.ReasonMissingInput, Message: message}
	}

	if !components.Selects(claim.Spec.Plugins, res.sections, pluginsv1alpha1.SlotManagementDatabases) {
		return refuse("the plane needs a selection in a section with the slot managementDatabases")
	}

	if res.platformConfig == "" {
		return refuse("the claim and its Environment name no CamundaPlatformConfig")
	}

	if components.Selects(claim.Spec.Plugins, res.sections, pluginsv1alpha1.SlotExternalAccess) {
		return nil
	}
	if missing := components.MissingURLs(claim.Spec.Management); len(missing) > 0 {
		return refuse("without an access selection, the claim must set " + strings.Join(missing, ", "))
	}

	return nil
}
