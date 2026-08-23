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
	"github.com/konsole-is/camunda-operator/pkg/credentials"
)

// keycloakServiceURL is the in-cluster address of the Keycloak that the
// Keycloak Operator runs for the fixture.
const keycloakServiceURL = "http://my-management-keycloak-service.camunda.svc:8080/auth"

// A Keycloak that the operator runs answers on two addresses: browsers reach
// it at the external URL, which signs the tokens, and containers reach it at
// the Service that the Keycloak Operator creates. The login redirect goes to
// the external URL, and the token exchange and the signing keys, which are
// server-to-server, go to the Service.
func TestResolveIdentityProviderSplitsTheURLsOfAManagedKeycloak(t *testing.T) {
	t.Parallel()

	provider := newKeycloakInput(t, true, nil).Provider

	assert.Equal(t, ModeKeycloak, provider.Mode)
	assert.Equal(t, identityTypeKeycloak, provider.Type)
	assert.Equal(t, identityProfileKeycloak, provider.SpringProfile)
	assert.Equal(
		t, keycloakServiceURL, provider.KeycloakURL,
	)
	assert.Equal(t, fixtureKeycloak, provider.KeycloakPublicURL)
	assert.Equal(t, keycloakDefaultRealm, provider.Realm)
	assert.Equal(t, fixtureKeycloak+"/realms/camunda-platform", provider.IssuerURL)
	assert.Equal(
		t,
		keycloakServiceURL+"/realms/camunda-platform",
		provider.IssuerBackendURL,
	)
	assert.Equal(
		t, fixtureKeycloak+"/realms/camunda-platform/protocol/openid-connect/auth", provider.AuthURL,
	)
	assert.Equal(
		t,
		keycloakServiceURL+"/realms/camunda-platform/protocol/openid-connect/token",
		provider.TokenURL,
	)
	assert.Equal(
		t,
		keycloakServiceURL+"/realms/camunda-platform/protocol/openid-connect/certs",
		provider.JwksURL,
	)
	assert.Equal(
		t,
		&v1.CredentialsSecretRef{
			Name:        "my-management-keycloak-initial-admin",
			Namespace:   fixtureNamespace,
			UsernameKey: "username",
			PasswordKey: "password",
		},
		provider.AdminCredentials,
	)
}

// A Keycloak that the user runs answers on one address, so the front-channel
// and the back-channel issuer are the same. The administrator comes from the
// copy of the referenced Secret in the management namespace, because the
// Identity pods mount it.
func TestResolveIdentityProviderReadsAnExistingKeycloak(t *testing.T) {
	t.Parallel()

	provider := newKeycloakInput(t, false, nil).Provider

	assert.Equal(t, ModeExternalKeycloak, provider.Mode)
	assert.Equal(t, fixtureKeycloak, provider.KeycloakURL)
	assert.Equal(t, provider.IssuerURL, provider.IssuerBackendURL)
	assert.Equal(t, fixtureKeycloak+"/realms/camunda-platform", provider.IssuerURL)
	assert.Equal(
		t,
		&v1.CredentialsSecretRef{
			Name:        MirroredSecretName(newKeycloakCluster(false, nil), MirrorPurposeKeycloakAdmin),
			Namespace:   fixtureNamespace,
			UsernameKey: "username",
			PasswordKey: "password",
		},
		provider.AdminCredentials,
	)
}

// A realm of your own reaches the issuer URLs of every consumer.
func TestResolveIdentityProviderTakesTheRealmOfAnExistingKeycloak(t *testing.T) {
	t.Parallel()

	in := newKeycloakInput(t, false, func(in *Input) {
		in.Cluster.Spec.IdentityProvider.ExternalKeycloak.Realm = "camunda-eu"
	})

	assert.Equal(t, "camunda-eu", in.Provider.Realm)
	assert.Equal(t, fixtureKeycloak+"/realms/camunda-eu", in.Provider.IssuerURL)
}

