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

// The condition vocabulary that only CamundaManagementCluster reports. The
// shared vocabulary is in conditions.go.
const (
	// ConditionKeycloakReady is the condition of the Keycloak that the
	// operator runs. It reads Disabled in the externalKeycloak and the oidc
	// mode, which run none.
	ConditionKeycloakReady = "KeycloakReady"
	// ConditionIdentityReady is the condition of the Management Identity
	// workload.
	ConditionIdentityReady = "IdentityReady"
	// ConditionConsoleReady is the condition of the Console workload. It
	// reads Disabled while console is unset.
	ConditionConsoleReady = "ConsoleReady"
	// ConditionWebModelerReady is the condition of the two Web Modeler
	// workloads. It reads Disabled while webModeler is unset.
	ConditionWebModelerReady = "WebModelerReady"
	// ConditionManagementAuthReady is the condition of the
	// ManagementAuthConfig that this management cluster writes.
	ConditionManagementAuthReady = "ManagementAuthReady"
	// ConditionSecretsReady is the condition of the Secrets that the operator
	// generates: the client secrets and the initial admin password. It reads
	// Disabled in the oidc mode, where the platform config names every client
	// secret and the first administrator is a token claim.
	ConditionSecretsReady = "SecretsReady"
	// ConditionOptimizeCallbacksReady reports what the operator did about the
	// login callback of every Optimize in status.optimize. It reads Healthy
	// once the Optimize client of the realm carries all of them and the first
	// administrator holds the Optimize role of the realm.
	//
	// Two reasons read True without saying anything about the realm. Disabled
	// is the oidc mode, where the identity provider of the platform config
	// holds the callback URLs and the operator administers nothing. Suspended
	// is a plane that spec.suspend scaled to zero, whose realm is left as it
	// is and never read.
	ConditionOptimizeCallbacksReady = "OptimizeCallbacksReady"

	// ReasonKeycloakOperatorNotInstalled means that spec.identityProvider
	// selects keycloak and that the Kubernetes cluster does not serve the
	// Keycloak kind of the Keycloak Operator. Install the Keycloak Operator,
	// or select another mode.
	ReasonKeycloakOperatorNotInstalled = "KeycloakOperatorNotInstalled"
	// ReasonConflict means that a cluster-scoped object of the name this
	// management cluster writes already exists and belongs to another owner.
	// The message names the object. Set managementAuthConfigName to a free
	// name, or remove the object.
	ReasonConflict = "Conflict"
	// ReasonWriteFailed means that a write of the operator was refused: the
	// ManagementAuthConfig on Ready, and the Optimize client of the realm on
	// OptimizeCallbacksReady. The message carries what the API server or
	// Keycloak answered. The operator tries again.
	ReasonWriteFailed = "WriteFailed"
	// ReasonStepFailed means that a step of the reconcile did not finish. A
	// step is work that the management plane does outside its workloads,
	// almost always a call to the Kubernetes API. The message names the step
	// and carries what went wrong. Ready is False for that pass even when
	// every workload is healthy, because the operator did not get to the end
	// of its work. Every other condition keeps the value it last had. The
	// operator tries again.
	ReasonStepFailed = "StepFailed"
	// ReasonUnsupportedVersion means that a version in the spec is outside
	// the range that the operator supports: below the floor of its component,
	// or, for the Keycloak that the operator runs, at or above the ceiling.
	// The message names the field and the bound it crossed.
	ReasonUnsupportedVersion = "UnsupportedVersion"
	// ReasonClaimedElsewhere means that a selected CamundaCluster is already
	// claimed by another management cluster. One cluster answers to one
	// management plane, so this operator leaves the cluster untouched. The
	// message names the holder.
	ReasonClaimedElsewhere = "ClaimedElsewhere"
	// ReasonNotReady means that a selected CamundaCluster is not attached
	// yet: it publishes no gateway endpoints, so Web Modeler cannot deploy to
	// it, or it changed while the operator claimed it. The state clears when
	// the cluster settles.
	ReasonNotReady = "NotReady"
	// ReasonImmutableAfterStart means that identity.admin changed after
	// Management Identity started. Identity stores the initial administrator
	// in its database and reads the setting only on the first start, so the
	// operator refuses the change instead of rendering a value that has no
	// effect.
	ReasonImmutableAfterStart = "ImmutableAfterStart"
	// ReasonBasicAuthUserFailed means that the operator could not create the
	// Web Modeler user on a basic-auth CamundaCluster. The row of that
	// cluster in status.clusters carries the message.
	ReasonBasicAuthUserFailed = "BasicAuthUserFailed"
	// ReasonOptimizeClientMissing means that the realm holds no Optimize
	// client and Management Identity has finished starting. Identity creates
	// that client while it starts and never after, so the client comes back
	// only when Management Identity starts again. While Identity is still
	// rolling out the condition reports PrerequisiteNotMet instead. The
	// operator looks again on its retry interval.
	ReasonOptimizeClientMissing = "OptimizeClientMissing"
	// ReasonNoCallbacks means that no Optimize behind this management plane
	// names a URL, so there is no login callback to register. Give a
	// CamundaOptimize that names this management plane a spec.externalUrl.
	ReasonNoCallbacks = "NoCallbacks"
	// ReasonAdminRoleGrantFailed means that the operator could not give the
	// first administrator the Optimize role of the realm. The message says
	// what stopped it: the realm holds no user of that name, it holds no
	// Optimize role, or Keycloak refused the write. The operator tries again
	// on its retry interval.
	ReasonAdminRoleGrantFailed = "AdminRoleGrantFailed"

	// ReasonInvalidCABundle means that the key of caBundleSecretRef holds no
	// certificate in PEM form, so the operator cannot verify the certificate
	// of Keycloak. Put the certificate authority of Keycloak in that key, in
	// PEM form.
	ReasonInvalidCABundle = "InvalidCABundle"
)

