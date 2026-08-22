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

package utils

// The environment variables that name the image versions the suite runs
// against. make test-e2e exports them from test/e2e/versions/<minor>.env,
// one file per supported Camunda minor, and the e2e workflow runs one job
// per file. The suite fails at start when one of them is unset.
const (
	EnvCamundaVersion       = "CAMUNDA_VERSION"
	EnvConnectorsVersion    = "CAMUNDA_CONNECTORS_VERSION"
	EnvOptimizeVersion      = "CAMUNDA_OPTIMIZE_VERSION"
	EnvElasticsearchVersion = "ELASTICSEARCH_VERSION"
)

// VersionEnv is every variable of the set, for the check at suite start.
var VersionEnv = []string{
	EnvCamundaVersion,
	EnvConnectorsVersion,
	EnvOptimizeVersion,
	EnvElasticsearchVersion,
}
