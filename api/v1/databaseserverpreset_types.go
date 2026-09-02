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
	// Server is the full configuration baseline consumers inherit. It reuses
	// the DatabaseServer spec type so the two never drift apart. The
	// instance-bound fields of that type, presetRef, databaseServerConfig,
	// and suspend, must be left unset inside a preset. Explicit zero values
	// (an empty presetRef, suspend: false), as templated YAML renders unset
	// fields, count as unset. archive is a baseline like any other field: one
	// bucket serves a fleet, because every server writes under a prefix of
	// its own.
	// +kubebuilder:validation:XValidation:rule="(!has(self.presetRef) || self.presetRef == '') && (!has(self.databaseServerConfig) || self.databaseServerConfig == '') && (!has(self.suspend) || !self.suspend)",message="instance-bound fields (presetRef, databaseServerConfig, suspend) must not be set in a preset"
	// +required
	Server DatabaseServerSpec `json:"server"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.server.version`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// DatabaseServerPreset is a cluster-scoped, passive baseline configuration
// for DatabaseServer resources: no controller reconciles it, it provisions
// nothing and reports no status. Consumers resolve it via their presetRef and
// overlay inline fields wholesale.
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