// A URL that ends in a slash is as valid as one that does not, and the CRD
// takes it. Every issuer URL appends a path to it, so it has to lose the
// slash before it reaches one.
func TestResolveIdentityProviderTrimsATrailingSlash(t *testing.T) {
	t.Parallel()

	in := newKeycloakInput(t, false, func(in *Input) {
		in.Cluster.Spec.IdentityProvider.ExternalKeycloak.URL = fixtureKeycloak + "/"
	})

	assert.Equal(t, fixtureKeycloak, in.Provider.KeycloakURL)
	assert.Equal(t, fixtureKeycloak, in.Provider.KeycloakPublicURL)
	assert.Equal(t, fixtureKeycloak+"/realms/camunda-platform", in.Provider.IssuerURL)
	assert.Equal(t, fixtureKeycloak+"/realms/camunda-platform", in.Provider.IssuerBackendURL)
	assert.Equal(
		t,
		fixtureKeycloak+"/realms/camunda-platform/protocol/openid-connect/certs",
		in.Provider.JwksURL,
	)
}

// Management Identity creates one client per component it serves, so a client
// of a component that the spec does not deploy would never exist.
func TestResolveIdentityProviderDeclaresAClientPerDeployedComponent(t *testing.T) {
	t.Parallel()

	minimal := newKeycloakInput(t, true, nil).Provider.Clients
	assert.Equal(t, keycloakClientIdentity, minimal.Identity.ID)
	assert.Equal(t, keycloakAudienceIdentity, minimal.Identity.Audience)
	assert.Equal(t, "my-management-identity-client", minimal.Identity.SecretRef.Name)
	assert.Equal(t, keycloakClientOptimize, minimal.Optimize.ID)
	assert.Equal(t, keycloakAudienceOptimize, minimal.Optimize.Audience)
	assert.Equal(t, "my-management-optimize-client", minimal.Optimize.SecretRef.Name)
	assert.Empty(t, minimal.Console.ID)
	assert.Empty(t, minimal.WebModeler.ID)

	full := newKeycloakInput(t, true, func(in *Input) {
		in.Cluster.Spec.Console = &v1.ConsoleSpec{Version: fixtureVersion, ExternalURL: fixtureExternal}
		in.Cluster.Spec.WebModeler = webModeler("web-modeler-db")
	}).Provider.Clients

	assert.Equal(t, keycloakClientConsole, full.Console.ID)
	assert.Equal(t, keycloakAudienceConsole, full.Console.Audience)
	assert.Equal(t, keycloakClientWebModeler, full.WebModeler.ID)
	assert.Equal(t, keycloakAudienceWebModeler, full.WebModeler.Audience)
	// Web Modeler validates two audiences and refuses to start with either
	// one empty: the internal API that its own user interface calls, and the
	// public API that a script calls.
	assert.Equal(t, keycloakAudienceWebModeler, full.WebModelerAPI.Audience)
	assert.Equal(t, keycloakAudienceWebModelerPublic, full.WebModelerPublicAPIAudience)
}

