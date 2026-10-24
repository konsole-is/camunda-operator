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
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// AllowedInput holds what CheckAllowed reads.
type AllowedInput struct {
	// Environment is the current Environment of the namespace. Nil when no
	// Environment exists.
	Environment *fleetv1alpha1.Environment `json:"environment,omitempty"`
	// Kind is the kind of the claim.
	Kind pluginsv1alpha1.ClaimKind `json:"kind,omitempty"`
	// Sections are the PluginSections. A section of the policy that is not
	// in Sections applies to every claim kind.
	Sections []pluginsv1alpha1.PluginSection `json:"sections,omitempty"`
	// Policy is the snapshot of the claim (Resolution.Policy).
	Policy fleetv1alpha1.ResolvedPolicy `json:"policy"`
	// PresetRef and ReleaseRef are empty for a plane claim.
	PresetRef  string                          `json:"presetRef,omitempty"`
	ReleaseRef string                          `json:"releaseRef,omitempty"`
	Selections []fleetv1alpha1.PluginSelection `json:"selections,omitempty"`
}

// CheckAllowed refuses with NotAllowed the first value of a claim that its
// policy does not allow, in this order:
//  1. A preset that spec.presets.camundaCluster of the current Environment
//     does not list. An empty list allows no preset.
//  2. A release that spec.releases.allowed of the current Environment does
//     not list. An empty release passes, because the default of the
//     Environment is one of allowed.
//  3. A selection, in claim order, in a section whose spec.appliesTo does
//     not list Kind.
//  4. A registration, in claim order, that the snapshot does not allow in
//     its section. A section without a policy allows no registration.
//  5. A section, in name order, that the snapshot marks required, that
//     applies to Kind, and in which the claim selects nothing.
//  6. A parameter, in claim order and then in key order. A key that is not
//     claim-editable passes only with a value equal as JSON to the value of
//     the snapshot. A claim-editable key with choices takes only the name of
//     a choice or a list of such names.
//
// Without an Environment, every preset and release passes. The registrations
// and the parameters are always checked against the snapshot, so a later
// change of the Environment does not refuse an existing claim. parameters of
// a selection that is not a JSON object is an error.
func CheckAllowed(in AllowedInput) error {
	if err := checkRefs(in); err != nil {
		return err
	}

	for _, s := range in.Selections {
		if !appliesTo(s.Section, in.Kind, in.Sections) {
			return refuse(fleetv1alpha1.ReasonNotAllowed, "section %q does not apply to a %s", s.Section, in.Kind)
		}
	}

	selected := map[string]bool{}
	for _, s := range in.Selections {
		if !slices.Contains(in.Policy.Sections[s.Section].Allowed, s.Registration) {
			return refuse(
				fleetv1alpha1.ReasonNotAllowed,
				"registration %q is not allowed in section %q",
				s.Registration,
				s.Section,
			)
		}
		selected[s.Section] = true
	}

	for _, section := range slices.Sorted(maps.Keys(in.Policy.Sections)) {
		if in.Policy.Sections[section].Required && !selected[section] && appliesTo(section, in.Kind, in.Sections) {
			return refuse(
				fleetv1alpha1.ReasonNotAllowed,
				"section %q is required, and the claim selects nothing in it",
				section,
			)
		}
	}

	for _, s := range in.Selections {
		if err := checkParameters(s, in.Policy.Parameters[s.Registration]); err != nil {
			return err
		}
	}

	return nil
}

func checkRefs(in AllowedInput) error {
	if in.Environment == nil {
		return nil
	}

	env := in.Environment
	if in.PresetRef != "" && !slices.Contains(env.Spec.Presets.CamundaCluster, in.PresetRef) {
		return refuse(
			fleetv1alpha1.ReasonNotAllowed,
			"CamundaClusterPreset %q is not in spec.presets.camundaCluster of Environment %q",
			in.PresetRef,
			env.Name,
		)
	}

	if in.ReleaseRef != "" && (env.Spec.Releases == nil || !slices.Contains(env.Spec.Releases.Allowed, in.ReleaseRef)) {
		return refuse(
			fleetv1alpha1.ReasonNotAllowed,
			"CamundaRelease %q is not in spec.releases.allowed of Environment %q",
			in.ReleaseRef,
			env.Name,
		)
	}

	return nil
}

func appliesTo(section string, kind pluginsv1alpha1.ClaimKind, sections []pluginsv1alpha1.PluginSection) bool {
	i := slices.IndexFunc(sections, func(s pluginsv1alpha1.PluginSection) bool { return s.Name == section })
	return i < 0 || slices.Contains(sections[i].Spec.AppliesTo, kind)
}

func checkParameters(selection fleetv1alpha1.PluginSelection, policies fleetv1alpha1.ParameterPolicies) error {
	claimed, err := decodeObject(selection.Parameters)
	if err != nil {
		return fmt.Errorf("parameters of the selection of section %q: %w", selection.Section, err)
	}

	for _, key := range slices.Sorted(maps.Keys(claimed)) {
		policy := policies[key]
		value := claimed[key]

		if !policy.ClaimEditable {
			if policy.Value == nil || !equalJSON(value, policy.Value.Raw) {
				return refuse(
					fleetv1alpha1.ReasonNotAllowed,
					"parameter %q of registration %q is not claim-editable",
					key,
					selection.Registration,
				)
			}
			continue
		}

		if len(policy.Allowed) > 0 {
			if err := checkNames(key, selection.Registration, value, policy.Allowed); err != nil {
				return err
			}
		}
	}

	return nil
}

func equalJSON(a, b json.RawMessage) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}

	return reflect.DeepEqual(x, y)
}

func checkNames(key, registration string, value json.RawMessage, choices []fleetv1alpha1.NamedValue) error {
	names, ok := listOfNames(value)
	if name, isName := nameOf(value); isName {
		names, ok = []string{name}, true
	}
	if !ok {
		return refuse(
			fleetv1alpha1.ReasonNotAllowed,
			"parameter %q of registration %q takes a name of its choices or a list of names",
			key,
			registration,
		)
	}

	known := make([]string, 0, len(choices))
	for _, c := range choices {
		known = append(known, c.Name)
	}

	for _, n := range names {
		if !slices.Contains(known, n) {
			return refuse(
				fleetv1alpha1.ReasonNotAllowed,
				"parameter %q of registration %q has the name %q, which is not one of %s",
				key,
				registration,
				n,
				strings.Join(known, ", "),
			)
		}
	}

	return nil
}
