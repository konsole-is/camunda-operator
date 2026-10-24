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
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/claimpolicy"
	"github.com/konsole-is/camunda-operator/pkg/planedatabases"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/camundamanagementcluster"
)

// managementClusterComponentName is the name of the management cluster
// component. The field manager of the CamundaManagementCluster is
// CamundaManagementClusterClaim/managementcluster/<claim UID>.
const managementClusterComponentName = "managementcluster"

// slotField returns the string field of a slot path in spec, or nil while
// spec does not run the component of the field.
type slotField func(spec *v1.CamundaManagementClusterSpec) *string

// slotFields holds the field of each slot path of kind
// CamundaManagementCluster. The API server refuses a plane in which a field
// of a component that it runs is empty.
var slotFields = map[string]slotField{
	claimpolicy.PathKeycloakExternalURL: keycloakField(func(k *v1.ManagedKeycloakSpec) *string {
		return &k.ExternalURL
	}),
	planedatabases.Of(planedatabases.Keycloak).FieldPath: keycloakField(func(k *v1.ManagedKeycloakSpec) *string {
		return &k.DatabaseConfigRef
	}),
	claimpolicy.PathIdentityExternalURL: func(s *v1.CamundaManagementClusterSpec) *string {
		return &s.Identity.ExternalURL
	},
	planedatabases.Of(planedatabases.Identity).FieldPath: func(s *v1.CamundaManagementClusterSpec) *string {
		return &s.Identity.DatabaseConfigRef
	},
	claimpolicy.PathConsoleExternalURL: func(s *v1.CamundaManagementClusterSpec) *string {
		if s.Console == nil {
			return nil
		}
		return &s.Console.ExternalURL
	},
	claimpolicy.PathWebModelerExternalURL: webModelerField(func(w *v1.WebModelerSpec) *string {
		return &w.ExternalURL
	}),
	claimpolicy.PathWebModelerWebsocketsExternalURL: webModelerField(func(w *v1.WebModelerSpec) *string {
		return &w.WebsocketsExternalURL
	}),
	planedatabases.Of(planedatabases.WebModeler).FieldPath: webModelerField(func(w *v1.WebModelerSpec) *string {
		return &w.DatabaseConfigRef
	}),
}

func keycloakField(field func(*v1.ManagedKeycloakSpec) *string) slotField {
	return func(s *v1.CamundaManagementClusterSpec) *string {
		if s.IdentityProvider.Keycloak == nil {
			return nil
		}
		return field(s.IdentityProvider.Keycloak)
	}
}

func webModelerField(field func(*v1.WebModelerSpec) *string) slotField {
	return func(s *v1.CamundaManagementClusterSpec) *string {
		if s.WebModeler == nil {
			return nil
		}
		return field(s.WebModeler)
	}
}

// managementClusterComponent builds the component of the
// CamundaManagementCluster of the claim. It waits for RequestsReady, and it
// holds back each apply while a field that the plane requires has no value.
func managementClusterComponent(in Input) (*component.Component, error) {
	desired, waiting, err := managementCluster(in)
	if err != nil {
		return nil, err
	}

	resource, err := camundamanagementcluster.NewBuilder(desired).WithGuard(waitFor(waiting)).Build()
	if err != nil {
		return nil, fmt.Errorf("building the CamundaManagementCluster %s: %w", desired.Name, err)
	}

	return component.NewComponentBuilder().
		WithName(managementClusterComponentName).
		WithConditionType(component.ConditionType(fleetv1alpha1.ConditionManagementClusterReady)).
		WithPrerequisite(component.DependsOn(component.ConditionType(fleetv1alpha1.ConditionPlaneRequestsReady))).
		WithResource(resource, component.BlockOnForeignController()).
		Build()
}

// waitFor returns the guard of the CamundaManagementCluster. It blocks with
// the reason waiting when waiting is not empty.
func waitFor(waiting string) func(v1.CamundaManagementCluster) (concepts.GuardStatusWithReason, error) {
	status := concepts.GuardStatusWithReason{Status: concepts.GuardStatusUnblocked}
	if waiting != "" {
		status = concepts.GuardStatusWithReason{Status: concepts.GuardStatusBlocked, Reason: waiting}
	}

	return func(v1.CamundaManagementCluster) (concepts.GuardStatusWithReason, error) {
		return status, nil
	}
}