// The Keycloak modes bind the keycloak profile of Management Identity, which
// reads a different set of names than the oidc profile, and they carry
// everything Identity creates in the realm on its first start.
func TestIdentityEnvInTheKeycloakModes(t *testing.T) {
	t.Parallel()

	env := renderedEnv(newKeycloakInput(t, true, func(in *Input) {
		in.Cluster.Spec.Console = &v1.ConsoleSpec{
			Version: fixtureVersion, ExternalURL: "https://console.example.com",
		}
		in.Cluster.Spec.WebModeler = webModeler("web-modeler-db")
		in.Cluster.Spec.WebModeler.ExternalURL = "https://modeler.example.com"
	}), ComponentIdentity)

	assert.Equal(
		t, map[string]string{
			"SPRING_PROFILES_ACTIVE":             "keycloak",
			"CAMUNDA_IDENTITY_TYPE":              "KEYCLOAK",
			"CAMUNDA_IDENTITY_AUDIENCE":          "camunda-identity-resource-server",
			"KEYCLOAK_URL":                       keycloakServiceURL,
			"KEYCLOAK_REALM":                     "camunda-platform",
			"IDENTITY_AUTH_PROVIDER_ISSUER_URL":  fixtureKeycloak + "/realms/camunda-platform",
			"IDENTITY_AUTH_PROVIDER_BACKEND_URL": keycloakServiceURL + "/realms/camunda-platform",
			"IDENTITY_CLIENT_ID":                 "camunda-identity",
			"IDENTITY_CLIENT_SECRET":             "secretKeyRef:my-management-identity-client/client-secret",
			"KEYCLOAK_SETUP_USER":                "secretKeyRef:my-management-keycloak-initial-admin/username",
			"KEYCLOAK_SETUP_PASSWORD":            "secretKeyRef:my-management-keycloak-initial-admin/password",
			"KEYCLOAK_SETUP_REALM":               "master",
			"KEYCLOAK_SETUP_CLIENT_ID":           "admin-cli",
			"KEYCLOAK_INIT_OPTIMIZE_ROOT_URL":    fixtureOptimize,
			"KEYCLOAK_INIT_OPTIMIZE_SECRET":      "secretKeyRef:my-management-optimize-client/client-secret",
			"KEYCLOAK_INIT_CONSOLE_ROOT_URL":     "https://console.example.com",
			"KEYCLOAK_INIT_WEBMODELER_ROOT_URL":  "https://modeler.example.com",
			"KEYCLOAK_USERS_0_USERNAME":          fixtureAdmin,
			"KEYCLOAK_USERS_0_PASSWORD":          "secretKeyRef:my-management-identity-admin/password",
			"KEYCLOAK_USERS_0_ROLES_0":           "ManagementIdentity",
			"KEYCLOAK_USERS_0_ROLES_1":           "Optimize",
			"KEYCLOAK_USERS_0_ROLES_2":           "Console",
			"KEYCLOAK_USERS_0_ROLES_3":           "Web Modeler",
			"KEYCLOAK_USERS_0_ROLES_4":           "Web Modeler Admin",
			"IDENTITY_URL":                       fixtureExternal,
			"IDENTITY_DATABASE_HOST":             "postgres.camunda.svc",
			"IDENTITY_DATABASE_PORT":             "5432",
			"IDENTITY_DATABASE_NAME":             "identity",
			"IDENTITY_DATABASE_USERNAME":         "secretKeyRef:identity-db-credentials/username",
			"IDENTITY_DATABASE_PASSWORD":         "secretKeyRef:identity-db-credentials/password",
		}, env,
	)
}

// The initial claim belongs to the oidc mode. In a Keycloak mode the first
// administrator is a Keycloak user, so a rendered claim would name a setting
// that Management Identity never reads.
func TestIdentityEnvInTheKeycloakModesCarriesNoInitialClaim(t *testing.T) {
	t.Parallel()

	env := renderedEnv(newKeycloakInput(t, true, nil), ComponentIdentity)

	assert.NotContains(t, env, "IDENTITY_INITIAL_CLAIM_NAME")
	assert.NotContains(t, env, "IDENTITY_INITIAL_CLAIM_VALUE")
}

// A password of your own replaces the generated one: Management Identity
// reads it from the Secret that the spec names.
func TestIdentityEnvReadsTheAdministratorPasswordOfTheSpec(t *testing.T) {
	t.Parallel()

	env := renderedEnv(fixtureKeycloakRealistic(t, true), ComponentIdentity)

	assert.Equal(t, "secretKeyRef:admin-password/password", env["KEYCLOAK_USERS_0_PASSWORD"])
	assert.Equal(t, "admin@example.com", env["KEYCLOAK_USERS_0_EMAIL"])
}

// Deleting a client secret rotates it: the apply precondition of the reused
// credential makes the delete win the race, and Management Identity applies
// the new secret to the client on every start. The administrator password is
// different. Management Identity creates the Keycloak user with it once, and
// never reads it again, so its Secret carries no precondition and a delete
// that races the apply republishes the password the user holds.
func TestGeneratedSecretsRotateExceptTheAdministratorPassword(t *testing.T) {
	t.Parallel()

	in := newKeycloakInput(t, true, func(in *Input) {
		for name, password := range in.Secrets.Values {
			in.Secrets.Values[name] = credentials.Password{
				Value: password.Value, SourceUID: "a-secret",
			}
		}
	})

	built, err := secretsComponents(in)
	require.NoError(t, err)
	require.Len(t, built.Components, 1)
	objects, err := built.Components[0].Preview()
	require.NoError(t, err)

	preconditions := map[string]string{}
	for _, object := range objects {
		preconditions[object.GetName()] = object.GetAnnotations()[credentials.PreconditionAnnotation]
	}
	assert.Equal(
		t,
		map[string]string{
			IdentityClientSecretName(in.Cluster): "a-secret",
			OptimizeClientSecretName(in.Cluster): "a-secret",
			IdentityAdminSecretName(in.Cluster):  "",
		},
		preconditions,
	)
}

