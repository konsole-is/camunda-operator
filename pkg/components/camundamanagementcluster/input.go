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
	"cmp"
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/credentials"
)

// Input is everything the pure package needs to render one management plane.
// The controller fills it in its pre-checks.
type Input struct {
	// Cluster is the CamundaManagementCluster as it was read.
	Cluster *v1.CamundaManagementCluster
	// Platform is the spec of the CamundaPlatformConfig that
	// spec.platformConfigRef names, with every Secret reference already
	// pointed at its copy in the management namespace.
	Platform *v1.CamundaPlatformConfigSpec
	// Provider is the identity provider that every renderer reads. Build it
	// with ResolveIdentityProvider.
	Provider IdentityProvider
	// Databases are the resolved DatabaseConfigs of the components that need
	// one, with every Secret reference already pointed at its copy in the
	// management namespace.
	Databases Databases
	// Secrets are the names of the Secrets that the operator generates. They
	// are empty in the oidc mode, which generates nothing.
	Secrets GeneratedSecrets
	// Mirrors are the copies of the referenced Secrets that live outside the
	// management namespace, by purpose. A pod mounts a Secret only from its
	// own namespace.
	Mirrors map[MirrorPurpose]map[string][]byte
	// Clusters are the orchestration clusters that the management plane
	// serves, ordered by namespace and name.
	Clusters []AttachedCluster
	// Suspended is spec.suspend. It scales every workload of the management
	// plane to zero.
	Suspended bool
	// HashInputs are the resource versions of the referenced Secrets and the
	// generations of the referenced custom resources, as
	// "kind/namespace/name=version" strings. ConfigHash sorts them, so the
	// order does not matter.
	HashInputs []string
	// KeycloakCRDServed reports whether the Kubernetes cluster serves the
	// Keycloak kind of the Keycloak Operator.
	KeycloakCRDServed bool
}

// ProviderMode is which of the three identity provider modes the spec selects.
type ProviderMode string

const (
	// ModeKeycloak runs Keycloak through the Keycloak Operator.
	ModeKeycloak ProviderMode = "keycloak"
	// ModeExternalKeycloak connects to a Keycloak that the user runs.
	ModeExternalKeycloak ProviderMode = "externalKeycloak"
	// ModeOIDC connects to the identity provider of the platform config.
	ModeOIDC ProviderMode = "oidc"
)

// IdentityProvider is where the components of the management plane send users
// to authenticate, resolved once for every mode. Renderers read it and never
// switch on the mode again.
type IdentityProvider struct {
	// Mode is the mode that the spec selects.
	Mode ProviderMode
	// Type is the CAMUNDA_IDENTITY_TYPE value: KEYCLOAK, GENERIC, or
	// MICROSOFT.
	Type string
	// SpringProfile is the SPRING_PROFILES_ACTIVE value that binds the
	// settings of this mode on Management Identity and Console. It is empty
	// in the Keycloak modes, which need no profile.
	SpringProfile string
	// IssuerURL is the issuer that a browser is redirected to.
	IssuerURL string
	// IssuerBackendURL is the issuer that a container reaches from inside the
	// Kubernetes cluster.
	IssuerBackendURL string
	// AuthURL, TokenURL, and JwksURL are the endpoints of the provider.
	AuthURL, TokenURL, JwksURL string
	// KeycloakURL is the in-cluster Keycloak URL, the /auth path included. It
	// is empty in the oidc mode.
	KeycloakURL string
	// KeycloakPublicURL is the Keycloak URL that a browser reaches. It is
	// empty in the oidc mode.
	KeycloakPublicURL string
	// Realm is the Keycloak realm. It is empty in the oidc mode.
	Realm string
	// UsernameClaim is the token claim that holds the username of a person,
	// or empty for the default of each component.
	UsernameClaim string
	// AdminCredentials names the Secret with the Keycloak administrator that
	// Management Identity signs in with to create the realm, the clients, and
	// the initial administrator. It is nil in the oidc mode, which bootstraps
	// nothing.
	AdminCredentials *v1.CredentialsSecretRef
	// Clients is the provider client of each component.
	Clients ProviderClients
}

