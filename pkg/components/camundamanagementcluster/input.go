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
	// Pusher are the credentials that pair the two Web Modeler processes. The
	// controller reads them back from the generated Secret, or generates
	// them. They are empty while spec.webModeler is unset.
	Pusher PusherCredentials
	// WebModelerMail names the SMTP credentials of Web Modeler, already
	// pointed at their copy in the management namespace. It is nil for an
	// SMTP server that needs none.
	WebModelerMail *v1.CredentialsSecretRef
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
	// ComponentHashInputs are the same strings for what one component alone
	// reads, by component name. ConfigHash adds them to that component and to
	// no other, so a credential that only Web Modeler reads never rolls
	// Management Identity.
	ComponentHashInputs map[string][]string
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
	// WebModelerPublicAPIAudience is the audience of the Web Modeler public
	// API, which your applications call. Web Modeler validates it separately
	// from the audience of the API that its own user interface calls, so the
	// WebModelerAPI client carries two.
	WebModelerPublicAPIAudience string
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
}

// PusherCredentials authenticate the Web Modeler restapi process at the
// WebSocket server that pushes live updates to the browser. Both processes
// read the same values, so a change to either one has to reach both.
type PusherCredentials struct {
	// Key and Secret are the generated credentials. They come from one
	// Secret, so they rotate together when it is deleted.
	Key, Secret credentials.Password
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
	spec := in.Cluster.Spec.IdentityProvider
	if spec.OIDC != nil {
		return resolveOIDC(in)
	}

	mode := ModeExternalKeycloak
	if spec.Keycloak != nil {
		mode = ModeKeycloak
	}

	return IdentityProvider{}, &conditions.PreCheckFailure{
		Reason: v1.ReasonInvalidReference,
		Message: fmt.Sprintf(
			"spec.identityProvider selects %s; this operator supports the oidc mode only", mode,
		),
	}
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
		clients.WebModelerPublicAPIAudience = cmp.Or(
			declared.WebModelerAPI.PublicAPIAudience, webModelerDefaultPublicAPIAudience,
		)
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

// console returns spec.console, or the zero value while the spec deploys no
// Console. The renderer runs either way: a Console that the spec dropped is
// rendered gated off, so that its component deletes what it left behind.
func (in Input) console() v1.ConsoleSpec {
	if in.Cluster.Spec.Console == nil {
		return v1.ConsoleSpec{}
	}

	return *in.Cluster.Spec.Console
}

// webModeler returns spec.webModeler, or the zero value while the spec deploys
// no Web Modeler. The renderer runs either way: a Web Modeler that the spec
// dropped is rendered gated off, so that its component deletes what it left
// behind.
func (in Input) webModeler() v1.WebModelerSpec {
	if in.Cluster.Spec.WebModeler == nil {
		return v1.WebModelerSpec{}
	}

	return *in.Cluster.Spec.WebModeler
}

// workload returns the WorkloadSpec of a component, or the zero value when the
// spec sets no block for it.
func (in Input) workload(comp string) v1.WorkloadSpec {
	webModeler := in.Cluster.Spec.WebModeler

	switch comp {
	case ComponentIdentity:
		return in.Cluster.Spec.Identity.WorkloadSpec
	case ComponentConsole:
		if in.Cluster.Spec.Console != nil {
			return in.Cluster.Spec.Console.WorkloadSpec
		}
	case ComponentWebModelerRestapi:
		if webModeler != nil && webModeler.Restapi != nil {
			return *webModeler.Restapi
		}
	case ComponentWebModelerWebsockets:
		if webModeler != nil && webModeler.Websockets != nil {
			return *webModeler.Websockets
		}
	}

	return v1.WorkloadSpec{}
}

// replicas returns the replica count of a component. Every component defaults
// to 1.
func (in Input) replicas(comp string) int32 {
	if r := in.workload(comp).Replicas; r != nil {
		return *r
	}

	return 1
}
