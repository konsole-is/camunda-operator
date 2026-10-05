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

// DatabaseServerPresetSpec defines the desired state of DatabaseServerPreset.
type DatabaseServerPresetSpec struct {
	// Server is the complete configuration baseline that the servers that
	// reference the preset get. It uses the DatabaseServer spec type. A
	// preset must not set the fields that belong to one server only:
	// presetRef, releaseRef, databaseServerConfig, and suspend. It must also
	// not set the version, which belongs to a CamundaRelease. An explicit
	// zero value, such as an empty presetRef or suspend: false, counts as
	// unset. Templated YAML often renders unset fields in this way. A preset
	// can set archive like any other field. One bucket can serve many
	// servers, because every server writes under its own prefix.
	// +kubebuilder:validation:XValidation:rule="(!has(self.presetRef) || self.presetRef == '') && (!has(self.releaseRef) || self.releaseRef == '') && (!has(self.databaseServerConfig) || self.databaseServerConfig == '') && (!has(self.suspend) || !self.suspend)",message="instance-bound fields (presetRef, releaseRef, databaseServerConfig, suspend) must not be set in a preset"
	// +kubebuilder:validation:XValidation:rule="!has(self.version) || self.version == ''",message="version belongs to a CamundaRelease and must not be set in a preset"
	// +required
	Server DatabaseServerSpec `json:"server"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// DatabaseServerPreset is a cluster-scoped baseline configuration for
// DatabaseServer resources. It has no controller, creates nothing, and
// reports no status. A server reads it through its presetRef. A field set on
// the server replaces the value of the preset for that field completely.
type DatabaseServerPreset struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of DatabaseServerPreset
	// +required
	Spec DatabaseServerPresetSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// DatabaseServerPresetList contains a list of DatabaseServerPreset
type DatabaseServerPresetList struct {
	metav1.TypeMeta `                       json:",inline"`
	metav1.ListMeta `                       json:"metadata,omitzero"`
	Items           []DatabaseServerPreset `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DatabaseServerPreset{}, &DatabaseServerPresetList{})
}
