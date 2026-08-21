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
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sourceDirEnv names the local checkout of camunda/camunda that the source
// scan reads. The scan is skipped without it.
const sourceDirEnv = "CAMUNDA_SOURCE_DIR"

// defaultsYAML is the generated configuration reference of the unified
// binary. Every property that the binary binds under camunda.* and zeebe.*
// with a generated default appears in it with its environment variable name.
const defaultsYAML = "dist/src/main/config/defaults.yaml"

// notInDefaultsYAML lists the declared keys that defaults.yaml does not carry,
// each with the reason. The scan asserts that a key is in this table only
// when defaults.yaml really lacks it, so the table stays minimal.
var notInDefaultsYAML = map[Key]string{
	KeyServerPort:           "Spring Boot property, set in dist application.properties",
	KeyManagementServerPort: "Spring Boot property, set in dist application.properties",
	KeyBrokerGatewayEnable:  "legacy broker property; defaults.yaml lists zeebe.broker.gateway as one block",

	KeyExporterElasticsearchClassName:   "camunda.data.exporters is a map; defaults.yaml lists the map, not its entries",
	KeyExporterElasticsearchURL:         "camunda.data.exporters is a map; defaults.yaml lists the map, not its entries",
	KeyExporterElasticsearchIndexPrefix: "camunda.data.exporters is a map; defaults.yaml lists the map, not its entries",
	KeyExporterElasticsearchUsername:    "camunda.data.exporters is a map; defaults.yaml lists the map, not its entries",
	KeyExporterElasticsearchPassword:    "camunda.data.exporters is a map; defaults.yaml lists the map, not its entries",

	KeyElasticsearchSecurityEnabled:         "SecondaryStorageSecurity fields are not generated into defaults.yaml",
	KeyElasticsearchSecurityCertificatePath: "SecondaryStorageSecurity fields are not generated into defaults.yaml",
	KeyElasticsearchSecurityVerifyHostname:  "SecondaryStorageSecurity fields are not generated into defaults.yaml",
	KeyElasticsearchSecuritySelfSigned:      "SecondaryStorageSecurity fields are not generated into defaults.yaml",
	KeyRDBMSDatabaseVendorID:                "read by MyBatisConfiguration, not a configuration class field",

	KeyAuthenticationMethod:       "security classes are not generated into defaults.yaml",
	KeyOIDCIssuerURI:              "security classes are not generated into defaults.yaml",
	KeyOIDCClientID:               "security classes are not generated into defaults.yaml",
	KeyOIDCClientSecret:           "security classes are not generated into defaults.yaml",
	KeyOIDCRedirectURI:            "security classes are not generated into defaults.yaml",
	KeyOIDCJWKSetURI:              "security classes are not generated into defaults.yaml",
	KeyOIDCAuthorizationURI:       "security classes are not generated into defaults.yaml",
	KeyOIDCTokenURI:               "security classes are not generated into defaults.yaml",
	KeyOIDCAudiences:              "security classes are not generated into defaults.yaml",
	KeyOIDCUsernameClaim:          "security classes are not generated into defaults.yaml",
	KeyOIDCClientIDClaim:          "security classes are not generated into defaults.yaml",
	KeyInitializationUsers:        "security classes are not generated into defaults.yaml",
	KeyInitializationUserUsername: "security classes are not generated into defaults.yaml",
	KeyInitializationUserPassword: "security classes are not generated into defaults.yaml",
	KeyInitializationUserName:     "security classes are not generated into defaults.yaml",
	KeyInitializationUserEmail:    "security classes are not generated into defaults.yaml",
	KeyDefaultRolesAdminUsers:     "security classes are not generated into defaults.yaml",
	KeyDefaultRolesAdminUserItem:  "security classes are not generated into defaults.yaml",
	KeyLicenseKey:                 "license class is not generated into defaults.yaml",

	KeyInitializationMappingRules:          "security classes are not generated into defaults.yaml",
	KeyInitializationMappingRuleID:         "security classes are not generated into defaults.yaml",
	KeyInitializationMappingRuleClaimName:  "security classes are not generated into defaults.yaml",
	KeyInitializationMappingRuleClaimValue: "security classes are not generated into defaults.yaml",
	KeyDefaultRolesAdminClients:            "security classes are not generated into defaults.yaml",
	KeyDefaultRolesAdminClientItem:         "security classes are not generated into defaults.yaml",
	KeyDefaultRolesAdminMappingRules:       "security classes are not generated into defaults.yaml",
	KeyDefaultRolesAdminMappingRuleItem:    "security classes are not generated into defaults.yaml",
	KeyDefaultRolesConnectorsClients:       "security classes are not generated into defaults.yaml",
	KeyDefaultRolesConnectorsClientItem:    "security classes are not generated into defaults.yaml",

	KeyClientMode:             "connectors runtime, separate repository",
	KeyClientGRPCAddress:      "connectors runtime, separate repository",
	KeyClientRESTAddress:      "connectors runtime, separate repository",
	KeyClientAuthMethod:       "connectors runtime, separate repository",
	KeyClientAuthUsername:     "connectors runtime, separate repository",
	KeyClientAuthPassword:     "connectors runtime, separate repository",
	KeyClientAuthClientID:     "connectors runtime, separate repository",
	KeyClientAuthClientSecret: "connectors runtime, separate repository",
	KeyClientAuthIssuerURL:    "connectors runtime, separate repository",
	KeyClientAuthAudience:     "connectors runtime, separate repository",
}

