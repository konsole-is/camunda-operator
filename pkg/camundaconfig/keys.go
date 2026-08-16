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

package camundaconfig

import (
	"regexp"
	"slices"
)

// Key is a unified-configuration property in dotted form, for example
// "camunda.data.secondary-storage.type". A list element key carries the
// placeholder "[N]" where Index puts the element number.
type Key string

// Every key names the configuration class or file of the camunda/camunda
// repository that declares it. keys_source_test.go proves the keys against a
// local checkout when CAMUNDA_SOURCE_DIR is set. The connectors runtime keys
// are the Camunda Spring Boot starter properties (clients/camunda-spring-boot-starter
// in the same repository); their environment variable names are documented
// on the page "Connectors > Configuration", section "Configure the
// Orchestration Cluster connection for Self-Managed":
// https://docs.camunda.io/docs/self-managed/components/connectors/connectors-configuration/
const (
	// KeyClusterName is camunda.cluster.name (Cluster.java).
	KeyClusterName Key = "camunda.cluster.name"
	// KeyClusterSize is camunda.cluster.size (Cluster.java).
	KeyClusterSize Key = "camunda.cluster.size"
	// KeyClusterPartitionCount is camunda.cluster.partition-count (Cluster.java).
	KeyClusterPartitionCount Key = "camunda.cluster.partition-count"
	// KeyClusterReplicationFactor is camunda.cluster.replication-factor (Cluster.java).
	KeyClusterReplicationFactor Key = "camunda.cluster.replication-factor"
	// KeyClusterNodeID is camunda.cluster.node-id (Cluster.java).
	KeyClusterNodeID Key = "camunda.cluster.node-id"
	// KeyClusterInitialContacts is camunda.cluster.initial-contact-points (Cluster.java).
	KeyClusterInitialContacts Key = "camunda.cluster.initial-contact-points"
	// KeyClusterGatewayID is camunda.cluster.gateway-id (Cluster.java).
	KeyClusterGatewayID Key = "camunda.cluster.gateway-id"

	// KeyGRPCAddress is camunda.api.grpc.address (Grpc.java).
	KeyGRPCAddress Key = "camunda.api.grpc.address"
	// KeyGRPCPort is camunda.api.grpc.port (Grpc.java).
	KeyGRPCPort Key = "camunda.api.grpc.port"
	// KeyServerPort is server.port (dist application.properties).
	KeyServerPort Key = "server.port"
	// KeyManagementServerPort is management.server.port (dist application.properties).
	KeyManagementServerPort Key = "management.server.port"

	// KeyBrokerGatewayEnable is zeebe.broker.gateway.enable (EmbeddedGatewayCfg.java,
	// ModesAndProfilesProcessor.java).
	KeyBrokerGatewayEnable Key = "zeebe.broker.gateway.enable"

	// KeySecondaryStorageType is camunda.data.secondary-storage.type (SecondaryStorage.java).
	KeySecondaryStorageType Key = "camunda.data.secondary-storage.type"

	// KeyElasticsearchURL is camunda.data.secondary-storage.elasticsearch.url (Elasticsearch.java).
	KeyElasticsearchURL Key = "camunda.data.secondary-storage.elasticsearch.url"
	// KeyElasticsearchUsername is camunda.data.secondary-storage.elasticsearch.username (Elasticsearch.java).
	KeyElasticsearchUsername Key = "camunda.data.secondary-storage.elasticsearch.username"
	// KeyElasticsearchPassword is camunda.data.secondary-storage.elasticsearch.password (Elasticsearch.java).
	KeyElasticsearchPassword Key = "camunda.data.secondary-storage.elasticsearch.password"
	// KeyElasticsearchSecurityEnabled is camunda.data.secondary-storage.elasticsearch.security.enabled
	// (SecondaryStorageSecurity.java).
	KeyElasticsearchSecurityEnabled Key = "camunda.data.secondary-storage.elasticsearch.security.enabled"
	// KeyElasticsearchSecurityCertificatePath is
	// camunda.data.secondary-storage.elasticsearch.security.certificate-path
	// (SecondaryStorageSecurity.java).
	KeyElasticsearchSecurityCertificatePath Key = "camunda.data.secondary-storage.elasticsearch.security.certificate-path"
	// KeyElasticsearchSecurityVerifyHostname is
	// camunda.data.secondary-storage.elasticsearch.security.verify-hostname
	// (SecondaryStorageSecurity.java).
	KeyElasticsearchSecurityVerifyHostname Key = "camunda.data.secondary-storage.elasticsearch.security.verify-hostname"
	// KeyElasticsearchSecuritySelfSigned is camunda.data.secondary-storage.elasticsearch.security.self-signed
	// (SecondaryStorageSecurity.java).
	KeyElasticsearchSecuritySelfSigned Key = "camunda.data.secondary-storage.elasticsearch.security.self-signed"

	// KeyRDBMSURL is camunda.data.secondary-storage.rdbms.url (Rdbms.java).
	KeyRDBMSURL Key = "camunda.data.secondary-storage.rdbms.url"
	// KeyRDBMSUsername is camunda.data.secondary-storage.rdbms.username (Rdbms.java).
	KeyRDBMSUsername Key = "camunda.data.secondary-storage.rdbms.username"
	// KeyRDBMSPassword is camunda.data.secondary-storage.rdbms.password (Rdbms.java).
	KeyRDBMSPassword Key = "camunda.data.secondary-storage.rdbms.password"
	// KeyRDBMSDatabaseVendorID is camunda.data.secondary-storage.rdbms.database-vendor-id
	// (MyBatisConfiguration.java).
	KeyRDBMSDatabaseVendorID Key = "camunda.data.secondary-storage.rdbms.database-vendor-id"

	// KeyAuthenticationMethod is camunda.security.authentication.method
	// (AuthenticationConfiguration.java).
	KeyAuthenticationMethod Key = "camunda.security.authentication.method"
	// KeyOIDCIssuerURI is camunda.security.authentication.oidc.issuer-uri
	// (OidcAuthenticationConfiguration.java).
	KeyOIDCIssuerURI Key = "camunda.security.authentication.oidc.issuer-uri"
	// KeyOIDCClientID is camunda.security.authentication.oidc.client-id
	// (OidcAuthenticationConfiguration.java).
	KeyOIDCClientID Key = "camunda.security.authentication.oidc.client-id"
	// KeyOIDCClientSecret is camunda.security.authentication.oidc.client-secret
	// (OidcAuthenticationConfiguration.java).
	KeyOIDCClientSecret Key = "camunda.security.authentication.oidc.client-secret"
	// KeyOIDCRedirectURI is camunda.security.authentication.oidc.redirect-uri
	// (OidcAuthenticationConfiguration.java).
	KeyOIDCRedirectURI Key = "camunda.security.authentication.oidc.redirect-uri"
	// KeyOIDCJWKSetURI is camunda.security.authentication.oidc.jwk-set-uri
	// (OidcAuthenticationConfiguration.java).
	KeyOIDCJWKSetURI Key = "camunda.security.authentication.oidc.jwk-set-uri"
	// KeyOIDCAuthorizationURI is camunda.security.authentication.oidc.authorization-uri
	// (OidcAuthenticationConfiguration.java).
	KeyOIDCAuthorizationURI Key = "camunda.security.authentication.oidc.authorization-uri"
	// KeyOIDCTokenURI is camunda.security.authentication.oidc.token-uri
	// (OidcAuthenticationConfiguration.java).
	KeyOIDCTokenURI Key = "camunda.security.authentication.oidc.token-uri"
	// KeyOIDCAudiences is camunda.security.authentication.oidc.audiences
	// (OidcAuthenticationConfiguration.java).
	KeyOIDCAudiences Key = "camunda.security.authentication.oidc.audiences"

	// KeyInitializationUsers is camunda.security.initialization.users
	// (InitializationConfiguration.java).
	KeyInitializationUsers Key = "camunda.security.initialization.users"
	// KeyInitializationUserUsername is the username of one list element of
	// KeyInitializationUsers (ConfiguredUser.java).
	KeyInitializationUserUsername Key = "camunda.security.initialization.users[N].username"
	// KeyInitializationUserPassword is the password of one list element of
	// KeyInitializationUsers (ConfiguredUser.java).
	KeyInitializationUserPassword Key = "camunda.security.initialization.users[N].password"
	// KeyInitializationUserName is the name of one list element of
	// KeyInitializationUsers (ConfiguredUser.java).
	KeyInitializationUserName Key = "camunda.security.initialization.users[N].name"
	// KeyInitializationUserEmail is the email of one list element of
	// KeyInitializationUsers (ConfiguredUser.java).
	KeyInitializationUserEmail Key = "camunda.security.initialization.users[N].email"
	// KeyDefaultRolesAdminUsers is camunda.security.initialization.default-roles.admin.users
	// (InitializationConfiguration.java, PlatformDefaultEntities.java).
	KeyDefaultRolesAdminUsers Key = "camunda.security.initialization.default-roles.admin.users"
	// KeyDefaultRolesAdminUserItem is one list element of KeyDefaultRolesAdminUsers.
	KeyDefaultRolesAdminUserItem Key = "camunda.security.initialization.default-roles.admin.users[N]"

	// KeyLicenseKey is camunda.license.key (ManagementServicesConfiguration.java).
	KeyLicenseKey Key = "camunda.license.key"

	// KeyClientMode is camunda.client.mode (CamundaClientProperties.java).
	KeyClientMode Key = "camunda.client.mode"
	// KeyClientGRPCAddress is camunda.client.grpc-address (CamundaClientProperties.java).
	KeyClientGRPCAddress Key = "camunda.client.grpc-address"
	// KeyClientRESTAddress is camunda.client.rest-address (CamundaClientProperties.java).
	KeyClientRESTAddress Key = "camunda.client.rest-address"
	// KeyClientAuthMethod is camunda.client.auth.method (CamundaClientAuthProperties.java).
	KeyClientAuthMethod Key = "camunda.client.auth.method"
	// KeyClientAuthUsername is camunda.client.auth.username (CamundaClientAuthProperties.java).
	KeyClientAuthUsername Key = "camunda.client.auth.username"
	// KeyClientAuthPassword is camunda.client.auth.password (CamundaClientAuthProperties.java).
	KeyClientAuthPassword Key = "camunda.client.auth.password"
	// KeyClientAuthClientID is camunda.client.auth.client-id (CamundaClientAuthProperties.java).
	KeyClientAuthClientID Key = "camunda.client.auth.client-id"
	// KeyClientAuthClientSecret is camunda.client.auth.client-secret (CamundaClientAuthProperties.java).
	KeyClientAuthClientSecret Key = "camunda.client.auth.client-secret"
	// KeyClientAuthIssuerURL is camunda.client.auth.issuer-url (CamundaClientAuthProperties.java).
	KeyClientAuthIssuerURL Key = "camunda.client.auth.issuer-url"
	// KeyClientAuthAudience is camunda.client.auth.audience (CamundaClientAuthProperties.java).
	KeyClientAuthAudience Key = "camunda.client.auth.audience"
)

