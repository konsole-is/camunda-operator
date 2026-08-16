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

// The source of each key is a file of the camunda/camunda repository at tag
// 8.9.9, with the line at the time of writing, or a path in the Camunda Helm
// chart 14.8.3. The connectors runtime keys are the Camunda Spring Boot
// starter properties, which live in the same repository under
// clients/camunda-spring-boot-starter; their environment variable names are
// confirmed by the Camunda docs, page "Connectors > Configuration", section
// "Configure the Orchestration Cluster connection for Self-Managed":
// https://docs.camunda.io/docs/self-managed/components/connectors/connectors-configuration/
const (
	// Cluster identity: Cluster.java, dist/src/main/config/defaults.yaml.
	KeyClusterName              Key = "camunda.cluster.name"                   // defaults.yaml:131
	KeyClusterSize              Key = "camunda.cluster.size"                   // defaults.yaml:290
	KeyClusterPartitionCount    Key = "camunda.cluster.partition-count"        // defaults.yaml:193
	KeyClusterReplicationFactor Key = "camunda.cluster.replication-factor"     // defaults.yaml:287
	KeyClusterNodeID            Key = "camunda.cluster.node-id"                // defaults.yaml:186
	KeyClusterInitialContacts   Key = "camunda.cluster.initial-contact-points" // defaults.yaml:94
	KeyClusterGatewayID         Key = "camunda.cluster.gateway-id"             // defaults.yaml:84

	// API and ports: Grpc.java, dist/src/main/resources/application.properties.
	KeyGRPCAddress          Key = "camunda.api.grpc.address" // defaults.yaml:15
	KeyGRPCPort             Key = "camunda.api.grpc.port"    // defaults.yaml:25
	KeyServerPort           Key = "server.port"              // application.properties:16
	KeyManagementServerPort Key = "management.server.port"   // application.properties:21

	// Embedded gateway switch of the brokers.
	// ModesAndProfilesProcessor.java:28, EmbeddedGatewayCfg.java:16.
	KeyBrokerGatewayEnable Key = "zeebe.broker.gateway.enable"

	// Secondary storage: SecondaryStorage.java, SecondaryStorageSecurity.java, defaults.yaml.
	KeySecondaryStorageType Key = "camunda.data.secondary-storage.type" // defaults.yaml:824, SecondaryStorage.java:118-123

	KeyElasticsearchURL      Key = "camunda.data.secondary-storage.elasticsearch.url"      // defaults.yaml:637
	KeyElasticsearchUsername Key = "camunda.data.secondary-storage.elasticsearch.username" // defaults.yaml:642
	KeyElasticsearchPassword Key = "camunda.data.secondary-storage.elasticsearch.password" // defaults.yaml:608
	// SecondaryStorageSecurity.java:18-27 (fields), :85-87 (prefix
	// camunda.data.secondary-storage.<database>.security).
	KeyElasticsearchSecurityEnabled         Key = "camunda.data.secondary-storage.elasticsearch.security.enabled"
	KeyElasticsearchSecurityCertificatePath Key = "camunda.data.secondary-storage.elasticsearch.security.certificate-path"
	KeyElasticsearchSecurityVerifyHostname  Key = "camunda.data.secondary-storage.elasticsearch.security.verify-hostname"
	KeyElasticsearchSecuritySelfSigned      Key = "camunda.data.secondary-storage.elasticsearch.security.self-signed"

	KeyRDBMSURL      Key = "camunda.data.secondary-storage.rdbms.url"      // defaults.yaml:814
	KeyRDBMSUsername Key = "camunda.data.secondary-storage.rdbms.username" // defaults.yaml:819
	KeyRDBMSPassword Key = "camunda.data.secondary-storage.rdbms.password" // defaults.yaml:798
	// MyBatisConfiguration.java:120.
	KeyRDBMSDatabaseVendorID Key = "camunda.data.secondary-storage.rdbms.database-vendor-id"

	// Authentication: CamundaSecurityConfiguration.java:93 binds
	// camunda.security to SecurityConfiguration.java.
	KeyAuthenticationMethod Key = "camunda.security.authentication.method" // AuthenticationConfiguration.java:16,36
	// AuthenticationConfiguration.java:48-53 binds oidc to
	// OidcAuthenticationConfiguration.java; the line of each field follows.
	KeyOIDCIssuerURI        Key = "camunda.security.authentication.oidc.issuer-uri"        // :33
	KeyOIDCClientID         Key = "camunda.security.authentication.oidc.client-id"         // :35
	KeyOIDCClientSecret     Key = "camunda.security.authentication.oidc.client-secret"     // :36
	KeyOIDCRedirectURI      Key = "camunda.security.authentication.oidc.redirect-uri"      // :39
	KeyOIDCJWKSetURI        Key = "camunda.security.authentication.oidc.jwk-set-uri"       // :41
	KeyOIDCAuthorizationURI Key = "camunda.security.authentication.oidc.authorization-uri" // :43
	KeyOIDCTokenURI         Key = "camunda.security.authentication.oidc.token-uri"         // :45
	KeyOIDCAudiences        Key = "camunda.security.authentication.oidc.audiences"         // :48

	// Initial entities: SecurityConfiguration.java:23, InitializationConfiguration.java.
	KeyInitializationUsers Key = "camunda.security.initialization.users" // InitializationConfiguration.java:23
	// ConfiguredUser.java:12-15 (fields of one list element).
	KeyInitializationUserUsername Key = "camunda.security.initialization.users[N].username"
	KeyInitializationUserPassword Key = "camunda.security.initialization.users[N].password"
	KeyInitializationUserName     Key = "camunda.security.initialization.users[N].name"
	KeyInitializationUserEmail    Key = "camunda.security.initialization.users[N].email"
	// InitializationConfiguration.java:25 (default-roles map),
	// PlatformDefaultEntities.java:271 ("users" member kind).
	KeyDefaultRolesAdminUsers    Key = "camunda.security.initialization.default-roles.admin.users"
	KeyDefaultRolesAdminUserItem Key = "camunda.security.initialization.default-roles.admin.users[N]"

	// License.
	KeyLicenseKey Key = "camunda.license.key" // ManagementServicesConfiguration.java:40-41

	// Connectors runtime: the Camunda Spring Boot starter properties
	// (CamundaClientProperties.java:28 binds camunda.client), rendered by the
	// Helm chart 14.8.3 in templates/connectors/files/_application.yaml:43-71.
	KeyClientMode             Key = "camunda.client.mode"               // CamundaClientProperties.java:38,341-344
	KeyClientGRPCAddress      Key = "camunda.client.grpc-address"       // CamundaClientProperties.java:90
	KeyClientRESTAddress      Key = "camunda.client.rest-address"       // CamundaClientProperties.java:97
	KeyClientAuthMethod       Key = "camunda.client.auth.method"        // CamundaClientAuthProperties.java:33, :436-440
	KeyClientAuthUsername     Key = "camunda.client.auth.username"      // CamundaClientAuthProperties.java:41
	KeyClientAuthPassword     Key = "camunda.client.auth.password"      // CamundaClientAuthProperties.java:47
	KeyClientAuthClientID     Key = "camunda.client.auth.client-id"     // CamundaClientAuthProperties.java:51
	KeyClientAuthClientSecret Key = "camunda.client.auth.client-secret" // CamundaClientAuthProperties.java:56
	KeyClientAuthIssuerURL    Key = "camunda.client.auth.issuer-url"    // CamundaClientAuthProperties.java:70
	KeyClientAuthAudience     Key = "camunda.client.auth.audience"      // CamundaClientAuthProperties.java:82

)

