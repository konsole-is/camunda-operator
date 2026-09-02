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
// that signs in to the realm of these tests at url with the Secret named
// secret.
func externalKeycloakCluster(url, secret string) *v1.CamundaManagementCluster {
	return &v1.CamundaManagementCluster{
		Spec: v1.CamundaManagementClusterSpec{
			IdentityProvider: v1.IdentityProviderSpec{
				ExternalKeycloak: &v1.ExternalKeycloakSpec{
					URL:   url,
					Realm: "camunda-platform",
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

// A rotation of the administrator Secret changes no realm, so a record from
// before it can still name the Secret that was replaced. The deletion signs
// in with the Secret of the spec first, and it keeps the recorded one as the
// second try, for a spec whose Secret is the broken one.
func TestWithdrawalRealmsTriesTheSpecThenTheRecordOfOneRealm(t *testing.T) {
	t.Parallel()

	mc := externalKeycloakCluster("https://kc.example.com/auth", "rotated")
	mc.Status.CallbackRealm = &v1.KeycloakRealmTarget{
		URL:   "https://kc.example.com/auth",
		Realm: "camunda-platform",
		AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
			Name:        "replaced",
			UsernameKey: "username",
			PasswordKey: "password",
		},
	}

	realms := withdrawalRealms(context.Background(), mc)

	require.Len(t, realms, 2)
	require.NotNil(t, realms[0].AdminCredentials)
	assert.Equal(t, "rotated", realms[0].AdminCredentials.Name)
	require.NotNil(t, realms[1].AdminCredentials)
	assert.Equal(t, "replaced", realms[1].AdminCredentials.Name)
}

// A record that names what the spec names is the same try twice.
func TestWithdrawalRealmsOfAnUnchangedRecord(t *testing.T) {
	t.Parallel()

	mc := externalKeycloakCluster("https://kc.example.com/auth", "keycloak-admin")
	mc.Status.CallbackRealm = &v1.KeycloakRealmTarget{
		URL:   "https://kc.example.com/auth",
		Realm: "camunda-platform",
		AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
			Name:        "keycloak-admin",
			UsernameKey: "username",
			PasswordKey: "password",
		},
	}

	realms := withdrawalRealms(context.Background(), mc)

	require.Len(t, realms, 1)
	assert.Equal(t, "https://kc.example.com/auth", realms[0].KeycloakURL)
}

// The spec names another realm than the record, so the record is the only way
// back to the realm that holds the callbacks.
func TestWithdrawalRealmsKeepTheRecordOfAnotherRealm(t *testing.T) {
	t.Parallel()

	mc := externalKeycloakCluster("https://new.example.com/auth", "new-admin")
	mc.Status.CallbackRealm = &v1.KeycloakRealmTarget{
		URL:   "https://old.example.com/auth",
		Realm: "camunda-platform",
		AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
			Name:        "old-admin",
			UsernameKey: "username",
			PasswordKey: "password",
		},
	}

	realms := withdrawalRealms(context.Background(), mc)

	require.Len(t, realms, 1)
	assert.Equal(t, "https://old.example.com/auth", realms[0].KeycloakURL)
	require.NotNil(t, realms[0].AdminCredentials)
	assert.Equal(t, "old-admin", realms[0].AdminCredentials.Name)
}

// A plane that recorded no realm still runs a Management Identity against the
// realm of the spec, so that realm is the one to tidy.
func TestWithdrawalRealmsFallBackToTheSpec(t *testing.T) {
	t.Parallel()

	mc := externalKeycloakCluster("https://kc.example.com/auth", "keycloak-admin")

	realms := withdrawalRealms(context.Background(), mc)

	require.Len(t, realms, 1)
	assert.Equal(t, "https://kc.example.com/auth", realms[0].KeycloakURL)
	assert.Equal(t, "camunda-platform", realms[0].Realm)
}

// The oidc mode registers nothing, so a plane that recorded no realm has
// nothing to withdraw from.
func TestWithdrawalRealmsOfTheOIDCModeWithoutARecord(t *testing.T) {
	t.Parallel()

	mc := &v1.CamundaManagementCluster{
		Spec: v1.CamundaManagementClusterSpec{
			IdentityProvider: v1.IdentityProviderSpec{OIDC: &v1.ManagementOIDCSpec{}},
		},
	}

	assert.Empty(t, withdrawalRealms(context.Background(), mc))
}