// A pod mounts a Secret of its own namespace alone, so an administrator
// password from another namespace is read from the copy the operator makes.
func TestIdentityEnvReadsTheAdministratorPasswordFromItsCopy(t *testing.T) {
	t.Parallel()

	in := newKeycloakInput(t, true, func(in *Input) {
		in.Cluster.Spec.Identity.Admin.PasswordSecretRef = &v1.SecretKeyRef{
			Name: "admin-password", Namespace: "elsewhere", Key: "password",
		}
		in.Secrets.IdentityAdmin = ""
	})

	env := renderedEnv(in, ComponentIdentity)

	assert.Equal(
		t,
		"secretKeyRef:"+MirroredSecretName(in.Cluster, MirrorPurposeIdentityAdmin)+"/password",
		env["KEYCLOAK_USERS_0_PASSWORD"],
	)
}

// The ManagementAuthConfig is what a CamundaOptimize reads, and in the
// Keycloak modes every field comes from the realm that Management Identity
// bootstraps.
func TestManagementAuthSpecInTheKeycloakModes(t *testing.T) {
	t.Parallel()

	spec := ManagementAuthSpec(newKeycloakInput(t, true, nil))

	assert.Equal(t, fixtureExternal, spec.BaseURL)
	assert.Equal(t, fixtureKeycloak+"/realms/camunda-platform", spec.IssuerURL)
	assert.Equal(
		t,
		keycloakServiceURL+"/realms/camunda-platform",
		spec.IssuerBackendURL,
	)
	assert.Equal(
		t,
		fixtureKeycloak+"/realms/camunda-platform/protocol/openid-connect/auth",
		spec.AuthURL,
	)
	assert.Equal(
		t,
		keycloakServiceURL+"/realms/camunda-platform/protocol/openid-connect/token",
		spec.TokenURL,
	)
	assert.Equal(
		t,
		keycloakServiceURL+"/realms/camunda-platform/protocol/openid-connect/certs",
		spec.JwksURL,
	)
	assert.Equal(t, keycloakClientOptimize, spec.ClientID)
	assert.Equal(t, keycloakAudienceOptimize, spec.Audience)
	assert.Equal(
		t,
		v1.SecretKeyRef{
			Name:      "my-management-optimize-client",
			Namespace: fixtureNamespace,
			Key:       ClientSecretKey,
		},
		spec.ClientSecretRef,
	)
}

// The oidc mode generates no credential of its own. The component is still
// built and gated off, so a move from a Keycloak mode to oidc deletes the
// client secrets and the administrator password that the operator published,
// and the gated-off component stays out of Ready.
func TestBuildRendersTheGeneratedSecretsOfTheKeycloakModesOnly(t *testing.T) {
	t.Parallel()

	oidc, err := Build(fixtureMinimal(t))
	require.NoError(t, err)
	assert.Contains(t, componentNames(oidc.Components), ComponentSecrets)
	assert.NotContains(t, componentNames(oidc.Ready), ComponentSecrets)

	objects, err := builtComponent(t, oidc, ComponentSecrets).Preview()
	require.NoError(t, err)
	assert.Empty(t, objects)

	keycloak, err := Build(newKeycloakInput(t, true, nil))
	require.NoError(t, err)
	assert.Equal(
		t,
		[]string{
			ComponentMirroredSecrets,
			ComponentSecrets,
			ComponentKeycloak,
			ComponentIdentity,
			ComponentConsole,
			ComponentWebModeler,
		},
		componentNames(keycloak.Components),
	)
	assert.Equal(
		t,
		[]string{ComponentSecrets, ComponentKeycloak, ComponentIdentity},
		componentNames(keycloak.Ready),
	)
}

