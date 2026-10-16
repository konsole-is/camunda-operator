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

package environment

import (
	"cmp"
	"slices"
	"strings"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
)

// The kinds of the cluster-scoped objects that an Environment names.
const (
	kindPlatformConfig       = "CamundaPlatformConfig"
	kindClusterPreset        = "CamundaClusterPreset"
	kindElasticsearchPreset  = "ElasticsearchClusterPreset"
	kindDatabaseServerPreset = "DatabaseServerPreset"
	kindRelease              = "CamundaRelease"
	kindSection              = "PluginSection"
	kindRegistration         = "PluginRegistration"
)

// kindOrder is the order of the spec fields, so the message lists the
// missing objects in the order that the admin reads the spec.
var kindOrder = []string{
	kindPlatformConfig,
	kindClusterPreset,
	kindElasticsearchPreset,
	kindDatabaseServerPreset,
	kindRelease,
	kindSection,
	kindRegistration,
}

// reference is one object that the spec of an Environment names.
type reference struct {
	Kind string
	Name string
}

// references returns each object that spec names, once, ordered by kindOrder
// and then by name. A section is named by its key in sections. A registration
// is named in the allowed list or the default of a section policy, or by its
// key in parameters.
func references(spec *fleetv1alpha1.EnvironmentSpec) []reference {
	var refs []reference
	add := func(kind string, names ...string) {
		for _, name := range names {
			if name != "" {
				refs = append(refs, reference{Kind: kind, Name: name})
			}
		}
	}

	add(kindPlatformConfig, spec.PlatformConfigRef)
	add(kindClusterPreset, spec.Presets.CamundaCluster...)
	add(kindElasticsearchPreset, spec.Presets.ElasticsearchCluster...)
	add(kindDatabaseServerPreset, spec.Presets.DatabaseServer...)
	if spec.Releases != nil {
		add(kindRelease, spec.Releases.Allowed...)
		add(kindRelease, spec.Releases.Default)
	}
	for section, policy := range spec.Sections {
		add(kindSection, section)
		add(kindRegistration, policy.Allowed...)
		add(kindRegistration, policy.Default)
	}
	for registration := range spec.Parameters {
		add(kindRegistration, registration)
	}

	slices.SortFunc(refs, func(a, b reference) int {
		return cmp.Or(
			cmp.Compare(slices.Index(kindOrder, a.Kind), slices.Index(kindOrder, b.Kind)),
			strings.Compare(a.Name, b.Name),
		)
	})

	return slices.Compact(refs)
}

// invalidReferenceMessage is the message of Ready=False, reason
// InvalidReference. It names the kind and the name of each missing object.
func invalidReferenceMessage(missing []reference) string {
	names := make([]string, len(missing))
	for i, ref := range missing {
		names[i] = ref.Kind + " " + ref.Name
	}

	return "The spec names objects that do not exist: " + strings.Join(names, ", ")
}
