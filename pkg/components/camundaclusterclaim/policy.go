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
	"slices"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// ClaimPolicy returns a copy of policy in which a section that sections
// marks as not applying to a CamundaClusterClaim is not required. An
// Environment serves both claim kinds, so its required sections of the
// management plane must not refuse a cluster claim. A section that is not
// in sections keeps its policy. policy is not changed.
func ClaimPolicy(
	policy fleetv1alpha1.ResolvedPolicy,
	sections []pluginsv1alpha1.PluginSection,
) fleetv1alpha1.ResolvedPolicy {
	claimPolicy := *policy.DeepCopy()
	for _, section := range sections {
		sectionPolicy, ok := claimPolicy.Sections[section.Name]
		if !ok || slices.Contains(section.Spec.AppliesTo, pluginsv1alpha1.ClaimKindCamundaClusterClaim) {
			continue
		}

		sectionPolicy.Required = false
		claimPolicy.Sections[section.Name] = sectionPolicy
	}

	return claimPolicy
}
