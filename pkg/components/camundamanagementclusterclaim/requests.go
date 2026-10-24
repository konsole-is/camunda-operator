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
	"slices"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/pkg/claimpolicy"
)

// ParameterDatabases is the request parameter that names the components of
// the plane that get a database.
const ParameterDatabases = "databases"

// The items of the databases parameter, in the order of the parameter.
const (
	DatabaseKeycloak   = "keycloak"
	DatabaseIdentity   = "identity"
	DatabaseWebModeler = "webModeler"
)

// Databases returns the components of management that get a database:
// keycloak only in the keycloak mode, identity always, and webModeler only
// while Web Modeler is set.
func Databases(management fleetv1alpha1.ManagementOverrides) []string {
	var databases []string
	if management.IdentityProvider.Keycloak != nil {
		databases = append(databases, DatabaseKeycloak)
	}
	databases = append(databases, DatabaseIdentity)
	if management.WebModeler != nil {
		databases = append(databases, DatabaseWebModeler)
	}

	return databases
}

// WithDatabases returns a copy of params in which the parameters of each
// selection in a section with the slot managementDatabases hold databases
// under ParameterDatabases, over any value of the policy or the claim.
// params is not changed. Parameters that are not a JSON object are an
// error.
func WithDatabases(
	params []claimpolicy.RequestParameters,
	sections []pluginsv1alpha1.PluginSection,
	databases []string,
) ([]claimpolicy.RequestParameters, error) {
	encoded, err := json.Marshal(databases)
	if err != nil {
		return nil, err
	}

	out := slices.Clone(params)
	for i, p := range out {
		if slotOf(sections, p.Section) != pluginsv1alpha1.SlotManagementDatabases {
			continue
		}

		values := map[string]json.RawMessage{}
		if p.Parameters != nil {
			if err := json.Unmarshal(p.Parameters.Raw, &values); err != nil || values == nil {
				return nil, fmt.Errorf("the parameters of the selection of section %q are not a JSON object", p.Section)
			}
		}
		values[ParameterDatabases] = encoded

		raw, err := json.Marshal(values)
		if err != nil {
			return nil, err
		}
		out[i].Parameters = &apiextensionsv1.JSON{Raw: raw}
	}

	return out, nil
}

// WriteOrder returns selections, the one in a section with the slot
// managementDatabases first and the one in a section with the slot
// externalAccess second, every other in claim order after them. The claim
// controller passes it to claimpolicy.Order, which keeps this order where no
// input links two selections. selections is not changed.
func WriteOrder(
	selections []fleetv1alpha1.PluginSelection,
	sections []pluginsv1alpha1.PluginSection,
) []fleetv1alpha1.PluginSelection {
	rank := func(s fleetv1alpha1.PluginSelection) int {
		switch slotOf(sections, s.Section) {
		case pluginsv1alpha1.SlotManagementDatabases:
			return 0
		case pluginsv1alpha1.SlotExternalAccess:
			return 1
		default:
			return 2
		}
	}

	ordered := slices.Clone(selections)
	slices.SortStableFunc(ordered, func(a, b fleetv1alpha1.PluginSelection) int { return rank(a) - rank(b) })

	return ordered
}

// Selects reports whether an item of selections is in a section of sections
// with slot.
func Selects(
	selections []fleetv1alpha1.PluginSelection,
	sections []pluginsv1alpha1.PluginSection,
	slot pluginsv1alpha1.Slot,
) bool {
	return slices.ContainsFunc(selections, func(s fleetv1alpha1.PluginSelection) bool {
		return slotOf(sections, s.Section) == slot
	})
}

// slotOf returns the slot of the section name, or "" when sections does not
// hold it.
func slotOf(sections []pluginsv1alpha1.PluginSection, name string) pluginsv1alpha1.Slot {
	for _, s := range sections {
		if s.Name == name {
			return s.Spec.Slot
		}
	}

	return ""
}