// The plain environment variables and the Spring profiles that the operator
// sets. They are not unified-configuration keys.
const (
	// EnvSpringProfilesActive selects the role of a unified process
	// (StandaloneCamunda.java).
	EnvSpringProfilesActive = "SPRING_PROFILES_ACTIVE"
	// EnvJavaToolOptions is the variable that the JVM reads its options from.
	EnvJavaToolOptions = "JAVA_TOOL_OPTIONS"
	// EnvLicenseKeyConnectors is the license variable of the connectors
	// runtime (Helm chart templates/connectors/deployment.yaml).
	EnvLicenseKeyConnectors = "CAMUNDA_LICENSE_KEY"

	// ProfileBroker runs the Zeebe broker (Profile.java).
	ProfileBroker = "broker"
	// ProfileGateway runs the gateway, which also hosts the web applications
	// (Profile.java).
	ProfileGateway = "gateway"
	// ProfileOperate serves the Operate web application (Profile.java).
	ProfileOperate = "operate"
	// ProfileTasklist serves the Tasklist web application (Profile.java).
	ProfileTasklist = "tasklist"
	// ProfileAdmin serves the Admin web application. "identity" is the
	// legacy name of the same profile (Profile.java).
	ProfileAdmin = "admin"
	// ProfileConsolidatedAuth activates the security filter chain of every
	// process (Profile.java, WebSecurityConfig.java).
	ProfileConsolidatedAuth = "consolidated-auth"
)

