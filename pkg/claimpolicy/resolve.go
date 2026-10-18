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

package claimpolicy

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// errNotAnObject is the error of a value that must be a JSON object. It
// holds no text of the JSON decoder, so that the cases keep their bytes from
// one Go version to the next.
var errNotAnObject = errors.New("the value is not a JSON object")

// ResolveInput holds what Resolve reads.
type ResolveInput struct {
	// Recorded is status.resolvedPolicy of the claim. Nil at creation.
	Recorded *fleetv1alpha1.ResolvedPolicy `json:"recorded,omitempty"`
	// Environment is the Environment of the namespace. Nil when no
	// Environment exists.
	Environment   *fleetv1alpha1.Environment           `json:"environment,omitempty"`
	Registrations []pluginsv1alpha1.PluginRegistration `json:"registrations,omitempty"`
	Selections    []fleetv1alpha1.PluginSelection      `json:"selections,omitempty"`
}

// Resolution is the snapshot of a claim and the parameters of its requests.
type Resolution struct {
	Policy fleetv1alpha1.ResolvedPolicy `json:"policy"`
	// Requests holds one item for each selection, in claim order.
	Requests []RequestParameters `json:"requests,omitempty"`
}

// RequestParameters are the parameters of the request of one selection.
type RequestParameters struct {
	Section      string `json:"section"`
	Registration string `json:"registration"`
	// Parameters is nil when the request has no parameter.
	Parameters *apiextensionsv1.JSON `json:"parameters,omitempty"`
}

// Resolve returns the policy snapshot of a claim and the parameters of each
// of its requests. It does not change its input.
//
// With Recorded set, the policy is a copy of it, and a change of the
// Environment or of a registration does not change it. Without it, the
// policy is taken from the Environment:
//   - Sections is a copy of spec.sections of the Environment. Without an
//     Environment, it holds each section that a selection names: Allowed is
//     the sorted names of the registrations that serve the section, and
//     Default is the one with mode Default when exactly one has it.
//   - Parameters holds each selected registration. Each key of its
//     parameterValues gets its value. With an Environment, the policy of
//     spec.parameters decides ClaimEditable and Allowed, replaces a value
//     when it has one, and adds its own keys. Without one, the keys of
//     claimEditable of the registration are claim-editable.
//   - DeletionPolicy is the one of the Environment, or Delete.
//
// The parameters of a request hold each value of the policy of its
// registration. A claim-editable key of the selection replaces its value. For
// a key with Allowed, a name becomes the value of its choice, and a list of
// names the list of their values. A name that is not a choice stays as it
// is, because CheckAllowed refuses it.
//
// A selected registration that does not exist is refused with
// InvalidReference. parameterValues of a registration or parameters of a
// selection that is not a JSON object is an error.
func Resolve(in ResolveInput) (*Resolution, error) {
	policy, err := snapshot(in)
	if err != nil {
		return nil, err
	}

	requests := make([]RequestParameters, 0, len(in.Selections))
	for _, s := range in.Selections {
		parameters, err := requestParameters(s, policy.Parameters[s.Registration])
		if err != nil {
			return nil, err
		}
		requests = append(requests, RequestParameters{
			Section:      s.Section,
			Registration: s.Registration,
			Parameters:   parameters,
		})
	}

	return &Resolution{Policy: policy, Requests: requests}, nil
}

func snapshot(in ResolveInput) (fleetv1alpha1.ResolvedPolicy, error) {
	if in.Recorded != nil {
		return *in.Recorded.DeepCopy(), nil
	}

	policy := fleetv1alpha1.ResolvedPolicy{
		Sections:       sectionPolicies(in),
		DeletionPolicy: pluginsv1alpha1.DeletionPolicyDelete,
	}
	if in.Environment != nil && in.Environment.Spec.DeletionPolicy != "" {
		policy.DeletionPolicy = in.Environment.Spec.DeletionPolicy
	}

	for _, s := range in.Selections {
		registration, err := registrationOf(s.Registration, in.Registrations)
		if err != nil {
			return fleetv1alpha1.ResolvedPolicy{}, err
		}

		parameters, err := parameterPolicies(registration, in.Environment)
		if err != nil {
			return fleetv1alpha1.ResolvedPolicy{}, err
		}
		if policy.Parameters == nil {
			policy.Parameters = map[string]fleetv1alpha1.ParameterPolicies{}
		}
		policy.Parameters[registration.Name] = parameters
	}

	return policy, nil
}