// ProviderClients is the identity provider client of each component of the
// management plane.
type ProviderClients struct {
	// Identity, Optimize, and WebModelerAPI authenticate with a secret.
	Identity, Optimize, WebModelerAPI Client
	// WebModeler and Console run in a browser, so they hold no secret.
	WebModeler, Console PublicClient
}

// Client is an identity provider client that authenticates with a secret.
type Client struct {
	// ID is the client id at the identity provider.
	ID string
	// Audience is the audience that the component validates in access tokens.
	Audience string
	// SecretRef names the Secret key that holds the client secret. It is nil
	// for a component that the spec does not deploy.
	SecretRef *v1.SecretKeyRef
}

// PublicClient is an identity provider client that a browser uses, so it has
// no secret.
type PublicClient struct {
	// ID is the client id at the identity provider.
	ID string
	// Audience is the audience that the component validates in access tokens.
	Audience string
}

// Database is one resolved DatabaseConfig: where the server is, which logical
// database to open, and which Secret holds the credentials of the application
// user.
type Database struct {
	// Host is the host name of the database server.
	Host string
	// Port is the port of the database server.
	Port int32
	// Name is the name of the logical database.
	Name string
	// Credentials names the Secret with the user and the password, in the
	// management namespace.
	Credentials v1.CredentialsSecretRef
}

// Databases holds the database of each component that needs one. Keycloak and
// Web Modeler are nil while the spec does not deploy them.
type Databases struct {
	// Identity is the database of Management Identity.
	Identity Database
	// Keycloak is the database of the Keycloak that the operator runs.
	Keycloak *Database
	// WebModeler is the database of Web Modeler.
	WebModeler *Database
}

// GeneratedSecrets holds the names of the Secrets that the operator
// generates, all of them in the management namespace. Every field is empty in
// the oidc mode, where the platform config names every client secret and the
// initial administrator is a token claim.
type GeneratedSecrets struct {
	// IdentityClient holds the Management Identity client secret.
	IdentityClient string
	// OptimizeClient holds the Optimize client secret.
	OptimizeClient string
	// IdentityAdmin holds the password of the first administrator.
	IdentityAdmin string
	// WebModelerPusher holds the credentials that the two Web Modeler
	// processes push live updates over.
	WebModelerPusher string
	// Values are the generated credentials, by the name of the Secret that
	// publishes them. The Secrets component writes them, and the config hash
	// of a component folds in the digest of every value that component reads,
	// so a rotation rolls that component and no other.
	Values map[string]credentials.Password
}

// AttachedCluster is one orchestration cluster that the management plane
// serves. Console lists it and Web Modeler deploys to it.
type AttachedCluster struct {
	// Name and Namespace identify the CamundaCluster.
	Name, Namespace string
	// UID is the UID of the CamundaCluster. It tells a cluster apart from a
	// later one with the same name.
	UID types.UID
	// Version is the Camunda version that the cluster runs, for example
	// 8.9.9.
	Version string
	// ExternalURL is the URL that a browser reaches the cluster at, or empty
	// when the cluster names none.
	ExternalURL string
	// GRPCEndpoint and RESTEndpoint are the in-cluster addresses of the
	// client APIs, from the gateway binding that the cluster publishes.
	GRPCEndpoint, RESTEndpoint string
	// AuthMethod is how the cluster authenticates its users and clients. Web
	// Modeler deploys to an oidc cluster with a bearer token and to a basic
	// cluster with a user of its own.
	AuthMethod v1.AuthenticationMethod
	// BasicUserSecret names the Secret with the password of the Web Modeler
	// user on a basic-auth cluster. It is empty under oidc.
	BasicUserSecret string
}

