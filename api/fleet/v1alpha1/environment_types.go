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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// EnvironmentSpec is the policy that an Environment sets for the claims in
// its namespaces. A namespace belongs to the Environment that its label
// fleet.camunda.io/environment names.
// +kubebuilder:validation:XValidation:rule="!has(self.releases) || !has(self.releases.default) || size(self.releases.default) == 0 || (has(self.releases.allowed) && self.releases.default in self.releases.allowed)",message="releases.default must be one of releases.allowed"
type EnvironmentSpec struct {
	// DisplayName is the name that the fleet manager shows for the
	// environment, at most 64 characters. It changes nothing for a claim.
	// +kubebuilder:validation:MaxLength=64
	// +optional
	DisplayName string `json:"displayName,omitempty"`
	// Color is the color that the fleet manager shows for the environment,
	// as a hex color such as #2E7D32. It changes nothing for a claim.
	// +kubebuilder:validation:Pattern=`^#[0-9a-fA-F]{6}$`
	// +optional
	Color string `json:"color,omitempty"`
	// PlatformConfigRef names the CamundaPlatformConfig of a claim of the
	// environment that names none itself.
	// +optional
	PlatformConfigRef string `json:"platformConfigRef,omitempty"`
	// Presets lists, for each preset kind, the presets that a claim of the
	// environment can use. The claim controller checks a new claim, and a
	// preset change of an existing claim, against the current lists.
	// +optional
	Presets EnvironmentPresets `json:"presets,omitzero"`
	// Releases lists the CamundaReleases that a claim of the environment can
	// use. The claim controller checks a new claim, and a release change of
	// an existing claim, against the current list.
	// +optional
	Releases *EnvironmentReleases `json:"releases,omitempty"`
	// Sections holds the policy of each PluginSection, by section name. A
	// claim of the environment can select only a registration that the
	// policy of its section allows. A section that is not listed offers
	// only the empty option. A change applies to new claims only.
	// +kubebuilder:validation:MaxProperties=64
	// +kubebuilder:validation:XValidation:rule="self.all(k, k.size() > 0)",message="sections cannot have an empty section name"
	// +optional
	Sections map[string]SectionPolicy `json:"sections,omitempty"`
	// Parameters holds the policy of the parameters of each
	// PluginRegistration, by registration name and then by parameter name.
	// A parameter without a policy keeps the value of its registration, and
	// a claim cannot change it. A change applies to new claims only.
	// +kubebuilder:validation:MaxProperties=64
	// +kubebuilder:validation:XValidation:rule="self.all(k, k.size() > 0)",message="parameters cannot have an empty registration name"
	// +optional
	Parameters map[string]ParameterPolicies `json:"parameters,omitempty"`
	// DeletionPolicy is the deletion policy of a claim of the environment
	// that sets none itself. Delete deletes the resources of its plugin
	// requests with the claim. Orphan keeps them. Unset means Delete. A
	// change applies to new claims only.
	// +kubebuilder:default=Delete
	// +optional
	DeletionPolicy pluginsv1alpha1.DeletionPolicy `json:"deletionPolicy,omitempty"`
	// NamespaceCreation lets the create flow of the fleet manager create a
	// namespace in the environment. Unset, the create flow cannot create a
	// namespace.
	// +optional
	NamespaceCreation *NamespaceCreation `json:"namespaceCreation,omitempty"`
}

// EnvironmentPresets lists the presets that the claims of an environment can
// use, for each preset kind.
type EnvironmentPresets struct {
	// CamundaCluster are the names of the CamundaClusterPresets that a
	// cluster claim can use.
	// +listType=set
	// +kubebuilder:validation:items:MinLength=1
	// +optional
	CamundaCluster []string `json:"camundaCluster,omitempty"`
	// ElasticsearchCluster are the names of the ElasticsearchClusterPresets
	// that the built-in Elasticsearch of a claim can use.
	// +listType=set
	// +kubebuilder:validation:items:MinLength=1
	// +optional
	ElasticsearchCluster []string `json:"elasticsearchCluster,omitempty"`
	// DatabaseServer are the names of the DatabaseServerPresets that the
	// built-in PostgreSQL of a claim can use.
	// +listType=set
	// +kubebuilder:validation:items:MinLength=1
	// +optional
	DatabaseServer []string `json:"databaseServer,omitempty"`
}

