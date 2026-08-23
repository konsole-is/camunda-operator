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
	"github.com/konsole-is/camunda-operator/pkg/images"
)

// Input is everything the pure package needs to render one Optimize instance.
// The controller fills it in its pre-checks.
type Input struct {
	// Optimize is the CamundaOptimize as it was read.
	Optimize *v1.CamundaOptimize
	// ClusterName is the name of the CamundaCluster that spec.clusterRef
	// names. It is the camunda.io/cluster label value of every rendered
	// resource, so an extension finds the Optimize workloads of a cluster the
	// same way it finds the cluster's own.
	ClusterName string
	// Partitions is the partition count of the referenced cluster. Optimize
	// reads every partition of the exported records.
	Partitions int32
	// Suspended is spec.suspend of the referenced cluster. Optimize follows
	// it: a suspended cluster scales both Optimize workloads to zero, the way
	// it scales its own. The importer reads Elasticsearch directly, so it
	// would otherwise keep importing while the cluster is down, and a restore
	// of that cluster would write analytics from half-restored indices.
	Suspended bool
	// Platform is the spec of the CamundaPlatformConfig that the referenced
	// cluster names. It gives the image registry prefix and the license. It is
	// the zero value when the cluster names none.
	Platform v1.CamundaPlatformConfigSpec
	// Storage is the elasticsearch block of the SecondaryStorageConfig that
	// the referenced cluster names, with every Secret reference already
	// pointed at its copy in the CamundaOptimize namespace.
	Storage v1.ElasticsearchStorage
	// Auth is the ManagementAuthConfig that spec.managementAuthRef names, with
	// its client secret reference already pointed at its copy in the
	// CamundaOptimize namespace.
	Auth *v1.ManagementAuthConfig
	// HashInputs are the resource versions of the referenced Secrets and the
	// generations of the referenced custom resources, as
	// "kind/namespace/name=version" strings. ConfigHash sorts them, so the
	// order does not matter.
	HashInputs []string
	// ServiceMonitorSupported reports whether the Kubernetes cluster serves
	// the ServiceMonitor kind. When false, no ServiceMonitor is rendered.
	ServiceMonitorSupported bool
}

// Image returns the container image of both workloads. Optimize has its own
// patch line, so the tag comes from the CamundaOptimize and not from the
// cluster. The platform config governs the repository and the registry.
func Image(in Input) string {
	return images.Resolve(&in.Platform, images.Optimize, in.Optimize.Spec.Version)
}

// workload returns the WorkloadSpec of a component, or the zero value when the
// spec sets no block for it.
func (in Input) workload(component string) v1.WorkloadSpec {
	var spec *v1.WorkloadSpec
	if component == ComponentWebapp {
		spec = in.Optimize.Spec.Webapp
	} else {
		spec = in.Optimize.Spec.Importer
	}
	if spec == nil {
		return v1.WorkloadSpec{}
	}

	return *spec
}

// replicas returns the replica count of a component. Both default to 1. CEL
// holds the importer to 0 or 1, because Optimize supports one active importer.
// 0 is the suspend value. It stops the import while a restore or an index
// rewrite runs.
func (in Input) replicas(component string) int32 {
	if r := in.workload(component).Replicas; r != nil {
		return *r
	}

	return 1
}

// serviceMonitorSpec returns the ServiceMonitor settings of the spec, or nil
// when it sets none.
func (in Input) serviceMonitorSpec() *v1.ServiceMonitorSpec {
	if in.Optimize.Spec.Monitoring == nil {
		return nil
	}

	return in.Optimize.Spec.Monitoring.ServiceMonitor
}
