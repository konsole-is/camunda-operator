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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// externalKeycloakCluster is a management plane in the externalKeycloak mode
// that signs in to realm at url with the Secret named secret.
func externalKeycloakCluster(url, realm, secret string) *v1.CamundaManagementCluster {
	return &v1.CamundaManagementCluster{
		Spec: v1.CamundaManagementClusterSpec{
			IdentityProvider: v1.IdentityProviderSpec{
				ExternalKeycloak: &v1.ExternalKeycloakSpec{
					URL:   url,
					Realm: realm,
					AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
						Name:        secret,
						UsernameKey: "username",
						PasswordKey: "password",
					},
				},
			},
		},
	}
}

// The record keeps the Secrets of the pass that wrote it, and a rotation of
// them changes no realm, so the record is not written again. A deletion in
// that state must sign in with the Secret of the spec and not with the one
// that was replaced.
func TestWithdrawalRealmPrefersTheSpecForTheRecordedRealm(t *testing.T) {
	t.Parallel()

	mc := externalKeycloakCluster("https://kc.example.com/auth", "camunda-platform", "rotated")
	mc.Status.CallbackRealm = &v1.KeycloakRealmTarget{
		URL:   "https://kc.example.com/auth",
		Realm: "camunda-platform",
		AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
			Name:        "replaced",
			UsernameKey: "username",
			PasswordKey: "password",
		},
	}

	provider, registered := withdrawalRealm(context.Background(), mc)

	require.True(t, registered)
	assert.Equal(t, "camunda-platform", provider.Realm)
	require.NotNil(t, provider.AdminCredentials)
	assert.Equal(t, "rotated", provider.AdminCredentials.Name)
}

// The spec names another realm than the record, so the record is the only way
// back to the realm that holds the callbacks.
func TestWithdrawalRealmKeepsTheRecordOfAnotherRealm(t *testing.T) {
	t.Parallel()

	mc := externalKeycloakCluster("https://new.example.com/auth", "camunda-platform", "new-admin")
	mc.Status.CallbackRealm = &v1.KeycloakRealmTarget{
		URL:   "https://old.example.com/auth",
		Realm: "camunda-platform",
		AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
			Name:        "old-admin",
			UsernameKey: "username",
			PasswordKey: "password",
		},
	}

	provider, registered := withdrawalRealm(context.Background(), mc)

	require.True(t, registered)
	assert.Equal(t, "https://old.example.com/auth", provider.KeycloakURL)
	require.NotNil(t, provider.AdminCredentials)
	assert.Equal(t, "old-admin", provider.AdminCredentials.Name)
}

// A plane that recorded no realm still runs a Management Identity against the
// realm of the spec, so that realm is the one to tidy.
func TestWithdrawalRealmFallsBackToTheSpec(t *testing.T) {
	t.Parallel()

	mc := externalKeycloakCluster("https://kc.example.com/auth", "camunda-platform", "keycloak-admin")

	provider, registered := withdrawalRealm(context.Background(), mc)

	require.True(t, registered)
	assert.Equal(t, "https://kc.example.com/auth", provider.KeycloakURL)
	assert.Equal(t, "camunda-platform", provider.Realm)
}

// The oidc mode registers nothing, so a plane that recorded no realm has
// nothing to withdraw from.
func TestWithdrawalRealmOfTheOIDCModeWithoutARecord(t *testing.T) {
	t.Parallel()

	mc := &v1.CamundaManagementCluster{
		Spec: v1.CamundaManagementClusterSpec{
			IdentityProvider: v1.IdentityProviderSpec{OIDC: &v1.ManagementOIDCSpec{}},
		},
	}

	_, registered := withdrawalRealm(context.Background(), mc)

	assert.False(t, registered)
}