// EnvironmentReleases lists the releases that the claims of an environment
// can use.
type EnvironmentReleases struct {
	// Allowed are the names of the CamundaReleases that a claim can use.
	// +listType=set
	// +kubebuilder:validation:MaxItems=64
	// +kubebuilder:validation:items:MinLength=1
	// +kubebuilder:validation:items:MaxLength=253
	// +optional
	Allowed []string `json:"allowed,omitempty"`
	// Default is the release of a claim that names none. It is one of
	// allowed.
	// +kubebuilder:validation:MaxLength=253
	// +optional
	Default string `json:"default,omitempty"`
}

// NamespaceCreation controls how the create flow of the fleet manager creates
// a namespace in an environment. The create flow always sets the label
// fleet.camunda.io/environment on the new namespace.
// +kubebuilder:validation:XValidation:rule="!has(self.labels) || !('fleet.camunda.io/environment' in self.labels)",message="the environment label is set by the app"
// +kubebuilder:validation:XValidation:rule="!has(self.requiredLabelKeys) || !('fleet.camunda.io/environment' in self.requiredLabelKeys)",message="the environment label is set by the app"
type NamespaceCreation struct {
	// Allowed lets the create flow create a namespace in the environment.
	// +optional
	Allowed bool `json:"allowed,omitempty"`
	// Labels are fixed labels of a new namespace. Each is a valid label. The
	// label fleet.camunda.io/environment is not allowed here, because the
	// create flow sets it.
	// +kubebuilder:validation:MaxProperties=32
	// +kubebuilder:validation:XValidation:rule="self.all(k, !format.qualifiedName().validate(k).hasValue())",message="namespaceCreation.labels must have valid label keys"
	// +optional
	Labels map[string]LabelValue `json:"labels,omitempty"`
	// RequiredLabelKeys are label keys, for example team. The create flow
	// asks for a value of each key and sets the label on the new namespace.
	// Each key is a valid label key. The key fleet.camunda.io/environment is
	// not allowed here.
	// +listType=set
	// +kubebuilder:validation:MaxItems=16
	// +kubebuilder:validation:items:MaxLength=317
	// +kubebuilder:validation:XValidation:rule="self.all(k, !format.qualifiedName().validate(k).hasValue())",message="namespaceCreation.requiredLabelKeys must be valid label keys"
	// +optional
	RequiredLabelKeys []string `json:"requiredLabelKeys,omitempty"`
}

// LabelValue is the value of a Kubernetes label.
// +kubebuilder:validation:MaxLength=63
// +kubebuilder:validation:Pattern=`^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$`
type LabelValue string

// EnvironmentStatus is the state of an Environment. The environment
// controller writes it.
type EnvironmentStatus struct {
	// ObservedGeneration is the last generation that the environment
	// controller processed.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions report the state of the environment. Ready is False with
	// reason InvalidReference when a release, preset, platform config,
	// section, or registration that the spec names does not exist. The
	// message names each missing object.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Namespaces is the number of namespaces that carry the label of this
	// environment. It is unset until the environment controller counts the
	// namespaces.
	// +optional
	Namespaces *int32 `json:"namespaces,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Namespaces",type=integer,JSONPath=`.status.namespaces`
// +kubebuilder:printcolumn:name="Display name",type=string,JSONPath=`.spec.displayName`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="self.metadata.name.size() <= 63",message="metadata.name must be at most 63 characters, because it is used as a label value"

// Environment sets the policy for the claims in its namespaces. The policy
// names the releases, presets, and plugin registrations that a claim can use.
// It also sets the parameter values and the deletion default. A namespace belongs to the Environment
// that its label fleet.camunda.io/environment names. Without any
// Environment, the modes and the parameter values of the registrations
// apply. The name is at most 63 characters, because other objects carry it
// in a label value.
type Environment struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec is the policy of the environment
	// +required
	Spec EnvironmentSpec `json:"spec"`

	// status is the state of the environment
	// +optional
	Status EnvironmentStatus `json:"status,omitzero"`
}

// GetStatusConditions returns a pointer to the status conditions. The
// component framework stages conditions on the resource through it.
func (in *Environment) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// GetKind returns the CRD kind. The component framework uses it for event and
// metric recording.
func (in *Environment) GetKind() string { return "Environment" }

// SetObservedGeneration records the last reconciled generation in status.
func (in *Environment) SetObservedGeneration(generation int64) {
	in.Status.ObservedGeneration = generation
}

// +kubebuilder:object:root=true

// EnvironmentList contains a list of Environment
type EnvironmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Environment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Environment{}, &EnvironmentList{})
}
