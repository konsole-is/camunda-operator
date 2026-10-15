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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The condition types that only CamundaManagementClusterClaim reports. Ready
// and its reasons come from api/v1. The reasons of a refused claim are in
// conditions.go.
const (
	// ConditionPlaneRequestsReady is the condition of the PluginRequests of a
	// plane claim. Its string is the RequestsReady of CamundaClusterClaim, so
	// one kubectl wait works on both claim kinds.
	ConditionPlaneRequestsReady = "RequestsReady"
	// ConditionManagementClusterReady carries the Ready reason and message of
	// the CamundaManagementCluster that the claim writes, unchanged.
	ConditionManagementClusterReady = "ManagementClusterReady"
)

// CamundaManagementClusterClaimSpec is the management plane that a team asks
// for. The claim controller writes one PluginRequest for each selected
// section and one CamundaManagementCluster with the name of the claim.
type CamundaManagementClusterClaimSpec struct {
	// Environments are the names of the Environments that the plane serves.
	// The plane serves the CamundaClusters in the namespaces of these
	// environments. The claim controller refuses a name that no Environment
	// has, with the reason InvalidReference.
	// +listType=set
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=64
	// +kubebuilder:validation:items:MinLength=1
	// +kubebuilder:validation:items:MaxLength=63
	Environments []string `json:"environments"`
	// ClusterSelector selects the CamundaClusters that Console and Web
	// Modeler serve, in the namespaces of the environments. An unset
	// selector selects no cluster. An empty selector ({}) selects every
	// cluster.
	// +optional
	ClusterSelector *metav1.LabelSelector `json:"clusterSelector,omitempty"`
	// PlatformConfigRef names the CamundaPlatformConfig of the plane. Empty
	// means the platformConfigRef of the Environment of the namespace of the
	// claim.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=253
	// +optional
	PlatformConfigRef string `json:"platformConfigRef,omitempty"`
	// Management holds the settings of the plane that the claim can set: the
	// identity provider, Management Identity, Console, and Web Modeler. The
	// versions are in versions. The database references come from the
	// management-database section.
	Management ManagementOverrides `json:"management"`
	// Versions holds the version of each component of the plane. The fleet
	// manager fills them from its images table, because Identity, Console,
	// and Web Modeler have their own version lines. For example, with
	// Camunda 8.9.23, Identity is 8.9.10.
	Versions ManagementVersions `json:"versions"`
	// Plugins holds the registration that the claim selects for each
	// section, for example access and management-database. The sections and
	// their registrations are fixed after creation. Only the parameters can
	// change.
	// +listType=map
	// +listMapKey=section
	// +listMapKey=registration
	// +kubebuilder:validation:MaxItems=32
	// +optional
	Plugins []PluginSelection `json:"plugins,omitempty"`
	// ManagementAuthConfigName is the name of the cluster-scoped
	// ManagementAuthConfig that the plane writes. A CamundaOptimize reads it
	// through its managementAuthRef. Empty means the name of the claim.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=253
	// +optional
	ManagementAuthConfigName string `json:"managementAuthConfigName,omitempty"`
	// DeletionPolicy says what happens to the resources of the
	// PluginRequests of the claim when the claim is deleted. Delete deletes
	// them. Orphan keeps them. Empty means the deletion policy of the
	// Environment. The CamundaManagementCluster is always deleted with the
	// claim.
	// +optional
	DeletionPolicy pluginsv1alpha1.DeletionPolicy `json:"deletionPolicy,omitempty"`
	// Suspend scales every workload of the plane to zero. The PluginRequests
	// do not change.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// ManagementOverrides holds the settings of a management plane that a plane
// claim can set. The claim controller writes them to the
// CamundaManagementCluster.
// +kubebuilder:validation:XValidation:rule="has(self.identityProvider.oidc) ? has(self.identity.admin.claimName) : has(self.identity.admin.username)",message="identity.admin: set claimName and claimValue in oidc mode, username in the keycloak modes"
// +kubebuilder:validation:XValidation:rule="!has(self.identityProvider.oidc) || !has(self.identity.admin.passwordSecretRef)",message="identity.admin.passwordSecretRef applies to the keycloak modes only"
// +kubebuilder:validation:XValidation:rule="!has(self.webModeler) || has(self.identityProvider.oidc) || has(self.identity.admin.email)",message="identity.admin.email is required when webModeler is set in a keycloak mode"
type ManagementOverrides struct {
	// IdentityProvider selects where users sign in. Exactly one of keycloak,
	// externalKeycloak, or oidc is set. The mode can change after creation.
	IdentityProvider ManagementIdentityProvider `json:"identityProvider"`
	// Identity configures Management Identity. The plane always runs it.
	Identity IdentityOverrides `json:"identity"`
	// Console configures Console. While it is unset, the plane has no
	// Console.
	// +optional
	Console *ConsoleOverrides `json:"console,omitempty"`
	// WebModeler configures Web Modeler. While it is unset, the plane has no
	// Web Modeler.
	// +optional
	WebModeler *WebModelerOverrides `json:"webModeler,omitempty"`
}

// ManagementIdentityProvider holds one of the three identity provider modes
// of a plane claim.
type ManagementIdentityProvider struct {
	// Keycloak runs Keycloak through the Keycloak Operator. The Keycloak
	// Operator must be installed on the Kubernetes cluster. versions.keycloak
	// is required in this mode.
	// +optional
	Keycloak *KeycloakOverrides `json:"keycloak,omitempty"`
	// ExternalKeycloak connects Management Identity to a Keycloak that you
	// run.
	// +optional
	ExternalKeycloak *v1.ExternalKeycloakSpec `json:"externalKeycloak,omitempty"`
	// OIDC connects Management Identity to the identity provider of the
	// CamundaPlatformConfig of the plane.
	// +optional
	OIDC *v1.ManagementOIDCSpec `json:"oidc,omitempty"`
}

// KeycloakOverrides configures the Keycloak that the operator runs for a
// plane claim.
type KeycloakOverrides struct {
	// ExternalURL is the URL where browsers reach Keycloak, with the /auth
	// path and no query and no fragment. The claim controller refuses this
	// field with the reason SlotConflict when the access section has a
	// selection, because the access section then gives the URL. Without a
	// selection, set it here.
	// +optional
	ExternalURL string `json:"externalUrl,omitempty"`
	// Replicas is the number of Keycloak instances. Defaults to 1.
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// Resources are the CPU and memory of the Keycloak container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// Scheduling constraints of the Keycloak pods.
	// +optional
	Scheduling *v1.SchedulingSpec `json:"scheduling,omitempty"`
}

// IdentityOverrides configures Management Identity for a plane claim.
type IdentityOverrides struct {
	// ExternalURL is the URL where browsers reach Management Identity. A URL
	// with a path other than / must use https. The claim controller refuses
	// this field with the reason SlotConflict when the access section has a
	// selection, because the access section then gives the URL. Without a
	// selection, set it here.
	// +optional
	ExternalURL string `json:"externalUrl,omitempty"`
	// Admin names the first administrator of the plane. Set claimName and
	// claimValue in the oidc mode, and username in the Keycloak modes.
	// Management Identity reads it only on its first start.
	Admin           v1.IdentityAdminSpec `json:"admin"`
	v1.WorkloadSpec `json:",inline"`
}

// ConsoleOverrides configures Console for a plane claim. versions.console is
// required while it is set.
type ConsoleOverrides struct {
	// ExternalURL is the URL where browsers reach Console. The claim
	// controller refuses this field with the reason SlotConflict when the
	// access section has a selection, because the access section then gives
	// the URL. Without a selection, set it here.
	// +optional
	ExternalURL     string `json:"externalUrl,omitempty"`
	v1.WorkloadSpec `json:",inline"`
}

// WebModelerOverrides configures Web Modeler for a plane claim.
// versions.webModeler is required while it is set.
type WebModelerOverrides struct {
	// ExternalURL is the URL where browsers reach Web Modeler. The claim
	// controller refuses this field with the reason SlotConflict when the
	// access section has a selection, because the access section then gives
	// the URL. Without a selection, set it here.
	// +optional
	ExternalURL string `json:"externalUrl,omitempty"`
	// WebsocketsExternalURL is the URL where browsers get the live updates
	// of Web Modeler. The claim controller refuses this field with the
	// reason SlotConflict when the access section has a selection, because
	// the access section then gives the URL. Without a selection, set it
	// here.
	// +optional
	WebsocketsExternalURL string `json:"websocketsExternalUrl,omitempty"`
	// Mail configures the SMTP server that Web Modeler sends notifications
	// through. Web Modeler does not start without it.
	Mail v1.WebModelerMailSpec `json:"mail"`
	// Restapi configures the workload of the restapi process.
	// +optional
	Restapi *v1.WorkloadSpec `json:"restapi,omitempty"`
	// Websockets configures the workload of the websockets process.
	// +optional
	Websockets *v1.WorkloadSpec `json:"websockets,omitempty"`
}

// ManagementVersions holds the version of each component of a plane claim,
// each as a full semantic version.
type ManagementVersions struct {
	// Identity is the Management Identity version, for example 8.9.10.
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+$`
	Identity string `json:"identity"`
	// Keycloak is the Keycloak version. It is required in the keycloak mode.
	// Camunda 8.9 supports only Keycloak 26.
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+$`
	// +optional
	Keycloak string `json:"keycloak,omitempty"`
	// Console is the Console version. It is required while
	// management.console is set.
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+$`
	// +optional
	Console string `json:"console,omitempty"`
	// WebModeler is the Web Modeler version. It is required while
	// management.webModeler is set.
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+$`
	// +optional
	WebModeler string `json:"webModeler,omitempty"`
}

// CamundaManagementClusterClaimStatus is the state of a plane claim. The
// claim controller writes it.
type CamundaManagementClusterClaimStatus struct {
	// ObservedGeneration is the last generation that the claim controller
	// processed.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions report the state of the claim. Ready is False with the
	// reason NoEnvironment, UnknownEnvironment, NotAllowed, MissingInput,
	// InvalidReference, SlotConflict, or WiringCycle when the claim
	// controller refuses the claim. The message names the cause. Otherwise
	// Ready follows RequestsReady and ManagementClusterReady.
	// ManagementClusterReady carries the Ready reason and message of the
	// CamundaManagementCluster.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Environment is the Environment of the namespace of the claim when the
	// claim was created.
	// +optional
	Environment string `json:"environment,omitempty"`
	// ResolvedPolicy is the policy of the environment when the claim was
	// created. A later change of the environment does not change it.
	// +optional
	ResolvedPolicy *ResolvedPolicy `json:"resolvedPolicy,omitempty"`
	// Requests lists the PluginRequests of the claim, one for each selected
	// section.
	// +listType=map
	// +listMapKey=section
	// +listMapKey=registration
	// +optional
	Requests []RequestStatus `json:"requests,omitempty"`
	// ManagementCluster names the CamundaManagementCluster of the claim. It
	// has the name of the claim.
	// +optional
	ManagementCluster *ManagementClusterRef `json:"managementCluster,omitempty"`
}

// ManagementClusterRef names a CamundaManagementCluster in the namespace of
// the claim.
type ManagementClusterRef struct {
	// Name is the name of the CamundaManagementCluster.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Plane",type=string,JSONPath=`.status.managementCluster.name`
// +kubebuilder:printcolumn:name="Environments",type=string,JSONPath=`.spec.environments`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// CamundaManagementClusterClaim asks for a management plane that serves the
// clusters of one or more environments. The claim controller writes a
// CamundaManagementCluster with the name of the claim, and the
// PluginRequests of the selected sections. The name is a DNS-1035 label of
// at most 40 characters, because the claim adds a suffix to it in the names
// of the resources that it writes.
type CamundaManagementClusterClaim struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec is the management plane that the claim asks for
	// +required
	Spec CamundaManagementClusterClaimSpec `json:"spec"`

	// status is the state of the claim
	// +optional
	Status CamundaManagementClusterClaimStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// CamundaManagementClusterClaimList contains a list of
// CamundaManagementClusterClaim
type CamundaManagementClusterClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []CamundaManagementClusterClaim `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CamundaManagementClusterClaim{}, &CamundaManagementClusterClaimList{})
}