// The plain environment variables and the Spring profiles that the operator
// sets. They are not unified-configuration keys, but they carry a source too.
const (
	// EnvSpringProfilesActive selects the role of a unified process.
	EnvSpringProfilesActive = "SPRING_PROFILES_ACTIVE" // Profile.java, StandaloneCamunda.java:44-52
	// EnvJavaToolOptions is the variable that the JVM reads its options from.
	EnvJavaToolOptions = "JAVA_TOOL_OPTIONS" // helm chart 14.8.3 templates/orchestration/statefulset.yaml:86-90
	// EnvLicenseKeyConnectors is the license variable of the connectors
	// runtime.
	EnvLicenseKeyConnectors = "CAMUNDA_LICENSE_KEY" // helm chart 14.8.3 templates/connectors/deployment.yaml:48-51

	// ProfileBroker runs the Zeebe broker.
	ProfileBroker = "broker" // Profile.java:19
	// ProfileGateway runs the gateway, which also hosts the web applications.
	ProfileGateway = "gateway" // Profile.java:20
	// ProfileOperate serves the Operate web application.
	ProfileOperate = "operate" // Profile.java:22, WebappsConfigurationInitializer.java:39
	// ProfileTasklist serves the Tasklist web application.
	ProfileTasklist = "tasklist" // Profile.java:23, WebappsConfigurationInitializer.java:39
	// ProfileAdmin serves the Identity web application. "identity" is the
	// legacy name of the same profile.
	ProfileAdmin = "admin" // Profile.java:24-25, WebappsConfigurationInitializer.java:39,112
	// ProfileConsolidatedAuth activates the security filter chain of every
	// process.
	ProfileConsolidatedAuth = "consolidated-auth" // Profile.java:34, WebSecurityConfig.java:148, StandaloneCamunda.java:52
)