func sectionPolicies(in ResolveInput) map[string]fleetv1alpha1.SectionPolicy {
	if in.Environment != nil {
		return in.Environment.Spec.DeepCopy().Sections
	}

	var sections map[string]fleetv1alpha1.SectionPolicy
	for _, s := range in.Selections {
		if _, done := sections[s.Section]; done {
			continue
		}

		var policy fleetv1alpha1.SectionPolicy
		var defaults []string
		for _, r := range in.Registrations {
			if !slices.Contains(r.Spec.Sections, s.Section) {
				continue
			}
			policy.Allowed = append(policy.Allowed, r.Name)
			if r.Spec.Mode == pluginsv1alpha1.ModeDefault {
				defaults = append(defaults, r.Name)
			}
		}
		slices.Sort(policy.Allowed)
		if len(defaults) == 1 {
			policy.Default = defaults[0]
		}

		if sections == nil {
			sections = map[string]fleetv1alpha1.SectionPolicy{}
		}
		sections[s.Section] = policy
	}

	return sections
}

func registrationOf(
	name string,
	registrations []pluginsv1alpha1.PluginRegistration,
) (*pluginsv1alpha1.PluginRegistration, error) {
	for i := range registrations {
		if registrations[i].Name == name {
			return &registrations[i], nil
		}
	}

	return nil, refuse(v1.ReasonInvalidReference, "PluginRegistration %q does not exist", name)
}

func parameterPolicies(
	registration *pluginsv1alpha1.PluginRegistration,
	environment *fleetv1alpha1.Environment,
) (fleetv1alpha1.ParameterPolicies, error) {
	values, err := decodeObject(registration.Spec.ParameterValues)
	if err != nil {
		return nil, fmt.Errorf("parameterValues of PluginRegistration %q: %w", registration.Name, err)
	}

	policies := fleetv1alpha1.ParameterPolicies{}
	for key, value := range values {
		policies[key] = fleetv1alpha1.ParameterPolicy{Value: &apiextensionsv1.JSON{Raw: value}}
	}

	if environment == nil {
		for _, key := range registration.Spec.ClaimEditable {
			policy := policies[key]
			policy.ClaimEditable = true
			policies[key] = policy
		}
		return policies, nil
	}

	for key, own := range environment.Spec.DeepCopy().Parameters[registration.Name] {
		policy := policies[key]
		policy.ClaimEditable = own.ClaimEditable
		policy.Allowed = own.Allowed
		if own.Value != nil {
			policy.Value = own.Value
		}
		policies[key] = policy
	}

	return policies, nil
}

// decodeObject reads a JSON object by key. An absent or null value is an
// empty object. Any other value that is not an object is errNotAnObject.
func decodeObject(raw *apiextensionsv1.JSON) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if raw != nil && len(raw.Raw) > 0 {
		if json.Unmarshal(raw.Raw, &object) != nil {
			return nil, errNotAnObject
		}
	}

	if object == nil {
		object = map[string]json.RawMessage{}
	}

	return object, nil
}

func requestParameters(
	selection fleetv1alpha1.PluginSelection,
	policies fleetv1alpha1.ParameterPolicies,
) (*apiextensionsv1.JSON, error) {
	claimed, err := decodeObject(selection.Parameters)
	if err != nil {
		return nil, fmt.Errorf("parameters of the selection of section %q: %w", selection.Section, err)
	}

	object := map[string]json.RawMessage{}
	for key, policy := range policies {
		if policy.Value != nil {
			object[key] = policy.Value.Raw
		}
	}

	for key, value := range claimed {
		if policy := policies[key]; policy.ClaimEditable {
			object[key] = valuesOfNames(value, policy.Allowed)
		}
	}

	if len(object) == 0 {
		return nil, nil
	}

	raw, err := json.Marshal(object)
	if err != nil {
		return nil, fmt.Errorf("encoding the parameters of the request of section %q: %w", selection.Section, err)
	}

	return &apiextensionsv1.JSON{Raw: raw}, nil
}

// valuesOfNames replaces a name of choices, or each name of a list, with the
// value of its choice. Any other value, and a name that is not a choice, stay
// as they are.
func valuesOfNames(value json.RawMessage, choices []fleetv1alpha1.NamedValue) json.RawMessage {
	if len(choices) == 0 {
		return value
	}

	var name string
	if json.Unmarshal(value, &name) == nil {
		return valueOfName(name, choices)
	}

	var names []string
	if json.Unmarshal(value, &names) != nil {
		return value
	}

	values := make([]json.RawMessage, 0, len(names))
	for _, n := range names {
		values = append(values, valueOfName(n, choices))
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return value
	}

	return raw
}

func valueOfName(name string, choices []fleetv1alpha1.NamedValue) json.RawMessage {
	for _, c := range choices {
		if c.Name == name && c.Value != nil {
			return c.Value.Raw
		}
	}

	raw, _ := json.Marshal(name)
	return raw
}
