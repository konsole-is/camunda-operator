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

package camundamanagementcluster

import (
	"fmt"
	"strconv"
	"strings"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// The versions the operator supports. Camunda 8.9 is the first release with
// the management plane in this shape. It supports Keycloak 26 only, so
// Keycloak carries a ceiling as well as a floor
// (https://docs.camunda.io/docs/reference/announcements-release-notes/890/890-announcements/#supported-environments).
const (
	camundaVersionFloor    = "8.9.0"
	keycloakVersionFloor   = "26.0.0"
	keycloakVersionCeiling = "27.0.0"
)

// versionedField is one version field of the spec and the range the operator
// supports in it. An empty ceiling means no upper bound.
type versionedField struct {
	field   string
	version string
	floor   string
	ceiling string
}

// referencedDatabase is one databaseConfigRef of the spec.
type referencedDatabase struct {
	field string
	ref   string
}

// ValidateSpec checks the rules that the API server cannot: the supported
// version range of each component of the management plane, and that no two
// components name one DatabaseConfig. A broken rule comes back as a
// *conditions.PreCheckFailure, with the reason UnsupportedVersion or
// InvalidReference, naming the field. A valid spec returns nil.
func ValidateSpec(mc *v1.CamundaManagementCluster) *conditions.PreCheckFailure {
	bounds := []versionedField{
		{field: "spec.identity.version", version: mc.Spec.Identity.Version, floor: camundaVersionFloor},
	}
	if keycloak := mc.Spec.IdentityProvider.Keycloak; keycloak != nil {
		bounds = append(bounds, versionedField{
			field:   "spec.identityProvider.keycloak.version",
			version: keycloak.Version,
			floor:   keycloakVersionFloor,
			ceiling: keycloakVersionCeiling,
		})
	}
	if console := mc.Spec.Console; console != nil {
		bounds = append(bounds, versionedField{
			field: "spec.console.version", version: console.Version, floor: camundaVersionFloor,
		})
	}
	if webModeler := mc.Spec.WebModeler; webModeler != nil {
		bounds = append(bounds, versionedField{
			field: "spec.webModeler.version", version: webModeler.Version, floor: camundaVersionFloor,
		})
	}

	var problems []string
	for _, f := range bounds {
		switch {
		case !atLeast(f.version, f.floor):
			problems = append(problems, fmt.Sprintf(
				"%s is %s; the operator supports %s and later", f.field, f.version, f.floor,
			))
		case f.ceiling != "" && atLeast(f.version, f.ceiling):
			problems = append(problems, fmt.Sprintf(
				"%s is %s; the operator supports versions below %s", f.field, f.version, f.ceiling,
			))
		}
	}
	if len(problems) > 0 {
		return &conditions.PreCheckFailure{
			Reason:  v1.ReasonUnsupportedVersion,
			Message: strings.Join(problems, "; "),
		}
	}

	return checkDistinctDatabases(mc)
}

// checkDistinctDatabases refuses two components that name one DatabaseConfig.
// Management Identity, Keycloak, and Web Modeler each own every table of the
// database they open, so two of them in one database overwrite each other.
func checkDistinctDatabases(mc *v1.CamundaManagementCluster) *conditions.PreCheckFailure {
	refs := []referencedDatabase{
		{"spec.identity.databaseConfigRef", mc.Spec.Identity.DatabaseConfigRef},
	}
	if keycloak := mc.Spec.IdentityProvider.Keycloak; keycloak != nil {
		refs = append(refs, referencedDatabase{
			"spec.identityProvider.keycloak.databaseConfigRef", keycloak.DatabaseConfigRef,
		})
	}
	if webModeler := mc.Spec.WebModeler; webModeler != nil {
		refs = append(refs, referencedDatabase{
			"spec.webModeler.databaseConfigRef", webModeler.DatabaseConfigRef,
		})
	}

	seen := map[string]string{}
	for _, r := range refs {
		if first, taken := seen[r.ref]; taken {
			return &conditions.PreCheckFailure{
				Reason: v1.ReasonInvalidReference,
				Message: fmt.Sprintf(
					"%s and %s both name DatabaseConfig %q; each component needs a database of its own",
					first, r.field, r.ref,
				),
			}
		}
		seen[r.ref] = r.field
	}

	return nil
}

// atLeast reports whether version is floor or later. Both carry three numeric
// segments, which the CRD pattern of every version field enforces, so a
// version of any other shape is below every floor.
func atLeast(version, floor string) bool {
	got, ok := parseVersion(version)
	if !ok {
		return false
	}
	want, _ := parseVersion(floor)

	for i := range got {
		if got[i] != want[i] {
			return got[i] > want[i]
		}
	}

	return true
}

// parseVersion reads the three numeric segments of a semantic version.
func parseVersion(version string) ([3]int, bool) {
	var parsed [3]int

	segments := strings.Split(version, ".")
	if len(segments) != len(parsed) {
		return parsed, false
	}

	for i, segment := range segments {
		n, err := strconv.Atoi(segment)
		if err != nil {
			return parsed, false
		}
		parsed[i] = n
	}

	return parsed, true
}
