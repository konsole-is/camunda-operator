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
	"strconv"

	corev1 "k8s.io/api/core/v1"

	"github.com/konsole-is/camunda-operator/pkg/camundaconfig"
)

// The environment variables that connect Management Identity to Keycloak.
//
// Configuration variables:
// https://docs.camunda.io/docs/self-managed/components/management-identity/miscellaneous/configuration-variables/
//
// Connect to an existing Keycloak instance:
// https://docs.camunda.io/docs/self-managed/components/management-identity/
// configuration/connect-to-an-existing-keycloak/
const (
	// keycloakEnvURL is the Keycloak that Identity administers, the base path
	// included. Identity signs in there to create the realm and its contents.
	keycloakEnvURL = "KEYCLOAK_URL"
	// keycloakEnvRealm is the realm that holds every Camunda client and user.
	keycloakEnvRealm = "KEYCLOAK_REALM"
	// keycloakEnvSetupUser and keycloakEnvSetupPassword are the Keycloak
	// administrator that Identity signs in as. keycloakEnvSetupRealm and
	// keycloakEnvSetupClientID are the realm and the client it signs in
	// through.
	keycloakEnvSetupUser     = "KEYCLOAK_SETUP_USER"
	keycloakEnvSetupPassword = "KEYCLOAK_SETUP_PASSWORD"
	keycloakEnvSetupRealm    = "KEYCLOAK_SETUP_REALM"
	keycloakEnvSetupClientID = "KEYCLOAK_SETUP_CLIENT_ID"
	// keycloakEnvIdentityClientID is the id of the Identity client in the
	// realm. Identity creates the client and makes a new secret for it on
	// every start, so the operator sets the id only. IDENTITY_CLIENT_SECRET
	// is for a realm that a person prepared by hand, the page above, and the
	// operator does not render it. With it set, Identity signs in with client
	// credentials before the realm exists and never runs its setup.
	keycloakEnvIdentityClientID = "IDENTITY_CLIENT_ID"
	// keycloakEnvIssuerURL is the front-channel issuer of the realm, used for
	// the login redirect and the logout.
	keycloakEnvIssuerURL = "IDENTITY_AUTH_PROVIDER_ISSUER_URL"
	// keycloakEnvBackendURL is the back-channel issuer of the realm, used for
	// token verification from inside the Kubernetes cluster.
	keycloakEnvBackendURL = "IDENTITY_AUTH_PROVIDER_BACKEND_URL"
)

// The environment variables that tell Management Identity which components to
// create a client for. Identity carries a preset per component and reads
// KEYCLOAK_INIT_<COMPONENT>_ROOT_URL and _SECRET to parameterize it. The
// preset also creates the resource server that the audience of the component
// names, which a hand-written KEYCLOAK_CLIENTS_<n> entry would not.
//
// Component configuration:
// https://docs.camunda.io/docs/self-managed/components/management-identity/
// miscellaneous/configuration-variables/#component-configuration
//
// The preset names and the root URLs they take are the ones the 8.9 Helm
// chart writes into the Identity configuration
// (charts/camunda-platform-8.9/templates/identity/configmap.yaml, the
// keycloak.init block).
const (
	// keycloakEnvInitOptimizeSecret is the client secret of the optimize
	// preset. The configuration-variables page above documents
	// KEYCLOAK_INIT_<COMPONENT>_SECRET. The 8.9 chart binds the same property
	// from VALUES_KEYCLOAK_INIT_OPTIMIZE_SECRET, in the application.yaml it
	// mounts ("secret: ${VALUES_KEYCLOAK_INIT_OPTIMIZE_SECRET:}" under
	// keycloak.init.optimize, in the configmap file below). The operator
	// mounts no such file, and an environment variable outranks a
	// configuration file in Spring, so the documented name is the one that
	// reaches keycloak.init.optimize.secret.
	keycloakEnvInitOptimizeSecret = "KEYCLOAK_INIT_OPTIMIZE_SECRET"
	// keycloakEnvInitOptimizeRootURL is the comma-separated list of URLs that
	// browsers reach the Optimize instances at. Management Identity splits it
	// on commas, drops the blank entries, and registers the login callback
	// under each one as a redirect URI of the optimize client
	// (ClientInitializationService.java, generateRedirectUrls). It re-applies
	// the whole client on every start, so what this variable carries is the
	// floor of the list and never less
	// (https://github.com/camunda/camunda/issues/59963). The operator adds the
	// Optimize instances it found after that write through the Keycloak
	// administration API.
	//
	// The value is read from a ConfigMap rather than written into the pod
	// template, because the pod template is what Kubernetes rolls on. A
	// configMapKeyRef does not change when the list behind it does, so adding
	// or removing an Optimize updates that object and restarts nothing.
	// Whether the variable is rendered at all still moves the template, and
	// that is the one roll this keeps: the first Optimize turns the preset on
	// and the last one gone turns it off.
	keycloakEnvInitOptimizeRootURL = "KEYCLOAK_INIT_OPTIMIZE_ROOT_URL"
	// keycloakEnvInitConsoleRootURL is the URL that browsers reach Console
	// at. The configuration-variables page names OPERATE, OPTIMIZE, TASKLIST,
	// and WEBMODELER as the components, and it leaves Console out. The preset
	// exists all the same. The 8.9 chart file
	// charts/camunda-platform-8.9/templates/identity/configmap.yaml sets
	// keycloak.init.console.root-url. The same file defines a
	// component-presets.console block with the console client, the
	// console-api resource server, and the Console role.
	keycloakEnvInitConsoleRootURL    = "KEYCLOAK_INIT_CONSOLE_ROOT_URL"
	keycloakEnvInitWebModelerRootURL = "KEYCLOAK_INIT_WEBMODELER_ROOT_URL"
)

