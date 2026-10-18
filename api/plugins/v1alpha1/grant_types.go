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

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// GrantSpec is the desired state of a Grant. A plugin controller writes it.
type GrantSpec struct {
	// Cluster names the CamundaCluster whose identity gets the permission,
	// in the namespace of the grant. It cannot change after the grant is
	// created.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="cluster is immutable: delete the grant and create a new one"
	Cluster v1.ClusterRef `json:"cluster"`

	// Template is the name of the grant template in the registration that
	// the fragment is rendered from. It cannot change after the grant is
	// created.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="template is immutable: delete the grant and create a new one"
	Template string `json:"template"`

	// Format is the format of the fragment. It cannot change after the
	// grant is created.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="format is immutable: delete the grant and create a new one"
	Format GrantFormat `json:"format"`

	// Fragment is one permission statement, filled in from the template. It
	// is a JSON object.
	// +kubebuilder:validation:Type=object
	Fragment *apiextensionsv1.JSON `json:"fragment"`
}

// GrantStatus is the observed state of a Grant. The identity extension of
// the cluster writes it.
type GrantStatus struct {
	// Conditions are Applied, MatchesTemplate, and Ready. Applied is False
	// with reason OverCeiling, OverSizeLimit, or Rejected when the identity
	// extension refuses the fragment. MatchesTemplate is False with reason
	// TemplateChanged when the fragment no longer matches its template.
	// Ready is True with reason Healthy when both are True. Otherwise Ready
	// has the reason and the message of the first of them that is False.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Applied",type=string,JSONPath=`.status.conditions[?(@.type=="Applied")].status`
// +kubebuilder:printcolumn:name="Format",type=string,JSONPath=`.spec.format`,priority=1
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=`.spec.cluster.name`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="self.metadata.name.size() <= 63",message="metadata.name must be at most 63 characters, because it is used as a label value"

// Grant asks the identity of a cluster for one permission on behalf of a
// PluginRequest. A plugin controller writes the spec, and the identity
// extension of the cluster writes the status. The name is at most 63
// characters, because other objects carry it in a label value.
type Grant struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Grant
	// +required
	Spec GrantSpec `json:"spec"`

	// status defines the observed state of Grant
	// +optional
	Status GrantStatus `json:"status,omitzero"`
}

// GetStatusConditions returns a pointer to the status conditions. The
// component framework stages conditions on the resource through it.
func (in *Grant) GetStatusConditions() *[]metav1.Condition { return &in.Status.Conditions }

// GetKind returns the CRD kind. The component framework uses it for event and
// metric recording.
func (in *Grant) GetKind() string { return "Grant" }

// +kubebuilder:object:root=true

// GrantList contains a list of Grant
type GrantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Grant `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Grant{}, &GrantList{})
}
