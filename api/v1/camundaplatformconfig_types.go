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
	// ClientSecretRef names the Secret key that holds the default OIDC client
	// secret.
	ClientSecretRef SecretKeyRef `json:"clientSecretRef"`
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
