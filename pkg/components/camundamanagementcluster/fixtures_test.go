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

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The fixture identities. Every value is fixed, so the golden manifests stay
// deterministic.
const (
	fixtureName      = "my-management"
	fixtureNamespace = "camunda"
	fixturePlatform  = "my-platform"
	fixtureVersion   = "8.9.4"
	fixtureIssuer    = "https://login.example.com"
	fixtureExternal  = "https://identity.example.com"
)

// newCluster returns the minimal CamundaManagementCluster in the oidc mode,
// with mutate applied to it.
func newCluster(mutate func(mc *v1.CamundaManagementCluster)) *v1.CamundaManagementCluster {
	mc := &v1.CamundaManagementCluster{
		ObjectMeta: metav1.ObjectMeta{Name: fixtureName, Namespace: fixtureNamespace},
		Spec: v1.CamundaManagementClusterSpec{
			PlatformConfigRef: fixturePlatform,
			IdentityProvider:  v1.IdentityProviderSpec{OIDC: &v1.ManagementOIDCSpec{}},
			Identity: v1.IdentitySpec{
				Version:           fixtureVersion,
				ExternalURL:       fixtureExternal,
				DatabaseConfigRef: "identity-db",
				Admin:             v1.IdentityAdminSpec{ClaimName: "oid", ClaimValue: "admin-oid"},
			},
		},
	}
	if mutate != nil {
		mutate(mc)
	}

	return mc
}

// newPlatform returns the minimal CamundaPlatformConfig spec of an oidc
// platform that declares the two clients the management plane always needs.
func newPlatform(mutate func(p *v1.CamundaPlatformConfigSpec)) *v1.CamundaPlatformConfigSpec {
	p := &v1.CamundaPlatformConfigSpec{
		Auth: &v1.PlatformAuthSpec{
			Method: v1.AuthenticationMethodOIDC,
			OIDC: &v1.OIDCSpec{
				IssuerURL: fixtureIssuer,
				AuthURL:   fixtureIssuer + "/oauth/authorize",
				TokenURL:  fixtureIssuer + "/oauth/token",
				JWKSURL:   fixtureIssuer + "/.well-known/jwks.json",
				ClientID:  "camunda",
				ClientSecretRef: v1.SecretKeyRef{
					Name:      "oidc-credentials",
					Namespace: "platform",
					Key:       "camunda-client-secret",
				},
				Management: &v1.ManagementOIDCClientsSpec{Clients: v1.ManagementClients{
					Identity: &v1.ConfidentialClientSpec{
						ClientID: "management-identity",
						Audience: "management-identity-api",
						ClientSecretRef: v1.SecretKeyRef{
							Name:      "oidc-credentials",
							Namespace: "platform",
							Key:       "identity-client-secret",
						},
					},
					Optimize: &v1.ConfidentialClientSpec{
						ClientID: "optimize",
						ClientSecretRef: v1.SecretKeyRef{
							Name:      "oidc-credentials",
							Namespace: "platform",
							Key:       "optimize-client-secret",
						},
					},
				}},
			},
		},
	}
	if mutate != nil {
		mutate(p)
	}

	return p
}

// newInput returns the minimal render input, with mutate applied to it. The
// identity provider is resolved from the platform config, the way the
// controller resolves it.
func newInput(t *testing.T, mutate func(in *Input)) Input {
	t.Helper()

	in := Input{
		Cluster:  newCluster(nil),
		Platform: newPlatform(nil),
		Databases: Databases{Identity: Database{
			Host: "postgres.camunda.svc",
			Port: 5432,
			Name: "identity",
			Credentials: v1.CredentialsSecretRef{
				Name:        "identity-db-credentials",
				Namespace:   fixtureNamespace,
				UsernameKey: "username",
				PasswordKey: "password",
			},
		}},
	}
	if mutate != nil {
		mutate(&in)
	}

	provider, err := ResolveIdentityProvider(in)
	require.NoError(t, err)
	in.Provider = provider

	return in
}

// fixtureMinimal is a management cluster with nothing but the required fields.
func fixtureMinimal(t *testing.T) Input {
	t.Helper()

	return newInput(t, nil)
}

// fixtureRealistic exercises every override surface: a Microsoft Entra ID
// provider with a username claim, a platform registry and license, copies of
// referenced Secrets, replicas, resources, scheduling, pod metadata, and extra
// environment.
func fixtureRealistic(t *testing.T) Input {
	t.Helper()

	return newInput(t, func(in *Input) {
		in.Platform = newPlatform(func(p *v1.CamundaPlatformConfigSpec) {
			p.ImageRegistry = "registry.example.com/mirror"
			p.LicenseSecretRef = &v1.SecretKeyRef{
				Name:      MirroredSecretName(in.Cluster, MirrorPurposeLicense),
				Namespace: fixtureNamespace,
				Key:       "license",
			}
			p.Auth.OIDC.ProviderType = v1.OIDCProviderMicrosoft
			p.Auth.OIDC.UsernameClaim = "unique_name"
			p.Auth.OIDC.Management.Clients.Identity.ClientSecretRef = v1.SecretKeyRef{
				Name:      MirroredSecretName(in.Cluster, MirrorPurposeIdentityClient),
				Namespace: fixtureNamespace,
				Key:       "identity-client-secret",
			}
		})
		in.Databases.Identity.Credentials.Name = MirroredSecretName(in.Cluster, MirrorPurposeIdentityDB)
		in.Mirrors = map[MirrorPurpose]map[string][]byte{
			MirrorPurposeLicense:        {"license": []byte("golden-license")},
			MirrorPurposeIdentityClient: {"identity-client-secret": []byte("golden-client-secret")},
			MirrorPurposeIdentityDB:     {"username": []byte("identity"), "password": []byte("golden-password")},
		}
		in.HashInputs = []string{"Secret/platform/oidc-credentials=42"}
		in.Cluster.Spec.Identity.WorkloadSpec = v1.WorkloadSpec{
			Replicas: new(int32(2)),
			Resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
			},
			ExtraEnv: []corev1.EnvVar{{Name: "IDENTITY_LOG_LEVEL", Value: "DEBUG"}},
			ExtraEnvFrom: []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: "identity-extra"},
			}}},
			PodLabels:      map[string]string{"team": "platform"},
			PodAnnotations: map[string]string{"prometheus.io/scrape": "true"},
			Scheduling: &v1.SchedulingSpec{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{{
							MatchExpressions: []corev1.NodeSelectorRequirement{{
								Key:      "topology.kubernetes.io/zone",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"eu-west-1a"},
							}},
						}},
					},
				},
				Tolerations: []corev1.Toleration{{Key: "camunda", Operator: corev1.TolerationOpExists}},
			},
		}
	})
}

// goldenFixtures are the fixtures that the golden test renders, by directory
// name.
func goldenFixtures(t *testing.T) map[string]Input {
	t.Helper()

	return map[string]Input{
		"oidc/minimal":   fixtureMinimal(t),
		"oidc/realistic": fixtureRealistic(t),
	}
}

// renderedEnv returns the rendered environment of Management Identity as a map
// from the variable name to its value or its reference.
func renderedEnv(in Input) map[string]string {
	env := map[string]string{}
	for _, e := range componentEnv(in, ComponentIdentity) {
		env[e.Name] = envValue(e)
	}

	return env
}
