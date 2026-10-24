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
	"slices"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// CheckServed refuses with NotAllowed the first selection whose
// registration does not list its section in spec.sections. A selection
// whose registration is not in registrations passes.
func CheckServed(
	selections []fleetv1alpha1.PluginSelection,
	registrations []pluginsv1alpha1.PluginRegistration,
) error {
	for _, s := range selections {
		i := slices.IndexFunc(registrations, func(r pluginsv1alpha1.PluginRegistration) bool {
			return r.Name == s.Registration
		})
		if i < 0 || slices.Contains(registrations[i].Spec.Sections, s.Section) {
			continue
		}

		return refuse(
			fleetv1alpha1.ReasonNotAllowed,
			"PluginRegistration %q does not serve section %q",
			s.Registration,
			s.Section,
		)
	}

	return nil
}