// CamundaManagementClusterSpec describes one management plane: Management
// Identity, its identity provider, and optionally Console and Web Modeler.
// +kubebuilder:validation:XValidation:rule="has(self.identityProvider.oidc) ? has(self.identity.admin.claimName) : has(self.identity.admin.username)",message="identity.admin: set claimName and claimValue in oidc mode, username in the keycloak modes"
// +kubebuilder:validation:XValidation:rule="!has(self.identityProvider.oidc) || !has(self.identity.admin.passwordSecretRef)",message="identity.admin.passwordSecretRef applies to the keycloak modes only"
// +kubebuilder:validation:XValidation:rule="!has(self.webModeler) || has(self.identityProvider.oidc) || has(self.identity.admin.email)",message="identity.admin.email is required when webModeler is set in a keycloak mode"
type CamundaManagementClusterSpec struct {
	// PlatformConfigRef names the cluster-scoped CamundaPlatformConfig that
	// carries the license, the image settings, and, in the oidc mode, the
	// identity provider and every client of the management plane.
	// +kubebuilder:validation:MinLength=1
	PlatformConfigRef string `json:"platformConfigRef"`
	// Suspend scales every workload of this management cluster to zero. The
	// ManagementAuthConfig, the claims on the orchestration clusters, and the
	// Console ping settings stay, so nothing else has to change while the
	// management plane is down.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
	// ClusterSelector selects the CamundaClusters, in every namespace that
	// namespaceSelector admits, that Console and Web Modeler serve. It
	// follows the Kubernetes label selector convention: an unset selector
	// selects no cluster, and an empty selector ({}) selects every cluster.
	// +optional
	ClusterSelector *metav1.LabelSelector `json:"clusterSelector,omitempty"`
	// NamespaceSelector narrows clusterSelector to the namespaces whose
	// labels match. It selects on the labels of the Namespace objects, the
	// way the namespaceSelector of an admission webhook does. An unset or
	// empty ({}) selector puts no bound on the namespace, so clusterSelector
	// alone decides.
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`
	// ManagementAuthConfigName is the name of the cluster-scoped
	// ManagementAuthConfig that this management cluster writes. A
	// CamundaOptimize reads it through its managementAuthRef. Empty means the
	// name of this resource.
	// +optional
	ManagementAuthConfigName string `json:"managementAuthConfigName,omitempty"`
	// IdentityProvider selects where users authenticate. Exactly one of
	// keycloak, externalKeycloak, or oidc is set.
	// +kubebuilder:validation:XValidation:rule="[has(self.keycloak), has(self.externalKeycloak), has(self.oidc)].filter(x, x).size() == 1",message="exactly one identity provider: keycloak, externalKeycloak, or oidc"
	IdentityProvider IdentityProviderSpec `json:"identityProvider"`
	// Identity configures Management Identity. Console, Web Modeler, and
	// Optimize all authenticate through it, so it is always deployed.
	Identity IdentitySpec `json:"identity"`
	// Console configures Console. Console is not deployed while this is unset.
	// +optional
	Console *ConsoleSpec `json:"console,omitempty"`
	// WebModeler configures Web Modeler. Web Modeler is not deployed while
	// this is unset.
	// +optional
	WebModeler *WebModelerSpec `json:"webModeler,omitempty"`
}

// IdentityProviderSpec holds one of the three identity provider modes.
type IdentityProviderSpec struct {
	// Keycloak runs Keycloak through the Keycloak Operator. The Keycloak
	// Operator must be installed on the Kubernetes cluster.
	// +optional
	Keycloak *ManagedKeycloakSpec `json:"keycloak,omitempty"`
	// ExternalKeycloak connects Management Identity to a Keycloak that you
	// run. Management Identity still creates the realm, the clients, and the
	// initial administrator in it.
	// +optional
	ExternalKeycloak *ExternalKeycloakSpec `json:"externalKeycloak,omitempty"`
	// OIDC connects Management Identity to the identity provider of the
	// referenced CamundaPlatformConfig.
	// +optional
	OIDC *ManagementOIDCSpec `json:"oidc,omitempty"`
}

// ManagedKeycloakSpec configures the Keycloak that the operator runs through
// the Keycloak Operator.
type ManagedKeycloakSpec struct {
	// Version is the Keycloak version, as a full semantic version. Camunda
	// 8.9 supports Keycloak 26 only. The image is
	// camunda/keycloak:quay-optimized-<version> unless the platform config
	// overrides the repository.
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+$`
	Version string `json:"version"`
	// ExternalURL is the URL that browsers reach Keycloak at, including the
	// /auth path. It is the front-channel issuer of every token. Management
	// Identity uses the front-channel URL since 8.5.3, so the Identity pods
	// must also reach it. Management Identity administers Keycloak through
	// the Service that the Keycloak Operator creates, not through this URL.
	//
	// The operator appends /realms/<realm> to this URL, so the URL carries no
	// query and no fragment.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https') && url(self).getHostname() != ''",message="externalUrl must be a valid http or https URL"
	// +kubebuilder:validation:XValidation:rule="!isURL(self) || url(self).getEscapedPath() == '/auth'",message="externalUrl must carry the /auth path, for example https://keycloak.example.com/auth"
	// +kubebuilder:validation:XValidation:rule="!self.contains('?') && !self.contains('#')",message="externalUrl must carry no query and no fragment"
	ExternalURL string `json:"externalUrl"`
	// DatabaseConfigRef names the DatabaseConfig of the Keycloak database, in
	// the namespace of this resource. Keycloak needs its own PostgreSQL
	// database.
	// +kubebuilder:validation:MinLength=1
	DatabaseConfigRef string `json:"databaseConfigRef"`
	// Replicas is the number of Keycloak instances. Defaults to 1.
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// Resources are the CPU and memory of the Keycloak container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// Scheduling constraints of the Keycloak pods.
	// +optional
	Scheduling *SchedulingSpec `json:"scheduling,omitempty"`
}

