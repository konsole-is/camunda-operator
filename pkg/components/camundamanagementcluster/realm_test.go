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
	appsv1 "k8s.io/api/apps/v1"
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
		{
			// A URL that carries a user reaches the same realm as one without
			// it, and the password must not reach the annotations of a claim.
			name: "drops a user in the URL",
			target: v1.KeycloakRealmTarget{
				URL: "https://admin:s3cret@kc.example.com/auth", Realm: "camunda-platform",
			},
			want: "https://kc.example.com/auth/realms/camunda-platform",
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
			name: "a terminating pod that is not ready still blocks it",
			pods: []corev1.Pod{pod("https://kc.example.com/auth", terminating)},
			want: true,
		},
		{
			name: "a terminating pod that is ready does not",
			pods: []corev1.Pod{pod("https://kc.example.com/auth", terminating, ready)},
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

// A pod that can still run keeps the record of its realm alive, ready or
// not: a restart of it writes the client again from its environment.
func TestIdentityPointsAtRealm(t *testing.T) {
	t.Parallel()

	target := v1.KeycloakRealmTarget{URL: "https://kc.example.com/auth", Realm: "camunda-platform"}
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
			name: "a ready pod against the realm keeps the record",
			pods: []corev1.Pod{identityRealmPod("https://kc.example.com/auth", ready)},
			want: true,
		},
		{
			name: "a terminating pod against the realm keeps it too",
			pods: []corev1.Pod{identityRealmPod("https://kc.example.com/auth", terminating)},
			want: true,
		},
		{
			name: "a pod against another Keycloak does not",
			pods: []corev1.Pod{identityRealmPod("https://other.example.com/auth", ready)},
			want: false,
		},
		{
			name: "a failed pod does not",
			pods: []corev1.Pod{identityRealmPod("https://kc.example.com/auth", failed)},
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

			assert.Equal(t, test.want, IdentityPointsAtRealm(test.pods, target))
		})
	}
}

// The sweep gives back every realm that nothing of the plane points at, so
// what counts here decides which claim it keeps.
func TestIdentityRealms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		pods []corev1.Pod
		want []string
	}{
		{
			name: "a running pod names its realm",
			pods: []corev1.Pod{identityRealmPod("https://kc.example.com/auth")},
			want: []string{"https://kc.example.com/auth"},
		},
		{
			// The initializer of a pod that is going away still writes the
			// clients of its realm, and the ReplicaSet of it starts the next
			// pod against the same realm.
			name: "a terminating pod names its realm until it is gone",
			pods: []corev1.Pod{identityRealmPod("https://kc.example.com/auth", func(p *corev1.Pod) {
				now := metav1.Now()
				p.DeletionTimestamp = &now
			})},
			want: []string{"https://kc.example.com/auth"},
		},
		{
			name: "a failed pod names none",
			pods: []corev1.Pod{identityRealmPod("https://kc.example.com/auth", func(p *corev1.Pod) {
				p.Status.Phase = corev1.PodFailed
			})},
			want: []string{},
		},
		{
			name: "a pod of the oidc mode names none",
			pods: []corev1.Pod{identityRealmPod("")},
			want: []string{},
		},
		{
			name: "the pods of a rollout name both realms",
			pods: []corev1.Pod{
				identityRealmPod("https://old.example.com/auth"),
				identityRealmPod("https://new.example.com/auth"),
			},
			want: []string{"https://old.example.com/auth", "https://new.example.com/auth"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			realms, unknown := IdentityRealms(test.pods)
			assert.False(t, unknown)
			urls := make([]string, 0, len(realms))
			for _, realm := range realms {
				urls = append(urls, realm.URL)
			}
			assert.Equal(t, test.want, urls)
		})
	}
}

