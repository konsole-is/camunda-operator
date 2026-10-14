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
)

// PluginSectionSpec describes one section of the create flow of a claim. The
// registrations that serve the section are its options.
// +kubebuilder:validation:XValidation:rule="!has(self.slot) || self.selection == 'One'",message="a section with a slot must have selection One, because a slot fills one field"
type PluginSectionSpec struct {
	// Label is the name of the section in the fleet manager, for example
	// "Backup storage". Display only. No controller reads it.
	// +kubebuilder:validation:MinLength=1
	Label string `json:"label"`
	// Description says what the section is for. Display only. No controller
	// reads it.
	// +optional
	Description string `json:"description,omitempty"`
	// Order is the position of the section in the create flow. A section
	// with a lower value comes first. Display only. No controller reads it.
	// +optional
	Order int32 `json:"order,omitempty"`
	// Selection says how many options of the section one claim can select.
	Selection Selection `json:"selection"`
	// EmptyOptionLabel is the label of the option that selects nothing, for
	// example "No backups". When it is empty, the section has no empty
	// option. Display only. No controller reads it.
	// +kubebuilder:validation:XValidation:rule="self.size() <= 64",message="emptyOptionLabel must be at most 64 characters"
	// +optional
	EmptyOptionLabel string `json:"emptyOptionLabel,omitempty"`
	// Slot names the fields of the generated resource that the outputs of
	// the section fill. When it is empty, the outputs are only inputs of
	// other registrations.
	// +optional
	Slot Slot `json:"slot,omitempty"`
	// AppliesTo lists the claim kinds whose create flow shows the section.
	// +kubebuilder:validation:XValidation:rule="self.size() >= 1",message="appliesTo must list at least one claim kind"
	AppliesTo []ClaimKind `json:"appliesTo"`
	// Outputs declares the outputs that every registration of the section
	// writes.
	// +listType=map
	// +listMapKey=key
	// +optional
	Outputs []OutputDecl `json:"outputs,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Label",type=string,JSONPath=`.spec.label`
// +kubebuilder:printcolumn:name="Slot",type=string,JSONPath=`.spec.slot`
// +kubebuilder:printcolumn:name="Order",type=integer,JSONPath=`.spec.order`,priority=1
// +kubebuilder:printcolumn:name="Selection",type=string,JSONPath=`.spec.selection`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="self.metadata.name.size() <= 63",message="metadata.name must be at most 63 characters, because it is used as a label value"

// PluginSection is one section of the create flow of a claim, such as
// "Backup storage" or "Access". The operator writes the core sections, and a
// plugin chart writes a section that it brings. The name is at most 63
// characters, because other objects carry it in a label value.
type PluginSection struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the section
	// +required
	Spec PluginSectionSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// PluginSectionList contains a list of PluginSection
type PluginSectionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []PluginSection `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PluginSection{}, &PluginSectionList{})
}