// chartConnectorsApplication is the chart file that renders the connectors
// runtime properties.
const chartConnectorsApplication = "helm chart 14.8.3 templates/connectors/files/_application.yaml"

// sources is the table that Declared reads. A key that is not in it is not
// declared. Every value repeats the source comment of the constant, so a test
// can prove that no key is without one.
var sources = map[Key]string{
	KeyClusterName:              "defaults.yaml:131",
	KeyClusterSize:              "defaults.yaml:290",
	KeyClusterPartitionCount:    "defaults.yaml:193",
	KeyClusterReplicationFactor: "defaults.yaml:287",
	KeyClusterNodeID:            "defaults.yaml:186",
	KeyClusterInitialContacts:   "defaults.yaml:94",
	KeyClusterGatewayID:         "defaults.yaml:84",

	KeyGRPCAddress:          "defaults.yaml:15",
	KeyGRPCPort:             "defaults.yaml:25",
	KeyServerPort:           "application.properties:16",
	KeyManagementServerPort: "application.properties:21",

	KeyBrokerGatewayEnable: "ModesAndProfilesProcessor.java:28, EmbeddedGatewayCfg.java:16",

	KeySecondaryStorageType: "defaults.yaml:824, SecondaryStorage.java:118-123",

	KeyElasticsearchURL:                     "defaults.yaml:637",
	KeyElasticsearchUsername:                "defaults.yaml:642",
	KeyElasticsearchPassword:                "defaults.yaml:608",
	KeyElasticsearchSecurityEnabled:         "SecondaryStorageSecurity.java:18,85-87",
	KeyElasticsearchSecurityCertificatePath: "SecondaryStorageSecurity.java:21,85-87",
	KeyElasticsearchSecurityVerifyHostname:  "SecondaryStorageSecurity.java:24,85-87",
	KeyElasticsearchSecuritySelfSigned:      "SecondaryStorageSecurity.java:27,85-87",

	KeyRDBMSURL:              "defaults.yaml:814",
	KeyRDBMSUsername:         "defaults.yaml:819",
	KeyRDBMSPassword:         "defaults.yaml:798",
	KeyRDBMSDatabaseVendorID: "MyBatisConfiguration.java:120",

	KeyAuthenticationMethod: "CamundaSecurityConfiguration.java:93, AuthenticationConfiguration.java:16,36",
	KeyOIDCIssuerURI:        "AuthenticationConfiguration.java:48-53, OidcAuthenticationConfiguration.java:33",
	KeyOIDCClientID:         "OidcAuthenticationConfiguration.java:35",
	KeyOIDCClientSecret:     "OidcAuthenticationConfiguration.java:36",
	KeyOIDCRedirectURI:      "OidcAuthenticationConfiguration.java:39, ClientRegistrationFactory.java:26,51-56",
	KeyOIDCJWKSetURI:        "OidcAuthenticationConfiguration.java:41",
	KeyOIDCAuthorizationURI: "OidcAuthenticationConfiguration.java:43",
	KeyOIDCTokenURI:         "OidcAuthenticationConfiguration.java:45",
	KeyOIDCAudiences:        "OidcAuthenticationConfiguration.java:48",

	KeyInitializationUsers:        "SecurityConfiguration.java:23, InitializationConfiguration.java:23",
	KeyInitializationUserUsername: "ConfiguredUser.java:12",
	KeyInitializationUserPassword: "ConfiguredUser.java:13",
	KeyInitializationUserName:     "ConfiguredUser.java:14",
	KeyInitializationUserEmail:    "ConfiguredUser.java:15",
	KeyDefaultRolesAdminUsers:     "InitializationConfiguration.java:25, PlatformDefaultEntities.java:271",
	KeyDefaultRolesAdminUserItem:  "InitializationConfiguration.java:25, PlatformDefaultEntities.java:271",

	KeyLicenseKey: "ManagementServicesConfiguration.java:40-41",

	KeyClientMode:             "CamundaClientProperties.java:38,341-344; " + chartConnectorsApplication + ":50",
	KeyClientGRPCAddress:      "CamundaClientProperties.java:90; " + chartConnectorsApplication + ":46",
	KeyClientRESTAddress:      "CamundaClientProperties.java:97; " + chartConnectorsApplication + ":45",
	KeyClientAuthMethod:       "CamundaClientAuthProperties.java:33,436-440; " + chartConnectorsApplication + ":54",
	KeyClientAuthUsername:     "CamundaClientAuthProperties.java:41; " + chartConnectorsApplication + ":55",
	KeyClientAuthPassword:     "CamundaClientAuthProperties.java:47; " + chartConnectorsApplication + ":56",
	KeyClientAuthClientID:     "CamundaClientAuthProperties.java:51; " + chartConnectorsApplication + ":64",
	KeyClientAuthClientSecret: "CamundaClientAuthProperties.java:56; " + chartConnectorsApplication + ":65",
	KeyClientAuthIssuerURL:    "CamundaClientAuthProperties.java:70; " + chartConnectorsApplication + ":60",
	KeyClientAuthAudience:     "CamundaClientAuthProperties.java:82; " + chartConnectorsApplication + ":66",
}

