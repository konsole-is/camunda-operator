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
)

// A management cluster that does not deploy Console renders no component for
// it, so nothing but the spec turns Console on.
func TestConsoleRendersNothingWhileItIsDisabled(t *testing.T) {
	t.Parallel()

	comps, err := consoleComponents(fixtureMinimal(t))

	require.NoError(t, err)
	assert.Empty(t, comps)
}

// The environment of Console in the oidc mode: the oidc profile, the Identity
// SDK settings, the public client, and the discovery mode that lets an
// orchestration cluster register itself.
func TestConsoleEnvInOIDCMode(t *testing.T) {
	t.Parallel()

	in := fixtureConsoleMinimal(t)

	assert.Equal(
		t, map[string]string{
			"SPRING_PROFILES_ACTIVE":                      "oidc",
			"CAMUNDA_IDENTITY_TYPE":                       "GENERIC",
			"CAMUNDA_IDENTITY_BASE_URL":                   "http://my-management-identity.camunda.svc:80",
			"CAMUNDA_IDENTITY_ISSUER":                     fixtureIssuer,
			"CAMUNDA_IDENTITY_ISSUER_BACKEND_URL":         fixtureIssuer,
			"CAMUNDA_IDENTITY_CLIENT_ID":                  "console",
			"CAMUNDA_IDENTITY_AUDIENCE":                   "console-api",
			"CAMUNDA_CONSOLE_EXPERIMENTAL_DISCOVERY_MODE": "true",
			"NODE_ENV": consoleNodeEnv,
		}, renderedEnv(in, ComponentConsole),
	)
}

// The Keycloak modes give Console the two Keycloak base URLs and the realm.
// Console reads them from the resolved identity provider, so the renderer
// never switches on the mode.
func TestConsoleEnvInAKeycloakMode(t *testing.T) {
	t.Parallel()

	in := fixtureConsoleMinimal(t)
	in.Provider.SpringProfile = ""
	in.Provider.Type = "KEYCLOAK"
	in.Provider.KeycloakURL = "http://my-management-keycloak-service.camunda.svc:8080/auth"
	in.Provider.KeycloakPublicURL = "https://keycloak.example.com/auth"
	in.Provider.Realm = "camunda-platform"

	env := renderedEnv(in, ComponentConsole)

	assert.Equal(t, "https://keycloak.example.com/auth", env["KEYCLOAK_BASE_URL"])
	assert.Equal(
		t,
		"http://my-management-keycloak-service.camunda.svc:8080/auth",
		env["KEYCLOAK_INTERNAL_BASE_URL"],
	)
	assert.Equal(t, "camunda-platform", env["KEYCLOAK_REALM"])
	assert.NotContains(t, env, "SPRING_PROFILES_ACTIVE")
}

// The oidc mode runs Console on no Keycloak, so the three Keycloak settings
// stay out of the container.
func TestConsoleRendersNoKeycloakSettingsInOIDCMode(t *testing.T) {
	t.Parallel()

	env := renderedEnv(fixtureConsoleMinimal(t), ComponentConsole)

	assert.NotContains(t, env, "KEYCLOAK_BASE_URL")
	assert.NotContains(t, env, "KEYCLOAK_INTERNAL_BASE_URL")
	assert.NotContains(t, env, "KEYCLOAK_REALM")
}

// Console runs under the path of its external URL. A URL without a path runs
// it at the root, which is what the container does without the setting.
func TestConsoleContextPathComesFromTheExternalURL(t *testing.T) {
	t.Parallel()

	assert.NotContains(t, renderedEnv(fixtureConsoleMinimal(t), ComponentConsole), "CAMUNDA_CONSOLE_CONTEXT_PATH")
	assert.Equal(
		t,
		"/console",
		renderedEnv(fixtureConsoleRealistic(t), ComponentConsole)["CAMUNDA_CONSOLE_CONTEXT_PATH"],
	)
}

// The license reaches the container only when the platform config names one.
func TestConsoleRendersTheLicenseOfThePlatformConfig(t *testing.T) {
	t.Parallel()

	assert.NotContains(t, renderedEnv(fixtureConsoleMinimal(t), ComponentConsole), "CAMUNDA_LICENSE_KEY")
	assert.Equal(
		t,
		"secretKeyRef:my-management-management-license/license",
		renderedEnv(fixtureConsoleRealistic(t), ComponentConsole)["CAMUNDA_LICENSE_KEY"],
	)
}

// The Deployment carries the workload overrides of spec.console, the image of
// the platform config, and a hash of the rendered configuration of Console
// alone.
func TestConsoleDeploymentCarriesTheOverridesAndTheConfigHash(t *testing.T) {
	t.Parallel()

	in := fixtureConsoleRealistic(t)
	comps, err := consoleComponents(in)
	require.NoError(t, err)
	require.Len(t, comps, 1)

	objects, err := comps[0].Preview()
	require.NoError(t, err)
	workload := previewedDeployment(t, objects)

	assert.Equal(t, int32(2), *workload.Spec.Replicas)
	assert.Equal(
		t,
		ConfigHash(in, ComponentConsole),
		workload.Spec.Template.Annotations[ConfigHashAnnotation],
	)
	assert.NotEqual(t, ConfigHash(in, ComponentIdentity), ConfigHash(in, ComponentConsole))

	container := workload.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "registry.example.com/mirror/camunda/console:8.9.4", container.Image)
	assert.Contains(t, container.Env, corev1.EnvVar{Name: "CAMUNDA_CONSOLE_TELEMETRY", Value: "online"})
	assert.Equal(t, consoleHealthPath, container.ReadinessProbe.HTTPGet.Path)
	assert.Nil(t, container.LivenessProbe)
}