// A pod whose Keycloak URL or realm comes from a reference can write any
// realm, so the sweep must keep every claim of the plane rather than read the
// pod as one that names none.
func TestIdentityRealmsOfAPodWithAReference(t *testing.T) {
	t.Parallel()

	pod := identityRealmPod("https://kc.example.com/auth", func(p *corev1.Pod) {
		p.Spec.Containers[0].Env[0].ValueFrom = identityRealmReference
	})

	realms, unknown := IdentityRealms([]corev1.Pod{pod})

	assert.True(t, unknown)
	assert.Empty(t, realms)
}

// A value derived from status.callbackRealm by hand matches the recorded
// identity whatever the case of its host or a spelled-out default port.
func TestNormalizeRealmIdentity(t *testing.T) {
	t.Parallel()

	target := v1.KeycloakRealmTarget{URL: "https://KC.Example.com:443/auth", Realm: "camunda-platform"}

	assert.Equal(t, RealmIdentity(target), NormalizeRealmIdentity(
		"https://KC.Example.com:443/auth/realms/camunda-platform",
	))
	assert.NotEqual(t, RealmIdentity(target), NormalizeRealmIdentity(
		"https://kc.example.com/auth/realms/Camunda-Platform",
	))
}

// A workload template is read the same way a pod is: what its Management
// Identity container points at is the realm its pods would write.
func TestIdentityTemplatePointsAtRealm(t *testing.T) {
	t.Parallel()

	target := v1.KeycloakRealmTarget{URL: "https://kc.example.com/auth", Realm: "camunda-platform"}
	spec := func(url string) *corev1.PodSpec {
		return &corev1.PodSpec{Containers: []corev1.Container{{
			Name: identityContainer,
			Env: []corev1.EnvVar{
				{Name: keycloakEnvURL, Value: url},
				{Name: keycloakEnvRealm, Value: "camunda-platform"},
			},
		}}}
	}

	assert.True(t, IdentityTemplatePointsAtRealm(spec("https://kc.example.com/auth"), target))
	assert.False(t, IdentityTemplatePointsAtRealm(spec("https://other.example.com/auth"), target))
	assert.False(t, IdentityTemplatePointsAtRealm(&corev1.PodSpec{}, target))
}

// spec.identity.extraEnv can replace the Keycloak URL or the realm with a
// reference, and the value behind it is not in the workload. Such a container
// can write any realm, so it must not read as one that writes none: the
// record it would silently release is the only way back to that realm.
func TestIdentityTemplateWithAReferenceWritesEveryRealm(t *testing.T) {
	t.Parallel()

	target := v1.KeycloakRealmTarget{URL: "https://kc.example.com/auth", Realm: "camunda-platform"}
	reference := identityRealmReference
	spec := func(env ...corev1.EnvVar) *corev1.PodSpec {
		return &corev1.PodSpec{Containers: []corev1.Container{{
			Name: identityContainer,
			Env:  env,
		}}}
	}

	assert.True(t, IdentityTemplatePointsAtRealm(spec(
		corev1.EnvVar{Name: keycloakEnvURL, ValueFrom: reference},
		corev1.EnvVar{Name: keycloakEnvRealm, Value: "camunda-platform"},
	), target))
	assert.True(t, IdentityTemplatePointsAtRealm(spec(
		corev1.EnvVar{Name: keycloakEnvURL, Value: "https://other.example.com/auth"},
		corev1.EnvVar{Name: keycloakEnvRealm, ValueFrom: reference},
	), target))
	// A reference on another variable says nothing about the realm.
	assert.False(t, IdentityTemplatePointsAtRealm(spec(
		corev1.EnvVar{Name: keycloakEnvURL, Value: "https://other.example.com/auth"},
		corev1.EnvVar{Name: keycloakEnvRealm, Value: "camunda-platform"},
		corev1.EnvVar{Name: "IDENTITY_LOG_LEVEL", ValueFrom: reference},
	), target))
}

