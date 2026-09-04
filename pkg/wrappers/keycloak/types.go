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

package keycloak

// This file mirrors the upstream Keycloak API of the Keycloak Operator, in
// the order upstream declares it. The file order rule of how-we-write-go does
// not apply to it.

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is the group and version of the Keycloak custom resource.
	GroupVersion = schema.GroupVersion{Group: "k8s.keycloak.org", Version: "v2alpha1"}

	// SchemeBuilder registers the Keycloak types. A manager and a test suite
	// that read or write the kind add them to their scheme through
	// AddToScheme.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme adds the Keycloak types to a scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func addKnownTypes(s *runtime.Scheme) error {
	s.AddKnownTypes(GroupVersion, &Keycloak{}, &KeycloakList{})
	metav1.AddToGroupVersion(s, GroupVersion)

	return nil
}

// The condition types that the Keycloak Operator reports.
const (
	// ConditionReady holds while the Keycloak instances serve requests.
	ConditionReady = "Ready"
	// ConditionHasErrors holds while the Keycloak Operator cannot reconcile
	// the resource, for example when the database is unreachable.
	ConditionHasErrors = "HasErrors"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Keycloak is one Keycloak instance run by the Keycloak Operator.
// +kubebuilder:object:generate=true
type Keycloak struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec KeycloakSpec `json:"spec,omitempty"`
	// +optional
	Status KeycloakStatus `json:"status,omitempty"`
}