// The source files that sourceEvidence points at, relative to
// CAMUNDA_SOURCE_DIR. A path named once here and read once by the scan, even
// though several keys point at the same file.
const (
	oidcConfigFile = "security/security-core/src/main/java/io/camunda/security/" +
		"configuration/OidcAuthenticationConfiguration.java"
	authConfigFile = "security/security-core/src/main/java/io/camunda/security/" +
		"configuration/AuthenticationConfiguration.java"
	initConfigFile = "security/security-core/src/main/java/io/camunda/security/" +
		"configuration/InitializationConfiguration.java"
	configuredUserFile = "security/security-core/src/main/java/io/camunda/security/" +
		"configuration/ConfiguredUser.java"
	mappingRuleFile = "security/security-core/src/main/java/io/camunda/security/" +
		"configuration/ConfiguredMappingRule.java"
	platformEntitiesFile = "zeebe/engine/src/main/java/io/camunda/zeebe/engine/" +
		"processing/identity/initialize/PlatformDefaultEntities.java"
	managementServicesFile = "dist/src/main/java/io/camunda/application/commons/" +
		"service/ManagementServicesConfiguration.java"
	applicationPropsFile = "dist/src/main/resources/application.properties"
	modesProcessorFile   = "dist/src/main/java/io/camunda/application/ModesAndProfilesProcessor.java"
	myBatisConfigFile    = "dist/src/main/java/io/camunda/application/commons/rdbms/MyBatisConfiguration.java"
	esSecurityFile       = "configuration/src/main/java/io/camunda/configuration/SecondaryStorageSecurity.java"
	exporterFile         = "configuration/src/main/java/io/camunda/configuration/Exporter.java"
	esExporterConfigFile = "zeebe/exporters/elasticsearch-exporter/src/main/java/io/camunda/zeebe/exporter/" +
		"ElasticsearchExporterConfiguration.java"
	clientPropsFile = "clients/camunda-spring-boot-starter/src/main/java/io/camunda/" +
		"client/spring/properties/CamundaClientProperties.java"
	clientAuthPropsFile = "clients/camunda-spring-boot-starter/src/main/java/io/camunda/" +
		"client/spring/properties/CamundaClientAuthProperties.java"
)

// sourceRef is one piece of evidence that a key excused in notInDefaultsYAML
// still exists in the camunda/camunda source: the file it lives in, relative
// to CAMUNDA_SOURCE_DIR, and a pattern that file must match.
type sourceRef struct {
	file    string
	pattern string
}

// fieldRef is a sourceRef for a Java field: it requires name inside one
// field declaration statement (from "private" to the terminating
// semicolon), so a getter or setter of the same name does not satisfy it,
// and a rename or removal of the field breaks the match.
func fieldRef(file, name string) sourceRef {
	return sourceRef{file, `private[^;]*\b` + regexp.QuoteMeta(name) + `\b[^;]*;`}
}

