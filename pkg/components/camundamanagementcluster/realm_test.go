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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

	assert.Equal(
		t, &v1.KeycloakRealmTarget{
			URL:   "https://keycloak.example.com/auth",
			Realm: "camunda-platform",
			AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
				Name: "keycloak-admin", UsernameKey: "username", PasswordKey: "password",
			},
			CABundleSecretRef: &v1.LocalSecretKeyRef{Name: "keycloak-ca", Key: "ca.crt"},
		}, RealmTarget(provider),
	)
}

// The oidc mode administers no realm, so there is nothing to record.
func TestRealmTargetOfTheOIDCModeIsNil(t *testing.T) {
	t.Parallel()

	assert.Nil(t, RealmTarget(IdentityProvider{Mode: ModeOIDC, IssuerURL: "https://login.example.com"}))
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

	require.NotNil(t, target)
	require.NotNil(t, target.CABundleSecretRef)
	assert.Equal(t, "keycloak-ca", target.CABundleSecretRef.Name)
}

func TestRealmTargetOfAKeycloakWithNoBundle(t *testing.T) {
	t.Parallel()

	target := RealmTarget(IdentityProvider{
		Mode:             ModeKeycloak,
		KeycloakURL:      "http://my-management-keycloak.my-ns.svc:8080/auth",
		Realm:            "camunda-platform",
		AdminCredentials: &v1.LocalCredentialsSecretRef{Name: "keycloak-admin"},
	})

	require.NotNil(t, target)
	assert.Nil(t, target.CABundleSecretRef)
}

// A provider that names no administrator gives the operator nothing to sign
// in with, so there is nothing worth recording.
func TestRealmTargetWithoutAnAdministratorIsNil(t *testing.T) {
	t.Parallel()

	assert.Nil(t, RealmTarget(IdentityProvider{
		Mode:        ModeExternalKeycloak,
		KeycloakURL: "https://kc.example.com/auth",
		Realm:       "camunda-platform",
	}))
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
		{
			name:   "folds the case of the scheme and of the host",
			target: v1.KeycloakRealmTarget{URL: "HTTPS://KC.Example.com/auth", Realm: "camunda-platform"},
			want:   "https://kc.example.com/auth/realms/camunda-platform",
		},
		{
			name:   "drops the default port of https",
			target: v1.KeycloakRealmTarget{URL: "https://kc.example.com:443/auth", Realm: "camunda-platform"},
			want:   "https://kc.example.com/auth/realms/camunda-platform",
		},
		{
			name:   "drops the default port of http",
			target: v1.KeycloakRealmTarget{URL: "http://kc.example.com:80/auth", Realm: "camunda-platform"},
			want:   "http://kc.example.com/auth/realms/camunda-platform",
		},
		{
			name:   "keeps a port that is not the default",
			target: v1.KeycloakRealmTarget{URL: "https://kc.example.com:8443/auth", Realm: "camunda-platform"},
			want:   "https://kc.example.com:8443/auth/realms/camunda-platform",
		},
		{
			name:   "keeps the case of the path",
			target: v1.KeycloakRealmTarget{URL: "https://kc.example.com/Auth", Realm: "camunda-platform"},
			want:   "https://kc.example.com/Auth/realms/camunda-platform",
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
			name:  "the case of the host and a default port name the same realm",
			other: v1.KeycloakRealmTarget{URL: "https://KC.example.com:443/auth", Realm: "camunda-platform"},
			want:  true,
		},
		{
			name:  "another case of the realm is another realm",
			other: v1.KeycloakRealmTarget{URL: "https://kc.example.com/auth", Realm: "Camunda-Platform"},
			want:  false,
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

// The pods below stand for the states one rollout of the Management Identity
// Deployment can hold at once. Only a pod that is starting against the realm
// of the target blocks a withdrawal from it.
func TestIdentityWritesRealm(t *testing.T) {
	t.Parallel()

	target := v1.KeycloakRealmTarget{URL: "https://kc.example.com/auth", Realm: "camunda-platform"}
	pod := func(url string, mutate ...func(p *corev1.Pod)) corev1.Pod {
		p := corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name: identityContainer,
			Env: []corev1.EnvVar{
				{Name: keycloakEnvURL, Value: url},
				{Name: keycloakEnvRealm, Value: "camunda-platform"},
			},
		}}}}
		for _, m := range mutate {
			m(&p)
		}

		return p
	}
	ready := func(p *corev1.Pod) {
		p.Status.Conditions = []corev1.PodCondition{{
			Type: corev1.PodReady, Status: corev1.ConditionTrue,
		}}
	}
	terminating := func(p *corev1.Pod) {
		now := metav1.Now()
		p.DeletionTimestamp = &now
	}
	failed := func(p *corev1.Pod) { p.Status.Phase = corev1.PodFailed }

	tests := []struct {
		name string
		pods []corev1.Pod
		want bool
	}{
		{
			name: "a starting pod against the realm blocks the withdrawal",
			pods: []corev1.Pod{pod("https://kc.example.com/auth")},
			want: true,
		},
		{
			name: "a ready pod against the realm does not",
			pods: []corev1.Pod{pod("https://kc.example.com/auth", ready)},
			want: false,
		},
		{
			name: "a starting pod against another Keycloak does not",
			pods: []corev1.Pod{pod("https://other.example.com/auth")},
			want: false,
		},
		{
			name: "a starting pod of the oidc mode does not",
			pods: []corev1.Pod{pod("")},
			want: false,
		},
		{
			name: "a terminating pod does not",
			pods: []corev1.Pod{pod("https://kc.example.com/auth", terminating)},
			want: false,
		},
		{
			name: "a failed pod does not",
			pods: []corev1.Pod{pod("https://kc.example.com/auth", failed)},
			want: false,
		},
		{
			name: "no pods at all do not",
			pods: nil,
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, IdentityWritesRealm(test.pods, target))
		})
	}
}

// A record round-trips: the provider built from it names the same realm.
func TestRealmProviderKeepsTheIdentityOfTheRecord(t *testing.T) {
	t.Parallel()

	target := v1.KeycloakRealmTarget{
		URL:                       "https://kc.example.com/auth",
		Realm:                     "camunda-platform",
		AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{Name: "keycloak-admin"},
	}

	recorded := RealmTarget(RealmProvider(target))
	require.NotNil(t, recorded)
	assert.True(t, SameRealm(target, *recorded))
}