// ResolveIdentityProvider builds the identity provider that every renderer
// reads. It uses Cluster, Platform, and Secrets of in, and ignores in.Provider.
//
// A state the user must correct comes back as a *conditions.PreCheckFailure
// with the reason InvalidReference: a platform config that does not select
// oidc, an endpoint that the contract needs and the platform config does not
// carry, or a component of the management plane with no client. The message
// names the field.
func ResolveIdentityProvider(in Input) (IdentityProvider, error) {
	switch Mode(in.Cluster) {
	case ModeOIDC:
		return resolveOIDC(in)
	case ModeKeycloak:
		return resolveManagedKeycloak(in), nil
	default:
		return resolveExternalKeycloak(in), nil
	}
}

// Mode returns the identity provider mode that the spec selects. The API
// server admits exactly one of the three blocks, so an unset keycloak and an
// unset oidc block leave externalKeycloak.
func Mode(mc *v1.CamundaManagementCluster) ProviderMode {
	switch {
	case mc.Spec.IdentityProvider.OIDC != nil:
		return ModeOIDC
	case mc.Spec.IdentityProvider.Keycloak != nil:
		return ModeKeycloak
	default:
		return ModeExternalKeycloak
	}
}

// resolveManagedKeycloak reads the Keycloak that the operator runs. Browsers
// and the Identity pods reach it at spec.externalUrl; every container reaches
// it at the Service that the Keycloak Operator creates. The two URLs are the
// front-channel and the back-channel issuer of the realm.
//
// The administrator is the one the Keycloak Operator writes into
// <keycloak>-initial-admin next to the Keycloak custom resource.
func resolveManagedKeycloak(in Input) IdentityProvider {
	spec := in.Cluster.Spec.IdentityProvider.Keycloak
	service := fmt.Sprintf(
		"http://%s.%s.svc:%d%s",
		KeycloakServiceName(in.Cluster), in.Cluster.Namespace, KeycloakServicePort, keycloakBasePath,
	)

	return keycloakProvider(in, ModeKeycloak, service, spec.ExternalURL, keycloakDefaultRealm, &v1.CredentialsSecretRef{
		Name:        KeycloakInitialAdminSecretName(in.Cluster),
		Namespace:   in.Cluster.Namespace,
		UsernameKey: KeycloakAdminUsernameKey,
		PasswordKey: KeycloakAdminPasswordKey,
	})
}

// resolveExternalKeycloak reads the Keycloak that the user runs. One URL
// serves browsers and containers alike, so the front-channel and the
// back-channel issuer are the same. The administrator comes from
// adminCredentialsSecretRef, through its copy in the management namespace.
func resolveExternalKeycloak(in Input) IdentityProvider {
	spec := in.Cluster.Spec.IdentityProvider.ExternalKeycloak
	admin := spec.AdminCredentialsSecretRef.DeepCopy()
	admin.Name = LocalSecretName(
		in.Cluster, admin.Namespace, admin.Name, MirrorPurposeKeycloakAdmin,
	)
	admin.Namespace = in.Cluster.Namespace

	return keycloakProvider(
		in, ModeExternalKeycloak, spec.URL, spec.URL, cmp.Or(spec.Realm, keycloakDefaultRealm), admin,
	)
}

// keycloakProvider builds the identity provider of both Keycloak modes.
// Management Identity creates every client in the realm on its first start,
// so the client ids and the audiences are the ones it uses, not values a user
// chooses.
func keycloakProvider(
	in Input,
	mode ProviderMode,
	backendURL, publicURL, realm string,
	admin *v1.CredentialsSecretRef,
) IdentityProvider {
	issuer := publicURL + keycloakRealmPath + realm

	return IdentityProvider{
		Mode:              mode,
		Type:              identityTypeKeycloak,
		SpringProfile:     identityProfileKeycloak,
		IssuerURL:         issuer,
		IssuerBackendURL:  backendURL + keycloakRealmPath + realm,
		AuthURL:           issuer + keycloakAuthPath,
		TokenURL:          issuer + keycloakTokenPath,
		JwksURL:           issuer + keycloakCertsPath,
		KeycloakURL:       backendURL,
		KeycloakPublicURL: publicURL,
		Realm:             realm,
		AdminCredentials:  admin,
		Clients:           keycloakClients(in),
	}
}