// managementCluster returns the CamundaManagementCluster of the claim:
// spec.management of the claim, the fields that the claim controller sets,
// and the slot fields from the outputs. A slot field whose output is missing
// keeps its value in in.Live. waiting is empty, or it names the sections
// whose outputs leave a field that the plane requires empty.
func managementCluster(in Input) (desired *v1.CamundaManagementCluster, waiting string, err error) {
	spec := planeSpec(in)

	fields, err := claimpolicy.SlotFields(
		pluginsv1alpha1.ClaimKindCamundaManagementClusterClaim, in.Sections, in.Claim.Spec.Plugins, in.Requests,
	)
	if err != nil {
		return nil, "", err
	}

	sections := map[string]string{}
	for _, f := range fields {
		if f.Kind != claimpolicy.KindCamundaManagementCluster {
			continue
		}

		field, ok := slotFields[f.Path]
		if !ok {
			return nil, "", fmt.Errorf(
				"no CamundaManagementCluster field for the slot path %s of section %q",
				f.Path,
				f.Section,
			)
		}
		sections[f.Path] = f.Section

		target := field(&spec)
		switch {
		case target == nil:
		case f.Value != nil:
			if err := json.Unmarshal(f.Value.Raw, target); err != nil {
				return nil, "", fmt.Errorf("setting %s from the outputs of section %q: %w", f.Path, f.Section, err)
			}
		case in.Live != nil:
			if live := field(in.Live.Spec.DeepCopy()); live != nil {
				*target = *live
			}
		}
	}

	return &v1.CamundaManagementCluster{
		TypeMeta: metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "CamundaManagementCluster"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      in.Claim.Name,
			Namespace: in.Claim.Namespace,
			Labels:    map[string]string{fleetv1alpha1.LabelClaim: in.Claim.Name},
		},
		Spec: spec,
	}, emptyFields(&spec, sections), nil
}

// planeSpec returns the spec of the plane from the claim of in, without the
// slot fields that the outputs fill.
func planeSpec(in Input) v1.CamundaManagementClusterSpec {
	claim := in.Claim.Spec.DeepCopy()
	management, versions := claim.Management, claim.Versions

	spec := v1.CamundaManagementClusterSpec{
		PlatformConfigRef:        in.PlatformConfigRef,
		Suspend:                  claim.Suspend,
		ClusterSelector:          claim.ClusterSelector,
		NamespaceSelector:        namespaceSelector(claim.Environments),
		ManagementAuthConfigName: claim.ManagementAuthConfigName,
		IdentityProvider: v1.IdentityProviderSpec{
			ExternalKeycloak: management.IdentityProvider.ExternalKeycloak,
			OIDC:             management.IdentityProvider.OIDC,
		},
		Identity: v1.IdentitySpec{
			Version:      versions.Identity,
			ExternalURL:  management.Identity.ExternalURL,
			Admin:        management.Identity.Admin,
			WorkloadSpec: management.Identity.WorkloadSpec,
		},
	}
	if keycloak := management.IdentityProvider.Keycloak; keycloak != nil {
		spec.IdentityProvider.Keycloak = &v1.ManagedKeycloakSpec{
			Version:     versions.Keycloak,
			ExternalURL: keycloak.ExternalURL,
			Replicas:    keycloak.Replicas,
			Resources:   keycloak.Resources,
			Scheduling:  keycloak.Scheduling,
		}
	}
	if console := management.Console; console != nil {
		spec.Console = &v1.ConsoleSpec{
			Version:      versions.Console,
			ExternalURL:  console.ExternalURL,
			WorkloadSpec: console.WorkloadSpec,
		}
	}
	if webModeler := management.WebModeler; webModeler != nil {
		spec.WebModeler = &v1.WebModelerSpec{
			Version:               versions.WebModeler,
			ExternalURL:           webModeler.ExternalURL,
			WebsocketsExternalURL: webModeler.WebsocketsExternalURL,
			Mail:                  webModeler.Mail,
			Restapi:               webModeler.Restapi,
			Websockets:            webModeler.Websockets,
		}
	}

	return spec
}

// namespaceSelector selects the namespaces of environments.
func namespaceSelector(environments []string) *metav1.LabelSelector {
	return &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{
		Key:      fleetv1alpha1.LabelEnvironment,
		Operator: metav1.LabelSelectorOpIn,
		Values:   environments,
	}}}
}

// emptyFields returns the guard reason of spec: the sections, sorted and
// distinct, whose slot field of a component that spec runs is empty, and
// each such path that no section fills. sections maps a slot path to the
// section that fills it. It returns "" when every field has a value.
func emptyFields(spec *v1.CamundaManagementClusterSpec, sections map[string]string) string {
	var waiting, unfilled []string
	for _, path := range slices.Sorted(maps.Keys(slotFields)) {
		value := slotFields[path](spec)
		if value == nil || *value != "" {
			continue
		}

		if section, ok := sections[path]; ok {
			waiting = append(waiting, section)
		} else {
			unfilled = append(unfilled, path)
		}
	}
	slices.Sort(waiting)

	var reasons []string
	if reason := claimpolicy.WaitingReason(slices.Compact(waiting)); reason != "" {
		reasons = append(reasons, reason)
	}
	if len(unfilled) > 0 {
		reasons = append(reasons, strings.Join(unfilled, ", ")+" has no value")
	}

	return strings.Join(reasons, "; ")
}
