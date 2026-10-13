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

// PluginRequestSpec is the desired state of a PluginRequest. A claim
// controller writes it.
type PluginRequestSpec struct {
	// Section is the name of the PluginSection that the request fills. It
	// cannot change after the request is created.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="section is immutable: delete the request and create a new one"
	Section string `json:"section"`

	// Registration is the name of the PluginRegistration that serves the
	// request. It cannot change after the request is created.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="registration is immutable: delete the request and create a new one"
	Registration string `json:"registration"`

	// ClaimRef names the claim that wrote the request. It cannot change
	// after the request is created.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="claimRef is immutable: delete the request and create a new one"
	ClaimRef ClaimRef `json:"claimRef"`

	// Target names the resource that the request serves, before the
	// resource exists. target.name cannot change after the request is
	// created.
	// +kubebuilder:validation:XValidation:rule="self.name == oldSelf.name",message="target.name is immutable: delete the request and create a new one"
	Target TargetRef `json:"target"`

	// Inputs are the values of the inputs of the registration, by input
	// name. The claim controller resolves them from the outputs of other
	// requests of the same claim.
	// +optional
	Inputs map[string]string `json:"inputs,omitempty"`

	// Parameters are the settings of the request, as free-form JSON. The
	// plugin checks them against the parameter schema of the registration.
	// +optional
	Parameters *apiextensionsv1.JSON `json:"parameters,omitempty"`

	// DeletionPolicy says what the plugin does with the resources of the
	// request when the request is deleted. The claim controller copies it
	// from the claim before it deletes the request.
	DeletionPolicy DeletionPolicy `json:"deletionPolicy"`
}

// PluginRequestStatus is the observed state of a PluginRequest. The plugin
// controller that serves the registration writes it.
type PluginRequestStatus struct {
	// ObservedGeneration is the generation of the spec that the plugin
	// processed last.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions are OutputsReady and Ready. OutputsReady is True when the
	// outputs are written for the current generation. Ready is False with
	// reason GrantRejected while a Grant of the request is not Ready, with
	// reason WaitingForEndpoints while an access request waits for the
	// endpoints of its target, and with reason InvalidParameters when the
	// parameters do not pass the registration. The plugin sets the other
	// reasons.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Outputs are the values that the plugin publishes for the request, by
	// output key. A reader trusts them only while OutputsReady is True and
	// the observedGeneration of that condition equals metadata.generation.
	// +optional
	Outputs map[string]string `json:"outputs,omitempty"`

	// Summary is a list of facts about the request, for display only. No
	// controller reads it. The first item is the headline.
	// +optional
	Summary []SummaryItem `json:"summary,omitempty"`

	// SummaryTime is when the plugin wrote the summary last, for display
	// only.
	// +optional
	SummaryTime *metav1.Time `json:"summaryTime,omitempty"`

	// Links are named URLs about the request, for example the console page
	// of a bucket, for display only.
	// +optional
	Links []Link `json:"links,omitempty"`

	// PodSelector selects the pods that the plugin runs for the request, for
	// display only.
	// +optional
	PodSelector *metav1.LabelSelector `json:"podSelector,omitempty"`

	// Resources are the resources that the plugin made for the request, in
	// the cluster or in a cloud, for display only.
	// +optional
	Resources []ResourceRef `json:"resources,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Outputs",type=string,JSONPath=`.status.conditions[?(@.type=="OutputsReady")].status`
// +kubebuilder:printcolumn:name="Section",type=string,JSONPath=`.spec.section`,priority=1
// +kubebuilder:printcolumn:name="Registration",type=string,JSONPath=`.spec.registration`,priority=1
// +kubebuilder:printcolumn:name="Claim",type=string,JSONPath=`.spec.claimRef.name`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="self.metadata.name.size() <= 63",message="metadata.name must be at most 63 characters, because it is used as a label value"

// PluginRequest asks a plugin for the resources of one section of a claim.
// A claim controller writes the spec, and the plugin controller that serves
// the registration writes the status. The claim names a request
// <claim>-<section>.
type PluginRequest struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of PluginRequest
	// +required
	Spec PluginRequestSpec `json:"spec"`

	// status defines the observed state of PluginRequest
	// +optional
	Status PluginRequestStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// PluginRequestList contains a list of PluginRequest.
type PluginRequestList struct {
	metav1.TypeMeta `                json:",inline"`
	metav1.ListMeta `                json:"metadata,omitzero"`
	Items           []PluginRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PluginRequest{}, &PluginRequestList{})
}