// sourceEvidence gives one sourceRef per key in notInDefaultsYAML. The test
// reads the referenced file and requires the pattern to match, so a renamed
// or removed property fails it instead of resting on the human-written
// reason string above. Several keys point at the same evidence: the
// role-membership keys share the entity-type cases of PlatformDefaultEntities
// because "admin"/"connectors"/"users"/"clients" are configuration map keys,
// not Java identifiers, and the list-item keys (the "[N]" forms) describe
// the same property as their non-indexed counterpart.
var sourceEvidence = map[Key]sourceRef{
	KeyServerPort:           {applicationPropsFile, `server\.port=8080`},
	KeyManagementServerPort: {applicationPropsFile, `management\.server\.port=9600`},
	KeyBrokerGatewayEnable:  {modesProcessorFile, `zeebe\.broker\.gateway\.enable`},

	// The exporter id "elasticsearch" is a map key, so the evidence is the
	// Exporter class that every map entry binds to, and the exporter's own
	// configuration class for the args. The two args fields that are public
	// take a literal pattern; fieldRef only matches a private declaration.
	KeyExporterElasticsearchClassName:   fieldRef(exporterFile, "className"),
	KeyExporterElasticsearchURL:         {esExporterConfigFile, `public String url`},
	KeyExporterElasticsearchIndexPrefix: {esExporterConfigFile, `public String prefix`},
	KeyExporterElasticsearchUsername:    fieldRef(esExporterConfigFile, "username"),
	KeyExporterElasticsearchPassword:    fieldRef(esExporterConfigFile, "password"),

	KeyElasticsearchSecurityEnabled:         fieldRef(esSecurityFile, "enabled"),
	KeyElasticsearchSecurityCertificatePath: fieldRef(esSecurityFile, "certificatePath"),
	KeyElasticsearchSecurityVerifyHostname:  fieldRef(esSecurityFile, "verifyHostname"),
	KeyElasticsearchSecuritySelfSigned:      fieldRef(esSecurityFile, "selfSigned"),

	KeyRDBMSDatabaseVendorID: {myBatisConfigFile, `database-vendor-id`},

	KeyAuthenticationMethod: fieldRef(authConfigFile, "method"),

	KeyOIDCIssuerURI:        fieldRef(oidcConfigFile, "issuerUri"),
	KeyOIDCClientID:         fieldRef(oidcConfigFile, "clientId"),
	KeyOIDCClientSecret:     fieldRef(oidcConfigFile, "clientSecret"),
	KeyOIDCRedirectURI:      fieldRef(oidcConfigFile, "redirectUri"),
	KeyOIDCJWKSetURI:        fieldRef(oidcConfigFile, "jwkSetUri"),
	KeyOIDCAuthorizationURI: fieldRef(oidcConfigFile, "authorizationUri"),
	KeyOIDCTokenURI:         fieldRef(oidcConfigFile, "tokenUri"),
	KeyOIDCAudiences:        fieldRef(oidcConfigFile, "audiences"),
	KeyOIDCUsernameClaim:    fieldRef(oidcConfigFile, "usernameClaim"),
	KeyOIDCClientIDClaim:    fieldRef(oidcConfigFile, "clientIdClaim"),

	KeyInitializationUsers:        fieldRef(initConfigFile, "users"),
	KeyInitializationUserUsername: fieldRef(configuredUserFile, "username"),
	KeyInitializationUserPassword: fieldRef(configuredUserFile, "password"),
	KeyInitializationUserName:     fieldRef(configuredUserFile, "name"),
	KeyInitializationUserEmail:    fieldRef(configuredUserFile, "email"),

	KeyDefaultRolesAdminUsers:    {platformEntitiesFile, `case "users"`},
	KeyDefaultRolesAdminUserItem: {platformEntitiesFile, `case "users"`},

	KeyLicenseKey: {managementServicesFile, `@ConfigurationProperties\("camunda\.license"\)`},

	KeyInitializationMappingRules:          fieldRef(initConfigFile, "mappingRules"),
	KeyInitializationMappingRuleID:         fieldRef(mappingRuleFile, "mappingRuleId"),
	KeyInitializationMappingRuleClaimName:  fieldRef(mappingRuleFile, "claimName"),
	KeyInitializationMappingRuleClaimValue: fieldRef(mappingRuleFile, "claimValue"),

	KeyDefaultRolesAdminClients:         {platformEntitiesFile, `case "clients"`},
	KeyDefaultRolesAdminClientItem:      {platformEntitiesFile, `case "clients"`},
	KeyDefaultRolesAdminMappingRules:    {platformEntitiesFile, `"mappingrules"`},
	KeyDefaultRolesAdminMappingRuleItem: {platformEntitiesFile, `"mappingrules"`},
	KeyDefaultRolesConnectorsClients:    {platformEntitiesFile, `case "clients"`},
	KeyDefaultRolesConnectorsClientItem: {platformEntitiesFile, `case "clients"`},

	KeyClientMode:        fieldRef(clientPropsFile, "mode"),
	KeyClientGRPCAddress: fieldRef(clientPropsFile, "grpcAddress"),
	KeyClientRESTAddress: fieldRef(clientPropsFile, "restAddress"),

	KeyClientAuthMethod:       fieldRef(clientAuthPropsFile, "method"),
	KeyClientAuthUsername:     fieldRef(clientAuthPropsFile, "username"),
	KeyClientAuthPassword:     fieldRef(clientAuthPropsFile, "password"),
	KeyClientAuthClientID:     fieldRef(clientAuthPropsFile, "clientId"),
	KeyClientAuthClientSecret: fieldRef(clientAuthPropsFile, "clientSecret"),
	KeyClientAuthIssuerURL:    fieldRef(clientAuthPropsFile, "issuerUrl"),
	KeyClientAuthAudience:     fieldRef(clientAuthPropsFile, "audience"),
}