// An administrator password of your own replaces the generated one, so the
// Secret that the operator published for an earlier spec is deleted rather
// than kept next to the one you named.
func TestBuildGatesTheGeneratedAdministratorPasswordOffForOneOfYourOwn(t *testing.T) {
	t.Parallel()

	in := fixtureKeycloakRealistic(t, true)
	built, err := Build(in)
	require.NoError(t, err)

	objects, err := builtComponent(t, built, ComponentSecrets).Preview()
	require.NoError(t, err)

	names := make([]string, 0, len(objects))
	for _, object := range objects {
		names = append(names, object.GetName())
	}
	assert.Equal(
		t,
		[]string{IdentityClientSecretName(in.Cluster), OptimizeClientSecretName(in.Cluster)},
		names,
	)
}

// A Keycloak that the user runs is not this operator's to reconcile. The
// component is still built and gated off, so a move from the keycloak mode to
// this one deletes the custom resource that the operator wrote, and the
// gated-off component stays out of Ready.
func TestBuildGatesTheKeycloakOffForAnExistingOne(t *testing.T) {
	t.Parallel()

	built, err := Build(newKeycloakInput(t, false, nil))
	require.NoError(t, err)

	assert.Contains(t, componentNames(built.Components), ComponentKeycloak)
	assert.NotContains(t, componentNames(built.Ready), ComponentKeycloak)

	comp := builtComponent(t, built, ComponentKeycloak)
	objects, err := comp.Preview()
	require.NoError(t, err)
	assert.Empty(t, objects)
}

// A Kubernetes cluster that serves no Keycloak kind has no Keycloak custom
// resource to delete, and a delete against an API that serves no such kind
// would fail every reconcile.
func TestBuildRendersNoKeycloakWithoutTheKeycloakOperator(t *testing.T) {
	t.Parallel()

	built, err := Build(newKeycloakInput(t, false, func(in *Input) {
		in.KeycloakCRDServed = false
	}))

	require.NoError(t, err)
	assert.NotContains(t, componentNames(built.Components), ComponentKeycloak)
}

// A generated client secret reaches Management Identity through a Secret
// reference, and the reference does not change when the secret behind it
// rotates. The rotation deletes the Secret, so the new one carries a UID of
// its own, and that UID in the config hash is what rolls the pods.
func TestConfigHashFollowsAGeneratedCredential(t *testing.T) {
	t.Parallel()

	in := newKeycloakInput(t, true, nil)
	before := ConfigHash(in, ComponentIdentity)

	in.Secrets.Values[IdentityClientSecretName(in.Cluster)] = credentials.Password{
		Value: "rotated", SourceUID: "a-new-secret",
	}

	assert.NotEqual(t, before, ConfigHash(in, ComponentIdentity))
}

// Management Identity is the one component that signs in with the
// administrator that the Keycloak Operator writes, so a rewrite of that
// Secret rolls Identity and leaves every other component where it is.
func TestConfigHashFollowsAComponentInput(t *testing.T) {
	t.Parallel()

	in := newKeycloakInput(t, true, nil)
	identity := ConfigHash(in, ComponentIdentity)
	keycloak := ConfigHash(in, ComponentKeycloak)

	in.ComponentHashInputs = map[string][]string{
		ComponentIdentity: {"Secret/camunda/my-management-keycloak-initial-admin=7"},
	}

	assert.NotEqual(t, identity, ConfigHash(in, ComponentIdentity))
	assert.Equal(t, keycloak, ConfigHash(in, ComponentKeycloak))
}

// The Keycloak Operator owns the pods of a Keycloak, so the spec offers the
// instance count and the resources instead of a whole WorkloadSpec.
func TestKeycloakTakesItsReplicasFromTheSpec(t *testing.T) {
	t.Parallel()

	assert.Equal(t, int32(1), newKeycloakInput(t, true, nil).replicas(ComponentKeycloak))
	assert.Equal(t, int32(2), fixtureKeycloakRealistic(t, true).replicas(ComponentKeycloak))
}
