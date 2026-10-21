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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PluginRegistrationSpec describes one option that a plugin offers in one or
// more sections: what it needs, what it writes, and which permissions it asks
// for.
// +kubebuilder:validation:XValidation:rule="!has(self.grantTemplates) || self.grantTemplates.all(t, !has(t.requiresInputs) || t.requiresInputs.all(r, has(self.inputs) && self.inputs.exists(i, i.name == r)))",message="each requiresInputs item of a grant template must name an input of the registration"
type PluginRegistrationSpec struct {
	// Sections lists the names of the PluginSections that the option serves.
	// +kubebuilder:validation:XValidation:rule="self.size() >= 1",message="sections must list at least one section"
	Sections []string `json:"sections"`
	// Label is the name of the option in the fleet manager. It names the
	// outcome, for example "New S3 bucket". Display only. No controller reads
	// it.
	// +kubebuilder:validation:MinLength=1
	Label string `json:"label"`
	// Description says what the option does. Display only. No controller
	// reads it.
	// +optional
	Description string `json:"description,omitempty"`
	// DocsURL is the address of the documentation of the option. Display
	// only. No controller reads it.
	// +optional
	DocsURL string `json:"docsUrl,omitempty"`
	// Mode says whether a new claim selects the option when no Environment
	// exists.
	Mode Mode `json:"mode"`
	// Controller names the plugin controller that serves the option and its
	// version. Display only. No controller reads it.
	// +optional
	Controller ControllerInfo `json:"controller,omitzero"`
	// ServiceAccount is the ServiceAccount of the plugin controller. The app
	// shows it. The API server admits a Grant for the option only from a
	// requester that has the RBAC verb serve on this registration.
	ServiceAccount ServiceAccountRef `json:"serviceAccount"`
	// OperatorVersions is the semver range of the operator versions that the
	// plugin supports, for example ">=0.3.0 <1.0.0". The operator reports a
	// range that does not hold its version, or that is not a valid range.
	// +optional
	OperatorVersions string `json:"operatorVersions,omitempty"`
	// Parameters is the OpenAPI v3 schema of the parameters of a request for
	// the option.
	// +optional
	Parameters *apiextensionsv1.JSON `json:"parameters,omitempty"`
	// ParameterValues are the parameter values that the admin set in the
	// plugin chart. An Environment can override them.
	// +optional
	ParameterValues *apiextensionsv1.JSON `json:"parameterValues,omitempty"`
	// ClaimEditable lists the parameters that a claim can change when no
	// Environment says otherwise.
	// +optional
	ClaimEditable []string `json:"claimEditable,omitempty"`
	// Choices are the presets or existing resources that a claim can pick
	// for a parameter. The claim stores the name of the pick in its
	// parameters. Display only. No controller reads it.
	// +optional
	Choices []Choice `json:"choices,omitempty"`
	// Inputs declares the values that the option reads from the outputs of
	// other sections. Each name is unique.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=32
	// +optional
	Inputs []InputDecl `json:"inputs,omitempty"`
	// Outputs declares the values that the plugin writes to the status of a
	// request. It includes the outputs of each section that the option
	// serves. Each key is unique.
	// +listType=map
	// +listMapKey=key
	// +optional
	Outputs []OutputDecl `json:"outputs,omitempty"`
	// Limits are the values that the plugin chart fixes, which no
	// Environment and no claim can change, for example a permissions
	// boundary. The plugin enforces them from its chart. Display only. No
	// controller reads it.
	// +optional
	Limits []Limit `json:"limits,omitempty"`
	// GrantTemplates are the permission statements that the option asks for,
	// one Grant for each template. Each name is unique.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=32
	// +optional
	GrantTemplates []GrantTemplate `json:"grantTemplates,omitempty"`
	// DeletionPolicies lists the deletion policies that the plugin supports.
	// Every plugin supports Delete.
	// +listType=set
	// +kubebuilder:validation:MaxItems=2
	// +kubebuilder:validation:XValidation:rule="self.exists(p, p == 'Delete')",message="deletionPolicies must contain Delete"
	DeletionPolicies []DeletionPolicy `json:"deletionPolicies"`
	// MetricQueries are the Prometheus queries that the fleet manager shows
	// for a request of the option. Display only. No controller reads it.
	// +optional
	MetricQueries []MetricQuery `json:"metricQueries,omitempty"`
}

// Limit is a value that the plugin chart fixes, which no Environment and no
// claim can change.
type Limit struct {
	// Label names the limit.
	// +kubebuilder:validation:MinLength=1
	Label string `json:"label"`
	// Value is the value of the limit.
	Value string `json:"value"`
	// Description says what the limit means for a claim.
	// +optional
	Description string `json:"description,omitempty"`
}

// ServiceAccountRef names a ServiceAccount in a named namespace.
type ServiceAccountRef struct {
	// Namespace is the namespace of the ServiceAccount.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	Namespace string `json:"namespace"`
	// Name is the name of the ServiceAccount.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	Name string `json:"name"`
}

// PluginRegistrationStatus is the observed state of a PluginRegistration.
type PluginRegistrationStatus struct {
	// ObservedGeneration is the last generation of the registration that the
	// plugin controller processed. Only the plugin controller sets it, so the
	// field is absent until the plugin controller processes the registration.
	// The operator is the plugin controller of the built-in registrations
	// whose requests it serves.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions holds Accepted and Ready. The operator sets Accepted. Its
	// False reasons are SectionMissing, OperatorVersionUnsupported, and
	// DefaultConflict. The plugin controller sets Ready from its health and
	// its credentials. On the built-in registrations whose requests the
	// operator serves, the operator sets Ready. The fleet manager offers the
	// option only when both are True.
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
// +kubebuilder:printcolumn:name="Accepted",type=string,JSONPath=`.status.conditions[?(@.type=="Accepted")].status`
// +kubebuilder:printcolumn:name="Sections",type=string,JSONPath=`.spec.sections`,priority=1
// +kubebuilder:printcolumn:name="Mode",type=string,JSONPath=`.spec.mode`,priority=1
// +kubebuilder:printcolumn:name="Controller",type=string,JSONPath=`.spec.controller.name`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="self.metadata.name.size() <= 63",message="metadata.name must be at most 63 characters, because it is used as a label value"

// PluginRegistration is one option that a plugin offers to the sections of
// the create flow of a claim. The plugin controller writes it at startup from
// its chart values, and the operator writes the built-in registrations. The
// name is at most 63 characters, because other objects carry it in a label
// value.
type PluginRegistration struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the option
	// +required
	Spec PluginRegistrationSpec `json:"spec"`

	// status defines the observed state of PluginRegistration
	// +optional
	Status PluginRegistrationStatus `json:"status,omitzero"`
}

// GetStatusConditions returns a pointer to the status conditions. The
// component framework stages conditions on the resource through it.
func (in *PluginRegistration) GetStatusConditions() *[]metav1.Condition { return &in.Status.Conditions }

// GetKind returns the CRD kind. The component framework uses it for event and
// metric recording.
func (in *PluginRegistration) GetKind() string { return "PluginRegistration" }

// SetObservedGeneration records the last reconciled generation in status.
func (in *PluginRegistration) SetObservedGeneration(generation int64) {
	in.Status.ObservedGeneration = generation
}

// +kubebuilder:object:root=true

// PluginRegistrationList contains a list of PluginRegistration
type PluginRegistrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []PluginRegistration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PluginRegistration{}, &PluginRegistrationList{})
}
