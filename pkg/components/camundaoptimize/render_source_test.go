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

package camundaoptimize

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sourceDirEnv names the local checkout of camunda/camunda that the source
// scan reads. The scan is skipped without it. It is the same variable that
// pkg/camundaconfig uses for the unified-configuration keys.
const sourceDirEnv = "CAMUNDA_SOURCE_DIR"

// serviceConfig is the configuration reference of Optimize. Every setting that
// an environment variable can override appears in it as a placeholder,
// "${NAME:default}".
const serviceConfig = "optimize/util/optimize-commons/src/main/resources/service-config.yaml"

// applicationProperties holds the actuator settings of Optimize, which are
// Spring Boot properties rather than Optimize settings.
const applicationProperties = "optimize/backend/src/main/resources/application.properties"

// serviceConfigEnv lists every variable that this package sets and that
// service-config.yaml declares.
var serviceConfigEnv = []string{
	envElasticsearchHost,
	envElasticsearchPort,
	envElasticsearchUsername,
	envElasticsearchPassword,
	envElasticsearchSSLEnabled,
	envElasticsearchCAs,
	envElasticsearchSelfSigned,
	envZeebeEnabled,
	envZeebeName,
	envZeebePartitionCount,
	envIdentityIssuerURL,
	envIdentityIssuerBackendURL,
	envIdentityBaseURL,
	envIdentityClientID,
	envIdentityClientSecret,
	envIdentityAudience,
}

// Every variable that this package sets exists in the Optimize source at the
// supported version. Spring silently ignores a variable it does not know, so a
// typo would leave the setting at its default with no error anywhere. Run with
// CAMUNDA_SOURCE_DIR pointing at a checkout of camunda/camunda at the
// supported tag.
func TestOptimizeEnvExistsInCamundaSource(t *testing.T) {
	t.Parallel()

	dir := os.Getenv(sourceDirEnv)
	if dir == "" {
		t.Skipf("%s is not set", sourceDirEnv)
	}

	config := readSource(t, dir, serviceConfig)
	for _, name := range serviceConfigEnv {
		assert.Contains(t, config, "${"+name, "%s is not a placeholder of %s", name, serviceConfig)
	}

	// The remaining values are constants of the Java source rather than
	// placeholders of the configuration file.
	assert.Contains(t, config, "ports:", "%s declares no container ports", serviceConfig)
	assert.Contains(
		t,
		readSource(t, dir, "optimize/util/optimize-commons/src/main/java/io/camunda/optimize/"+
			"service/util/configuration/ConfigurationServiceConstants.java"),
		`CCSM_PROFILE = "`+profileCCSM+`"`,
	)
	assert.Contains(
		t,
		readSource(t, dir, "optimize/util/optimize-commons/src/main/java/io/camunda/optimize/"+
			"service/util/configuration/ConfigurationServiceConstants.java"),
		envDatabase+` = "`+envDatabase+`"`,
	)
	assert.Contains(
		t,
		readSource(t, dir, "optimize/backend/src/main/java/io/camunda/optimize/rest/HealthRestService.java"),
		`READYZ_PATH = "`+strings.TrimPrefix(readinessPath, "/api")+`"`,
	)
	properties := readSource(t, dir, applicationProperties)
	assert.Contains(t, properties, "management.endpoints.web.base-path=/actuator")
	assert.Contains(t, properties, "prometheus")
	// The liveness probe reads the Spring Boot health endpoint, so Optimize
	// must expose it. Nothing puts an Elasticsearch check into the aggregate:
	// Optimize registers no HealthIndicator, and it depends on the raw
	// Elasticsearch client rather than on spring-boot-starter-data-elasticsearch,
	// which is what would auto-configure one.
	assert.Contains(t, properties, "health")
	assert.NotContains(
		t,
		readSource(t, dir, "optimize/backend/pom.xml"),
		"spring-boot-starter-data-elasticsearch",
	)
}

// readSource returns the content of a file of the camunda/camunda checkout.
func readSource(t *testing.T, dir, path string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(dir, path))
	require.NoError(t, err, "reading %s", path)

	return string(content)
}
