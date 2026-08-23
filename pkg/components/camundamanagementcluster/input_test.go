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
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// The oidc mode reads the issuer, the endpoints, and the clients of the
// platform config. The issuer is both the front-channel and the back-channel
// issuer, because Camunda does not support split-horizon URLs for a generic
// provider.
func TestResolveIdentityProviderReadsThePlatformConfig(t *testing.T) {
	t.Parallel()

	provider := fixtureMinimal(t).Provider

	assert.Equal(t, ModeOIDC, provider.Mode)
	assert.Equal(t, "GENERIC", provider.Type)
	assert.Equal(t, "oidc", provider.SpringProfile)
	assert.Equal(t, fixtureIssuer, provider.IssuerURL)
	assert.Equal(t, fixtureIssuer, provider.IssuerBackendURL)
	assert.Equal(t, fixtureIssuer+"/oauth/authorize", provider.AuthURL)
	assert.Equal(t, "management-identity", provider.Clients.Identity.ID)
	assert.Equal(t, "optimize", provider.Clients.Optimize.ID)
	assert.Empty(t, provider.KeycloakURL)
	assert.Empty(t, provider.Realm)
}

// An audience that the platform config leaves empty means the client id.
func TestResolveIdentityProviderDefaultsTheAudience(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "optimize", fixtureMinimal(t).Provider.Clients.Optimize.Audience)
}

// A platform config that authenticates with basic cannot serve the oidc mode.
func TestResolveIdentityProviderRefusesABasicPlatformConfig(t *testing.T) {
	t.Parallel()

	_, err := ResolveIdentityProvider(Input{
		Cluster:  newCluster(nil),
		Platform: &v1.CamundaPlatformConfigSpec{},
	})

	failure := preCheckFailure(t, err)
	assert.Equal(t, v1.ReasonInvalidReference, failure.Reason)
	assert.Contains(t, failure.Message, "spec.auth.method is oidc")
}

// The ManagementAuthConfig carries all three endpoints, and the operator makes
// no request to the identity provider, so the platform config must name them.
func TestResolveIdentityProviderRequiresTheEndpoints(t *testing.T) {
	t.Parallel()

	_, err := ResolveIdentityProvider(Input{
		Cluster: newCluster(nil),
		Platform: newPlatform(func(p *v1.CamundaPlatformConfigSpec) {
			p.Auth.OIDC.AuthURL = ""
			p.Auth.OIDC.JWKSURL = ""
		}),
	})

	failure := preCheckFailure(t, err)
	assert.Equal(t, v1.ReasonInvalidReference, failure.Reason)
	assert.Contains(t, failure.Message, "spec.auth.oidc.authUrl")
	assert.Contains(t, failure.Message, "spec.auth.oidc.jwksUrl")
	assert.NotContains(t, failure.Message, "spec.auth.oidc.tokenUrl")
}

// Every component that the spec deploys needs a client. Management Identity
// and Optimize are always required; Console and Web Modeler only when the spec
// deploys them.
func TestResolveIdentityProviderRequiresAClientPerComponent(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		mutate  func(mc *v1.CamundaManagementCluster, p *v1.CamundaPlatformConfigSpec)
		missing string
	}{
		{
			name: "identity",
			mutate: func(_ *v1.CamundaManagementCluster, p *v1.CamundaPlatformConfigSpec) {
				p.Auth.OIDC.Management.Clients.Identity = nil
			},
			missing: "clients.identity",
		},
		{
			name: "optimize",
			mutate: func(_ *v1.CamundaManagementCluster, p *v1.CamundaPlatformConfigSpec) {
				p.Auth.OIDC.Management.Clients.Optimize = nil
			},
			missing: "clients.optimize",
		},
		{
			name: "console",
			mutate: func(mc *v1.CamundaManagementCluster, _ *v1.CamundaPlatformConfigSpec) {
				mc.Spec.Console = &v1.ConsoleSpec{Version: fixtureVersion, ExternalURL: fixtureExternal}
			},
			missing: "clients.console",
		},
		{
			name: "management block absent",
			mutate: func(_ *v1.CamundaManagementCluster, p *v1.CamundaPlatformConfigSpec) {
				p.Auth.OIDC.Management = nil
			},
			missing: "clients.identity",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mc := newCluster(nil)
			platform := newPlatform(nil)
			tt.mutate(mc, platform)

			_, err := ResolveIdentityProvider(Input{Cluster: mc, Platform: platform})

			failure := preCheckFailure(t, err)
			assert.Equal(t, v1.ReasonInvalidReference, failure.Reason)
			assert.Contains(t, failure.Message, tt.missing)
		})
	}
}

// preCheckFailure asserts that err is a pre-check failure and returns it.
func preCheckFailure(t *testing.T, err error) *conditions.PreCheckFailure {
	t.Helper()

	var failure *conditions.PreCheckFailure
	require.ErrorAs(t, err, &failure)

	return failure
}
