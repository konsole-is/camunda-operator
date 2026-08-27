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
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/credentials"
)

// The Web Modeler fixture identities. Every value is fixed, so the golden
// manifests stay deterministic.
const (
	fixtureWebModelerVersion     = "8.9.12"
	fixtureWebModelerURL         = "https://modeler.example.com"
	fixtureWebSocketsURL         = "https://modeler.example.com/modeler-ws"
	fixtureWebModelerClientID    = "web-modeler"
	fixtureWebModelerAPIAudience = "web-modeler-api"
	fixturePusherKey             = "golden-pusher-key"
	fixturePusherSecret          = "golden-pusher-secret"
	fixturePusherSourceUID       = types.UID("cccccccc-dddd-eeee-ffff-000000000000")
	fixtureClusterOIDCUID        = types.UID("11111111-2222-3333-4444-555555555555")
	fixtureClusterBasicUID       = types.UID("66666666-7777-8888-9999-000000000000")
	fixtureClusterNotReadyUID    = types.UID("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
)

// fixtureWebModelerMinimal is a management cluster that deploys Web Modeler
// with the required fields and no attached cluster.
func fixtureWebModelerMinimal(t *testing.T) Input {
	t.Helper()

	return newInput(t, func(in *Input) {
		withWebModeler(in.Cluster)
		in.Platform = newPlatform(withWebModelerClients)
		in.Databases.WebModeler = webModelerDatabase()
		in.Pusher = PusherCredentials{
			Key:    credentials.Password{Value: fixturePusherKey},
			Secret: credentials.Password{Value: fixturePusherSecret},
		}
	})
}

// fixtureWebModelerRealistic exercises every override surface of Web Modeler:
// SMTP credentials, a sender name, a public API audience of its own, a
// username claim, the override blocks of both workloads, and three attached
// clusters, one of each kind the cluster list has to tell apart.
func fixtureWebModelerRealistic(t *testing.T) Input {
	t.Helper()

	in := fixtureWebModelerMinimal(t)
	withWebModelerOverrides(in.Cluster.Spec.WebModeler)
	in.WebModelerMail = in.Cluster.Spec.WebModeler.Mail.CredentialsSecretRef.DeepCopy()

	in.Platform.Auth.OIDC.Management.Clients.WebModelerAPI.PublicAPIAudience = "modeler-public"
	in.Platform.Auth.OIDC.UsernameClaim = "preferred_username"
	in.Pusher = PusherCredentials{
		Key:    credentials.Password{Value: fixturePusherKey, SourceUID: fixturePusherSourceUID},
		Secret: credentials.Password{Value: fixturePusherSecret, SourceUID: fixturePusherSourceUID},
	}
	in.Clusters = fixtureAttachedClusters()

	provider, err := ResolveIdentityProvider(in)
	require.NoError(t, err)
	in.Provider = provider

	return in
}

// withWebModeler enables Web Modeler on a management cluster, with the
// required fields and nothing else.
func withWebModeler(mc *v1.CamundaManagementCluster) {
	mc.Spec.WebModeler = &v1.WebModelerSpec{
		Version:               fixtureWebModelerVersion,
		ExternalURL:           fixtureWebModelerURL,
		WebsocketsExternalURL: fixtureWebSocketsURL,
		DatabaseConfigRef:     "modeler-db",
		Mail: v1.WebModelerMailSpec{
			SMTPHost:    "smtp.example.com",
			SMTPPort:    587,
			FromAddress: "noreply@example.com",
		},
	}
}

// withWebModelerOverrides fills every optional field of the Web Modeler block.
func withWebModelerOverrides(spec *v1.WebModelerSpec) {
	spec.Mail.FromName = "Camunda"
	spec.Mail.TLS = new(false)
	spec.Mail.CredentialsSecretRef = &v1.LocalCredentialsSecretRef{
		Name:        "smtp-credentials",
		UsernameKey: "username",
		PasswordKey: "password",
	}
	spec.Restapi = &v1.WorkloadSpec{
		Replicas: new(int32(2)),
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
		},
		ExtraEnv: []corev1.EnvVar{{Name: "LOGGING_LEVEL_IO_CAMUNDA_MODELER", Value: "DEBUG"}},
	}
	spec.Websockets = &v1.WorkloadSpec{PodLabels: map[string]string{"team": "platform"}}
}

