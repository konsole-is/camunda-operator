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
	"strconv"

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
	// ComponentWebModeler is the ocf component that holds everything Web
	// Modeler needs: both workloads and the Secret that pairs them. The
	// workloads carry ComponentWebModelerRestapi and
	// ComponentWebModelerWebsockets in their component label.
	ComponentWebModeler = "web-modeler"
	// ComponentWebModelerRestapi is the Web Modeler restapi process.
	ComponentWebModelerRestapi = "web-modeler-restapi"
	// ComponentWebModelerWebsockets is the Web Modeler websockets process.
	ComponentWebModelerWebsockets = "web-modeler-websockets"
	// ComponentSecrets is the Secrets that the operator generates.
	ComponentSecrets = "management-secrets"
	// ComponentMirroredSecrets is the copies of the referenced Secrets that
	// live outside the management namespace.
	ComponentMirroredSecrets = "management-mirrored-secrets"
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
	// AttachmentFieldManager owns the claim annotation that the management
	// cluster writes on an orchestration cluster. It is separate from
	// FieldManager so that a withdrawal removes that annotation and nothing
	// else.
	AttachmentFieldManager = "camunda-operator/camundamanagementcluster-attachment"
	// PingFieldManager owns the Console ping settings that the management
	// cluster writes into spec.extraEnv of an orchestration cluster. Two
	// server-side applies under one manager strip each other's fields, so the
	// ping never shares AttachmentFieldManager with the claim.
	PingFieldManager = "camunda-operator/camundamanagementcluster-ping"
)

// The keys of the Secrets that the operator generates and of the one the
// Keycloak Operator writes.
const (
	// ClientSecretKey holds an identity provider client secret.
	ClientSecretKey = "client-secret"
	// PasswordKey holds the password of the first administrator.
	PasswordKey = "password"
	// KeycloakAdminUsernameKey and KeycloakAdminPasswordKey are the keys of
	// the Secret that the Keycloak Operator writes for the first Keycloak
	// administrator.
	KeycloakAdminUsernameKey = "username"
	KeycloakAdminPasswordKey = "password"
)

// KeycloakServicePort is the HTTP port of the Keycloak that the operator
// renders. The rendered resource sets it as spec.http.httpPort, the port the
// container listens on. It sets no spec.http.serviceHttpPort, so the Service
// that the Keycloak Operator creates publishes the same port. The value is
// the Keycloak default.
const KeycloakServicePort int32 = 8080

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

// The ports of the Console container, and the ports of its Service. Both
// follow the 8.9 Helm chart, which serves the web application on 8080 and the
// health endpoints on 9100, and publishes them on 80 and 9100.
//
// Console templates of the 8.9 Helm chart:
// https://github.com/camunda/camunda-platform-helm/tree/main/charts/camunda-platform-8.9/templates/console
const (
	ConsolePortHTTP              int32 = 8080
	ConsolePortManagement        int32 = 9100
	ConsoleServicePortHTTP       int32 = 80
	ConsoleServicePortManagement int32 = 9100
)

// The ports of the Web Modeler containers. The restapi process serves the
// application on 8081 and its actuator endpoints on a management port of its
// own; the websockets process serves everything on 8060
// (https://docs.camunda.io/docs/self-managed/components/modeler/web-modeler/monitoring/,
// https://docs.camunda.io/docs/self-managed/reference-architecture/kubernetes/#networking).
const (
	WebModelerRestapiPortHTTP       int32 = 8081
	WebModelerRestapiPortManagement int32 = 8091
	WebModelerWebsocketsPortHTTP    int32 = 8060
)

// The Service ports of Web Modeler. Both processes publish HTTP under 80, the
// shape the 8.9 Helm chart uses, and the restapi management port keeps the
// number it answers on. The port forwards of the Helm guides show the two
// Services:
// https://docs.camunda.io/docs/self-managed/deployment/helm/configure/authentication-and-authorization/microsoft-entra/
const (
	WebModelerRestapiServicePortHTTP       int32 = 80
	WebModelerRestapiServicePortManagement int32 = 8091
	WebModelerWebsocketsServicePortHTTP    int32 = 80
)

// The keys of the generated Web Modeler Secrets.
const (
	// PusherAppIDKey, PusherAppKeyKey, and PusherAppSecretKey are the keys of
	// the Secret that pairs the two Web Modeler processes. Both containers
	// read all three, and the two sides must carry the same values.
	PusherAppIDKey     = "app-id"
	PusherAppKeyKey    = "app-key"
	PusherAppSecretKey = "app-secret"
	// WebModelerClusterUserPasswordKey holds the password of the Web Modeler
	// user on one basic-auth orchestration cluster.
	WebModelerClusterUserPasswordKey = "password"
	// WebModelerClusterUserAppliedKey records that the cluster holds the user
	// under the password beside it, with its authorizations. The controller
	// writes it only after both calls succeed, so a Secret without it is a
	// password that never reached the cluster.
	WebModelerClusterUserAppliedKey = "applied"
)

