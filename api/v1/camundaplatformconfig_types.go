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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AuthenticationMethod selects how users and clients authenticate against an
// orchestration cluster.
// +kubebuilder:validation:Enum=basic;oidc
type AuthenticationMethod string

const (
	// AuthenticationMethodBasic selects username and password authentication
	// against users that the orchestration cluster stores itself.
	AuthenticationMethodBasic AuthenticationMethod = "basic"
	// AuthenticationMethodOIDC selects an external OIDC identity provider.
	AuthenticationMethodOIDC AuthenticationMethod = "oidc"
)

// OIDCSpec is the identity provider connection of a platform config. The
// fields follow the OIDC discovery vocabulary and work with any OIDC-compliant
// provider.
type OIDCSpec struct {
	// IssuerURL is the issuer URL of the identity provider. Consumers resolve
	// the endpoints from its OIDC discovery document unless the explicit
	// endpoint fields override them.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https')",message="issuerUrl must be a valid http or https URL"
	IssuerURL string `json:"issuerUrl"`
	// ProviderType names the kind of identity provider. Management Identity
	// reads it and changes how it resolves users and groups. Empty means
	// generic, which fits any OIDC-compliant provider. Set microsoft for
	// Microsoft Entra ID.
	// +kubebuilder:validation:Enum=generic;microsoft
	// +optional
	ProviderType string `json:"providerType,omitempty"`
	// JWKSURL is an explicit JWKS endpoint. It overrides the value from OIDC
	// discovery.
	// +kubebuilder:validation:XValidation:rule="self == '' || (isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https'))",message="jwksUrl must be empty or a valid http or https URL"
	// +optional
	JWKSURL string `json:"jwksUrl,omitempty"`
	// TokenURL is an explicit token endpoint. It overrides the value from OIDC
	// discovery.
	// +kubebuilder:validation:XValidation:rule="self == '' || (isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https'))",message="tokenUrl must be empty or a valid http or https URL"
	// +optional
	TokenURL string `json:"tokenUrl,omitempty"`
	// AuthURL is an explicit authorization endpoint. It overrides the value
	// from OIDC discovery.
	// +kubebuilder:validation:XValidation:rule="self == '' || (isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https'))",message="authUrl must be empty or a valid http or https URL"
	// +optional
	AuthURL string `json:"authUrl,omitempty"`
	// ClientID is the default OIDC client ID that all clusters share unless a
	// preset or a cluster overrides it.
	// +kubebuilder:validation:MinLength=1
	ClientID string `json:"clientId"`
	// Audience is the audience that consumers validate in access tokens.
	// Consumers default it to ClientID when empty.
	// +optional
	Audience string `json:"audience,omitempty"`
	// UsernameClaim is the token claim that holds the username of a person.
	// Empty means the default of the orchestration cluster, which is "sub".
	// +optional
	UsernameClaim string `json:"usernameClaim,omitempty"`
	// ClientIDClaim is the token claim that holds the id of a machine client.
	// Empty means that no claim identifies a client, and every token becomes a
	// person. The claim must be absent from the tokens of persons, because a
	// token that carries it always becomes a client.
	// +optional
	ClientIDClaim string `json:"clientIdClaim,omitempty"`
	// ClientSecretRef names the Secret key that holds the default OIDC client
	// secret.
	ClientSecretRef SecretKeyRef `json:"clientSecretRef"`
	// Management holds the clients that the management plane uses at this
	// identity provider. A CamundaManagementCluster in the oidc mode reads
	// them. Register one client per component at the provider first.
	// +optional
	Management *ManagementOIDCClientsSpec `json:"management,omitempty"`
}

// ManagementOIDCClientsSpec holds the identity provider clients of the
// management plane.
type ManagementOIDCClientsSpec struct {
	// Clients holds one entry per component of the management plane.
	Clients ManagementClients `json:"clients"`
}

// ManagementClients names the identity provider client of each component of
// the management plane. A CamundaManagementCluster reports InvalidReference
// when a component it deploys has no client here.
type ManagementClients struct {
	// Identity is the client of Management Identity.
	// +optional
	Identity *ConfidentialClientSpec `json:"identity,omitempty"`
	// Optimize is the client of Optimize. The ManagementAuthConfig that the
	// management cluster writes carries it, and Optimize reads it from there.
	// +optional
	Optimize *ConfidentialClientSpec `json:"optimize,omitempty"`
	// WebModeler is the client of the Web Modeler user interface. The browser
	// holds no secret, so it is a public client.
	// +optional
	WebModeler *PublicClientSpec `json:"webModeler,omitempty"`
	// WebModelerAPI is the client of the Web Modeler API.
	// +optional
	WebModelerAPI *WebModelerAPIClientSpec `json:"webModelerApi,omitempty"`
	// Console is the client of Console. The browser holds no secret, so it is
	// a public client.
	// +optional
	Console *PublicClientSpec `json:"console,omitempty"`
}

// ConfidentialClientSpec is an identity provider client that authenticates
// with a secret.
type ConfidentialClientSpec struct {
	// ClientID is the client id at the identity provider.
	// +kubebuilder:validation:MinLength=1
	ClientID string `json:"clientId"`
	// Audience is the audience that the component validates in access tokens.
	// Empty means the client id.
	// +optional
	Audience string `json:"audience,omitempty"`
	// ClientSecretRef names the Secret key that holds the client secret.
	ClientSecretRef SecretKeyRef `json:"clientSecretRef"`
}

// PublicClientSpec is an identity provider client that a browser uses, so it
// has no secret.
type PublicClientSpec struct {
	// ClientID is the client id at the identity provider.
	// +kubebuilder:validation:MinLength=1
	ClientID string `json:"clientId"`
	// Audience is the audience that the component validates in access tokens.
	// Empty means the client id.
	// +optional
	Audience string `json:"audience,omitempty"`
}

