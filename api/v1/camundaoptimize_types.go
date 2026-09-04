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

// The condition vocabulary that only CamundaOptimize reports. The shared
// vocabulary is in conditions.go.
const (
	// ConditionWebappReady is the condition of the Optimize webapp workload.
	ConditionWebappReady = "WebappReady"
	// ConditionImporterReady is the condition of the Optimize importer
	// workload.
	ConditionImporterReady = "ImporterReady"

	// ReasonVersionMismatch means the major and the minor of spec.version
	// differ from those of the effective version of the referenced cluster.
	// Camunda supports Optimize only on a matching minor.
	ReasonVersionMismatch = "VersionMismatch"
	// ReasonClusterAlreadyAttached means that another CamundaOptimize is
	// already attached to the referenced cluster. One cluster carries one
	// Optimize instance: the Optimize index prefix is fixed, so two instances
	// write the same analytics indices of the same Elasticsearch. The message
	// names the CamundaOptimize that holds the attachment.
	ReasonClusterAlreadyAttached = "ClusterAlreadyAttached"
	// ReasonExporterConflict means that spec.zeebe.extraEnv of the referenced
	// cluster already carries an entry with the name of an exporter setting,
	// and that entry supplies its value the other way: a literal where the
	// operator needs a Secret reference, or the reverse. A container rejects
	// an entry that carries both, so the operator reports the collision
	// instead of applying it.
	ReasonExporterConflict = "ExporterConflict"
)

// CamundaOptimizeSpec is the desired state of one Optimize instance. It
// attaches to one CamundaCluster and reads the data of that cluster from
// Elasticsearch.
//
// The spec has no platformConfigRef. The image repository and the license come
// from the CamundaPlatformConfig of the referenced cluster, so the two cannot
// disagree.
type CamundaOptimizeSpec struct {
	// Version is the Optimize version to deploy, as a full semantic version.
	// Optimize has its own patch line, so it does not follow the version of
	// the cluster. The major and the minor must match the effective version
	// of the referenced cluster; the controller reports VersionMismatch when
	// they differ.
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+$`
	Version string `json:"version"`
	// ManagementAuthRef names the cluster-scoped ManagementAuthConfig that
	// provides the Management Identity OIDC configuration. Optimize
	// authenticates against Management Identity, not against the built-in
	// auth of the orchestration cluster.
	// +kubebuilder:validation:MinLength=1
	ManagementAuthRef string `json:"managementAuthRef"`
	// ExternalURL is the URL that browsers reach this Optimize at.
	//
	// In the two Keycloak modes the management plane behind managementAuthRef
	// registers <externalUrl>/api/authentication/callback on the optimize
	// client of the realm, so a person who signs in here comes back here. An
	// Optimize that sets no URL gets no callback from that plane, so Keycloak
	// refuses the return, unless somebody put that callback in the realm by
	// hand.
	//
	// In the oidc mode the field has no effect. The identity provider of the
	// platform config holds the callback URLs, so add this one there.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https') && url(self).getHostname() != ''",message="externalUrl must be a valid http or https URL"
	// +kubebuilder:validation:XValidation:rule="!self.contains(',')",message="externalUrl must carry no comma: Management Identity reads the callback list as comma-separated"
	// +kubebuilder:validation:XValidation:rule="!self.endsWith('/')",message="externalUrl must not end with a slash: the login callback is appended to it"
	// +kubebuilder:validation:XValidation:rule="!self.contains('?') && !self.contains('#')",message="externalUrl must carry no query and no fragment: the login callback is appended to it"
	// +kubebuilder:validation:XValidation:rule="!self.matches('[[:space:]]')",message="externalUrl must carry no whitespace: Management Identity deletes whitespace from every root URL and the operator does not, so the two would register different callbacks"
	// +optional
	ExternalURL string `json:"externalUrl,omitempty"`
	// ClusterRef names the CamundaCluster that this Optimize instance reads.
	// The secondary storage of that cluster must be Elasticsearch, and no
	// other CamundaOptimize may be attached to it.
	//
	// The reference is immutable. A repoint would apply the exporter settings
	// to the new cluster while the old cluster keeps the settings this
	// operator applied, and it would change the pod selectors of the
	// Deployments, which Kubernetes does not allow.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="clusterRef is immutable: delete this CamundaOptimize and create a new one to attach it to another cluster"
	ClusterRef ClusterRef `json:"clusterRef"`
	// Webapp configures the Deployment that serves the Optimize user
	// interface. It runs with data import off.
	// +optional
	Webapp *WorkloadSpec `json:"webapp,omitempty"`
	// Importer configures the Deployment that imports the exported cluster
	// data into the Optimize indices. Optimize supports one active importer,
	// so replicas must be 0 or 1. Set 0 to stop the import, for example while
	// a restore or an index rewrite runs; the webapp keeps serving what is
	// already imported.
	// +optional
	// +kubebuilder:validation:XValidation:rule="!has(self.replicas) || self.replicas <= 1",message="importer.replicas must be 0 or 1: Optimize supports one active importer"
	Importer *WorkloadSpec `json:"importer,omitempty"`
	// Monitoring configures the monitoring integrations.
	// +optional
	Monitoring *OptimizeMonitoringSpec `json:"monitoring,omitempty"`
}

// OptimizeMonitoringSpec groups the monitoring integrations of a
// CamundaOptimize.
type OptimizeMonitoringSpec struct {
	// ServiceMonitor configures the Prometheus ServiceMonitors. When enabled,
	// the operator creates one ServiceMonitor per Deployment, named like the
	// workload, that scrapes /actuator/prometheus on the management port.
	// +optional
	ServiceMonitor *ServiceMonitorSpec `json:"serviceMonitor,omitempty"`
}

// CamundaOptimizeStatus is the observed state of a CamundaOptimize.
type CamundaOptimizeStatus struct {
	// ObservedGeneration is the last generation reconciled by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the current state. Ready carries a pre-check
	// reason, or it is derived from the conditions of the two workloads.
	// The per-workload conditions (WebappReady, ImporterReady) also appear
	// here.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=`.spec.clusterRef.name`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// CamundaOptimize describes one Camunda Optimize instance attached to one
// CamundaCluster. The operator turns it into a webapp Deployment, an importer
// Deployment, and their Services. It also turns on the Elasticsearch exporter
// of the referenced cluster, so Optimize has data to import.
type CamundaOptimize struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of CamundaOptimize
	// +required
	Spec CamundaOptimizeSpec `json:"spec"`

	// status defines the observed state of CamundaOptimize
	// +optional
	Status CamundaOptimizeStatus `json:"status,omitzero"`
}

// GetStatusConditions returns a pointer to the status conditions. The
// component framework stages conditions on the resource through it.
func (in *CamundaOptimize) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// GetKind returns the CRD kind. The component framework uses it for event and
// metric recording.
func (in *CamundaOptimize) GetKind() string { return "CamundaOptimize" }

// SetObservedGeneration records the last reconciled generation in status.
func (in *CamundaOptimize) SetObservedGeneration(generation int64) {
	in.Status.ObservedGeneration = generation
}

// +kubebuilder:object:root=true

// CamundaOptimizeList contains a list of CamundaOptimize
type CamundaOptimizeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []CamundaOptimize `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CamundaOptimize{}, &CamundaOptimizeList{})
}
