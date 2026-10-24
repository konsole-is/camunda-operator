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
	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
)

// MissingURLs returns the claim path of each URL that the plane of
// management needs and management leaves empty, in this order: the
// Keycloak URL in the keycloak mode, the Identity URL, the Console URL
// while Console is set, and the two Web Modeler URLs while Web Modeler is
// set. A claim without an access selection must set each of them.
func MissingURLs(management fleetv1alpha1.ManagementOverrides) []string {
	var missing []string
	check := func(value, path string) {
		if value == "" {
			missing = append(missing, path)
		}
	}

	if keycloak := management.IdentityProvider.Keycloak; keycloak != nil {
		check(keycloak.ExternalURL, "spec.management.identityProvider.keycloak.externalUrl")
	}
	check(management.Identity.ExternalURL, "spec.management.identity.externalUrl")
	if console := management.Console; console != nil {
		check(console.ExternalURL, "spec.management.console.externalUrl")
	}
	if webModeler := management.WebModeler; webModeler != nil {
		check(webModeler.ExternalURL, "spec.management.webModeler.externalUrl")
		check(webModeler.WebsocketsExternalURL, "spec.management.webModeler.websocketsExternalUrl")
	}

	return missing
}