// WebModelerClusterUsername is the user that the operator creates on every
// attached basic-auth orchestration cluster. A person deploying from Web
// Modeler authenticates the cluster with it, instead of with the cluster
// administrator.
const WebModelerClusterUsername = "web-modeler"

// PusherAppID identifies the single Web Modeler application at its WebSocket
// server. Web Modeler serves one tenant, so the value is fixed; it must only
// match on both sides, per "Configuration of the websocket component" on the
// configuration page:
// https://docs.camunda.io/docs/self-managed/components/modeler/web-modeler/configuration/
const PusherAppID = "web-modeler"

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
	keycloakServiceSuffix       = "-service"
	keycloakInitialAdminSuffix  = "-initial-admin"
	keycloakDiscoverySuffix     = "-discovery"
	webModelerClusterUserPrefix = "web-modeler-cluster-"
)

// keycloakDerivedSuffixMax is the longest suffix that the Keycloak Operator
// appends to the name of a Keycloak: -service, -initial-admin, and
// -discovery.
const keycloakDerivedSuffixMax = max(
	len(keycloakServiceSuffix), len(keycloakInitialAdminSuffix), len(keycloakDiscoverySuffix),
)

// clusterUIDPrefixLength is how much of a CamundaCluster UID goes into the
// name of its Web Modeler user Secret. Eight hexadecimal characters keep the
// name readable and tell the clusters of one management plane apart.
const clusterUIDPrefixLength = 8

// IdentityName returns the name of the Management Identity Deployment and
// Service.
func IdentityName(mc *v1.CamundaManagementCluster) string { return suffixed(mc.Name, identitySuffix) }

// IdentityServiceURL returns the URL of Management Identity inside the
// Kubernetes cluster. A component of the management plane calls the Identity
// API over it, so it stays reachable while no browser can reach the external
// URL.
func IdentityServiceURL(mc *v1.CamundaManagementCluster) string {
	return serviceURL(IdentityName(mc), mc.Namespace, IdentityServicePortHTTP)
}

// KeycloakName returns the name of the Keycloak custom resource. It is
// shorter than the other names of the management plane: the Keycloak Operator
// names what it creates after the Keycloak, with a suffix of its own, and
// those names are DNS labels too.
func KeycloakName(mc *v1.CamundaManagementCluster) string {
	limit := validation.DNS1123LabelMaxLength - len(keycloakSuffix) - 1 - keycloakDerivedSuffixMax

	return labels.BoundedName(mc.Name, limit) + "-" + keycloakSuffix
}

// ConsoleName returns the name of the Console Deployment and Service.
func ConsoleName(mc *v1.CamundaManagementCluster) string { return suffixed(mc.Name, consoleSuffix) }

// ConsoleServiceURL returns the URL of Console inside the Kubernetes cluster.
// An orchestration cluster of any namespace reports to it, so it must not
// depend on an Ingress. It is derived from the name of the management cluster
// alone, which lets a caller name the Console of a management plane that
// deploys none, and tell the ping entries of two management planes apart.
func ConsoleServiceURL(mc *v1.CamundaManagementCluster) string {
	return serviceURL(ConsoleName(mc), mc.Namespace, ConsoleServicePortHTTP)
}

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

// KeycloakServiceName returns the name of the Service that the Keycloak
// Operator creates for the Keycloak custom resource. The Keycloak Operator
// names it after the resource, with a "-service" suffix.
func KeycloakServiceName(mc *v1.CamundaManagementCluster) string {
	return KeycloakName(mc) + keycloakServiceSuffix
}

// KeycloakInitialAdminSecretName returns the name of the Secret that the
// Keycloak Operator writes with the first Keycloak administrator. Management
// Identity signs in with it to create the realm, the clients, and the initial
// administrator.
func KeycloakInitialAdminSecretName(mc *v1.CamundaManagementCluster) string {
	return KeycloakName(mc) + keycloakInitialAdminSuffix
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

// serviceURL returns the HTTP URL of one Service of the management plane, as a
// pod of any namespace reaches it.
func serviceURL(name, namespace string, port int32) string {
	return "http://" + name + "." + namespace + ".svc:" + strconv.Itoa(int(port))
}

// shortUID returns the head of a UID, or the whole UID when it is shorter.
func shortUID(uid types.UID) string {
	if len(uid) <= clusterUIDPrefixLength {
		return string(uid)
	}

	return string(uid)[:clusterUIDPrefixLength]
}
