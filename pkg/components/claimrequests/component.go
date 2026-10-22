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

package claimrequests

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/pkg/claimpolicy"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/pluginrequest"
)

// ComponentName is the name of the requests component. The field manager of
// every request is <claim kind>/requests/<claim UID>.
const ComponentName = "requests"

// releaseRefParameter is the parameter that gets the release of the claim
// (CL55).
const releaseRefParameter = "releaseRef"

// Component builds the requests component of claim: at most one
// PluginRequest for each item of in.Order. A request whose inputs wait for
// outputs is guarded. A request of a section without a slot is Auxiliary,
// and it is left out while its inputs wait. Every request is blocked by a
// foreign controller reference. A selection whose section or registration
// is not in in is an error.
func Component(claim client.Object, in Input) (*component.Component, error) {
	builder := component.NewComponentBuilder().
		WithName(ComponentName).
		WithConditionType(component.ConditionType(in.ConditionType))

	for _, s := range in.Order {
		section := find(in.Sections, s.Section)
		if section == nil {
			return nil, fmt.Errorf(
				"selection %s/%s: PluginSection %q is not in the input",
				s.Section,
				s.Registration,
				s.Section,
			)
		}
		registration := find(in.Registrations, s.Registration)
		if registration == nil {
			return nil, fmt.Errorf(
				"selection %s/%s: PluginRegistration %q is not in the input", s.Section, s.Registration, s.Registration,
			)
		}

		inputs := claimpolicy.Inputs(registration, in.Selections, in.Requests)
		auxiliary := section.Spec.Slot == ""
		if auxiliary && len(inputs.Waiting) > 0 {
			// A guarded request would block the requests after it.
			continue
		}

		request, err := desiredRequest(claim, in, s, section, registration, inputs.Values)
		if err != nil {
			return nil, fmt.Errorf("selection %s/%s: %w", s.Section, s.Registration, err)
		}

		resource, err := pluginrequest.NewBuilder(request).WithGuard(waitFor(inputs.Waiting)).Build()
		if err != nil {
			return nil, fmt.Errorf("building the PluginRequest %s: %w", request.Name, err)
		}

		opts := []component.ResourceOption{component.BlockOnForeignController()}
		if auxiliary {
			opts = append(opts, component.Auxiliary())
		}
		builder.WithResource(resource, opts...)
	}

	return builder.Build()
}

// find returns the item of objects named name, or nil.
func find[T any, P interface {
	*T
	GetName() string
}](objects []T, name string) P {
	for i := range objects {
		if p := P(&objects[i]); p.GetName() == name {
			return p
		}
	}

	return nil
}

// desiredRequest returns the PluginRequest of selection s.
func desiredRequest(
	claim client.Object,
	in Input,
	s fleetv1alpha1.PluginSelection,
	section *pluginsv1alpha1.PluginSection,
	registration *pluginsv1alpha1.PluginRegistration,
	inputs map[string]string,
) (*pluginsv1alpha1.PluginRequest, error) {
	parameters, err := requestParameters(in, s, registration)
	if err != nil {
		return nil, err
	}

	return &pluginsv1alpha1.PluginRequest{
		TypeMeta: metav1.TypeMeta{APIVersion: pluginsv1alpha1.GroupVersion.String(), Kind: "PluginRequest"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      Name(claim.GetName(), s, section.Spec.Selection),
			Namespace: claim.GetNamespace(),
			Labels: map[string]string{
				pluginsv1alpha1.LabelSection:      s.Section,
				pluginsv1alpha1.LabelRegistration: s.Registration,
				fleetv1alpha1.LabelClaim:          claim.GetName(),
			},
		},
		Spec: pluginsv1alpha1.PluginRequestSpec{
			Section:      s.Section,
			Registration: s.Registration,
			ClaimRef:     pluginsv1alpha1.ClaimRef{Kind: in.ClaimKind, Name: claim.GetName()},
			Target: pluginsv1alpha1.TargetRef{
				Kind:                      in.TargetKind,
				Name:                      claim.GetName(),
				ServiceAccountName:        claimpolicy.TargetServiceAccount(claim.GetName(), section),
				AdditionalServiceAccounts: slices.Clone(in.AdditionalServiceAccounts[s.Section]),
			},
			Inputs:         inputs,
			Parameters:     parameters,
			DeletionPolicy: in.DeletionPolicy,
		},
	}, nil
}

// requestParameters returns the parameters of selection s from
// in.Parameters, with the release of the claim when registration declares
// it.
func requestParameters(
	in Input,
	s fleetv1alpha1.PluginSelection,
	registration *pluginsv1alpha1.PluginRegistration,
) (*apiextensionsv1.JSON, error) {
	var parameters *apiextensionsv1.JSON
	for _, p := range in.Parameters {
		if p.Section == s.Section && p.Registration == s.Registration {
			parameters = p.Parameters.DeepCopy()
			break
		}
	}

	if in.ReleaseRef == "" || !declaresRelease(registration) {
		return parameters, nil
	}

	var values map[string]json.RawMessage
	if parameters != nil {
		if err := json.Unmarshal(parameters.Raw, &values); err != nil {
			return nil, fmt.Errorf("the parameters are not a JSON object: %w", err)
		}
	}
	if values == nil {
		values = map[string]json.RawMessage{}
	}

	release, err := json.Marshal(in.ReleaseRef)
	if err != nil {
		return nil, err
	}
	values[releaseRefParameter] = release

	raw, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}

	return &apiextensionsv1.JSON{Raw: raw}, nil
}

// declaresRelease reports whether the parameter schema of registration has
// a top-level string property releaseRef.
func declaresRelease(registration *pluginsv1alpha1.PluginRegistration) bool {
	if registration.Spec.Parameters == nil {
		return false
	}

	var schema struct {
		Properties map[string]struct {
			Type string `json:"type"`
		} `json:"properties"`
	}
	if json.Unmarshal(registration.Spec.Parameters.Raw, &schema) != nil {
		return false
	}

	property, ok := schema.Properties[releaseRefParameter]

	return ok && property.Type == "string"
}

// waitFor returns the guard of a request whose inputs wait for the outputs
// of the sections in waiting. The guard closes over the value of the build,
// because the inputs of the request are fixed there.
func waitFor(waiting []string) func(pluginsv1alpha1.PluginRequest) (concepts.GuardStatusWithReason, error) {
	status := concepts.GuardStatusWithReason{Status: concepts.GuardStatusUnblocked}
	if len(waiting) > 0 {
		status = concepts.GuardStatusWithReason{
			Status: concepts.GuardStatusBlocked,
			Reason: "waiting for the outputs of " + strings.Join(waiting, ", "),
		}
	}

	return func(pluginsv1alpha1.PluginRequest) (concepts.GuardStatusWithReason, error) {
		return status, nil
	}
}