// envComment matches the environment variable name that the generator writes
// next to every property of defaults.yaml.
var envComment = regexp.MustCompile(`Env: ([A-Z0-9_]+)`)

// Every declared key exists in the Camunda source. A key listed in
// defaults.yaml is verified by that listing; a key excused in
// notInDefaultsYAML is verified against its sourceEvidence entry instead, so
// the excuse table cannot silently outlive the property it excuses. Run with
// CAMUNDA_SOURCE_DIR pointing at a checkout of camunda/camunda at the
// supported tag.
func TestDeclaredKeysExistInCamundaSource(t *testing.T) {
	t.Parallel()

	dir := os.Getenv(sourceDirEnv)
	if dir == "" {
		t.Skipf("%s is not set", sourceDirEnv)
	}

	content, err := os.ReadFile(filepath.Join(dir, defaultsYAML))
	require.NoError(t, err)

	listed := map[string]bool{}
	for _, match := range envComment.FindAllStringSubmatch(string(content), -1) {
		listed[match[1]] = true
	}
	require.NotEmpty(t, listed, "%s lists no Env: names", defaultsYAML)

	fileContent := map[string]string{}
	for _, k := range Declared() {
		reason, excused := notInDefaultsYAML[k]
		if listed[k.Env()] {
			assert.False(t, excused, "%s is listed in %s; remove it from notInDefaultsYAML", k, defaultsYAML)
			continue
		}

		assert.True(t, excused, "%s (%s) is not listed in %s and has no reason", k, k.Env(), defaultsYAML)
		assert.NotEmpty(t, reason, "%s has an empty reason", k)

		ref, hasEvidence := sourceEvidence[k]
		if !assert.True(t, hasEvidence, "%s has no sourceEvidence entry", k) {
			continue
		}

		body, read := fileContent[ref.file]
		if !read {
			raw, err := os.ReadFile(filepath.Join(dir, ref.file))
			if !assert.NoError(t, err, "%s: reading %s", k, ref.file) {
				continue
			}
			body = string(raw)
			fileContent[ref.file] = body
		}

		assert.Regexp(t, ref.pattern, body, "%s: %q not found in %s", k, ref.pattern, ref.file)
	}
}