// keycloakClients returns the clients that Management Identity creates in the
// realm, one per component the spec deploys. A Keycloak client holds no
// secret of its own for a browser application, and Web Modeler is one client
// in front of two resource servers, so its two entries carry the same id.
func keycloakClients(in Input) ProviderClients {
	clients := ProviderClients{
		Identity: Client{
			ID:       keycloakClientIdentity,
			Audience: keycloakAudienceIdentity,
			SecretRef: &v1.SecretKeyRef{
				Name:      in.Secrets.IdentityClient,
				Namespace: in.Cluster.Namespace,
				Key:       ClientSecretKey,
			},
		},
		Optimize: Client{
			ID:       keycloakClientOptimize,
			Audience: keycloakAudienceOptimize,
			SecretRef: &v1.SecretKeyRef{
				Name:      in.Secrets.OptimizeClient,
				Namespace: in.Cluster.Namespace,
				Key:       ClientSecretKey,
			},
		},
	}
	if in.Cluster.Spec.Console != nil {
		clients.Console = PublicClient{ID: keycloakClientConsole, Audience: keycloakAudienceConsole}
	}
	if in.Cluster.Spec.WebModeler != nil {
		clients.WebModeler = PublicClient{
			ID: keycloakClientWebModeler, Audience: keycloakAudienceWebModeler,
		}
		clients.WebModelerAPI = Client{
			ID: keycloakClientWebModeler, Audience: keycloakAudienceWebModelerPublic,
		}
	}

	return clients
}

// resolveOIDC reads the identity provider of the platform config. The issuer
// is both the front-channel and the back-channel issuer: Camunda does not
// support split-horizon URLs for a generic OIDC provider, see
// https://docs.camunda.io/docs/self-managed/components/management-identity/configuration/connect-to-an-oidc-provider/
func resolveOIDC(in Input) (IdentityProvider, error) {
	if in.Platform.Method() != v1.AuthenticationMethodOIDC || in.Platform.Auth.OIDC == nil {
		return IdentityProvider{}, &conditions.PreCheckFailure{
			Reason: v1.ReasonInvalidReference,
			Message: fmt.Sprintf(
				"CamundaPlatformConfig %q authenticates with %s; "+
					"the oidc mode needs a platform config whose spec.auth.method is oidc",
				in.Cluster.Spec.PlatformConfigRef, in.Platform.Method(),
			),
		}
	}

	oidc := in.Platform.Auth.OIDC
	if err := checkEndpoints(in, oidc); err != nil {
		return IdentityProvider{}, err
	}

	clients, err := resolveOIDCClients(in, oidc)
	if err != nil {
		return IdentityProvider{}, err
	}

	return IdentityProvider{
		Mode:             ModeOIDC,
		Type:             identityType(oidc.ProviderType),
		SpringProfile:    identityProfileOIDC,
		IssuerURL:        oidc.IssuerURL,
		IssuerBackendURL: oidc.IssuerURL,
		AuthURL:          oidc.AuthURL,
		TokenURL:         oidc.TokenURL,
		JwksURL:          oidc.JWKSURL,
		UsernameClaim:    oidc.UsernameClaim,
		Clients:          clients,
	}, nil
}

