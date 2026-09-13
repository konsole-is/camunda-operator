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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CamundaReleaseSpec holds what a platform runs: the version of the
// orchestration cluster, the versions of the storage it runs on, the image
// references that replace the ones the versions produce, and the environment
// that a version needs.
type CamundaReleaseSpec struct {
	// Version is the Camunda version of the orchestration cluster processes,
	// as a full semantic version. The floor of 8.9.0 is enforced by the
	// controller of each referencing cluster on the merged spec.
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+$`
	// +required
	Version string `json:"version"`
	// Connectors holds the version and the environment of the connectors
	// runtime.
	// +optional
	Connectors *ReleaseConnectorsSpec `json:"connectors,omitempty"`
	// Elasticsearch holds the version of the Elasticsearch clusters of this
	// release. An ElasticsearchCluster takes it through its releaseRef.
	// +optional
	Elasticsearch *ReleaseElasticsearchSpec `json:"elasticsearch,omitempty"`
	// DatabaseServer holds the version of the PostgreSQL servers of this
	// release. A DatabaseServer takes it through its releaseRef.
	// +optional
	DatabaseServer *ReleaseDatabaseServerSpec `json:"databaseServer,omitempty"`
	// Images replaces the image reference of a process. An entry is used as
	// it is, tag or digest included. It changes only what is pulled: the
	// version above stays the version that the operator believes the process
	// runs, for the version gates, the downgrade rule, and the environment
	// that the operator computes.
	// +optional
	Images *ReleaseImagesSpec `json:"images,omitempty"`
	// ExtraEnv are extra environment variables of every workload. The entries
	// merge by name over the preset entries and under the cluster entries.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:XValidation:rule="self.all(e, !(has(e.value) && has(e.valueFrom)))",message="an extraEnv entry sets value or valueFrom, never both"
	// +optional
	ExtraEnv []corev1.EnvVar `json:"extraEnv,omitempty"`
	// ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) of
	// every workload. They follow the preset sources and precede the cluster
	// sources.
	// +optional
	ExtraEnvFrom []corev1.EnvFromSource `json:"extraEnvFrom,omitempty"`
	// Zeebe holds the environment of the brokers.
	// +optional
	Zeebe *ReleaseEnvSpec `json:"zeebe,omitempty"`
	// Gateway holds the environment of the gateway.
	// +optional
	Gateway *ReleaseEnvSpec `json:"gateway,omitempty"`
	// Operate holds the environment of Operate.
	// +optional
	Operate *ReleaseEnvSpec `json:"operate,omitempty"`
	// Tasklist holds the environment of Tasklist.
	// +optional
	Tasklist *ReleaseEnvSpec `json:"tasklist,omitempty"`
	// Admin holds the environment of the Admin web application.
	// +optional
	Admin *ReleaseEnvSpec `json:"admin,omitempty"`
}

// ReleaseEnvSpec is the environment of one component that a release adds.
type ReleaseEnvSpec struct {
	// ExtraEnv are extra environment variables of the container of this
	// process. An entry here wins over a top-level entry of the release with
	// the same name.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:XValidation:rule="self.all(e, !(has(e.value) && has(e.valueFrom)))",message="an extraEnv entry sets value or valueFrom, never both"
	// +optional
	ExtraEnv []corev1.EnvVar `json:"extraEnv,omitempty"`
	// ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) of the
	// container of this process.
	// +optional
	ExtraEnvFrom []corev1.EnvFromSource `json:"extraEnvFrom,omitempty"`
}

// ReleaseConnectorsSpec is the connectors runtime of a release.
type ReleaseConnectorsSpec struct {
	// Version is the version of the connectors bundle image, as a full
	// semantic version. The bundle has its own patch line, so it does not
	// follow the cluster version. A cluster that runs connectors needs it
	// from the release or from its own spec.
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+$`
	// +optional
	Version        string `json:"version,omitempty"`
	ReleaseEnvSpec `       json:",inline"`
}

// ReleaseElasticsearchSpec is the Elasticsearch of a release.
type ReleaseElasticsearchSpec struct {
	// Version is the Elasticsearch version of the clusters of this release,
	// as a full semantic version. Elasticsearch has a patch line of its own,
	// so it does not follow the Camunda version. The floor of Elasticsearch
	// 8.19 or 9.2 is enforced by the controller of each referencing cluster
	// on the merged spec.
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+$`
	// +optional
	Version string `json:"version,omitempty"`
}

// ReleaseDatabaseServerSpec is the PostgreSQL server of a release.
type ReleaseDatabaseServerSpec struct {
	// Version is the PostgreSQL major version of the servers of this release,
	// as a bare number such as "17". The floor of PostgreSQL 14 is enforced
	// by the controller of each referencing server on the merged spec. A
	// server that already runs another major refuses the change and keeps the
	// major it has.
	// +kubebuilder:validation:Pattern=`^\d+$`
	// +optional
	Version string `json:"version,omitempty"`
}

// ReleaseImagesSpec holds the image references that a release pins. Each
// value is a complete reference, with its tag or digest.
type ReleaseImagesSpec struct {
	// Camunda is the image of every orchestration cluster process. Unset
	// means the camunda image of the platform config at the version of the
	// release.
	// +kubebuilder:validation:MinLength=1
	// +optional
	Camunda string `json:"camunda,omitempty"`
	// Connectors is the image of the connectors runtime. Unset means the
	// connectors image of the platform config at connectors.version.
	// +kubebuilder:validation:MinLength=1
	// +optional
	Connectors string `json:"connectors,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.version`
// +kubebuilder:printcolumn:name="Connectors",type=string,JSONPath=`.spec.connectors.version`,priority=1
// +kubebuilder:printcolumn:name="Elasticsearch",type=string,JSONPath=`.spec.elasticsearch.version`,priority=1
// +kubebuilder:printcolumn:name="PostgreSQL",type=string,JSONPath=`.spec.databaseServer.version`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// CamundaRelease is a cluster-scoped, passive description of what a platform
// runs: the versions, the pinned images, and the environment a version needs.
// No controller reconciles it, it provisions nothing and reports no status. A
// CamundaCluster, an ElasticsearchCluster, and a DatabaseServer each resolve
// it through their releaseRef and merge it between their preset and their own
// spec.
type CamundaRelease struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of CamundaRelease
	// +required
	Spec CamundaReleaseSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// CamundaReleaseList contains a list of CamundaRelease
type CamundaReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []CamundaRelease `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CamundaRelease{}, &CamundaReleaseList{})
}
