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

// Missing says how CheckDeletionPolicy treats a selection whose registration
// is not in its list of registrations.
type Missing int

const (
	// MissingPasses passes such a selection, for a caller whose reference
	// check refuses a registration that does not exist.
	MissingPasses Missing = iota
	// MissingRefuses refuses Orphan for such a selection, because nothing
	// shows that the registration supports it.
	MissingRefuses
)

// CheckDeletionPolicy refuses with NotAllowed the deletion policy Orphan when
// the registration of a selection does not list Orphan in
// spec.deletionPolicies. The message names each such section. Delete always
// passes. A selection whose registration is not in registrations passes
// with MissingPasses, and refuses Orphan with MissingRefuses. policy is the
// policy that the requests get, as DeletionPolicy returns it.
func CheckDeletionPolicy(
	policy pluginsv1alpha1.DeletionPolicy,
	selections []fleetv1alpha1.PluginSelection,
	registrations []pluginsv1alpha1.PluginRegistration,
	missing Missing,
) error {
	if policy != pluginsv1alpha1.DeletionPolicyOrphan {
		return nil
	}

	var unsupported, unknown []string
	for _, s := range selections {
		name := fmt.Sprintf("PluginRegistration %q of section %q", s.Registration, s.Section)
		i := slices.IndexFunc(registrations, func(r pluginsv1alpha1.PluginRegistration) bool {
			return r.Name == s.Registration
		})
		switch {
		case i < 0 && missing == MissingRefuses:
			unknown = append(unknown, name)
		case i < 0, slices.Contains(registrations[i].Spec.DeletionPolicies, policy):
		default:
			unsupported = append(unsupported, name)
		}
	}

	var parts []string
	if len(unsupported) > 0 {
		parts = append(parts, "these registrations do not support it: "+strings.Join(unsupported, ", "))
	}
	if len(unknown) > 0 {
		parts = append(
			parts,
			"the claim cannot tell that these registrations support it, because they do not exist: "+
				strings.Join(unknown, ", "),
		)
	}
	if len(parts) == 0 {
		return nil
	}

	return refuse(
		fleetv1alpha1.ReasonNotAllowed,
		"the deletion policy is Orphan, and %s",
		strings.Join(parts, ", and "),
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
