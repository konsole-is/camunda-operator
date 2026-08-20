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
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The component values of the camunda.io/component label. They carry the
// optimize prefix because the workloads sit next to the workloads of the
// cluster they attach to, which label themselves by process name.
const (
	// ComponentWebapp serves the Optimize user interface and its API. It runs
	// with the Zeebe import off.
	ComponentWebapp = "optimize-webapp"
	// ComponentImporter imports the exported cluster records into the
	// Optimize indices. Optimize supports one active importer.
	ComponentImporter = "optimize-importer"
)

// The names that a user or another operator can observe on the rendered
// resources.
const (
	// OptimizeImage is the image of both workloads.
	OptimizeImage = "camunda/optimize"
	// ZeebeRecordPrefix is the index prefix that the exporter writes and that
	// Optimize reads. It is the default of both sides
	// (ElasticsearchExporterConfiguration.IndexConfiguration.prefix and
	// zeebe.name of the Optimize service-config.yaml), and the operator keeps
	// it fixed: the two must agree, and nothing else in the operator reads
	// these indices.
	ZeebeRecordPrefix = "zeebe-record"
	// ConfigHashAnnotation is the pod template annotation that carries the
	// hash of the rendered configuration. A change to it rolls the pods. It is
	// the same annotation key that the CamundaCluster workloads carry.
	ConfigHashAnnotation = "camunda.io/config-hash"
	// CAMountPath is where the Elasticsearch CA certificate is mounted in the
	// Optimize containers.
	CAMountPath = "/etc/camunda/es-ca"
	// caVolumeName is the volume that carries the Elasticsearch CA.
	caVolumeName = "es-ca"
	// optimizeContainer is the container of both workloads.
	optimizeContainer = "optimize"
)

// The ports of the Optimize container. The HTTP port is
// container.ports.http and the management port is container.ports.actuator
// of the Optimize service-config.yaml.
const (
	PortHTTP       int32 = 8090
	PortManagement int32 = 8092
)

// The port names of the container and the Service.
const (
	portNameHTTP       = "http"
	portNameManagement = "management"
)

// The workload name suffixes, one per component.
const (
	webappSuffix   = "webapp"
	importerSuffix = "importer"
)

// WorkloadName returns the name of the Deployment, the Service, and the
// ServiceMonitor of a component: the CamundaOptimize name and the short name
// of the component, joined by a dash. It is derived from the CamundaOptimize
// and not from the cluster, so two instances attached to one cluster cannot
// collide.
func WorkloadName(o *v1.CamundaOptimize, component string) string {
	return o.Name + "-" + shortName(component)
}

// shortName returns the workload name suffix of a component. An unknown
// component keeps its own name, so a caller that adds a component without a
// suffix still renders a unique name instead of a colliding one.
func shortName(component string) string {
	switch component {
	case ComponentWebapp:
		return webappSuffix
	case ComponentImporter:
		return importerSuffix
	default:
		return component
	}
}
