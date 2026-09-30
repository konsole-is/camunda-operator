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

// ElasticsearchClusterPresetSpec defines the desired state of
// ElasticsearchClusterPreset.
type ElasticsearchClusterPresetSpec struct {
	// Cluster is the complete configuration baseline that the clusters that
	// reference the preset get. It uses the ElasticsearchCluster spec type. A
	// preset must not set the fields that belong to one cluster only:
	// presetRef, releaseRef, secondaryStorageConfig, and suspend. It must
	// also not set the version, which belongs to a CamundaRelease. An explicit
	// zero value, such as an empty presetRef or suspend: false, counts as
	// unset. Templated YAML often renders unset fields in this way. A preset
	// can set monitoring like any other field. Thus it can enable scraping
	// and set the exporter image and resources for every cluster that
	// references it.
	// +kubebuilder:validation:XValidation:rule="(!has(self.presetRef) || self.presetRef == '') && (!has(self.releaseRef) || self.releaseRef == '') && (!has(self.secondaryStorageConfig) || self.secondaryStorageConfig == '') && (!has(self.suspend) || !self.suspend)",message="instance-bound fields (presetRef, releaseRef, secondaryStorageConfig, suspend) must not be set in a preset"
	// +kubebuilder:validation:XValidation:rule="!has(self.version) || self.version == ''",message="version belongs to a CamundaRelease and must not be set in a preset"
	// +required
	Cluster ElasticsearchClusterSpec `json:"cluster"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ElasticsearchClusterPreset is a cluster-scoped baseline configuration for
// ElasticsearchCluster resources. It has no controller, creates nothing, and
// reports no status. A cluster reads it through its presetRef. A field set on
// the cluster replaces the value of the preset for that field completely.
type ElasticsearchClusterPreset struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ElasticsearchClusterPreset
	// +required
	Spec ElasticsearchClusterPresetSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// ElasticsearchClusterPresetList contains a list of ElasticsearchClusterPreset
type ElasticsearchClusterPresetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ElasticsearchClusterPreset `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ElasticsearchClusterPreset{}, &ElasticsearchClusterPresetList{})
}