// WebModelerAPIClientSpec is the client of the Web Modeler API. Web Modeler
// validates two audiences: one for the API that its own user interface calls,
// and one for the public API that your applications call.
type WebModelerAPIClientSpec struct {
	ConfidentialClientSpec `json:",inline"`
	// PublicAPIAudience is the audience of the Web Modeler public API. Empty
	// means web-modeler-public-api.
	// +optional
	PublicAPIAudience string `json:"publicApiAudience,omitempty"`
}

// PlatformAuthSpec selects the authentication method of every orchestration
// cluster that references the platform config.
// +kubebuilder:validation:XValidation:rule="(self.method == 'oidc') == has(self.oidc)",message="oidc is required when method is oidc and must not be set when method is basic"
type PlatformAuthSpec struct {
	// Method is the authentication method. An unset auth block and an unset
	// method both mean basic.
	// +kubebuilder:default=basic
	// +optional
	Method AuthenticationMethod `json:"method,omitempty"`
	// OIDC is the identity provider connection. Required when method is oidc,
	// forbidden otherwise.
	// +optional
	OIDC *OIDCSpec `json:"oidc,omitempty"`
}

// CamundaPlatformConfigSpec holds the settings that are identical across all
// orchestration clusters of an environment.
type CamundaPlatformConfigSpec struct {
	// Auth holds the authentication settings of the orchestration clusters.
	// Unset means basic authentication.
	// +optional
	Auth *PlatformAuthSpec `json:"auth,omitempty"`
	// LicenseSecretRef names the Secret key that holds the Camunda license
	// key. Without it, clusters run in unlicensed non-production mode.
	// +optional
	LicenseSecretRef *SecretKeyRef `json:"licenseSecretRef,omitempty"`
	// ImageRegistry is the registry prefix of all Camunda component images.
	// Empty means the upstream Camunda registry.
	// +optional
	ImageRegistry string `json:"imageRegistry,omitempty"`
	// Images renames one image, for example to a mirror that keeps a
	// different repository path. The tag always comes from the version field
	// of the resource that runs the image. An entry here replaces both the
	// repository and the imageRegistry prefix for that one image.
	// +optional
	Images *ImagesSpec `json:"images,omitempty"`
}

// ImagesSpec renames the container images that the operator pulls. Each field
// holds a full repository, for example mirror.example.com/camunda/optimize.
// An empty field means the default repository of that image.
type ImagesSpec struct {
	// Camunda is the image of the orchestration cluster processes. Defaults
	// to camunda/camunda.
	// +optional
	Camunda string `json:"camunda,omitempty"`
	// Connectors is the image of the connectors runtime. Defaults to
	// camunda/connectors-bundle.
	// +optional
	Connectors string `json:"connectors,omitempty"`
	// Optimize is the image of Optimize. Defaults to camunda/optimize.
	// +optional
	Optimize string `json:"optimize,omitempty"`
	// Identity is the image of Management Identity. Defaults to
	// camunda/identity.
	// +optional
	Identity string `json:"identity,omitempty"`
	// Console is the image of Console. Defaults to camunda/console.
	// +optional
	Console string `json:"console,omitempty"`
	// WebModelerRestapi is the image of the Web Modeler restapi process.
	// Defaults to camunda/web-modeler-restapi.
	// +optional
	WebModelerRestapi string `json:"webModelerRestapi,omitempty"`
	// WebModelerWebsockets is the image of the Web Modeler websockets
	// process. Defaults to camunda/web-modeler-websockets.
	// +optional
	WebModelerWebsockets string `json:"webModelerWebsockets,omitempty"`
	// Keycloak is the image of the Keycloak that the operator runs. Defaults
	// to camunda/keycloak.
	// +optional
	Keycloak string `json:"keycloak,omitempty"`
}

// Method returns the effective authentication method: basic when auth or its
// method is unset.
func (in *CamundaPlatformConfigSpec) Method() AuthenticationMethod {
	if in.Auth == nil || in.Auth.Method == "" {
		return AuthenticationMethodBasic
	}
	return in.Auth.Method
}

// CamundaPlatformConfigStatus is the observed validation state of the
// platform config.
type CamundaPlatformConfigStatus struct {
	// ObservedGeneration is the last generation reconciled by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the current validation state; the Ready condition
	// carries reasons Healthy or MissingSecret.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// CamundaPlatformConfig is the cluster-scoped CRD that holds the
// environment-wide platform settings — identity provider, license, and image
// registry — that every orchestration cluster referencing it shares.
type CamundaPlatformConfig struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of CamundaPlatformConfig
	// +required
	Spec CamundaPlatformConfigSpec `json:"spec"`

	// status defines the observed state of CamundaPlatformConfig
	// +optional
	Status CamundaPlatformConfigStatus `json:"status,omitzero"`
}

// GetStatusConditions returns a pointer to the status conditions. The
// component framework stages conditions on the resource through it.
func (in *CamundaPlatformConfig) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// GetKind returns the CRD kind. The component framework uses it for event and
// metric recording.
func (in *CamundaPlatformConfig) GetKind() string { return "CamundaPlatformConfig" }

// SetObservedGeneration records the last reconciled generation in status.
func (in *CamundaPlatformConfig) SetObservedGeneration(generation int64) {
	in.Status.ObservedGeneration = generation
}

// +kubebuilder:object:root=true

// CamundaPlatformConfigList contains a list of CamundaPlatformConfig
type CamundaPlatformConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []CamundaPlatformConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CamundaPlatformConfig{}, &CamundaPlatformConfigList{})
}