// ExternalKeycloakSpec connects Management Identity to a Keycloak that you
// run.
// +kubebuilder:validation:XValidation:rule="!has(self.caBundleSecretRef) || !isURL(self.url) || url(self.url).getScheme() == 'https'",message="caBundleSecretRef requires an https url"
type ExternalKeycloakSpec struct {
	// URL is the URL of Keycloak, including the /auth path when it has one.
	// Management Identity reaches this URL, so it must resolve from inside
	// the Kubernetes cluster.
	//
	// The operator appends /realms/<realm> to this URL, so the URL carries no
	// query and no fragment.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https') && url(self).getHostname() != ''",message="url must be a valid http or https URL"
	// +kubebuilder:validation:XValidation:rule="!self.contains('?') && !self.contains('#')",message="url must carry no query and no fragment"
	URL string `json:"url"`
	// Realm is the realm that Management Identity uses and creates. Empty
	// means camunda-platform. The realm lands in the issuer, the token, and
	// the JWKS path of every URL that Management Identity builds, so it holds
	// letters, digits, dots, hyphens, and underscores only.
	// +kubebuilder:validation:XValidation:rule="self == '' || self.matches('^[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?$')",message="realm must hold letters, digits, dots, hyphens, and underscores, and start and end with a letter or a digit"
	// +optional
	Realm string `json:"realm,omitempty"`
	// AdminCredentialsSecretRef names the Secret with the Keycloak
	// administrator credentials. Management Identity uses them to create the
	// realm, the clients, and the initial administrator.
	AdminCredentialsSecretRef LocalCredentialsSecretRef `json:"adminCredentialsSecretRef"`
	// CABundleSecretRef names the Secret key that holds the certificate
	// authority of Keycloak, in PEM form. The operator trusts it in addition
	// to the trust store of its own image when it signs in to Keycloak to
	// register the login callbacks of Optimize. Set it when Keycloak serves a
	// certificate that a public authority did not sign. It is only valid with
	// an https url.
	// +optional
	CABundleSecretRef *LocalSecretKeyRef `json:"caBundleSecretRef,omitempty"`
}

