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

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// A management cluster that does not deploy Console renders no object for it.
// The component is built either way, gated off, so that a management cluster
// that drops Console has its workload deleted rather than left running.
func TestConsoleRendersNothingWhileItIsDisabled(t *testing.T) {
	t.Parallel()

	built, err := consoleComponents(fixtureMinimal(t))
	require.NoError(t, err)
	require.Len(t, built.Components, 1)
	assert.Empty(t, built.Ready)

	objects, err := built.Components[0].Preview()

	require.NoError(t, err)
	assert.Empty(t, objects)
}

// Console starts only after Management Identity is ready, in every identity
// provider mode.
func TestConsoleWaitsForManagementIdentity(t *testing.T) {
	t.Parallel()

	modes := map[string]func(t *testing.T) Input{
		"oidc":             fixtureConsoleMinimal,
		"keycloak":         func(t *testing.T) Input { return fixtureKeycloakRealistic(t, true) },
		"externalKeycloak": func(t *testing.T) Input { return fixtureKeycloakRealistic(t, false) },
	}

	for mode, fixture := range modes {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()

			t.Run("waits while Management Identity is not ready", func(t *testing.T) {
				t.Parallel()

				cond, deployed := reconcileConsole(t, fixture(t), metav1.ConditionFalse)

				assert.Equal(t, metav1.ConditionFalse, cond.Status)
				assert.Equal(t, string(component.PrerequisiteNotMet), cond.Reason)
				assert.False(t, deployed, "the Console Deployment exists before Management Identity is ready")
			})

			t.Run("starts once Management Identity is ready", func(t *testing.T) {
				t.Parallel()

				cond, deployed := reconcileConsole(t, fixture(t), metav1.ConditionTrue)

				assert.Equal(t, string(component.AliveCreating), cond.Reason)
				assert.True(t, deployed, "the Console Deployment is missing after Management Identity is ready")
			})
		})
	}
}

// A management cluster without Console reports Disabled, not a wait for
// Management Identity, so Ready does not wait for a Console it does not run.
func TestDisabledConsoleDoesNotWaitForManagementIdentity(t *testing.T) {
	t.Parallel()

	cond, deployed := reconcileConsole(t, fixtureMinimal(t), metav1.ConditionFalse)

	assert.Equal(t, metav1.ConditionTrue, cond.Status)
	assert.Equal(t, string(component.Disabled), cond.Reason)
	assert.False(t, deployed)
}

// reconcileConsole reconciles the Console component once against a fake API
// server, on an owner whose IdentityReady condition has the given status. It
// returns the ConsoleReady condition and whether the Console Deployment exists.
func reconcileConsole(t *testing.T, in Input, identity metav1.ConditionStatus) (component.Condition, bool) {
	t.Helper()

	built, err := consoleComponents(in)
	require.NoError(t, err)
	comp := builtComponent(t, built, ComponentConsole)

	owner := in.Cluster.DeepCopy()
	owner.Status.Conditions = []metav1.Condition{{
		Type:               v1.ConditionIdentityReady,
		Status:             identity,
		Reason:             "Stamped",
		LastTransitionTime: metav1.Now(),
	}}

	scheme := goldenScheme(t)
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(testrestmapper.TestOnlyStaticRESTMapper(scheme)).
		Build()
	require.NoError(t, comp.Reconcile(context.Background(), component.ReconcileContext{
		Client:        c,
		APIReader:     c,
		Scheme:        scheme,
		EventRecorder: events.NewFakeRecorder(100),
		Owner:         owner,
	}))

	key := client.ObjectKey{Namespace: owner.Namespace, Name: ConsoleName(owner)}
	err = c.Get(context.Background(), key, &appsv1.Deployment{})
	if !apierrors.IsNotFound(err) {
		require.NoError(t, err)
	}

	return comp.GetCondition(owner), err == nil
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
	built, err := consoleComponents(in)
	require.NoError(t, err)
	require.Len(t, built.Components, 1)
	require.Len(t, built.Ready, 1)

	objects, err := built.Components[0].Preview()
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
