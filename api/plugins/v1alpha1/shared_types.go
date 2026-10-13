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
)

// The labels and the finalizer of the plugins group.
const (
	// LabelSection names the PluginSection of a PluginRequest.
	LabelSection = "plugins.camunda.io/section"
	// LabelRegistration names the PluginRegistration that a PluginRequest,
	// a Grant, or a resource that a plugin writes belongs to.
	LabelRegistration = "plugins.camunda.io/registration"
	// LabelRequest names the PluginRequest that a Grant or a resource that a
	// plugin writes belongs to.
	LabelRequest = "plugins.camunda.io/request"
	// LabelCluster names the CamundaCluster whose identity a Grant changes.
	LabelCluster = "plugins.camunda.io/cluster"
	// FinalizerCleanup is the finalizer that the plugin controller holds on a
	// PluginRequest until it applied the deletion policy of the request.
	FinalizerCleanup = "plugins.camunda.io/cleanup"
)

// DeletionPolicy says what happens to the resources of a request when the
// request is deleted.
// +kubebuilder:validation:Enum=Delete;Orphan
type DeletionPolicy string

const (
	// DeletionPolicyDelete deletes the resources that the plugin wrote.
	DeletionPolicyDelete DeletionPolicy = "Delete"
	// DeletionPolicyOrphan keeps the resources and removes their owner
	// reference to the request.
	DeletionPolicyOrphan DeletionPolicy = "Orphan"
)

// Mode is the fallback of a registration when no Environment exists.
// +kubebuilder:validation:Enum=Default;Optional
type Mode string

const (
	// ModeDefault selects the option in a new claim.
	ModeDefault Mode = "Default"
	// ModeOptional offers the option, and a new claim does not select it.
	ModeOptional Mode = "Optional"
)

// Selection says how many options of a section one claim can select.
// +kubebuilder:validation:Enum=One;Many
type Selection string

const (
	// SelectionOne lets a claim select at most one option of the section.
	SelectionOne Selection = "One"
	// SelectionMany lets a claim select more than one option of the section.
	SelectionMany Selection = "Many"
)

// Slot names the fields of the generated resource that the outputs of a
// section fill. The set is closed.
// +kubebuilder:validation:Enum=workloadIdentity;volumeEncryption;backupStorage;secondaryStorage;externalAccess;managementDatabases;connectorsIdentity
type Slot string

const (
	// SlotWorkloadIdentity fills the ServiceAccount annotations of the cluster.
	SlotWorkloadIdentity Slot = "workloadIdentity"
	// SlotVolumeEncryption fills the storage class of the Zeebe volumes.
	SlotVolumeEncryption Slot = "volumeEncryption"
	// SlotBackupStorage fills the backup storage reference of the cluster.
	SlotBackupStorage Slot = "backupStorage"
	// SlotSecondaryStorage fills the secondary storage reference of the cluster.
	SlotSecondaryStorage Slot = "secondaryStorage"
	// SlotExternalAccess fills the external URLs of a cluster or a
	// management cluster.
	SlotExternalAccess Slot = "externalAccess"
	// SlotManagementDatabases fills the database references of Keycloak,
	// Identity, and Web Modeler in a management cluster.
	SlotManagementDatabases Slot = "managementDatabases"
	// SlotConnectorsIdentity fills the ServiceAccount annotations of the
	// Connectors.
	SlotConnectorsIdentity Slot = "connectorsIdentity"
)

// ClaimKind names a claim kind of the fleet.camunda.io group.
// +kubebuilder:validation:Enum=CamundaClusterClaim;CamundaManagementClusterClaim
type ClaimKind string

const (
	// ClaimKindCamundaClusterClaim is the claim of a CamundaCluster.
	ClaimKindCamundaClusterClaim ClaimKind = "CamundaClusterClaim"
	// ClaimKindCamundaManagementClusterClaim is the claim of a
	// CamundaManagementCluster.
	ClaimKindCamundaManagementClusterClaim ClaimKind = "CamundaManagementClusterClaim"
)

// GrantFormat is the format of the fragment of a grant. A later version adds
// gcp-iam-binding and azure-role-assignment.
// +kubebuilder:validation:Enum=aws-iam-statement
type GrantFormat string

// GrantFormatAWSIAMStatement is one AWS IAM policy statement.
const GrantFormatAWSIAMStatement GrantFormat = "aws-iam-statement"

// Severity is the weight of a summary item.
// +kubebuilder:validation:Enum=Info;Warning;Error
type Severity string

const (
	// SeverityInfo is a fact that needs no action.
	SeverityInfo Severity = "Info"
	// SeverityWarning is a fact that needs attention soon.
	SeverityWarning Severity = "Warning"
	// SeverityError is a fact that needs action now.
	SeverityError Severity = "Error"
)

// OnOrphan says what happens to a resource of a request with the deletion
// policy Orphan.
// +kubebuilder:validation:Enum=Kept;Deleted
type OnOrphan string

const (
	// OnOrphanKept keeps the resource.
	OnOrphanKept OnOrphan = "Kept"
	// OnOrphanDeleted deletes the resource, for example a route to a Service
	// that is gone.
	OnOrphanDeleted OnOrphan = "Deleted"
)

// ControllerInfo identifies the plugin controller that serves a registration.
type ControllerInfo struct {
	// Name is the name of the plugin controller.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Version is the version of the plugin controller, from its build info.
	// +optional
	Version string `json:"version,omitempty"`
}

