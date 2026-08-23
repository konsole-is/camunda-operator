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

package camundamanagementcluster

import (
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// The component values of the camunda.io/component label: one per workload of
// the management plane, and one for the contract it writes.
const (
	// ComponentKeycloak is the Keycloak that the operator runs through the
	// Keycloak Operator.
	ComponentKeycloak = "keycloak"
	// ComponentIdentity is Management Identity.
	ComponentIdentity = "management-identity"
	// ComponentConsole is Console.
	ComponentConsole = "console"
	// ComponentWebModelerRestapi is the Web Modeler restapi process.
	ComponentWebModelerRestapi = "web-modeler-restapi"
	// ComponentWebModelerWebsockets is the Web Modeler websockets process.
	ComponentWebModelerWebsockets = "web-modeler-websockets"
	// ComponentSecrets is the Secrets that the operator generates.
	ComponentSecrets = "management-secrets"
	// ComponentManagementAuth is the ManagementAuthConfig that the management
	// cluster writes. The contract is not a workload, so a selector on the
	// owner and this value reaches the contract alone.
	ComponentManagementAuth = "management-auth"
)

// The keys and identities that a user or another controller can observe on
// what this operator writes.
const (
	// ConfigHashAnnotation is the pod template annotation that carries the
	// hash of the rendered configuration. A change to it rolls the pods. It is
	// the same annotation key that the CamundaCluster workloads carry.
	ConfigHashAnnotation = "camunda.io/config-hash"
	// ClaimAnnotation is the annotation that a management cluster puts on
	// each orchestration cluster it serves. Its value is
	// "<namespace>/<name>" of the CamundaManagementCluster.
	ClaimAnnotation = labels.ManagementClusterKey
	// InitialClaimAnnotation records the initial administrator claim that
	// Management Identity started with, as "<claimName>=<claimValue>".
	// Identity reads the claim on its first start only and stores the result
	// in its database, so the operator keeps rendering the recorded value.
	InitialClaimAnnotation = "camunda.io/identity-initial-claim"
	// FieldManager owns every resource of the management cluster itself, the
	// ManagementAuthConfig included.
	FieldManager = "camunda-operator/camundamanagementcluster"
	// AttachmentFieldManager owns what the management cluster writes on an
	// orchestration cluster: the claim annotation and the Console ping
	// settings. It is separate from FieldManager so that a withdrawal removes
	// those fields and nothing else.
	AttachmentFieldManager = "camunda-operator/camundamanagementcluster-attachment"
)

// The ports of the Management Identity container. The HTTP port is the
// default SERVER_PORT of the image, which IDENTITY_URL also defaults to, and
// the management port serves /actuator
// (https://docs.camunda.io/docs/self-managed/components/management-identity/miscellaneous/application-monitoring/).
const (
	IdentityPortHTTP       int32 = 8080
	IdentityPortManagement int32 = 8082
)

// The Service ports of Management Identity. They follow the ports of the
// container, in the shape the 8.9 Helm chart publishes: 80 for HTTP and the
// management port under 82.
const (
	IdentityServicePortHTTP       int32 = 80
	IdentityServicePortManagement int32 = 82
)

// The workload name suffixes, one per component.
const (
	identitySuffix              = "identity"
	keycloakSuffix              = "keycloak"
	consoleSuffix               = "console"
	webModelerRestapiSuffix     = "web-modeler-restapi"
	webModelerWebsocketsSuffix  = "web-modeler-websockets"
	identityClientSuffix        = "identity-client"
	optimizeClientSuffix        = "optimize-client"
	identityAdminSuffix         = "identity-admin"
	pusherSuffix                = "web-modeler-pusher"
	webModelerClusterUserPrefix = "web-modeler-cluster-"
)

// clusterUIDPrefixLength is how much of a CamundaCluster UID goes into the
// name of its Web Modeler user Secret. Eight hexadecimal characters keep the
// name readable and tell the clusters of one management plane apart.
const clusterUIDPrefixLength = 8

// IdentityName returns the name of the Management Identity Deployment and
// Service.
func IdentityName(mc *v1.CamundaManagementCluster) string { return suffixed(mc.Name, identitySuffix) }

// KeycloakName returns the name of the Keycloak custom resource. The Keycloak
// Operator names the Service it creates for it "<this>-service".
func KeycloakName(mc *v1.CamundaManagementCluster) string { return suffixed(mc.Name, keycloakSuffix) }

// ConsoleName returns the name of the Console Deployment and Service.
func ConsoleName(mc *v1.CamundaManagementCluster) string { return suffixed(mc.Name, consoleSuffix) }

// WebModelerRestapiName returns the name of the Web Modeler restapi
// Deployment and Service.
func WebModelerRestapiName(mc *v1.CamundaManagementCluster) string {
	return suffixed(mc.Name, webModelerRestapiSuffix)
}

// WebModelerWebsocketsName returns the name of the Web Modeler websockets
// Deployment and Service.
func WebModelerWebsocketsName(mc *v1.CamundaManagementCluster) string {
	return suffixed(mc.Name, webModelerWebsocketsSuffix)
}

// IdentityClientSecretName returns the name of the generated Secret that
// holds the Management Identity client secret. The operator generates it in
// the Keycloak modes only.
func IdentityClientSecretName(mc *v1.CamundaManagementCluster) string {
	return suffixed(mc.Name, identityClientSuffix)
}

// OptimizeClientSecretName returns the name of the generated Secret that
// holds the Optimize client secret. The operator generates it in the Keycloak
// modes only.
func OptimizeClientSecretName(mc *v1.CamundaManagementCluster) string {
	return suffixed(mc.Name, optimizeClientSuffix)
}

// IdentityAdminSecretName returns the name of the generated Secret that holds
// the password of the first administrator. The operator generates it in the
// Keycloak modes only, and only when identity.admin names no Secret of its
// own.
func IdentityAdminSecretName(mc *v1.CamundaManagementCluster) string {
	return suffixed(mc.Name, identityAdminSuffix)
}

// PusherSecretName returns the name of the generated Secret that holds the
// credentials the two Web Modeler processes push live updates over.
func PusherSecretName(mc *v1.CamundaManagementCluster) string {
	return suffixed(mc.Name, pusherSuffix)
}

// WebModelerClusterUserSecretName returns the name of the generated Secret
// that holds the password of the Web Modeler user on one basic-auth
// orchestration cluster. The UID tells two clusters of the same name in
// different namespaces apart.
func WebModelerClusterUserSecretName(mc *v1.CamundaManagementCluster, uid types.UID) string {
	return suffixed(mc.Name, webModelerClusterUserPrefix+shortUID(uid))
}

// ContractName returns the name of the cluster-scoped ManagementAuthConfig
// that this management cluster writes: spec.managementAuthConfigName, or the
// name of the resource when that is empty.
func ContractName(mc *v1.CamundaManagementCluster) string {
	if mc.Spec.ManagementAuthConfigName != "" {
		return mc.Spec.ManagementAuthConfigName
	}

	return mc.Name
}

// suffixed joins the name of the management cluster and a suffix with a dash.
// The Service is the tightest bound of the resources that carry these names, a
// DNS label of 63 characters, so a long name truncates and keeps its identity
// in a hash.
func suffixed(name, suffix string) string {
	return labels.BoundedName(name, validation.DNS1123LabelMaxLength-len(suffix)-1) + "-" + suffix
}

// shortUID returns the head of a UID, or the whole UID when it is shorter.
func shortUID(uid types.UID) string {
	if len(uid) <= clusterUIDPrefixLength {
		return string(uid)
	}

	return string(uid)[:clusterUIDPrefixLength]
}