// The template outlives the pods of it, so a Deployment that still names a
// realm holds the claim on it through the gap between two pods.
func TestIdentityTemplateRealms(t *testing.T) {
	t.Parallel()

	deployment := func(url string, replicas *int32) *appsv1.Deployment {
		return &appsv1.Deployment{Spec: appsv1.DeploymentSpec{
			Replicas: replicas,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: identityRealmContainers(url)},
			},
		}}
	}
	zero, one := int32(0), int32(1)

	t.Run("a template that names a Keycloak points at its realm", func(t *testing.T) {
		t.Parallel()

		realms, unknown := IdentityTemplateRealms(deployment("https://kc.example.com/auth", &one))
		assert.False(t, unknown)
		require.Len(t, realms, 1)
		assert.Equal(t, "https://kc.example.com/auth", realms[0].URL)
		assert.Equal(t, "camunda-platform", realms[0].Realm)
	})

	t.Run("an unset replica count is one replica", func(t *testing.T) {
		t.Parallel()

		realms, _ := IdentityTemplateRealms(deployment("https://kc.example.com/auth", nil))
		assert.Len(t, realms, 1)
	})

	t.Run("a Deployment scaled to zero points at none", func(t *testing.T) {
		t.Parallel()

		realms, unknown := IdentityTemplateRealms(deployment("https://kc.example.com/auth", &zero))
		assert.Empty(t, realms)
		assert.False(t, unknown)
	})

	t.Run("a template of the oidc mode points at none", func(t *testing.T) {
		t.Parallel()

		realms, unknown := IdentityTemplateRealms(deployment("", &one))
		assert.Empty(t, realms)
		assert.False(t, unknown)
	})

	// The realm behind a reference is not in the template, so no claim of the
	// plane can be shown to be unused.
	t.Run("a template that takes its realm from a reference names none", func(t *testing.T) {
		t.Parallel()

		d := deployment("https://kc.example.com/auth", &one)
		d.Spec.Template.Spec.Containers[0].Env[1].ValueFrom = identityRealmReference

		realms, unknown := IdentityTemplateRealms(d)

		assert.Empty(t, realms)
		assert.True(t, unknown)
	})
}

// The old ReplicaSet of a rollout keeps the template the Deployment left, so
// it can start a pod against the old realm until it is scaled to zero.
func TestIdentityReplicaSetRealms(t *testing.T) {
	t.Parallel()

	set := func(url string, replicas int32) appsv1.ReplicaSet {
		return appsv1.ReplicaSet{Spec: appsv1.ReplicaSetSpec{
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: identityRealmContainers(url)},
			},
		}}
	}

	realms, unknown := IdentityReplicaSetRealms([]appsv1.ReplicaSet{
		set("https://old.example.com/auth", 1),
		set("https://new.example.com/auth", 2),
		set("https://scaled-down.example.com/auth", 0),
		set("", 1),
	})

	assert.False(t, unknown)
	urls := make([]string, 0, len(realms))
	for _, realm := range realms {
		urls = append(urls, realm.URL)
	}
	assert.Equal(t, []string{"https://old.example.com/auth", "https://new.example.com/auth"}, urls)
}

// identityRealmPod is a Management Identity pod that points at url. An empty
// url is the oidc mode, which names no Keycloak.
func identityRealmPod(url string, mutate ...func(p *corev1.Pod)) corev1.Pod {
	pod := corev1.Pod{Spec: corev1.PodSpec{Containers: identityRealmContainers(url)}}
	for _, m := range mutate {
		m(&pod)
	}

	return pod
}

// identityRealmContainers is the container list of a Management Identity pod
// or pod template that points at url.
func identityRealmContainers(url string) []corev1.Container {
	return []corev1.Container{{
		Name: identityContainer,
		Env: []corev1.EnvVar{
			{Name: keycloakEnvURL, Value: url},
			{Name: keycloakEnvRealm, Value: "camunda-platform"},
		},
	}}
}

// identityRealmReference stands for a value that spec.identity.extraEnv took
// from a ConfigMap, which is not in the workload.
var identityRealmReference = &corev1.EnvVarSource{
	ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "identity-extra"},
		Key:                  "url",
	},
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