// plainSources is the source table of the plain variables and profiles.
var plainSources = map[string]string{
	EnvSpringProfilesActive: "Profile.java, StandaloneCamunda.java:44-52",
	EnvJavaToolOptions:      "helm chart 14.8.3 templates/orchestration/statefulset.yaml:86-90",
	EnvLicenseKeyConnectors: "helm chart 14.8.3 templates/connectors/deployment.yaml:48-51",
	ProfileBroker:           "Profile.java:19",
	ProfileGateway:          "Profile.java:20",
	ProfileOperate:          "Profile.java:22, WebappsConfigurationInitializer.java:39",
	ProfileTasklist:         "Profile.java:23, WebappsConfigurationInitializer.java:39",
	ProfileAdmin:            "Profile.java:24-25, WebappsConfigurationInitializer.java:39,112",
	ProfileConsolidatedAuth: "Profile.java:34, WebSecurityConfig.java:148, StandaloneCamunda.java:52",
}

// Declared returns every key the operator may set, sorted.
func Declared() []Key {
	keys := make([]Key, 0, len(sources))
	for k := range sources {
		keys = append(keys, k)
	}
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
	if _, ok := plainSources[envName]; ok {
		return true
	}

	normalized := listIndex.ReplaceAllString(envName, "_N$1")
	for k := range sources {
		if k.Env() == normalized {
			return true
		}
	}
	return false
}