// declared lists every key the operator may set. IsDeclared and Declared read
// it; a key that is not in it is not declared.
var declared = []Key{
	KeyClusterName,
	KeyClusterSize,
	KeyClusterPartitionCount,
	KeyClusterReplicationFactor,
	KeyClusterNodeID,
	KeyClusterInitialContacts,
	KeyClusterGatewayID,
	KeyGRPCAddress,
	KeyGRPCPort,
	KeyServerPort,
	KeyManagementServerPort,
	KeyBrokerGatewayEnable,
	KeySecondaryStorageType,
	KeyElasticsearchURL,
	KeyElasticsearchUsername,
	KeyElasticsearchPassword,
	KeyElasticsearchSecurityEnabled,
	KeyElasticsearchSecurityCertificatePath,
	KeyElasticsearchSecurityVerifyHostname,
	KeyElasticsearchSecuritySelfSigned,
	KeyRDBMSURL,
	KeyRDBMSUsername,
	KeyRDBMSPassword,
	KeyRDBMSDatabaseVendorID,
	KeyAuthenticationMethod,
	KeyOIDCIssuerURI,
	KeyOIDCClientID,
	KeyOIDCClientSecret,
	KeyOIDCRedirectURI,
	KeyOIDCJWKSetURI,
	KeyOIDCAuthorizationURI,
	KeyOIDCTokenURI,
	KeyOIDCAudiences,
	KeyInitializationUsers,
	KeyInitializationUserUsername,
	KeyInitializationUserPassword,
	KeyInitializationUserName,
	KeyInitializationUserEmail,
	KeyDefaultRolesAdminUsers,
	KeyDefaultRolesAdminUserItem,
	KeyLicenseKey,
	KeyClientMode,
	KeyClientGRPCAddress,
	KeyClientRESTAddress,
	KeyClientAuthMethod,
	KeyClientAuthUsername,
	KeyClientAuthPassword,
	KeyClientAuthClientID,
	KeyClientAuthClientSecret,
	KeyClientAuthIssuerURL,
	KeyClientAuthAudience,
}

// plainEnv lists the plain environment variables that IsDeclared accepts
// next to the declared keys.
var plainEnv = []string{EnvSpringProfilesActive, EnvJavaToolOptions, EnvLicenseKeyConnectors}

// Declared returns every key the operator may set, sorted.
func Declared() []Key {
	keys := slices.Clone(declared)
	slices.Sort(keys)
	return keys
}

// listIndex matches a rendered list index in an environment variable name:
// "_0_" in the middle or "_0" at the end.
var listIndex = regexp.MustCompile(`_[0-9]+(_|$)`)

// IsDeclared reports whether envName is the environment variable of a
// declared key or one of the plain variables. A list index in the name
// matches the "[N]" placeholder of the declared element key.
func IsDeclared(envName string) bool {
	if slices.Contains(plainEnv, envName) {
		return true
	}

	normalized := listIndex.ReplaceAllString(envName, "_N$1")
	return slices.ContainsFunc(declared, func(k Key) bool { return k.Env() == normalized })
}