// KeycloakSpec is the desired state of a Keycloak. It holds the fields that
// this operator sets, not every field the Keycloak Operator accepts.
// +kubebuilder:object:generate=true
type KeycloakSpec struct {
	// Instances is the number of Keycloak pods. The Keycloak Operator
	// defaults it to 1.
	// +optional
	Instances *int32 `json:"instances,omitempty"`
	// Image is the Keycloak container image. An image that is already
	// optimized, such as the Camunda build, starts without an augmentation
	// step.
	// +optional
	Image string `json:"image,omitempty"`
	// DB is the database connection.
	// +optional
	DB *KeycloakDBSpec `json:"db,omitempty"`
	// HTTP configures the HTTP listener and the Service in front of it.
	// +optional
	HTTP *KeycloakHTTPSpec `json:"http,omitempty"`
	// Hostname is the address that Keycloak builds its URLs from.
	// +optional
	Hostname *KeycloakHostnameSpec `json:"hostname,omitempty"`
	// Ingress configures the Ingress that the Keycloak Operator creates by
	// default.
	// +optional
	Ingress *KeycloakIngressSpec `json:"ingress,omitempty"`
	// Proxy configures the reverse proxy that stands in front of Keycloak.
	// +optional
	Proxy *KeycloakProxySpec `json:"proxy,omitempty"`
	// Scheduling carries the scheduling constraints of the Keycloak pods.
	// +optional
	Scheduling *KeycloakSchedulingSpec `json:"scheduling,omitempty"`
	// AdditionalOptions are Keycloak server options that have no field of
	// their own, as the keys of https://www.keycloak.org/server/all-config.
	// The Keycloak Operator reports an option that does have a field of its
	// own as a warning on the resource, so a first-class field always wins.
	// +optional
	AdditionalOptions []KeycloakValueOrSecret `json:"additionalOptions,omitempty"`
	// Unsupported carries the pod template that the Keycloak Operator merges
	// into the one it builds. It is the only way to put labels and other pod
	// settings on the Keycloak pods.
	// +optional
	Unsupported *KeycloakUnsupportedSpec `json:"unsupported,omitempty"`
	// Resources are the CPU and memory of the Keycloak container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// KeycloakDBSpec is the database connection of a Keycloak.
// +kubebuilder:object:generate=true
type KeycloakDBSpec struct {
	// Vendor is the database vendor, for example postgres. Keycloak ignores
	// it when URL is set.
	// +optional
	Vendor string `json:"vendor,omitempty"`
	// URL is the whole JDBC URL. It replaces Vendor, Host, Port, and
	// Database, which Keycloak only uses to build a default URL.
	// +optional
	URL string `json:"url,omitempty"`
	// Schema is the database schema that Keycloak opens.
	// +optional
	Schema string `json:"schema,omitempty"`
	// Host is the database host of the JDBC URL that Keycloak builds.
	// +optional
	Host string `json:"host,omitempty"`
	// Port is the database port of the JDBC URL that Keycloak builds.
	// +optional
	Port *int32 `json:"port,omitempty"`
	// Database is the database name of the JDBC URL that Keycloak builds.
	// +optional
	Database string `json:"database,omitempty"`
	// UsernameSecret names the Secret key that holds the database user.
	// +optional
	UsernameSecret *corev1.SecretKeySelector `json:"usernameSecret,omitempty"`
	// PasswordSecret names the Secret key that holds the database password.
	// +optional
	PasswordSecret *corev1.SecretKeySelector `json:"passwordSecret,omitempty"`
}

// KeycloakHTTPSpec configures the HTTP listener of a Keycloak.
// +kubebuilder:object:generate=true
type KeycloakHTTPSpec struct {
	// HTTPEnabled turns the plain HTTP listener on. TLS between the ingress
	// and Keycloak is out of scope of this operator.
	// +optional
	HTTPEnabled *bool `json:"httpEnabled,omitempty"`
	// HTTPPort is the port of the plain HTTP listener.
	// +optional
	HTTPPort *int32 `json:"httpPort,omitempty"`
}

// KeycloakHostnameSpec is the address that Keycloak builds its URLs from.
// +kubebuilder:object:generate=true
type KeycloakHostnameSpec struct {
	// Hostname is the URL that browsers reach Keycloak at, including the path.
	// +optional
	Hostname string `json:"hostname,omitempty"`
	// Strict stops Keycloak from resolving the hostname from request headers.
	// +optional
	Strict *bool `json:"strict,omitempty"`
}

// KeycloakIngressSpec configures the Ingress of a Keycloak.
// +kubebuilder:object:generate=true
type KeycloakIngressSpec struct {
	// Enabled turns the Ingress of the Keycloak Operator on. The route to a
	// Keycloak that this operator runs is yours, so it is off.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// KeycloakProxySpec configures the reverse proxy settings of a Keycloak.
// +kubebuilder:object:generate=true
type KeycloakProxySpec struct {
	// Headers is the proxy header set that Keycloak reads the scheme and the
	// host of a request from: forwarded or xforwarded. An unset value makes
	// Keycloak ignore both, so the URLs it builds carry the address it
	// listens on rather than the one the browser used.
	// +optional
	Headers string `json:"headers,omitempty"`
}

// KeycloakSchedulingSpec is the scheduling block of a Keycloak. It declares
// the fields the operator sets; the custom resource also offers
// priorityClassName and topologySpreadConstraints.
// +kubebuilder:object:generate=true
type KeycloakSchedulingSpec struct {
	// Affinity of the Keycloak pods.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
	// Tolerations of the Keycloak pods.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// KeycloakValueOrSecret is one Keycloak server option. It carries a literal
// value or a Secret key, never both.
// +kubebuilder:object:generate=true
type KeycloakValueOrSecret struct {
	// Name is the option key.
	Name string `json:"name"`
	// Value is the literal value of the option.
	// +optional
	Value string `json:"value,omitempty"`
	// Secret names the Secret key that holds the value of the option.
	// +optional
	Secret *corev1.SecretKeySelector `json:"secret,omitempty"`
}

// KeycloakUnsupportedSpec carries the pod template of a Keycloak.
// +kubebuilder:object:generate=true
type KeycloakUnsupportedSpec struct {
	// PodTemplate is merged into the pod template that the Keycloak Operator
	// builds.
	// +optional
	PodTemplate *corev1.PodTemplateSpec `json:"podTemplate,omitempty"`
}

// KeycloakStatus is the observed state of a Keycloak.
// +kubebuilder:object:generate=true
type KeycloakStatus struct {
	// Conditions are the conditions that the Keycloak Operator reports. Their
	// status is the string "True", "False", or "Unknown", not a boolean.
	// +optional
	Conditions []KeycloakCondition `json:"conditions,omitempty"`
	// Instances is the number of ready Keycloak pods.
	// +optional
	Instances int32 `json:"instances,omitempty"`
	// ObservedGeneration is the last generation that the Keycloak Operator
	// reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Selector is the label selector of the Keycloak pods.
	// +optional
	Selector string `json:"selector,omitempty"`
}

// KeycloakCondition is one condition of a Keycloak.
// +kubebuilder:object:generate=true
type KeycloakCondition struct {
	// Type is the condition type, for example Ready.
	// +optional
	Type string `json:"type,omitempty"`
	// Status is "True", "False", or "Unknown".
	// +optional
	Status string `json:"status,omitempty"`
	// Message explains the status.
	// +optional
	Message string `json:"message,omitempty"`
	// ObservedGeneration is the generation this condition was computed from.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// LastTransitionTime is when the status last changed. The Keycloak
	// Operator writes it as a string, not as a Kubernetes timestamp.
	// +optional
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
}

// +kubebuilder:object:root=true

// KeycloakList is a list of Keycloak.
// +kubebuilder:object:generate=true
type KeycloakList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Keycloak `json:"items"`
}