// withWebModelerClients declares the two Web Modeler clients that the oidc
// mode needs on the platform config.
func withWebModelerClients(p *v1.CamundaPlatformConfigSpec) {
	p.Auth.OIDC.Management.Clients.WebModeler = &v1.PublicClientSpec{
		ClientID: fixtureWebModelerClientID,
	}
	p.Auth.OIDC.Management.Clients.WebModelerAPI = &v1.WebModelerAPIClientSpec{
		ConfidentialClientSpec: v1.ConfidentialClientSpec{
			ClientID: fixtureWebModelerAPIAudience,
			ClientSecretRef: v1.SecretKeyRef{
				Name:      "oidc-credentials",
				Namespace: "platform",
				Key:       "web-modeler-api-client-secret",
			},
		},
	}
}

// webModelerDatabase is the resolved DatabaseConfig of Web Modeler.
func webModelerDatabase() *Database {
	return &Database{
		Host: "postgres.camunda.svc",
		Port: 5432,
		Name: "web-modeler",
		Credentials: v1.LocalCredentialsSecretRef{
			Name:        "modeler-db-credentials",
			UsernameKey: "username",
			PasswordKey: "password",
		},
	}
}

// fixtureAttachedClusters are one oidc cluster, one basic-auth cluster, and
// one that publishes no gateway endpoints.
func fixtureAttachedClusters() []AttachedCluster {
	return []AttachedCluster{
		{
			Name:         "prod",
			Namespace:    "prod-ns",
			UID:          fixtureClusterOIDCUID,
			Version:      "8.9.9",
			ExternalURL:  "https://prod.example.com",
			GRPCEndpoint: "prod-gateway.prod-ns.svc:26500",
			RESTEndpoint: "http://prod-gateway.prod-ns.svc:8080",
			AuthMethod:   v1.AuthenticationMethodOIDC,
		},
		{
			Name:            "staging",
			Namespace:       "staging-ns",
			UID:             fixtureClusterBasicUID,
			Version:         "8.9.9",
			GRPCEndpoint:    "staging-gateway.staging-ns.svc:26500",
			RESTEndpoint:    "http://staging-gateway.staging-ns.svc:8080",
			AuthMethod:      v1.AuthenticationMethodBasic,
			BasicUserSecret: "my-management-web-modeler-cluster-66666666",
		},
		{
			Name:       "starting",
			Namespace:  "starting-ns",
			UID:        fixtureClusterNotReadyUID,
			Version:    "8.9.9",
			AuthMethod: v1.AuthenticationMethodOIDC,
		},
	}
}

// A management cluster that does not deploy Web Modeler renders neither
// workload nor the credential that pairs them. The component is built either
// way, gated off, so that a management cluster that drops Web Modeler has its
// workloads deleted rather than left running.
func TestWebModelerRendersNothingWhileTheSpecDoesNotDeployIt(t *testing.T) {
	t.Parallel()

	built, err := webModelerComponents(fixtureMinimal(t))
	require.NoError(t, err)
	require.Len(t, built.Components, 1)
	assert.Empty(t, built.Ready)

	objects, err := built.Components[0].Preview()

	require.NoError(t, err)
	assert.Empty(t, objects)
}

