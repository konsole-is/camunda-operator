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
	"testing"

	"github.com/stretchr/testify/assert"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestMissingURLsNamesEachRequiredURL(t *testing.T) {
	tests := []struct {
		name       string
		management fleetv1alpha1.ManagementOverrides
		want       []string
	}{
		{
			name: "oidc with the identity url",
			management: fleetv1alpha1.ManagementOverrides{
				IdentityProvider: fleetv1alpha1.ManagementIdentityProvider{OIDC: &v1.ManagementOIDCSpec{}},
				Identity:         fleetv1alpha1.IdentityOverrides{ExternalURL: "https://plane.example.com/identity"},
			},
		},
		{
			name: "keycloak with console and web modeler and no url",
			management: fleetv1alpha1.ManagementOverrides{
				IdentityProvider: fleetv1alpha1.ManagementIdentityProvider{
					Keycloak: &fleetv1alpha1.KeycloakOverrides{},
				},
				Console:    &fleetv1alpha1.ConsoleOverrides{},
				WebModeler: &fleetv1alpha1.WebModelerOverrides{},
			},
			want: []string{
				"spec.management.identityProvider.keycloak.externalUrl",
				"spec.management.identity.externalUrl",
				"spec.management.console.externalUrl",
				"spec.management.webModeler.externalUrl",
				"spec.management.webModeler.websocketsExternalUrl",
			},
		},
		{
			name: "web modeler with only its external url",
			management: fleetv1alpha1.ManagementOverrides{
				IdentityProvider: fleetv1alpha1.ManagementIdentityProvider{OIDC: &v1.ManagementOIDCSpec{}},
				Identity:         fleetv1alpha1.IdentityOverrides{ExternalURL: "https://plane.example.com/identity"},
				WebModeler:       &fleetv1alpha1.WebModelerOverrides{ExternalURL: "https://modeler.example.com"},
			},
			want: []string{"spec.management.webModeler.websocketsExternalUrl"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, MissingURLs(tt.management))
		})
	}
}