// ManagementOIDCSpec selects the identity provider of the referenced
// CamundaPlatformConfig. The clients of the management plane live there, under
// spec.auth.oidc.management.clients, so this block carries no fields.
type ManagementOIDCSpec struct{}

// IdentitySpec configures Management Identity.
type IdentitySpec struct {
	// Version is the Management Identity version, as a full semantic version.
	// The operator supports 8.9.0 and later.
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+$`
	Version string `json:"version"`
	// ExternalURL is the URL that browsers reach Management Identity at.
	// Identity registers it as the redirect URI of its own client.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https') && url(self).getHostname() != ''",message="externalUrl must be a valid http or https URL"
	ExternalURL string `json:"externalUrl"`
	// DatabaseConfigRef names the DatabaseConfig of the Management Identity
	// database, in the namespace of this resource. Identity needs its own
	// PostgreSQL database.
	// +kubebuilder:validation:MinLength=1
	DatabaseConfigRef string `json:"databaseConfigRef"`
	// Admin names the first administrator of the management plane.
	Admin        IdentityAdminSpec `json:"admin"`
	WorkloadSpec `json:",inline"`
}

// IdentityAdminSpec names the first administrator of the management plane.
// Management Identity reads it on the first start only and stores the result
// in its database.
//
// In the oidc mode the administrator is a claim of the tokens that the
// provider issues, so set claimName and claimValue; a later change of the
// claim reports ImmutableAfterStart. In the two Keycloak modes the
// administrator is the first Keycloak user, so set username; a later change
// creates a second user and the first one keeps its access.
// +kubebuilder:validation:XValidation:rule="(has(self.claimName) && has(self.claimValue)) != has(self.username)",message="set claimName and claimValue (oidc mode) or username (keycloak modes)"
// +kubebuilder:validation:XValidation:rule="has(self.claimName) == has(self.claimValue)",message="set claimName and claimValue together"
type IdentityAdminSpec struct {
	// ClaimName is the token claim that identifies the administrator, for
	// example oid or sub. Set it in the oidc mode. The operator records the
	// claim as <claimName>=<claimValue> and reads it back at the first equals
	// sign, so the name holds no equals sign.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="!self.contains('=')",message="claimName must not hold an equals sign"
	// +optional
	ClaimName string `json:"claimName,omitempty"`
	// ClaimValue is the value that the claim carries for the administrator.
	// Set it in the oidc mode.
	// +kubebuilder:validation:MinLength=1
	// +optional
	ClaimValue string `json:"claimValue,omitempty"`
	// Username is the name of the first Keycloak user. Set it in the keycloak
	// and the externalKeycloak mode. Management Identity creates the user on
	// its first start, so a later change to this field creates a second user
	// rather than renaming the first one.
	// +kubebuilder:validation:MinLength=1
	// +optional
	Username string `json:"username,omitempty"`
	// PasswordSecretRef names the Secret key that holds the password of the
	// first Keycloak user. The operator generates a password when this is
	// unset.
	// +optional
	PasswordSecretRef *LocalSecretKeyRef `json:"passwordSecretRef,omitempty"`
	// Email is the email address of the first Keycloak user. Web Modeler
	// needs an address for every person who signs in, so it is required when
	// webModeler is set in a Keycloak mode.
	// +kubebuilder:validation:MinLength=3
	// +optional
	Email string `json:"email,omitempty"`
}

// ConsoleSpec configures Console.
type ConsoleSpec struct {
	// Version is the Console version, as a full semantic version. The
	// operator supports 8.9.0 and later.
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+$`
	Version string `json:"version"`
	// ExternalURL is the URL that browsers reach Console at. Console serves
	// under the path of this URL. Every selected CamundaCluster reports to
	// Console over the Service of the Kubernetes cluster, so the operator
	// needs no Ingress in front of it.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https') && url(self).getHostname() != ''",message="externalUrl must be a valid http or https URL"
	ExternalURL  string `json:"externalUrl"`
	WorkloadSpec `json:",inline"`
}

