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
	KeyInitializationUsers:        "security classes are not generated into defaults.yaml",
	KeyInitializationUserUsername: "security classes are not generated into defaults.yaml",
	KeyInitializationUserPassword: "security classes are not generated into defaults.yaml",
	KeyInitializationUserName:     "security classes are not generated into defaults.yaml",
	KeyInitializationUserEmail:    "security classes are not generated into defaults.yaml",
	KeyDefaultRolesAdminUsers:     "security classes are not generated into defaults.yaml",
	KeyDefaultRolesAdminUserItem:  "security classes are not generated into defaults.yaml",
	KeyLicenseKey:                 "license class is not generated into defaults.yaml",

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

// envComment matches the environment variable name that the generator writes
// next to every property of defaults.yaml.
var envComment = regexp.MustCompile(`Env: ([A-Z0-9_]+)`)

// Every declared key exists in the Camunda source: its environment variable
// name is listed in defaults.yaml, or the key is in notInDefaultsYAML with a
// reason. A key that is found and still in the table fails, so the table
// cannot grow stale. Run with CAMUNDA_SOURCE_DIR pointing at a checkout of
// camunda/camunda at the supported tag.
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

	for _, k := range Declared() {
		reason, excused := notInDefaultsYAML[k]
		if listed[k.Env()] {
			assert.False(t, excused, "%s is listed in %s; remove it from notInDefaultsYAML", k, defaultsYAML)
			continue
		}
		assert.True(t, excused, "%s (%s) is not listed in %s and has no reason", k, k.Env(), defaultsYAML)
		assert.NotEmpty(t, reason, "%s has an empty reason", k)
	}
}