// One component covers both processes and the Secret that pairs them: Web
// Modeler answers only when all of them are there.
func TestWebModelerRendersOneComponentOverBothProcesses(t *testing.T) {
	t.Parallel()

	built, err := webModelerComponents(fixtureWebModelerMinimal(t))
	require.NoError(t, err)
	require.Len(t, built.Components, 1)
	require.Len(t, built.Ready, 1)

	objects, err := built.Components[0].Preview()
	require.NoError(t, err)

	names := make([]string, 0, len(objects))
	for _, obj := range objects {
		names = append(names, obj.GetName())
	}
	assert.Equal(
		t, []string{
			"my-management-web-modeler-pusher",
			"my-management-web-modeler-restapi",
			"my-management-web-modeler-restapi",
			"my-management-web-modeler-websockets",
			"my-management-web-modeler-websockets",
		}, names,
	)
}

// The environment of the restapi process is the table of the configuration
// page: every key that Web Modeler needs to reach its database, its SMTP
// server, its identity provider, and the WebSocket server.
func TestWebModelerRestapiEnvIsTheDocumentedTable(t *testing.T) {
	t.Parallel()

	env := renderedEnv(fixtureWebModelerMinimal(t), ComponentWebModelerRestapi)

	assert.Equal(
		t, map[string]string{
			"SPRING_DATASOURCE_URL":                                 "jdbc:postgresql://postgres.camunda.svc:5432/web-modeler",
			"SPRING_DATASOURCE_USERNAME":                            "secretKeyRef:modeler-db-credentials/username",
			"SPRING_DATASOURCE_PASSWORD":                            "secretKeyRef:modeler-db-credentials/password",
			"RESTAPI_MAIL_HOST":                                     "smtp.example.com",
			"RESTAPI_MAIL_PORT":                                     "587",
			"RESTAPI_MAIL_ENABLE_TLS":                               "true",
			"RESTAPI_MAIL_FROM_ADDRESS":                             "noreply@example.com",
			"RESTAPI_SERVER_URL":                                    fixtureWebModelerURL,
			"SERVER_HTTPS_ONLY":                                     "true",
			"OAUTH2_CLIENT_ID":                                      fixtureWebModelerClientID,
			"CAMUNDA_IDENTITY_BASEURL":                              "http://my-management-identity.camunda.svc:80",
			"CAMUNDA_IDENTITY_TYPE":                                 "GENERIC",
			"CAMUNDA_MODELER_SECURITY_JWT_AUDIENCE_INTERNAL_API":    fixtureWebModelerAPIAudience,
			"CAMUNDA_MODELER_SECURITY_JWT_AUDIENCE_PUBLIC_API":      "web-modeler-public-api",
			"SPRING_SECURITY_OAUTH2_RESOURCESERVER_JWT_ISSUER_URI":  fixtureIssuer,
			"SPRING_SECURITY_OAUTH2_RESOURCESERVER_JWT_JWK_SET_URI": fixtureIssuer + "/.well-known/jwks.json",
			"RESTAPI_OAUTH2_TOKEN_ISSUER_BACKEND_URL":               fixtureIssuer,
			"RESTAPI_PUSHER_HOST":                                   "my-management-web-modeler-websockets.camunda.svc",
			"RESTAPI_PUSHER_PORT":                                   "80",
			"RESTAPI_PUSHER_APP_ID":                                 "web-modeler",
			"RESTAPI_PUSHER_KEY":                                    "secretKeyRef:my-management-web-modeler-pusher/app-key",
			"RESTAPI_PUSHER_SECRET":                                 "secretKeyRef:my-management-web-modeler-pusher/app-secret",
			"CLIENT_PUSHER_HOST":                                    "modeler.example.com",
			"CLIENT_PUSHER_PORT":                                    "443",
			"CLIENT_PUSHER_PATH":                                    "/modeler-ws",
			"CLIENT_PUSHER_FORCE_TLS":                               "true",
		}, env,
	)
}