// WebModelerSpec configures Web Modeler. Web Modeler runs as two workloads:
// the restapi process and the websockets process.
type WebModelerSpec struct {
	// Version is the Web Modeler version, as a full semantic version. The
	// operator supports 8.9.0 and later.
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+$`
	Version string `json:"version"`
	// ExternalURL is the URL that browsers reach Web Modeler at.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https') && url(self).getHostname() != ''",message="externalUrl must be a valid http or https URL"
	ExternalURL string `json:"externalUrl"`
	// WebsocketsExternalURL is the URL that browsers reach the websockets
	// process at. Web Modeler pushes live updates over it.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https') && url(self).getHostname() != ''",message="websocketsExternalUrl must be a valid http or https URL"
	WebsocketsExternalURL string `json:"websocketsExternalUrl"`
	// DatabaseConfigRef names the DatabaseConfig of the Web Modeler database,
	// in the namespace of this resource. Web Modeler needs its own PostgreSQL
	// database.
	// +kubebuilder:validation:MinLength=1
	DatabaseConfigRef string `json:"databaseConfigRef"`
	// Mail configures the SMTP server that Web Modeler sends notifications
	// through. Web Modeler does not start without it.
	Mail WebModelerMailSpec `json:"mail"`
	// Restapi configures the workload of the restapi process.
	// +optional
	Restapi *WorkloadSpec `json:"restapi,omitempty"`
	// Websockets configures the workload of the websockets process.
	// +optional
	Websockets *WorkloadSpec `json:"websockets,omitempty"`
}

// WebModelerMailSpec configures the SMTP server of Web Modeler.
type WebModelerMailSpec struct {
	// SMTPHost is the host name of the SMTP server.
	// +kubebuilder:validation:MinLength=1
	SMTPHost string `json:"smtpHost"`
	// SMTPPort is the port of the SMTP server. Defaults to 587.
	// +kubebuilder:default=587
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	SMTPPort int32 `json:"smtpPort,omitempty"`
	// FromAddress is the address that Web Modeler sends from.
	// +kubebuilder:validation:MinLength=3
	FromAddress string `json:"fromAddress"`
	// FromName is the display name that Web Modeler sends under.
	// +optional
	FromName string `json:"fromName,omitempty"`
	// TLS turns STARTTLS on. Defaults to true.
	// +optional
	TLS *bool `json:"tls,omitempty"`
	// CredentialsSecretRef names the Secret with the user and the password of
	// the SMTP server. Leave it unset for a server that needs no credentials.
	// +optional
	CredentialsSecretRef *LocalCredentialsSecretRef `json:"credentialsSecretRef,omitempty"`
}

// CamundaManagementClusterStatus is the observed state of a
// CamundaManagementCluster.
type CamundaManagementClusterStatus struct {
	// ObservedGeneration is the last generation reconciled by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// ManagementAuthConfig is the name of the ManagementAuthConfig that this
	// management cluster writes. A CamundaOptimize reads it through its
	// managementAuthRef.
	// +optional
	ManagementAuthConfig string `json:"managementAuthConfig,omitempty"`
	// Clusters lists every CamundaCluster that clusterSelector matched, and
	// reports whether the management plane serves it.
	// +listType=map
	// +listMapKey=namespace
	// +listMapKey=name
	// +optional
	Clusters []AttachedClusterStatus `json:"clusters,omitempty"`
	// Optimize lists every CamundaOptimize that names the ManagementAuthConfig
	// of this management cluster and sets spec.externalUrl, ordered by
	// namespace and name. The management plane registers the login callback of
	// each one on the Optimize client of the realm.
	//
	// The list holds no row in the oidc mode, where the identity provider of
	// the platform config holds the callback URLs.
	// +listType=map
	// +listMapKey=namespace
	// +listMapKey=name
	// +optional
	Optimize []AttachedOptimizeStatus `json:"optimize,omitempty"`
	// CallbackRealm is the Keycloak realm that this management plane last
	// registered the login callbacks of Optimize in. When the spec names
	// another realm, or the oidc mode, the operator signs in to this one and
	// removes the callbacks before it registers them in the new realm.
	//
	// It is absent in the keycloak mode, where the operator runs Keycloak and
	// deletes it with the management plane, and absent while no login callback
	// of this operator is registered anywhere. A Keycloak that is gone for
	// good never answers, so the annotation camunda.io/forget-callback-realm,
	// set to <url>/realms/<realm> of this field, lets go of it with the
	// callbacks still in it.
	// +optional
	CallbackRealm *KeycloakRealmTarget `json:"callbackRealm,omitempty"`
	// Conditions represent the current state. Ready carries a pre-check
	// reason, or it is derived from the conditions of the deployed components.
	// The per-component conditions (KeycloakReady, IdentityReady,
	// ConsoleReady, WebModelerReady, ManagementAuthReady, SecretsReady,
	// MirroredSecretsReady) and OptimizeCallbacksReady also appear here.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// AttachedClusterStatus is one CamundaCluster that clusterSelector matched.
type AttachedClusterStatus struct {
	// Name is the name of the CamundaCluster.
	Name string `json:"name"`
	// Namespace is the namespace of the CamundaCluster.
	Namespace string `json:"namespace"`
	// Attached reports whether the management plane serves this cluster.
	// Console lists it and Web Modeler deploys to it only while this is true.
	Attached bool `json:"attached"`
	// Reason names what the management plane found on this cluster. It is one
	// of five values. Four of them say why the cluster is not attached:
	// ClaimedElsewhere, another management plane holds it; NotReady, it
	// publishes no gateway endpoints or it changed while the operator claimed
	// it; InvalidReference, its platform config cannot be read, or the cluster
	// authenticates with OIDC and names another issuer than the management
	// plane; WriteFailed, the Console ping settings were refused. The fifth,
	// BasicAuthUserFailed, accompanies an attached row: the management plane
	// serves the cluster, and only the Web Modeler user on it is missing.
	// +optional
	Reason string `json:"reason,omitempty"`
	// Message explains the reason in one sentence.
	// +optional
	Message string `json:"message,omitempty"`
}

// AttachedOptimizeStatus is one CamundaOptimize whose login callback this
// management plane registers.
type AttachedOptimizeStatus struct {
	// Name is the name of the CamundaOptimize.
	Name string `json:"name"`
	// Namespace is the namespace of the CamundaOptimize.
	Namespace string `json:"namespace"`
	// ExternalURL is spec.externalUrl of that CamundaOptimize. The registered
	// callback is this URL plus the login path of Optimize.
	ExternalURL string `json:"externalUrl"`
}

// KeycloakRealmTarget is one Keycloak realm and how the operator signs in to
// it. Two targets name the same realm when their url and their realm are the
// same, whichever administrator and certificate authority each of them
// carries.
type KeycloakRealmTarget struct {
	// URL is the URL of Keycloak that the operator reaches, the /auth path
	// included when Keycloak serves one.
	URL string `json:"url"`
	// Realm is the Keycloak realm.
	Realm string `json:"realm"`
	// AdminCredentialsSecretRef names the Secret with the Keycloak
	// administrator that the operator signs in with, in the namespace of this
	// resource.
	AdminCredentialsSecretRef LocalCredentialsSecretRef `json:"adminCredentialsSecretRef"`
	// CABundleSecretRef names the Secret key with the certificate authority of
	// Keycloak, in the namespace of this resource. It is absent for a Keycloak
	// whose certificate a public authority signed.
	// +optional
	CABundleSecretRef *LocalSecretKeyRef `json:"caBundleSecretRef,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// CamundaManagementCluster describes one Camunda management plane: Management
// Identity, an identity provider, and optionally Console and Web Modeler. The
// operator turns it into Deployments and Services, writes the
// ManagementAuthConfig that Optimize reads, and attaches the management plane
// to the orchestration clusters that clusterSelector matches.
//
// Creating a CamundaManagementCluster is a platform-administrator action,
// because the selector reaches CamundaClusters in every namespace and the
// operator annotates the ones it matches.
type CamundaManagementCluster struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of CamundaManagementCluster
	// +required
	Spec CamundaManagementClusterSpec `json:"spec"`

	// status defines the observed state of CamundaManagementCluster
	// +optional
	Status CamundaManagementClusterStatus `json:"status,omitzero"`
}

// GetStatusConditions returns a pointer to the status conditions. The
// component framework stages conditions on the resource through it.
func (in *CamundaManagementCluster) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// GetKind returns the CRD kind. The component framework uses it for event and
// metric recording.
func (in *CamundaManagementCluster) GetKind() string { return "CamundaManagementCluster" }

// SetObservedGeneration records the last reconciled generation in status.
func (in *CamundaManagementCluster) SetObservedGeneration(generation int64) {
	in.Status.ObservedGeneration = generation
}

// +kubebuilder:object:root=true

// CamundaManagementClusterList contains a list of CamundaManagementCluster
type CamundaManagementClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []CamundaManagementCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CamundaManagementCluster{}, &CamundaManagementClusterList{})
}
