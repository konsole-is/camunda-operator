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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The environment of Management Identity in the oidc mode is the table of the
// Camunda documentation: the oidc profile, the provider type, the two issuer
// URLs, the client, the audience, the initial administrator claim, the URL of
// Identity itself, and the database.
func TestIdentityEnvInOIDCMode(t *testing.T) {
	t.Parallel()

	env := renderedEnv(fixtureMinimal(t), ComponentIdentity)

	assert.Equal(
		t, map[string]string{
			"SPRING_PROFILES_ACTIVE":              "oidc",
			"CAMUNDA_IDENTITY_TYPE":               "GENERIC",
			"CAMUNDA_IDENTITY_BASE_URL":           fixtureExternal,
			"CAMUNDA_IDENTITY_ISSUER":             fixtureIssuer,
			"CAMUNDA_IDENTITY_ISSUER_BACKEND_URL": fixtureIssuer,
			"CAMUNDA_IDENTITY_CLIENT_ID":          "management-identity",
			"CAMUNDA_IDENTITY_AUDIENCE":           "management-identity-api",
			"CAMUNDA_IDENTITY_CLIENT_SECRET":      "secretKeyRef:oidc-credentials/identity-client-secret",
			"IDENTITY_INITIAL_CLAIM_NAME":         "oid",
			"IDENTITY_INITIAL_CLAIM_VALUE":        "admin-oid",
			"IDENTITY_URL":                        fixtureExternal,
			"IDENTITY_DATABASE_HOST":              "postgres.camunda.svc",
			"IDENTITY_DATABASE_PORT":              "5432",
			"IDENTITY_DATABASE_NAME":              "identity",
			"IDENTITY_DATABASE_USERNAME":          "secretKeyRef:identity-db-credentials/username",
			"IDENTITY_DATABASE_PASSWORD":          "secretKeyRef:identity-db-credentials/password",
		}, env,
	)
}

// The audience is always set. Identity refuses to start without it, so a
// platform config that names none falls back to the client id.
func TestIdentityAudienceFallsBackToTheClientID(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Platform.Auth.OIDC.Management.Clients.Identity.Audience = ""
	})

	assert.Equal(t, "management-identity", renderedEnv(in, ComponentIdentity)["CAMUNDA_IDENTITY_AUDIENCE"])
}

// A Microsoft Entra ID platform config renders the MICROSOFT provider type,
// and a username claim reaches the container only when the platform config
// names one.
func TestIdentityReadsTheProviderTypeAndTheUsernameClaim(t *testing.T) {
	t.Parallel()

	minimal := renderedEnv(fixtureMinimal(t), ComponentIdentity)
	assert.NotContains(t, minimal, "CAMUNDA_IDENTITY_USERNAMECLAIM")

	realistic := renderedEnv(fixtureRealistic(t), ComponentIdentity)
	assert.Equal(t, "MICROSOFT", realistic["CAMUNDA_IDENTITY_TYPE"])
	assert.Equal(t, "unique_name", realistic["CAMUNDA_IDENTITY_USERNAMECLAIM"])
}

// The license reaches the container only when the platform config names one.
func TestIdentityRendersTheLicenseOfThePlatformConfig(t *testing.T) {
	t.Parallel()

	assert.NotContains(t, renderedEnv(fixtureMinimal(t), ComponentIdentity), "CAMUNDA_LICENSE_KEY")
	assert.Equal(
		t,
		"secretKeyRef:my-management-management-license/license",
		renderedEnv(fixtureRealistic(t), ComponentIdentity)["CAMUNDA_LICENSE_KEY"],
	)
}

// Management Identity reads the initial administrator claim on its first
// start only, so the recorded claim keeps being rendered after the spec
// changes.
func TestIdentityKeepsTheRecordedInitialClaim(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Cluster.Annotations = map[string]string{InitialClaimAnnotation: "oid=first-admin"}
		in.Cluster.Spec.Identity.Admin.ClaimValue = "second-admin"
	})

	env := renderedEnv(in, ComponentIdentity)
	assert.Equal(t, "oid", env["IDENTITY_INITIAL_CLAIM_NAME"])
	assert.Equal(t, "first-admin", env["IDENTITY_INITIAL_CLAIM_VALUE"])
	assert.Equal(t, "oid=second-admin", SpecInitialClaim(in.Cluster))
	assert.Equal(t, "oid=first-admin", RecordedInitialClaim(in.Cluster))
}

// A rollout runs two Identity pods at once, and only the older one holds the
// claim that Identity wrote into its database.
func TestStartedInitialClaimReadsThePodThatStartedFirst(t *testing.T) {
	t.Parallel()

	first := metav1.NewTime(time.Now().Add(-time.Minute))
	pods := []corev1.Pod{
		startedIdentityPod("second-admin", corev1.ContainerState{
			Running: &corev1.ContainerStateRunning{StartedAt: metav1.Now()},
		}),
		startedIdentityPod("first-admin", corev1.ContainerState{
			Running: &corev1.ContainerStateRunning{StartedAt: first},
		}),
	}

	assert.Equal(t, "oid=first-admin", StartedInitialClaim(pods))
}