// The optional settings appear only when the spec asks for them.
func TestWebModelerRestapiEnvCarriesTheOptionalSettings(t *testing.T) {
	t.Parallel()

	env := renderedEnv(fixtureWebModelerRealistic(t), ComponentWebModelerRestapi)

	assert.Equal(t, "Camunda", env["RESTAPI_MAIL_FROM_NAME"])
	assert.Equal(t, "false", env["RESTAPI_MAIL_ENABLE_TLS"])
	assert.Equal(t, "secretKeyRef:smtp-credentials/username", env["RESTAPI_MAIL_USER"])
	assert.Equal(t, "secretKeyRef:smtp-credentials/password", env["RESTAPI_MAIL_PASSWORD"])
	assert.Equal(t, "modeler-public", env["CAMUNDA_MODELER_SECURITY_JWT_AUDIENCE_PUBLIC_API"])
	assert.Equal(t, "preferred_username", env["CAMUNDA_MODELER_OAUTH2_TOKEN_USERNAMECLAIM"])
}

// Web Modeler validates two audiences, one per resource server that
// Management Identity creates in the realm. A blank public API audience
// refuses the start of the restapi process, so a Keycloak mode has to render
// both, and it must not render the public audience as the internal one.
func TestWebModelerRestapiEnvCarriesBothAudiencesInTheKeycloakModes(t *testing.T) {
	t.Parallel()

	env := renderedEnv(newKeycloakInput(t, true, func(in *Input) {
		in.Cluster.Spec.WebModeler = webModeler("web-modeler-db")
	}), ComponentWebModelerRestapi)

	assert.Equal(t, "web-modeler-api", env["CAMUNDA_MODELER_SECURITY_JWT_AUDIENCE_INTERNAL_API"])
	assert.Equal(t, "web-modeler-public-api", env["CAMUNDA_MODELER_SECURITY_JWT_AUDIENCE_PUBLIC_API"])
}

// Web Modeler redirects a browser to https unless it is told not to, so an
// http external URL has to turn that off.
func TestWebModelerRestapiEnvFollowsTheSchemeOfTheExternalURL(t *testing.T) {
	t.Parallel()

	in := fixtureWebModelerMinimal(t)
	in.Cluster.Spec.WebModeler.ExternalURL = "http://modeler.example.com/modeler"
	in.Cluster.Spec.WebModeler.WebsocketsExternalURL = "http://modeler.example.com:8060"

	env := renderedEnv(in, ComponentWebModelerRestapi)

	assert.Equal(t, "false", env["SERVER_HTTPS_ONLY"])
	assert.Equal(t, "/modeler", env["SERVER_SERVLET_CONTEXTPATH"])
	assert.Equal(t, "8060", env["CLIENT_PUSHER_PORT"])
	assert.Equal(t, "/", env["CLIENT_PUSHER_PATH"])
	assert.Equal(t, "false", env["CLIENT_PUSHER_FORCE_TLS"])
}

// An external URL that points at the root of its domain needs no context path.
func TestWebModelerRestapiEnvOmitsTheContextPathOfARootURL(t *testing.T) {
	t.Parallel()

	env := renderedEnv(fixtureWebModelerMinimal(t), ComponentWebModelerRestapi)

	assert.NotContains(t, env, "SERVER_SERVLET_CONTEXTPATH")
}

// The license reaches Web Modeler the same way it reaches Management
// Identity: through the reference of the platform config, or not at all.
func TestWebModelerRestapiEnvCarriesTheLicense(t *testing.T) {
	t.Parallel()

	minimal := renderedEnv(fixtureWebModelerMinimal(t), ComponentWebModelerRestapi)
	assert.NotContains(t, minimal, "CAMUNDA_LICENSE_KEY")

	in := fixtureWebModelerMinimal(t)
	in.Platform.LicenseSecretRef = &v1.SecretKeyRef{
		Name: "license-copy", Namespace: fixtureNamespace, Key: "license",
	}

	assert.Equal(
		t,
		"secretKeyRef:license-copy/license",
		renderedEnv(in, ComponentWebModelerRestapi)["CAMUNDA_LICENSE_KEY"],
	)
}

