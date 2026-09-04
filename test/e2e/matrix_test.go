//go:build e2e
// +build e2e

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

package e2e

import (
	"github.com/konsole-is/camunda-operator/test/utils"
)

// The environment of one matrix entry: the file test/e2e/matrix/<minor>.env
// that make test-e2e exports before it runs the suite. The versions are
// required, and the suite fails at start when one is unset. The label
// list is optional. It names the spec flows that run for the minor in the
// syntax of utils.LabelFilter, and an empty or absent list runs them all.
const (
	envCamundaVersion       = "CAMUNDA_VERSION"
	envConnectorsVersion    = "CAMUNDA_CONNECTORS_VERSION"
	envOptimizeVersion      = "CAMUNDA_OPTIMIZE_VERSION"
	envElasticsearchVersion = "ELASTICSEARCH_VERSION"
	envIdentityVersion      = "CAMUNDA_IDENTITY_VERSION"
	envConsoleVersion       = "CAMUNDA_CONSOLE_VERSION"
	envWebModelerVersion    = "CAMUNDA_WEB_MODELER_VERSION"
	envKeycloakVersion      = "KEYCLOAK_VERSION"
	envPostgresVersion      = "POSTGRES_VERSION"
	envLabels               = "E2E_LABELS"
)

// versionEnv is every version variable of a matrix entry: the images of the
// Camunda minor, and the third-party operator releases the suite installs.
var versionEnv = []string{
	envCamundaVersion,
	envConnectorsVersion,
	envOptimizeVersion,
	envElasticsearchVersion,
	envIdentityVersion,
	envConsoleVersion,
	envWebModelerVersion,
	envKeycloakVersion,
	envPostgresVersion,
	utils.EnvCNPGVersion,
	utils.EnvBarmanPluginVersion,
}