// The environment variables of the first Keycloak user. Identity creates it
// on its first start and assigns it the roles of every component the
// management plane runs.
const (
	keycloakEnvUserUsername = "KEYCLOAK_USERS_0_USERNAME"
	keycloakEnvUserPassword = "KEYCLOAK_USERS_0_PASSWORD"
	keycloakEnvUserEmail    = "KEYCLOAK_USERS_0_EMAIL"
	// keycloakEnvUserRolePrefix carries the index of the role after it, for
	// example KEYCLOAK_USERS_0_ROLES_0.
	keycloakEnvUserRolePrefix = "KEYCLOAK_USERS_0_ROLES_"
)

// The literal values of the two Keycloak modes. Every client id, audience,
// and role is the one that Management Identity creates in the realm, from the
// Identity configuration of the 8.9 Helm chart
// (charts/camunda-platform-8.9/templates/identity/configmap.yaml) and the
// starting configuration page
// (https://docs.camunda.io/docs/self-managed/components/management-identity/miscellaneous/starting-configuration/).
const (
	// keycloakDefaultRealm is the realm that Management Identity creates and
	// uses when the spec names none.
	keycloakDefaultRealm = "camunda-platform"
	// keycloakSetupRealm and keycloakSetupClientID are the realm the Keycloak
	// administrator lives in and the client Management Identity signs in
	// through. Both are the documented defaults of Management Identity, and
	// both are rendered so that the configuration says where the
	// administrator comes from. The Keycloak Operator writes its first
	// administrator into the master realm, and every Keycloak serves
	// admin-cli there.
	keycloakSetupRealm    = "master"
	keycloakSetupClientID = "admin-cli"
	// keycloakBasePath is the path that the Camunda build of Keycloak serves
	// under. The image bakes it in, and the rendered Keycloak sets the same
	// value in its server options
	// (https://docs.camunda.io/docs/self-managed/deployment/helm/configure/operator-based-infrastructure/).
	keycloakBasePath = "/auth"
	// keycloakRealmPath, keycloakAuthPath, keycloakTokenPath, and
	// keycloakCertsPath are the OpenID Connect endpoints of a Keycloak realm.
	keycloakRealmPath = "/realms/"
	keycloakAuthPath  = "/protocol/openid-connect/auth"
	keycloakTokenPath = "/protocol/openid-connect/token"
	keycloakCertsPath = "/protocol/openid-connect/certs"

	// The client ids that Management Identity creates.
	keycloakClientIdentity   = "camunda-identity"
	keycloakClientOptimize   = "optimize"
	keycloakClientConsole    = "console"
	keycloakClientWebModeler = "web-modeler"

	// The audiences of the resource servers that Management Identity creates.
	// Web Modeler has two: the internal API that its own user interface calls
	// and the public API that a script calls.
	keycloakAudienceIdentity         = "camunda-identity-resource-server"
	keycloakAudienceOptimize         = "optimize-api"
	keycloakAudienceConsole          = "console-api"
	keycloakAudienceWebModeler       = "web-modeler-api"
	keycloakAudienceWebModelerPublic = "web-modeler-public-api"

	// The roles that Management Identity creates, one per component.
	keycloakRoleIdentity        = "ManagementIdentity"
	keycloakRoleOptimize        = "Optimize"
	keycloakRoleConsole         = "Console"
	keycloakRoleWebModeler      = "Web Modeler"
	keycloakRoleWebModelerAdmin = "Web Modeler Admin"
)