// checkEndpoints refuses a platform config that leaves an endpoint of the
// ManagementAuthConfig empty. The three endpoints are optional on the
// platform config, because a consumer of an orchestration cluster reads them
// from the discovery document of the provider, and the contract requires them.
// The operator makes no request to the provider, so the platform config is the
// only place they can come from.
func checkEndpoints(in Input, oidc *v1.OIDCSpec) error {
	var missing []string
	for _, endpoint := range []struct {
		field string
		value string
	}{
		{"authUrl", oidc.AuthURL},
		{"tokenUrl", oidc.TokenURL},
		{"jwksUrl", oidc.JWKSURL},
	} {
		if endpoint.value == "" {
			missing = append(missing, "spec.auth.oidc."+endpoint.field)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonInvalidReference,
		Message: fmt.Sprintf(
			"CamundaPlatformConfig %q sets no %v; the ManagementAuthConfig carries all three, "+
				"so read them from the discovery document of your identity provider and set them there",
			in.Cluster.Spec.PlatformConfigRef, missing,
		),
	}
}

// resolveOIDCClients reads the client of each component that the spec
// deploys. Management Identity and Optimize are always required: Identity is
// always deployed, and the ManagementAuthConfig always carries the Optimize
// client.
func resolveOIDCClients(in Input, oidc *v1.OIDCSpec) (ProviderClients, error) {
	var declared v1.ManagementClients
	if oidc.Management != nil {
		declared = oidc.Management.Clients
	}

	var clients ProviderClients
	for _, required := range []struct {
		field  string
		spec   *v1.ConfidentialClientSpec
		target *Client
	}{
		{"identity", declared.Identity, &clients.Identity},
		{"optimize", declared.Optimize, &clients.Optimize},
	} {
		if required.spec == nil {
			return ProviderClients{}, missingClient(in, required.field)
		}
		*required.target = confidentialClient(*required.spec)
	}

	if in.Cluster.Spec.Console != nil {
		if declared.Console == nil {
			return ProviderClients{}, missingClient(in, "console")
		}
		clients.Console = publicClient(*declared.Console)
	}

	if in.Cluster.Spec.WebModeler != nil {
		if declared.WebModeler == nil {
			return ProviderClients{}, missingClient(in, "webModeler")
		}
		if declared.WebModelerAPI == nil {
			return ProviderClients{}, missingClient(in, "webModelerApi")
		}
		clients.WebModeler = publicClient(*declared.WebModeler)
		clients.WebModelerAPI = confidentialClient(declared.WebModelerAPI.ConfidentialClientSpec)
	}

	return clients, nil
}

// missingClient reports a component of the management plane that the platform
// config declares no client for.
func missingClient(in Input, field string) error {
	return &conditions.PreCheckFailure{
		Reason: v1.ReasonInvalidReference,
		Message: fmt.Sprintf(
			"CamundaPlatformConfig %q declares no spec.auth.oidc.management.clients.%s; "+
				"register an application for that component at your identity provider and name it there",
			in.Cluster.Spec.PlatformConfigRef, field,
		),
	}
}

// confidentialClient reads a client that authenticates with a secret. An
// empty audience means the client id.
func confidentialClient(spec v1.ConfidentialClientSpec) Client {
	return Client{
		ID:        spec.ClientID,
		Audience:  cmp.Or(spec.Audience, spec.ClientID),
		SecretRef: spec.ClientSecretRef.DeepCopy(),
	}
}

// publicClient reads a client that a browser uses. An empty audience means the
// client id.
func publicClient(spec v1.PublicClientSpec) PublicClient {
	return PublicClient{ID: spec.ClientID, Audience: cmp.Or(spec.Audience, spec.ClientID)}
}

// identityType returns the CAMUNDA_IDENTITY_TYPE value of a provider type. An
// unset provider type means a generic OIDC provider
// (https://docs.camunda.io/docs/self-managed/components/management-identity/miscellaneous/configuration-variables/).
func identityType(providerType v1.OIDCProviderType) string {
	if providerType == v1.OIDCProviderMicrosoft {
		return identityTypeMicrosoft
	}

	return identityTypeGeneric
}

// workload returns the WorkloadSpec of a component, or the zero value when the
// spec sets no block for it.
func (in Input) workload(comp string) v1.WorkloadSpec {
	if comp == ComponentIdentity {
		return in.Cluster.Spec.Identity.WorkloadSpec
	}

	return v1.WorkloadSpec{}
}

// replicas returns the replica count of a component. Every component defaults
// to 1.
func (in Input) replicas(comp string) int32 {
	if comp == ComponentKeycloak {
		// Keycloak has no WorkloadSpec: the Keycloak Operator owns its pods,
		// so the spec offers the replica count and the resources alone.
		if keycloak := in.Cluster.Spec.IdentityProvider.Keycloak; keycloak != nil &&
			keycloak.Replicas != nil {
			return *keycloak.Replicas
		}

		return 1
	}

	if r := in.workload(comp).Replicas; r != nil {
		return *r
	}

	return 1
}
