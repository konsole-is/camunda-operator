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
	// Cluster is the full configuration baseline consumers inherit. It reuses
	// the ElasticsearchCluster spec type so the two never drift apart; the
	// instance-bound fields of that type — presetRef, secondaryStorageConfig,
	// suspend, and monitoring — must be left unset inside a preset. Explicit
	// zero values (an empty presetRef, suspend: false, an empty monitoring
	// object), as templated YAML renders unset fields, count as unset.
	// +kubebuilder:validation:XValidation:rule="(!has(self.presetRef) || self.presetRef == '') && (!has(self.secondaryStorageConfig) || self.secondaryStorageConfig == '') && (!has(self.suspend) || !self.suspend) && (!has(self.monitoring) || !has(self.monitoring.serviceMonitor))",message="instance-bound fields (presetRef, secondaryStorageConfig, suspend, monitoring.serviceMonitor) must not be set in a preset"
	// +required
	Cluster ElasticsearchClusterSpec `json:"cluster"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// ElasticsearchClusterPreset is a cluster-scoped, passive baseline
// configuration for ElasticsearchCluster resources: no controller reconciles
// it, it provisions nothing and reports no status. Consumers resolve it via
// their presetRef and overlay inline fields wholesale.
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
