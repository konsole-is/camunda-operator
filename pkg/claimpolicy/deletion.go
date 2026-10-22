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
	"fmt"
	"slices"
	"strings"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// CheckDeletionPolicy refuses with NotAllowed the deletion policy Orphan when
// the registration of a selection does not list Orphan in
// spec.deletionPolicies. The message names each such section. Delete always
// passes, and so does a selection whose registration is not in
// registrations. policy is the policy that the requests get, as
// DeletionPolicy returns it.
func CheckDeletionPolicy(
	policy pluginsv1alpha1.DeletionPolicy,
	selections []fleetv1alpha1.PluginSelection,
	registrations []pluginsv1alpha1.PluginRegistration,
) error {
	if policy != pluginsv1alpha1.DeletionPolicyOrphan {
		return nil
	}

	var unsupported []string
	for _, s := range selections {
		i := slices.IndexFunc(registrations, func(r pluginsv1alpha1.PluginRegistration) bool {
			return r.Name == s.Registration
		})
		if i < 0 || slices.Contains(registrations[i].Spec.DeletionPolicies, policy) {
			continue
		}
		unsupported = append(unsupported, fmt.Sprintf("PluginRegistration %q of section %q", s.Registration, s.Section))
	}

	if len(unsupported) == 0 {
		return nil
	}

	return refuse(
		fleetv1alpha1.ReasonNotAllowed,
		"the deletion policy is Orphan, and these registrations do not support it: %s",
		strings.Join(unsupported, ", "),
	)
}

// DeletionPolicy returns the deletion policy that the requests of a claim
// get: own, the spec.deletionPolicy of the claim, when it is set, else the
// default of resolved, else Delete. resolved may be nil.
func DeletionPolicy(
	own pluginsv1alpha1.DeletionPolicy,
	resolved *fleetv1alpha1.ResolvedPolicy,
) pluginsv1alpha1.DeletionPolicy {
	if own != "" {
		return own
	}
	if resolved != nil && resolved.DeletionPolicy != "" {
		return resolved.DeletionPolicy
	}

	return pluginsv1alpha1.DeletionPolicyDelete
}
