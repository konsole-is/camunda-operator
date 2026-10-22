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
	"fmt"
	"slices"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// CheckServed refuses with NotAllowed the first selection whose
// registration does not list its section in spec.sections. A selection
// whose registration is not in registrations passes. The refusal is a
// *conditions.PreCheckFailure.
func CheckServed(
	selections []fleetv1alpha1.PluginSelection,
	registrations []pluginsv1alpha1.PluginRegistration,
) error {
	for _, s := range selections {
		registration := find(registrations, s.Registration)
		if registration == nil || slices.Contains(registration.Spec.Sections, s.Section) {
			continue
		}

		return &conditions.PreCheckFailure{
			Reason:  fleetv1alpha1.ReasonNotAllowed,
			Message: fmt.Sprintf("PluginRegistration %q does not serve section %q", s.Registration, s.Section),
		}
	}

	return nil
}

// CheckNames refuses with NotAllowed the first two selections of claim
// whose requests get one name: two options of a One section, or names that
// meet across sections. A selection whose section is not in sections counts
// as One. The refusal is a *conditions.PreCheckFailure.
func CheckNames(
	claim string,
	selections []fleetv1alpha1.PluginSelection,
	sections []pluginsv1alpha1.PluginSection,
) error {
	seen := map[string]fleetv1alpha1.PluginSelection{}
	for _, s := range selections {
		name := Name(claim, s, modeOf(sections, s.Section))
		first, taken := seen[name]
		if !taken {
			seen[name] = s
			continue
		}

		message := fmt.Sprintf(
			"the selections %s/%s and %s/%s get the same request name %q",
			first.Section,
			first.Registration,
			s.Section,
			s.Registration,
			name,
		)
		if first.Section == s.Section {
			message = fmt.Sprintf(
				"section %q takes one option, and the claim selects %q and %q",
				s.Section,
				first.Registration,
				s.Registration,
			)
		}

		return &conditions.PreCheckFailure{Reason: fleetv1alpha1.ReasonNotAllowed, Message: message}
	}

	return nil
}