// The websockets process needs the pairing credentials and the base path the
// browser is told, and nothing else.
func TestWebModelerWebsocketsEnvIsThePairing(t *testing.T) {
	t.Parallel()

	env := renderedEnv(fixtureWebModelerMinimal(t), ComponentWebModelerWebsockets)

	assert.Equal(
		t, map[string]string{
			"PUSHER_APP_ID":     "web-modeler",
			"PUSHER_APP_KEY":    "secretKeyRef:my-management-web-modeler-pusher/app-key",
			"PUSHER_APP_SECRET": "secretKeyRef:my-management-web-modeler-pusher/app-secret",
			"PUSHER_APP_PATH":   "/modeler-ws",
		}, env,
	)
}

// Both processes read the same pusher Secret, so a rotation must roll both.
// The key and the secret are looked up one by one, so a new source of either
// one is a rotation. The components of the management plane that do not read
// the Secret are unaffected.
func TestWebModelerConfigHashFollowsThePusherCredential(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		rotate func(in *Input)
	}{
		{"the app key", func(in *Input) { in.Pusher.Key.SourceUID = "a-new-secret" }},
		{"the app secret", func(in *Input) { in.Pusher.Secret.SourceUID = "a-new-secret" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			before := fixtureWebModelerRealistic(t)
			after := fixtureWebModelerRealistic(t)
			tc.rotate(&after)

			assert.NotEqual(
				t,
				ConfigHash(before, ComponentWebModelerRestapi),
				ConfigHash(after, ComponentWebModelerRestapi),
			)
			assert.NotEqual(
				t,
				ConfigHash(before, ComponentWebModelerWebsockets),
				ConfigHash(after, ComponentWebModelerWebsockets),
			)
			assert.Equal(
				t, ConfigHash(before, ComponentIdentity), ConfigHash(after, ComponentIdentity),
			)
		})
	}
}

// A credential that only Web Modeler reads is resolved under its component, so
// rotating it rolls Web Modeler and leaves Management Identity where it is.
func TestWebModelerComponentInputsRollWebModelerAlone(t *testing.T) {
	t.Parallel()

	before := fixtureWebModelerRealistic(t)
	after := fixtureWebModelerRealistic(t)
	after.ComponentHashInputs = map[string][]string{
		ComponentWebModelerRestapi: {"Secret/management/modeler-db-credentials=1234"},
	}

	assert.NotEqual(
		t,
		ConfigHash(before, ComponentWebModelerRestapi),
		ConfigHash(after, ComponentWebModelerRestapi),
	)
	assert.Equal(
		t, ConfigHash(before, ComponentIdentity), ConfigHash(after, ComponentIdentity),
	)
}

// The pusher Secret carries the precondition of a reused credential, so a
// delete of it rotates the pairing instead of republishing what was there.
func TestWebModelerPusherSecretCarriesTheApplyPrecondition(t *testing.T) {
	t.Parallel()

	fresh, err := webModelerPusherSecret(fixtureWebModelerMinimal(t))
	require.NoError(t, err)
	object, err := fresh.Preview()
	require.NoError(t, err)
	assert.NotContains(t, object.GetAnnotations(), credentials.PreconditionAnnotation)

	reused, err := webModelerPusherSecret(fixtureWebModelerRealistic(t))
	require.NoError(t, err)
	object, err = reused.Preview()
	require.NoError(t, err)
	assert.Equal(
		t,
		string(fixturePusherSourceUID),
		object.GetAnnotations()[credentials.PreconditionAnnotation],
	)
}

// Each workload reads the override block of its own process.
func TestWebModelerReplicasComeFromTheProcessBlock(t *testing.T) {
	t.Parallel()

	in := fixtureWebModelerRealistic(t)

	assert.Equal(t, int32(2), in.replicas(ComponentWebModelerRestapi))
	assert.Equal(t, int32(1), in.replicas(ComponentWebModelerWebsockets))
}
