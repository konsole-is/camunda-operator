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

import "os"

// The image versions that the e2e suite runs against. Each default is the
// newest patch of the minor the operator supports. The marker comment above
// a pin is for Renovate: renovate.json5 reads it and opens a pull request when
// a newer patch lands, and holds the bound that keeps a pin on its minor.
//
// An environment variable overrides each pin, so a workflow can run the suite
// against another release without a code change.
const (
	// renovate: datasource=docker depName=camunda/camunda
	defaultCamundaVersion = "8.9.17"
	// renovate: datasource=docker depName=camunda/connectors-bundle
	defaultConnectorsVersion = "8.9.8"
	// renovate: datasource=docker depName=camunda/optimize
	defaultOptimizeVersion = "8.9.17"
	// renovate: datasource=docker depName=docker.elastic.co/elasticsearch/elasticsearch
	defaultElasticsearchVersion = "9.2.8"
)

// CamundaVersion returns the camunda/camunda release under test:
// CAMUNDA_VERSION, or the pinned default.
func CamundaVersion() string {
	return envOr("CAMUNDA_VERSION", defaultCamundaVersion)
}

// ConnectorsVersion returns the camunda/connectors-bundle release under test:
// CAMUNDA_CONNECTORS_VERSION, or the pinned default. The bundle has its own
// patch line. Only its minor has to match CamundaVersion.
func ConnectorsVersion() string {
	return envOr("CAMUNDA_CONNECTORS_VERSION", defaultConnectorsVersion)
}

// OptimizeVersion returns the camunda/optimize release under test:
// CAMUNDA_OPTIMIZE_VERSION, or the pinned default. Optimize has its own patch
// line. Only its minor has to match CamundaVersion.
func OptimizeVersion() string {
	return envOr("CAMUNDA_OPTIMIZE_VERSION", defaultOptimizeVersion)
}

// ElasticsearchVersion returns the Elasticsearch release that ECK runs for
// the suite: ELASTICSEARCH_VERSION, or the pinned default.
func ElasticsearchVersion() string {
	return envOr("ELASTICSEARCH_VERSION", defaultElasticsearchVersion)
}

// envOr returns the value of the environment variable name, or def when the
// variable is unset or empty.
func envOr(name, def string) string {
	if v, ok := os.LookupEnv(name); ok && v != "" {
		return v
	}
	return def
}
