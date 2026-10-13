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

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// The labels, the annotation, the finalizer, and the field managers of the
// fleet group.
const (
	// LabelEnvironment on a Namespace names the Environment that the
	// namespace belongs to.
	LabelEnvironment = "fleet.camunda.io/environment"
	// LabelClaim names the claim that a generated resource or a
	// PluginRequest belongs to.
	LabelClaim = "fleet.camunda.io/claim"
	// AnnotationRevision holds the content hash of an object that the fleet
	// manager pushes to Git.
	AnnotationRevision = "fleet.camunda.io/revision"
	// FinalizerCleanup is the finalizer that a claim controller holds on its
	// claim until the PluginRequests of the claim are gone.
	FinalizerCleanup = "fleet.camunda.io/cleanup"

	// FieldManagerPrefixClusterClaim starts the field manager of every field
	// that the cluster claim controller applies. ocf sets the field manager
	// <Kind>/<component>/<owner UID>, so a field whose manager starts with
	// this prefix belongs to the claim.
	FieldManagerPrefixClusterClaim = "CamundaClusterClaim/"
	// FieldManagerPrefixPlaneClaim starts the field manager of every field
	// that the management cluster claim controller applies.
	FieldManagerPrefixPlaneClaim = "CamundaManagementClusterClaim/"
	// FieldManagerApp is the field manager of every object that the fleet
	// manager writes.
	FieldManagerApp = "fleet-manager"
)

// SectionPolicy is the policy of an Environment for one PluginSection.
// +kubebuilder:validation:XValidation:rule="!has(self.default) || size(self.default) == 0 || (has(self.allowed) && self.default in self.allowed)",message="default must be one of allowed"
// +kubebuilder:validation:XValidation:rule="!has(self.required) || !self.required || (has(self.allowed) && size(self.allowed) > 0)",message="required needs at least one registration in allowed"
type SectionPolicy struct {
	// Required makes a claim of the environment select a registration of the
	// section. The fleet manager then hides the empty option.
	// +optional
	Required bool `json:"required,omitempty"`
	// Default is the registration that the create flow of the fleet manager
	// preselects. It is empty or one of allowed.
	// +kubebuilder:validation:MaxLength=253
	// +optional
	Default string `json:"default,omitempty"`
	// Allowed are the names of the PluginRegistrations of the section that a
	// claim of the environment can select. With no item, a claim can select
	// only the empty option.
	// +listType=set
	// +kubebuilder:validation:MaxItems=64
	// +kubebuilder:validation:items:MaxLength=253
	// +optional
	Allowed []string `json:"allowed,omitempty"`
}

// ParameterPolicies holds the policy of each parameter of one
// PluginRegistration, by parameter name.
// +kubebuilder:validation:MaxProperties=64
type ParameterPolicies map[string]ParameterPolicy

// ParameterPolicy is the policy of an Environment for one parameter of a
// PluginRegistration.
// +kubebuilder:validation:XValidation:rule="!has(self.allowed) || size(self.allowed) == 0 || (has(self.claimEditable) && self.claimEditable)",message="allowed needs claimEditable: true"
type ParameterPolicy struct {
	// Value replaces the value that the registration gives the parameter.
	// +optional
	Value *apiextensionsv1.JSON `json:"value,omitempty"`
	// ClaimEditable lets a claim of the environment change the parameter.
	// Without it, the claim gets the value of the policy or of the
	// registration.
	// +optional
	ClaimEditable bool `json:"claimEditable,omitempty"`
	// Allowed are the choices that a claim can pick for the parameter. The
	// claim stores the name of a choice, or a list of names for an array
	// parameter. The claim controller puts the value of each choice in the
	// request, and refuses an unknown name with NotAllowed. Allowed needs
	// claimEditable.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=32
	// +optional
	Allowed []NamedValue `json:"allowed,omitempty"`
}

// NamedValue is one choice of a claim-editable parameter.
type NamedValue struct {
	// Name is what the claim stores when it picks the choice, for example
	// sqs. It is unique in the list.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name"`
	// Label is what the fleet manager shows for the choice, for example SQS
	// access.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	Label string `json:"label"`
	// Value is what the request gets, for example the ARN of a policy. It
	// can be any JSON value.
	Value *apiextensionsv1.JSON `json:"value"`
}

// PluginSelection is the registration that a claim selects for one section.
type PluginSelection struct {
	// Section is the name of the PluginSection.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Section string `json:"section"`
	// Registration is the name of the PluginRegistration.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Registration string `json:"registration"`
	// Parameters are the parameter values of the selection. Only the
	// claim-editable parameters of the policy can differ from the policy
	// values. The claim controller refuses another change with NotAllowed.
	// +optional
	Parameters *apiextensionsv1.JSON `json:"parameters,omitempty"`
}

// ResolvedPolicy is the policy that a claim recorded from its environment at
// creation. A later change of the environment does not change it.
type ResolvedPolicy struct {
	// Sections holds the policy of each section, by section name.
	// +kubebuilder:validation:MaxProperties=64
	// +optional
	Sections map[string]SectionPolicy `json:"sections,omitempty"`
	// Parameters holds the parameter values and the claim-editable
	// parameters, by registration name.
	// +kubebuilder:validation:MaxProperties=64
	// +optional
	Parameters map[string]ParameterPolicies `json:"parameters,omitempty"`
	// DeletionPolicy is the deletion default of the environment.
	// +optional
	DeletionPolicy pluginsv1alpha1.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// RequestStatus is the state of one PluginRequest of a claim.
type RequestStatus struct {
	// Section is the name of the PluginSection of the request.
	Section string `json:"section"`
	// Registration is the name of the PluginRegistration of the request.
	Registration string `json:"registration"`
	// Name is the name of the PluginRequest.
	Name string `json:"name"`
	// OutputsReady is the status of the OutputsReady condition of the
	// request.
	// +optional
	OutputsReady metav1.ConditionStatus `json:"outputsReady,omitempty"`
	// Ready is the status of the Ready condition of the request.
	// +optional
	Ready metav1.ConditionStatus `json:"ready,omitempty"`
}
