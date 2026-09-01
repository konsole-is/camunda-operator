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

func TestRealmTargetRecordsWhereTheRealmIsAndHowToSignIn(t *testing.T) {
	t.Parallel()

	provider := IdentityProvider{
		Mode:        ModeExternalKeycloak,
		KeycloakURL: "https://keycloak.example.com/auth",
		Realm:       "camunda-platform",
		AdminCredentials: &v1.LocalCredentialsSecretRef{
			Name: "keycloak-admin", UsernameKey: "username", PasswordKey: "password",
		},
		CABundle: &v1.LocalSecretKeyRef{Name: "keycloak-ca", Key: "ca.crt"},
	}

	assert.Equal(t, v1.KeycloakRealmTarget{
		URL:   "https://keycloak.example.com/auth",
		Realm: "camunda-platform",
		AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
			Name: "keycloak-admin", UsernameKey: "username", PasswordKey: "password",
		},
		CABundleSecretRef: &v1.LocalSecretKeyRef{Name: "keycloak-ca", Key: "ca.crt"},
	}, RealmTarget(provider))
}

// The record outlives the provider it came from, so it must not point into it.
func TestRealmTargetCopiesTheReferences(t *testing.T) {
	t.Parallel()

	bundle := &v1.LocalSecretKeyRef{Name: "keycloak-ca", Key: "ca.crt"}
	provider := IdentityProvider{
		KeycloakURL:      "https://keycloak.example.com/auth",
		Realm:            "camunda-platform",
		AdminCredentials: &v1.LocalCredentialsSecretRef{Name: "keycloak-admin"},
		CABundle:         bundle,
	}

	target := RealmTarget(provider)
	bundle.Name = "another-ca"

	require.NotNil(t, target.CABundleSecretRef)
	assert.Equal(t, "keycloak-ca", target.CABundleSecretRef.Name)
}

func TestRealmTargetOfAKeycloakWithNoBundle(t *testing.T) {
	t.Parallel()

	target := RealmTarget(IdentityProvider{
		KeycloakURL: "http://my-management-keycloak.my-ns.svc:8080/auth",
		Realm:       "camunda-platform",
	})

	assert.Nil(t, target.CABundleSecretRef)
	assert.Equal(t, v1.LocalCredentialsSecretRef{}, target.AdminCredentialsSecretRef)
}

func TestRealmIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target v1.KeycloakRealmTarget
		want   string
	}{
		{
			name:   "names the URL and the realm",
			target: v1.KeycloakRealmTarget{URL: "https://kc.example.com/auth", Realm: "camunda-platform"},
			want:   "https://kc.example.com/auth/realms/camunda-platform",
		},
		{
			name:   "ignores a trailing slash of the URL",
			target: v1.KeycloakRealmTarget{URL: "https://kc.example.com/auth//", Realm: "camunda-platform"},
			want:   "https://kc.example.com/auth/realms/camunda-platform",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, RealmIdentity(test.target))
		})
	}
}

func TestSameRealm(t *testing.T) {
	t.Parallel()

	target := v1.KeycloakRealmTarget{
		URL:                       "https://kc.example.com/auth",
		Realm:                     "camunda-platform",
		AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{Name: "keycloak-admin"},
	}

	tests := []struct {
		name  string
		other v1.KeycloakRealmTarget
		want  bool
	}{
		{
			name:  "the same URL and realm are one realm",
			other: target,
			want:  true,
		},
		{
			name: "the administrator takes no part in the identity",
			other: v1.KeycloakRealmTarget{
				URL:                       "https://kc.example.com/auth",
				Realm:                     "camunda-platform",
				AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{Name: "another-admin"},
				CABundleSecretRef:         &v1.LocalSecretKeyRef{Name: "ca", Key: "ca.crt"},
			},
			want: true,
		},
		{
			name:  "a trailing slash names the same realm",
			other: v1.KeycloakRealmTarget{URL: "https://kc.example.com/auth/", Realm: "camunda-platform"},
			want:  true,
		},
		{
			name:  "another Keycloak is another realm",
			other: v1.KeycloakRealmTarget{URL: "https://other.example.com/auth", Realm: "camunda-platform"},
			want:  false,
		},
		{
			name:  "another realm of the same Keycloak is another realm",
			other: v1.KeycloakRealmTarget{URL: "https://kc.example.com/auth", Realm: "another-realm"},
			want:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, SameRealm(target, test.other))
		})
	}
}

// The withdrawal from a realm that the spec no longer names is built from the
// record alone, so the record has to carry everything the sign-in needs.
func TestRealmProviderSignsInToTheRecordedRealm(t *testing.T) {
	t.Parallel()

	provider := RealmProvider(v1.KeycloakRealmTarget{
		URL:   "https://kc.example.com/auth/",
		Realm: "camunda-platform",
		AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
			Name: "keycloak-admin", UsernameKey: "username", PasswordKey: "password",
		},
		CABundleSecretRef: &v1.LocalSecretKeyRef{Name: "keycloak-ca", Key: "ca.crt"},
	})

	assert.Equal(t, "https://kc.example.com/auth", provider.KeycloakURL)
	assert.Equal(t, "camunda-platform", provider.Realm)
	assert.Equal(t, "optimize", provider.Clients.Optimize.ID)
	require.NotNil(t, provider.AdminCredentials)
	assert.Equal(t, "keycloak-admin", provider.AdminCredentials.Name)
	require.NotNil(t, provider.CABundle)
	assert.Equal(t, "ca.crt", provider.CABundle.Key)
}

func TestRealmProviderOfARecordWithNoBundle(t *testing.T) {
	t.Parallel()

	provider := RealmProvider(v1.KeycloakRealmTarget{
		URL:                       "https://kc.example.com/auth",
		Realm:                     "camunda-platform",
		AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{Name: "keycloak-admin"},
	})

	assert.Nil(t, provider.CABundle)
}

// A record round-trips: the provider built from it names the same realm.
func TestRealmProviderKeepsTheIdentityOfTheRecord(t *testing.T) {
	t.Parallel()

	target := v1.KeycloakRealmTarget{
		URL:                       "https://kc.example.com/auth",
		Realm:                     "camunda-platform",
		AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{Name: "keycloak-admin"},
	}

	assert.True(t, SameRealm(target, RealmTarget(RealmProvider(target))))
}
