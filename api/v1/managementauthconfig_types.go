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

// ManagementAuthConfigSpec carries the Management Identity OIDC configuration:
// endpoints, machine-to-machine client credentials, and audience.
type ManagementAuthConfigSpec struct {
	// BaseURL is the base URL of the Management Identity service.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https')",message="baseUrl must be a valid http or https URL"
	BaseURL string `json:"baseUrl"`
	// IssuerURL is the OIDC issuer URL used to validate tokens.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https')",message="issuerUrl must be a valid http or https URL"
	IssuerURL string `json:"issuerUrl"`
	// IssuerBackendURL is the issuer URL for in-cluster container-to-container
	// communication. Consumers default it to IssuerURL when empty.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https')",message="issuerBackendUrl must be a valid http or https URL"
	// +optional
	IssuerBackendURL string `json:"issuerBackendUrl,omitempty"`
	// AuthURL is the OIDC authorization endpoint used for browser login
	// redirects.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https')",message="authUrl must be a valid http or https URL"
	AuthURL string `json:"authUrl"`
	// TokenURL is the OIDC token endpoint used to acquire machine-to-machine
	// tokens.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https')",message="tokenUrl must be a valid http or https URL"
	TokenURL string `json:"tokenUrl"`
	// JwksURL is the JWKS endpoint used to fetch token signing keys.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https')",message="jwksUrl must be a valid http or https URL"
	JwksURL string `json:"jwksUrl"`
	// ClientID is the default machine-to-machine client ID.
	// +kubebuilder:validation:MinLength=1
	ClientID string `json:"clientId"`
	// Audience expected in access tokens issued for this client.
	// +kubebuilder:validation:MinLength=1
	Audience string `json:"audience"`
	// ClientSecretRef names the Secret key holding the client secret for the
	// machine-to-machine client.
	ClientSecretRef SecretKeyRef `json:"clientSecretRef"`
}

// ManagementAuthConfigStatus is the observed validation state of the contract.
type ManagementAuthConfigStatus struct {
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

// ManagementAuthConfig is the contract CRD that carries the Management
// Identity OIDC configuration — endpoints, client credentials, and audience —
// for components that live outside the orchestration cluster.
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

// GetConditions returns the resource's status conditions.
func (in *ManagementAuthConfig) GetConditions() []metav1.Condition { return in.Status.Conditions }

// GetObservedGeneration returns the last reconciled generation recorded in status.
func (in *ManagementAuthConfig) GetObservedGeneration() int64 { return in.Status.ObservedGeneration }

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
