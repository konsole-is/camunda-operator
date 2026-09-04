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

// CamundaClusterPresetSpec defines the desired state of CamundaClusterPreset.
type CamundaClusterPresetSpec struct {
	// Cluster is the configuration baseline that referencing clusters
	// inherit. It reuses the CamundaCluster spec type so the two never drift
	// apart. The instance-bound fields of that type (platformConfigRef,
	// presetRef, releaseRef, externalUrl, serviceAccount, storageRef,
	// backupStorageRef, documentStorageRef, monitoring, suspend, pause) must
	// be left unset inside a preset, and so must the fields that belong to a
	// CamundaRelease (version, connectors.version). Explicit zero values (an
	// empty presetRef, suspend: false), as templated YAML renders unset
	// fields, count as unset. The CamundaCluster doc lists the field details,
	// the CamundaClusterPreset doc lists the merge rules.
	// +kubebuilder:validation:XValidation:rule="(!has(self.platformConfigRef) || self.platformConfigRef == '') && (!has(self.presetRef) || self.presetRef == '') && (!has(self.releaseRef) || self.releaseRef == '') && (!has(self.externalUrl) || self.externalUrl == '') && !has(self.serviceAccount) && (!has(self.storageRef) || self.storageRef == '') && (!has(self.backupStorageRef) || self.backupStorageRef == '') && (!has(self.documentStorageRef) || self.documentStorageRef == '') && !has(self.monitoring) && (!has(self.suspend) || !self.suspend) && (!has(self.pause) || !self.pause)",message="instance-bound fields (platformConfigRef, presetRef, releaseRef, externalUrl, serviceAccount, storageRef, backupStorageRef, documentStorageRef, monitoring, suspend, pause) must not be set in a preset"
	// +kubebuilder:validation:XValidation:rule="(!has(self.version) || self.version == '') && (!has(self.connectors) || !has(self.connectors.version) || self.connectors.version == '')",message="version and connectors.version belong to a CamundaRelease and must not be set in a preset"
	// +required
	Cluster CamundaClusterSpec `json:"cluster"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.cluster.version`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// CamundaClusterPreset is a cluster-scoped, passive baseline configuration
// for CamundaCluster resources: no controller reconciles it, it provisions
// nothing and reports no status. A CamundaCluster resolves it through its
// presetRef and merges its own fields over it under the rules of the preset
// doc.
type CamundaClusterPreset struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of CamundaClusterPreset
	// +required
	Spec CamundaClusterPresetSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// CamundaClusterPresetList contains a list of CamundaClusterPreset
type CamundaClusterPresetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []CamundaClusterPreset `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CamundaClusterPreset{}, &CamundaClusterPresetList{})
}
