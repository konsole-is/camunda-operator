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

// Package camundaoptimize renders the resources of one CamundaOptimize: the
// webapp Deployment, the importer Deployment, a Service and an optional
// ServiceMonitor for each of them, and the copies of referenced Secrets from
// other namespaces. It also renders the patch that turns on the legacy Zeebe
// Elasticsearch exporter on the referenced CamundaCluster, which writes the
// indices that Optimize imports.
//
// The package is pure: spec in, resources out, no API calls. The controller in
// internal/controller/camundaoptimize resolves the references into Input and
// applies what this package renders.
package camundaoptimize
