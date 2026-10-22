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
	"slices"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// Own returns the requests whose claimRef is claim and whose name is the
// request name of an item of selections in sections. A request of another
// claim kind, or of another name, with the same claim label stays out, so
// the claim never reads its outputs.
func Own(
	requests []pluginsv1alpha1.PluginRequest,
	claim pluginsv1alpha1.ClaimRef,
	selections []fleetv1alpha1.PluginSelection,
	sections []pluginsv1alpha1.PluginSection,
) []pluginsv1alpha1.PluginRequest {
	names := make([]string, 0, len(selections))
	for _, s := range selections {
		names = append(names, Name(claim.Name, s, modeOf(sections, s.Section)))
	}

	return slices.DeleteFunc(slices.Clone(requests), func(r pluginsv1alpha1.PluginRequest) bool {
		return r.Spec.ClaimRef != claim || !slices.Contains(names, r.Name)
	})
}
