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

// ManagementAuthConfigSpec holds the Management Identity OIDC configuration:
// the endpoints, the machine-to-machine client credentials, and the audience.
type ManagementAuthConfigSpec struct {
	// BaseURL is the base URL of the Management Identity service.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https')",message="baseUrl must be a valid http or https URL"
	BaseURL string `json:"baseUrl"`
	// IssuerURL is the OIDC issuer URL for the validation of tokens.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https')",message="issuerUrl must be a valid http or https URL"
	IssuerURL string `json:"issuerUrl"`
	// IssuerBackendURL is the issuer URL for calls between containers inside
	// the Kubernetes cluster. When empty, consumers use IssuerURL.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https')",message="issuerBackendUrl must be a valid http or https URL"
	// +optional
	IssuerBackendURL string `json:"issuerBackendUrl,omitempty"`
	// AuthURL is the OIDC authorization endpoint for the login redirects of
	// the browser.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https')",message="authUrl must be a valid http or https URL"
	AuthURL string `json:"authUrl"`
	// TokenURL is the OIDC token endpoint for machine-to-machine tokens.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https')",message="tokenUrl must be a valid http or https URL"
	TokenURL string `json:"tokenUrl"`
	// JwksURL is the JWKS endpoint that gives the token signing keys.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https')",message="jwksUrl must be a valid http or https URL"
	JwksURL string `json:"jwksUrl"`
	// ClientID is the ID of the client that Optimize signs in with.
	// +kubebuilder:validation:MinLength=1
	ClientID string `json:"clientId"`
	// Audience is the audience that access tokens for this client must have.
	// +kubebuilder:validation:MinLength=1
	Audience string `json:"audience"`
	// ClientSecretRef names the Secret key that holds the secret of that
	// client.
	ClientSecretRef SecretKeyRef `json:"clientSecretRef"`
}

// ManagementAuthConfigStatus is the observed validation state of the contract.
type ManagementAuthConfigStatus struct {
	// ObservedGeneration is the last generation that the operator processed.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the current validation state. The Ready condition
	// has the reasons Healthy or MissingSecret.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ManagementAuthConfig is the contract CRD that holds the Management Identity
// OIDC configuration: the endpoints, the client credentials, and the
// audience. Components outside the orchestration cluster, such as Optimize,
// read it.
type ManagementAuthConfig struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ManagementAuthConfig
	// +required
	Spec ManagementAuthConfigSpec `json:"spec"`

	// status defines the observed state of ManagementAuthConfig
	// +optional
	Status ManagementAuthConfigStatus `json:"status,omitzero"`
}

// GetStatusConditions returns a pointer to the status conditions. The
// component framework stages conditions on the resource through it.
func (in *ManagementAuthConfig) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// GetKind returns the CRD kind. The component framework uses it for event and
// metric recording.
func (in *ManagementAuthConfig) GetKind() string { return "ManagementAuthConfig" }

// SetObservedGeneration records the last reconciled generation in status.
func (in *ManagementAuthConfig) SetObservedGeneration(generation int64) {
	in.Status.ObservedGeneration = generation
}

// +kubebuilder:object:root=true

// ManagementAuthConfigList contains a list of ManagementAuthConfig
type ManagementAuthConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ManagementAuthConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ManagementAuthConfig{}, &ManagementAuthConfigList{})
}
