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

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The component conditions of a CamundaClusterClaim. Ready comes from api/v1.
const (
	// ConditionRequestsReady reports the PluginRequests of the claim. While a
	// request waits for its own outputs, the reason is OperationPending.
	// While a request waits for the outputs of another section, the reason
	// is Blocked. The message names the sections.
	ConditionRequestsReady = "RequestsReady"
	// ConditionClusterReady reports the CamundaCluster of the claim. ocf sets
	// its reason, such as Creating, Healthy, or Down. While the reason is
	// Creating, Updating, Failing, Degraded, or Down, the message holds the
	// Ready reason and message of the CamundaCluster, or says that it has not
	// reported Ready yet.
	ConditionClusterReady = "ClusterReady"
	// ConditionAttachmentsReady reports the CamundaOptimize and the
	// BackupSchedule of the claim.
	ConditionAttachmentsReady = "AttachmentsReady"
)

// CamundaClusterClaimSpec is the cluster that a claim asks for. The cluster
// claim controller expands it into a CamundaCluster, a PluginRequest for each
// selection, and the attachments.
// +kubebuilder:validation:XValidation:rule="(has(self.plugins) ? size(self.plugins) : 0) == (has(oldSelf.plugins) ? size(oldSelf.plugins) : 0) && (!has(oldSelf.plugins) || oldSelf.plugins.all(o, self.plugins.exists(n, n.section == o.section && n.registration == o.registration)))",message="sections and their registrations are fixed after creation"
type CamundaClusterClaimSpec struct {
	// PresetRef names the CamundaClusterPreset of the cluster. When the
	// namespace belongs to an Environment, the Environment must list the
	// preset in spec.presets.camundaCluster. The claim controller checks a
	// change of an existing claim against the current Environment.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +required
	PresetRef string `json:"presetRef"`
	// ReleaseRef names the CamundaRelease of the cluster, which sets its
	// versions. Empty means the default release of the Environment. The
	// claim controller checks a change of an existing claim against the
	// current Environment.
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +optional
	ReleaseRef string `json:"releaseRef,omitempty"`
	// PlatformConfigRef names the CamundaPlatformConfig of the cluster.
	// Empty means spec.platformConfigRef of the Environment.
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +optional
	PlatformConfigRef string `json:"platformConfigRef,omitempty"`
	// Cluster holds the overrides of the cluster. It has the shape of the
	// CamundaCluster spec, and it merges over the preset and the release.
	//
	// The API server refuses these fields here. suspend: false and an empty
	// presetRef, releaseRef, storageRef, or backupStorageRef count as unset.
	//   - presetRef, releaseRef, platformConfigRef, and suspend: the claim
	//     has its own fields for them.
	//   - version and connectors.version: the release sets them. Use
	//     allowVersionDowngrade for a downgrade.
	//   - serviceAccount.name and connectors.serviceAccount.name: the claim
	//     controller sets them.
	//   - storageRef and backupStorageRef: the secondary-storage and
	//     backup-storage sections fill them.
	//
	// A selected section fills these fields. The claim controller refuses a
	// claim that sets one of them with reason SlotConflict. Without a
	// selection in that section, the claim can set the field.
	//   - externalUrl: access.
	//   - serviceAccount.annotations: cloud-identity.
	//   - connectors.serviceAccount.annotations: connectors-permissions.
	//   - zeebe.storageClassName: volume-encryption.
	// +kubebuilder:validation:XValidation:rule="(!has(self.presetRef) || self.presetRef == '') && (!has(self.releaseRef) || self.releaseRef == '') && (!has(self.platformConfigRef) || self.platformConfigRef == '') && (!has(self.suspend) || !self.suspend) && (!has(self.version) || self.version == '') && (!has(self.connectors) || !has(self.connectors.version) || self.connectors.version == '') && (!has(self.serviceAccount) || !has(self.serviceAccount.name) || self.serviceAccount.name == '') && (!has(self.connectors) || !has(self.connectors.serviceAccount) || !has(self.connectors.serviceAccount.name) || self.connectors.serviceAccount.name == '') && (!has(self.storageRef) || self.storageRef == '') && (!has(self.backupStorageRef) || self.backupStorageRef == '')",message="spec.cluster must not set presetRef, releaseRef, platformConfigRef, suspend, version, connectors.version, serviceAccount.name, connectors.serviceAccount.name, storageRef, or backupStorageRef: the claim, its release, the claim controller, or a selected section sets them"
	// +optional
	Cluster *v1.CamundaClusterSpec `json:"cluster,omitempty"`
	// Plugins are the registrations that the claim selects, one item for
	// each section and registration. The sections and their registrations
	// are fixed after creation. Only the parameters of a selection can
	// change.
	// +listType=map
	// +listMapKey=section
	// +listMapKey=registration
	// +kubebuilder:validation:MaxItems=32
	// +optional
	Plugins []PluginSelection `json:"plugins,omitempty"`
	// Attachments are the resources that the claim controller attaches to
	// the cluster.
	// +optional
	Attachments *ClusterAttachments `json:"attachments,omitempty"`
	// DeletionPolicy says what happens to the resources of the plugin
	// requests when the claim is deleted. Delete deletes them. Orphan keeps
	// them. Empty means the deletion default that the claim recorded from its
	// Environment at creation, or Delete when it recorded none. The claim
	// controller refuses Orphan with the reason NotAllowed when a selected
	// registration does not support it. The CamundaCluster and the
	// attachments are always deleted with the claim, under their own
	// retention rules.
	// +optional
	DeletionPolicy pluginsv1alpha1.DeletionPolicy `json:"deletionPolicy,omitempty"`
	// AllowVersionDowngrade is the target version of a confirmed downgrade,
	// as a full version such as 8.9.3. Set it to the version of the release
	// of the claim. The claim controller sets the annotation
	// camunda.io/allow-version-downgrade on the CamundaCluster to this
	// version until the cluster reports it. A version that the release does
	// not have sets no annotation, and neither does a CamundaCluster whose
	// spec.version another writer set. While the cluster is suspended, the
	// claim controller neither adds nor removes the annotation, so a
	// downgrade set during a suspension waits for the resume. The fleet
	// manager clears the field after the cluster reports the target version.
	// +kubebuilder:validation:Pattern=`^(\d+\.\d+\.\d+)?$`
	// +optional
	AllowVersionDowngrade string `json:"allowVersionDowngrade,omitempty"`
	// Suspend scales every workload of the cluster to zero and keeps the
	// data.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// ClusterAttachments are the resources that a claim attaches to its cluster.
type ClusterAttachments struct {
	// Optimize adds a CamundaOptimize named <claim>-optimize that reads the
	// cluster. Unset, the claim has no Optimize.
	// +optional
	Optimize *OptimizeAttachment `json:"optimize,omitempty"`
	// BackupSchedule adds a BackupSchedule named <claim>-backups for the
	// cluster. Unset, the claim has no backup schedule.
	// +optional
	BackupSchedule *BackupScheduleAttachment `json:"backupSchedule,omitempty"`
}

// OptimizeAttachment holds the CamundaOptimize fields that a claim can set.
// The claim controller sets the version from the release and the cluster
// reference to the cluster of the claim.
type OptimizeAttachment struct {
	// ManagementAuthConfigRef names the cluster-scoped ManagementAuthConfig
	// that Optimize signs in against. The claim controller writes it to
	// managementAuthRef of the CamundaOptimize.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +required
	ManagementAuthConfigRef string `json:"managementAuthConfigRef"`
	// ExternalURL is the URL where browsers reach this Optimize, with the
	// rules of externalUrl on the CamundaOptimize. The claim controller
	// refuses it with reason SlotConflict when the access section has a
	// selection, because that section fills it.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https') && url(self).getHostname() != ''",message="externalUrl must be a valid http or https URL"
	// +kubebuilder:validation:XValidation:rule="!self.contains(',')",message="externalUrl must carry no comma: Management Identity reads the callback list as comma-separated"
	// +kubebuilder:validation:XValidation:rule="!self.contains('%')",message="externalUrl must carry no percent-encoded character: the operator passes the decoded path to Optimize"
	// +kubebuilder:validation:XValidation:rule="!self.contains('\\\\')",message="externalUrl must carry no backslash: browsers read one in the path as a slash, and the operator does not"
	// +kubebuilder:validation:XValidation:rule="!self.matches('^[^?#]*/[.][.]?([/?#]|$)')",message="externalUrl must carry no . or .. path segment: browsers remove them, and the operator does not"
	// +kubebuilder:validation:XValidation:rule="!self.endsWith('/')",message="externalUrl must not end with a slash: the login callback is appended to it"
	// +kubebuilder:validation:XValidation:rule="!self.contains('?') && !self.contains('#')",message="externalUrl must carry no query and no fragment: the login callback is appended to it"
	// +kubebuilder:validation:XValidation:rule="!self.matches('[[:space:]]')",message="externalUrl must carry no whitespace: Management Identity deletes whitespace from every root URL and the operator does not, so the two would register different callbacks"
	// +optional
	ExternalURL string `json:"externalUrl,omitempty"`
	// Webapp configures the Deployment that serves the Optimize user
	// interface.
	// +optional
	Webapp *v1.WorkloadSpec `json:"webapp,omitempty"`
	// Importer configures the Deployment that imports the cluster data into
	// the Optimize indices. Optimize supports one active importer, so
	// replicas must be 0 or 1.
	// +kubebuilder:validation:XValidation:rule="!has(self.replicas) || self.replicas <= 1",message="importer.replicas must be 0 or 1: Optimize supports one active importer"
	// +optional
	Importer *v1.WorkloadSpec `json:"importer,omitempty"`
	// IndexReplicas is the replica count of each Optimize index. Unset, the
	// storage of the cluster gives the count, as on the CamundaOptimize.
	// +kubebuilder:validation:Minimum=0
	// +optional
	IndexReplicas *int32 `json:"indexReplicas,omitempty"`
	// Monitoring configures the monitoring integrations of Optimize.
	// +optional
	Monitoring *v1.OptimizeMonitoringSpec `json:"monitoring,omitempty"`
}

// BackupScheduleAttachment holds the BackupSchedule fields that a claim can
// set. The claim controller sets the cluster reference to the cluster of the
// claim.
type BackupScheduleAttachment struct {
	// Schedule is when the backups run: a five-field cron expression
	// (minute, hour, day of month, month, day of week), evaluated in UTC.
	// +kubebuilder:validation:Pattern=`^\s*([0-9A-Za-z*?,/-]+\s+){4}[0-9A-Za-z*?,/-]+\s*$`
	// +required
	Schedule string `json:"schedule"`
	// Retained limits how many backups of the schedule stay. Unset, the
	// defaults of the BackupSchedule apply.
	// +optional
	Retained *v1.RetainedBackups `json:"retained,omitempty"`
}

// CamundaClusterClaimStatus is the state of a CamundaClusterClaim. The
// cluster claim controller writes it.
type CamundaClusterClaimStatus struct {
	// ObservedGeneration is the last generation that the cluster claim
	// controller processed.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions report the state of the claim. Ready is False with reason
	// NoEnvironment, UnknownEnvironment, NotAllowed, MissingInput,
	// InvalidReference, SlotConflict, or WiringCycle when a check of the
	// claim fails. Otherwise Ready follows RequestsReady, ClusterReady, and
	// AttachmentsReady. When ClusterReady has the reason Creating, Updating,
	// Failing, Degraded, or Down, its message holds the Ready reason and
	// message of the CamundaCluster. If the CamundaCluster has no Ready
	// condition yet, the message says so. With other reasons, such as
	// Error or PrerequisiteNotMet, the message gives that cause. While the
	// claim is deleted and waits for objects, Ready is False with the reason
	// Deleting, and the message names those objects.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Environment is the Environment of the namespace when the claim was
	// created. It is empty when no Environment applied.
	// +optional
	Environment string `json:"environment,omitempty"`
	// ResolvedPolicy is the policy that the claim recorded from its
	// Environment at creation. A later change of the Environment does not
	// change it.
	// +optional
	ResolvedPolicy *ResolvedPolicy `json:"resolvedPolicy,omitempty"`
	// Requests is the state of each PluginRequest of the claim.
	// +listType=map
	// +listMapKey=name
	// +optional
	Requests []RequestStatus `json:"requests,omitempty"`
	// Cluster names the CamundaCluster of the claim. It is unset until the
	// claim controller writes the cluster.
	// +optional
	Cluster *v1.ClusterRef `json:"cluster,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=`.status.cluster.name`
// +kubebuilder:printcolumn:name="Preset",type=string,JSONPath=`.spec.presetRef`,priority=1
// +kubebuilder:printcolumn:name="Release",type=string,JSONPath=`.spec.releaseRef`,priority=1
// +kubebuilder:printcolumn:name="Environment",type=string,JSONPath=`.status.environment`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="oldSelf.hasValue() || (self.metadata.name.matches('^[a-z]([-a-z0-9]*[a-z0-9])?$') && self.metadata.name.size() <= 40)",message="metadata.name must be a DNS-1035 label of at most 40 characters, because the claim adds a suffix to it in the names of the resources that it writes",optionalOldSelf=true

// CamundaClusterClaim asks for one Camunda cluster. The cluster claim
// controller expands it into a CamundaCluster of the same name, a
// PluginRequest for each selection, and the attachments. The Environment of
// the namespace sets which presets, releases, and registrations the claim can
// use. The name is a DNS-1035 label of at most 40 characters. The claim adds
// a suffix to it in the names of the resources that it writes, such as
// <claim>-es-es-http.
type CamundaClusterClaim struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec is the cluster that the claim asks for
	// +required
	Spec CamundaClusterClaimSpec `json:"spec"`

	// status is the state of the claim
	// +optional
	Status CamundaClusterClaimStatus `json:"status,omitzero"`
}

// GetStatusConditions returns a pointer to the status conditions. The
// component framework stages conditions on the resource through it.
func (in *CamundaClusterClaim) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// GetKind returns the CRD kind. The component framework uses it for event and
// metric recording.
func (in *CamundaClusterClaim) GetKind() string { return "CamundaClusterClaim" }

// SetObservedGeneration records the last reconciled generation in status.
func (in *CamundaClusterClaim) SetObservedGeneration(generation int64) {
	in.Status.ObservedGeneration = generation
}

// +kubebuilder:object:root=true

// CamundaClusterClaimList contains a list of CamundaClusterClaim
type CamundaClusterClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []CamundaClusterClaim `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CamundaClusterClaim{}, &CamundaClusterClaimList{})
}