// A container that crash-loops already reached the database, so the run it
// had counts as a start.
func TestStartedInitialClaimCountsAContainerThatRanAndStopped(t *testing.T) {
	t.Parallel()

	crashLooping := startedIdentityPod("first-admin", corev1.ContainerState{
		Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
	})
	crashLooping.Status.ContainerStatuses[0].LastTerminationState = corev1.ContainerState{
		Terminated: &corev1.ContainerStateTerminated{StartedAt: metav1.Now()},
	}

	assert.Equal(t, "oid=first-admin", StartedInitialClaim([]corev1.Pod{crashLooping}))
}

// A container that restarted runs again with a later start time. Its first
// run is the one that reached the database, so a restarted older pod still
// wins over a newer pod.
func TestStartedInitialClaimUsesTheFirstRunOfARestartedContainer(t *testing.T) {
	t.Parallel()

	firstRun := metav1.NewTime(time.Now().Add(-2 * time.Minute))
	newer := metav1.NewTime(time.Now().Add(-time.Minute))
	restarted := startedIdentityPod("first-admin", corev1.ContainerState{
		Running: &corev1.ContainerStateRunning{StartedAt: metav1.Now()},
	})
	restarted.Status.ContainerStatuses[0].LastTerminationState = corev1.ContainerState{
		Terminated: &corev1.ContainerStateTerminated{StartedAt: firstRun},
	}
	pods := []corev1.Pod{
		startedIdentityPod("second-admin", corev1.ContainerState{
			Running: &corev1.ContainerStateRunning{StartedAt: newer},
		}),
		restarted,
	}

	assert.Equal(t, "oid=first-admin", StartedInitialClaim(pods))
}

// A pod whose container never ran carries no claim. Its container is waiting
// on an image or on a Secret, so Management Identity read nothing.
func TestStartedInitialClaimIsEmptyWhileNoContainerRan(t *testing.T) {
	t.Parallel()

	pending := startedIdentityPod("first-admin", corev1.ContainerState{
		Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"},
	})

	assert.Empty(t, StartedInitialClaim([]corev1.Pod{pending}))
	assert.Empty(t, StartedInitialClaim(nil))
}

// startedIdentityPod returns a pod of the Management Identity Deployment
// whose container carries the given administrator claim value and is in the
// given state.
func startedIdentityPod(claimValue string, state corev1.ContainerState) corev1.Pod {
	return corev1.Pod{
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name: identityContainer,
			Env: []corev1.EnvVar{
				{Name: identityEnvInitialClaimName, Value: "oid"},
				{Name: identityEnvInitialClaimValue, Value: claimValue},
			},
		}}},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
			Name:  identityContainer,
			State: state,
		}}},
	}
}

// The Deployment carries the workload overrides of spec.identity, and its pod
// template carries a hash of the rendered configuration.
func TestIdentityDeploymentCarriesTheOverridesAndTheConfigHash(t *testing.T) {
	t.Parallel()

	in := fixtureRealistic(t)
	built, err := identityComponents(in)
	require.NoError(t, err)
	require.Len(t, built.Components, 1)
	require.Len(t, built.Ready, 1)

	objects, err := built.Components[0].Preview()
	require.NoError(t, err)
	workload := previewedDeployment(t, objects)

	assert.Equal(t, int32(2), *workload.Spec.Replicas)
	assert.Equal(t, "platform", workload.Spec.Template.Labels["team"])
	assert.Equal(
		t,
		ConfigHash(in, ComponentIdentity),
		workload.Spec.Template.Annotations[ConfigHashAnnotation],
	)

	container := workload.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "registry.example.com/mirror/camunda/identity:8.9.4", container.Image)
	assert.Contains(t, container.Env, corev1.EnvVar{Name: "IDENTITY_LOG_LEVEL", Value: "DEBUG"})
	assert.Equal(t, identityHealthPath, container.ReadinessProbe.HTTPGet.Path)
	assert.Nil(t, container.LivenessProbe)
}

// A rotated Secret behind an unchanged reference rolls the pods, because the
// hash covers the resource versions the controller read.
func TestConfigHashFollowsTheHashInputs(t *testing.T) {
	t.Parallel()

	in := fixtureMinimal(t)
	before := ConfigHash(in, ComponentIdentity)

	in.HashInputs = []string{"Secret/platform/oidc-credentials=43"}

	assert.NotEqual(t, before, ConfigHash(in, ComponentIdentity))
}
