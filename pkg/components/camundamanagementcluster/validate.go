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

// The lowest versions the operator supports. Camunda 8.9 is the first release
// with the management plane in this shape, and it supports Keycloak 26 only
// (https://docs.camunda.io/docs/reference/announcements-release-notes/890/890-announcements/#supported-environments).
const (
	camundaVersionFloor  = "8.9.0"
	keycloakVersionFloor = "26.0.0"
)

// versionedField is one version field of the spec and the lowest value the
// operator supports in it.
type versionedField struct {
	field   string
	version string
	floor   string
}

// ValidateSpec checks the rules that the API server cannot: the version floor
// of each component of the management plane. A version below its floor comes
// back as a *conditions.PreCheckFailure with the reason UnsupportedVersion,
// naming the field and the floor. A valid spec returns nil.
func ValidateSpec(mc *v1.CamundaManagementCluster) *conditions.PreCheckFailure {
	floors := []versionedField{
		{"spec.identity.version", mc.Spec.Identity.Version, camundaVersionFloor},
	}
	if keycloak := mc.Spec.IdentityProvider.Keycloak; keycloak != nil {
		floors = append(floors, versionedField{
			"spec.identityProvider.keycloak.version", keycloak.Version, keycloakVersionFloor,
		})
	}
	if console := mc.Spec.Console; console != nil {
		floors = append(floors, versionedField{"spec.console.version", console.Version, camundaVersionFloor})
	}
	if webModeler := mc.Spec.WebModeler; webModeler != nil {
		floors = append(
			floors, versionedField{"spec.webModeler.version", webModeler.Version, camundaVersionFloor},
		)
	}

	var problems []string
	for _, f := range floors {
		if !atLeast(f.version, f.floor) {
			problems = append(problems, fmt.Sprintf(
				"%s is %s; the operator supports %s and later", f.field, f.version, f.floor,
			))
		}
	}
	if len(problems) == 0 {
		return nil
	}

	return &conditions.PreCheckFailure{
		Reason:  v1.ReasonUnsupportedVersion,
		Message: strings.Join(problems, "; "),
	}
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