// keycloakProviderEnv renders the connection of Management Identity to
// Keycloak, and everything Identity creates in the realm on its first start:
// the client of each component the management plane serves, and the first
// user.
func keycloakProviderEnv(in Input) []corev1.EnvVar {
	provider := in.Provider

	env := []corev1.EnvVar{
		{Name: camundaconfig.EnvSpringProfilesActive, Value: provider.SpringProfile},
		{Name: identityEnvType, Value: provider.Type},
		{Name: identityEnvAudience, Value: provider.Clients.Identity.Audience},
		{Name: keycloakEnvURL, Value: provider.KeycloakURL},
		{Name: keycloakEnvRealm, Value: provider.Realm},
		{Name: keycloakEnvIssuerURL, Value: provider.IssuerURL},
		{Name: keycloakEnvBackendURL, Value: provider.IssuerBackendURL},
		{Name: keycloakEnvIdentityClientID, Value: provider.Clients.Identity.ID},
	}
	if admin := provider.AdminCredentials; admin != nil {
		env = append(
			env,
			corev1.EnvVar{
				Name:      keycloakEnvSetupUser,
				ValueFrom: secretSource(admin.Name, admin.UsernameKey),
			},
			corev1.EnvVar{
				Name:      keycloakEnvSetupPassword,
				ValueFrom: secretSource(admin.Name, admin.PasswordKey),
			},
			corev1.EnvVar{Name: keycloakEnvSetupRealm, Value: keycloakSetupRealm},
			corev1.EnvVar{Name: keycloakEnvSetupClientID, Value: keycloakSetupClientID},
		)
	}

	env = append(env, keycloakPresetEnv(in)...)

	return append(env, keycloakFirstUserEnv(in)...)
}

// keycloakPresetEnv selects the client preset of each component that
// authenticates through this management plane.
//
// A management plane that serves no Optimize renders no KEYCLOAK_INIT_OPTIMIZE
// variable at all. Management Identity processes a preset only for the
// components its environment names (KeycloakPresetInitializer.java, which
// iterates the keys of keycloak.init), and it shuts itself down when a preset
// that is not machine-to-machine carries a blank root URL
// (ClientInitializationService.java, validateClientRootUrl). Leaving the
// variables out is therefore what keeps Identity running. The realm then holds
// no optimize client, and the ManagementAuthConfig names a client that nothing
// created yet; the client arrives with the first Optimize that names a URL.
func keycloakPresetEnv(in Input) []corev1.EnvVar {
	spec := in.Cluster.Spec

	var env []corev1.EnvVar
	if len(in.OptimizeURLs) > 0 {
		env = append(
			env,
			corev1.EnvVar{
				Name: keycloakEnvInitOptimizeRootURL,
				ValueFrom: configMapSource(
					IdentityOptimizeURLsName(in.Cluster), OptimizeRootURLKey,
				),
			},
		)
		if ref := in.Provider.Clients.Optimize.SecretRef; ref != nil {
			env = append(env, corev1.EnvVar{
				Name:      keycloakEnvInitOptimizeSecret,
				ValueFrom: secretSource(ref.Name, ref.Key),
			})
		}
	}
	if console := spec.Console; console != nil {
		env = append(env, corev1.EnvVar{
			Name:  keycloakEnvInitConsoleRootURL,
			Value: console.ExternalURL,
		})
	}
	if webModeler := spec.WebModeler; webModeler != nil {
		env = append(env, corev1.EnvVar{
			Name:  keycloakEnvInitWebModelerRootURL,
			Value: webModeler.ExternalURL,
		})
	}

	return env
}

// keycloakFirstUserEnv renders the first Keycloak user: the administrator of
// spec.identity.admin, with the role of every component the management plane
// runs. Identity creates the user on its first start.
func keycloakFirstUserEnv(in Input) []corev1.EnvVar {
	admin := in.Cluster.Spec.Identity.Admin

	env := []corev1.EnvVar{{Name: keycloakEnvUserUsername, Value: admin.Username}}
	if name := in.Secrets.IdentityAdmin; name != "" {
		env = append(env, corev1.EnvVar{
			Name:      keycloakEnvUserPassword,
			ValueFrom: secretSource(name, PasswordKey),
		})
	} else if ref := admin.PasswordSecretRef; ref != nil {
		env = append(env, corev1.EnvVar{
			Name:      keycloakEnvUserPassword,
			ValueFrom: secretSource(ref.Name, ref.Key),
		})
	}
	if admin.Email != "" {
		env = append(env, corev1.EnvVar{Name: keycloakEnvUserEmail, Value: admin.Email})
	}

	for i, role := range keycloakAdminRoles(in) {
		env = append(env, corev1.EnvVar{
			Name:  keycloakEnvUserRolePrefix + strconv.Itoa(i),
			Value: role,
		})
	}

	return env
}

// keycloakAdminRoles returns the roles of the first user: Management Identity
// always, and one per component the management plane serves. Identity creates a
// role together with the preset of its component, so a role of a component
// with no preset would not exist. A management plane that serves no Optimize
// renders no Optimize preset, so the Optimize role is left out too.
func keycloakAdminRoles(in Input) []string {
	spec := in.Cluster.Spec

	roles := []string{keycloakRoleIdentity}
	if len(in.OptimizeURLs) > 0 {
		roles = append(roles, keycloakRoleOptimize)
	}
	if spec.Console != nil {
		roles = append(roles, keycloakRoleConsole)
	}
	if spec.WebModeler != nil {
		roles = append(roles, keycloakRoleWebModeler, keycloakRoleWebModelerAdmin)
	}

	return roles
}