// InputDecl declares a value that a registration reads from the output of
// another section.
type InputDecl struct {
	// Name is the name of the input. The request carries its value under
	// this name.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name"`
	// From names the output that gives the value, in the form
	// <section>/<output>.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*/[A-Za-z][A-Za-z0-9]*$`
	From string `json:"from"`
	// Required refuses a selection of the option when the input has no value.
	// +optional
	Required bool `json:"required,omitempty"`
	// Formats lists the formats of the value that the registration accepts,
	// for example aws-kms-arn. Empty accepts every format.
	// +optional
	Formats []string `json:"formats,omitempty"`
}

// OutputDecl declares a value that a plugin writes to the status of a request.
type OutputDecl struct {
	// Key is the key of the output in the status of the request. It starts
	// with a letter and holds only letters and digits.
	// +kubebuilder:validation:Pattern=`^[A-Za-z][A-Za-z0-9]*$`
	Key string `json:"key"`
	// Description says what the value is.
	// +optional
	Description string `json:"description,omitempty"`
	// Format is the format of the value, for example aws-kms-arn.
	// +optional
	Format string `json:"format,omitempty"`
}

// GrantTemplate is one permission statement that a registration asks for,
// with placeholders that the plugin fills for each request.
type GrantTemplate struct {
	// Name is the name of the template. A Grant names the template it was
	// rendered from.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Format is the format of the statement.
	Format GrantFormat `json:"format"`
	// Template is the statement, with Go template placeholders such as
	// {{ .parameters.x }} and {{ .outputs.x }}.
	Template *apiextensionsv1.JSON `json:"template"`
	// Description says what the permission is for.
	// +optional
	Description string `json:"description,omitempty"`
	// RequiresInputs names the inputs that must have a value before the
	// plugin writes the grant.
	// +kubebuilder:validation:MaxItems=32
	// +kubebuilder:validation:items:MaxLength=63
	// +optional
	RequiresInputs []string `json:"requiresInputs,omitempty"`
}

// Choice is a preset or an existing resource that a claim can pick for a
// parameter of a registration.
type Choice struct {
	// Name is the value that the claim stores when it picks the choice.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Specs are the facts that the fleet manager shows for the choice, for
	// example the version, the node count, or the memory.
	// +optional
	Specs []ChoiceSpec `json:"specs,omitempty"`
}

// ChoiceSpec is one fact of a choice.
type ChoiceSpec struct {
	// Label names the fact.
	// +kubebuilder:validation:MinLength=1
	Label string `json:"label"`
	// Value is the value of the fact.
	Value string `json:"value"`
}

// MetricQuery is a Prometheus query that the fleet manager shows for a
// request of the registration.
type MetricQuery struct {
	// Label names the metric.
	// +kubebuilder:validation:MinLength=1
	Label string `json:"label"`
	// Query is PromQL with Go template placeholders, for example
	// {{ .namespace }} and {{ .pods }}.
	// +kubebuilder:validation:MinLength=1
	Query string `json:"query"`
	// Unit is the unit of the value, for example bytes.
	// +optional
	Unit string `json:"unit,omitempty"`
}

// ClaimRef names the claim that wrote a request.
type ClaimRef struct {
	// Kind is the kind of the claim.
	Kind ClaimKind `json:"kind"`
	// Name is the name of the claim, in the namespace of the request.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// TargetRef names the resource that a request serves. The claim names it
// before the resource exists.
type TargetRef struct {
	// Kind is the kind of the resource, for example CamundaCluster.
	// +kubebuilder:validation:MinLength=1
	Kind string `json:"kind"`
	// Name is the name of the resource, in the namespace of the request.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// ServiceAccountName is the ServiceAccount that the pods of the resource
	// run under.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
	// AdditionalServiceAccounts are the names of more ServiceAccounts that
	// the cloud identity of the request also trusts. Examples are the
	// ServiceAccounts of the built-in storage that takes the backup storage
	// input. Each name is a ServiceAccount in the namespace of the request. A name with a namespace part is refused,
	// so that a request never makes a cloud identity trust a ServiceAccount
	// of another namespace.
	// +kubebuilder:validation:items:MaxLength=253
	// +kubebuilder:validation:items:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	// +optional
	AdditionalServiceAccounts []string `json:"additionalServiceAccounts,omitempty"`
}

// SummaryItem is one fact that a plugin reports about a request.
type SummaryItem struct {
	// Label names the fact.
	// +kubebuilder:validation:MinLength=1
	Label string `json:"label"`
	// Value is the value of the fact.
	Value string `json:"value"`
	// Severity is the weight of the fact. The fleet manager lists Warning
	// and Error items as warnings.
	// +kubebuilder:default=Info
	// +optional
	Severity Severity `json:"severity,omitempty"`
}

// Link is a named URL that a plugin reports for a request, for example the
// console page of a bucket.
type Link struct {
	// Name names the link.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// URL is the address of the link.
	// +kubebuilder:validation:MinLength=1
	URL string `json:"url"`
}

// ResourceRef is one resource that a plugin made for a request, in the
// cluster or in a cloud.
type ResourceRef struct {
	// Kind is the kind of the resource, for example S3Bucket or StorageClass.
	// +kubebuilder:validation:MinLength=1
	Kind string `json:"kind"`
	// Name is the name of the resource.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// ID is the identifier of the resource in its system, for example an ARN.
	// +optional
	ID string `json:"id,omitempty"`
	// Location is where the resource is, for example a cloud region.
	// +optional
	Location string `json:"location,omitempty"`
	// Role says what the resource does for the request.
	// +optional
	Role string `json:"role,omitempty"`
	// OnOrphan says what happens to the resource with the deletion policy
	// Orphan.
	OnOrphan OnOrphan `json:"onOrphan"`
}
