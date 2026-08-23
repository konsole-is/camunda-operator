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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// Camunda 8.9 is the first release with the management plane in this shape.
func TestValidateSpecChecksTheCamundaVersionFloor(t *testing.T) {
	t.Parallel()

	assert.Nil(t, ValidateSpec(newCluster(nil)))

	below := ValidateSpec(newCluster(func(mc *v1.CamundaManagementCluster) {
		mc.Spec.Identity.Version = "8.8.0"
	}))
	require.NotNil(t, below)
	assert.Equal(t, v1.ReasonUnsupportedVersion, below.Reason)
	assert.Contains(t, below.Message, "spec.identity.version is 8.8.0")
	assert.Contains(t, below.Message, "the operator supports 8.9.0 and later")
}

// Camunda 8.9 supports Keycloak 26 only, so the Keycloak floor is its own.
func TestValidateSpecChecksTheKeycloakVersionFloor(t *testing.T) {
	t.Parallel()

	below := ValidateSpec(newCluster(func(mc *v1.CamundaManagementCluster) {
		mc.Spec.IdentityProvider = v1.IdentityProviderSpec{
			Keycloak: &v1.ManagedKeycloakSpec{Version: "25.0.6"},
		}
	}))
	require.NotNil(t, below)
	assert.Contains(t, below.Message, "spec.identityProvider.keycloak.version is 25.0.6")
	assert.Contains(t, below.Message, "the operator supports 26.0.0 and later")

	assert.Nil(t, ValidateSpec(newCluster(func(mc *v1.CamundaManagementCluster) {
		mc.Spec.IdentityProvider = v1.IdentityProviderSpec{
			Keycloak: &v1.ManagedKeycloakSpec{Version: "26.1.0"},
		}
	})))
}

// The version of a component that the spec does not deploy is not checked, and
// every component that it does deploy is.
func TestValidateSpecChecksEveryDeployedComponent(t *testing.T) {
	t.Parallel()

	both := ValidateSpec(newCluster(func(mc *v1.CamundaManagementCluster) {
		mc.Spec.Console = &v1.ConsoleSpec{Version: "8.8.9", ExternalURL: fixtureExternal}
		mc.Spec.WebModeler = &v1.WebModelerSpec{Version: "8.7.0", ExternalURL: fixtureExternal}
	}))
	require.NotNil(t, both)
	assert.Contains(t, both.Message, "spec.console.version is 8.8.9")
	assert.Contains(t, both.Message, "spec.webModeler.version is 8.7.0")
}

// Management Identity, Keycloak, and Web Modeler each own every table of the
// database they open, so two of them in one database overwrite each other.
func TestCheckDistinctDatabases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(mc *v1.CamundaManagementCluster)
		want   []string
	}{
		{
			name: "a database of its own for each component",
			mutate: func(mc *v1.CamundaManagementCluster) {
				mc.Spec.IdentityProvider = keycloakProvider("keycloak-db")
				mc.Spec.WebModeler = webModeler("web-modeler-db")
			},
		},
		{
			name: "Management Identity and Web Modeler share one",
			mutate: func(mc *v1.CamundaManagementCluster) {
				mc.Spec.WebModeler = webModeler(mc.Spec.Identity.DatabaseConfigRef)
			},
			want: []string{
				"spec.identity.databaseConfigRef",
				"spec.webModeler.databaseConfigRef",
				`DatabaseConfig "identity-db"`,
			},
		},
		{
			name: "Keycloak and Management Identity share one",
			mutate: func(mc *v1.CamundaManagementCluster) {
				mc.Spec.IdentityProvider = keycloakProvider(mc.Spec.Identity.DatabaseConfigRef)
			},
			want: []string{
				"spec.identity.databaseConfigRef",
				"spec.identityProvider.keycloak.databaseConfigRef",
				`DatabaseConfig "identity-db"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			failure := checkDistinctDatabases(newCluster(tt.mutate))

			if tt.want == nil {
				assert.Nil(t, failure)
				return
			}
			require.NotNil(t, failure)
			assert.Equal(t, v1.ReasonInvalidReference, failure.Reason)
			for _, want := range tt.want {
				assert.Contains(t, failure.Message, want)
			}
		})
	}
}

// keycloakProvider returns an operator-run Keycloak that stores its data in
// the given DatabaseConfig.
func keycloakProvider(databaseConfigRef string) v1.IdentityProviderSpec {
	return v1.IdentityProviderSpec{Keycloak: &v1.ManagedKeycloakSpec{
		Version:           "26.1.0",
		DatabaseConfigRef: databaseConfigRef,
	}}
}

// webModeler returns a Web Modeler that stores its data in the given
// DatabaseConfig.
func webModeler(databaseConfigRef string) *v1.WebModelerSpec {
	return &v1.WebModelerSpec{
		Version:           fixtureVersion,
		ExternalURL:       fixtureExternal,
		DatabaseConfigRef: databaseConfigRef,
	}
}
