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

package camundacluster

import (
	"slices"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaconfig"
)

// ProcessKind is the workload kind of a process.
type ProcessKind string

const (
	// ProcessStatefulSet is the kind of the brokers.
	ProcessStatefulSet ProcessKind = "StatefulSet"
	// ProcessDeployment is the kind of every other process.
	ProcessDeployment ProcessKind = "Deployment"
)

// Process is one workload of the cluster and the role it plays.
type Process struct {
	// Component is the label value and the name suffix of the process.
	Component string
	// Kind is the workload kind.
	Kind ProcessKind
	// Replicas is the number of pods.
	Replicas int32
	// Profiles is SPRING_PROFILES_ACTIVE, sorted. It always contains
	// consolidated-auth for a unified process and is empty for connectors.
	Profiles []string
	// EmbeddedGateway is true on the brokers when they run the gateway
	// (ZEEBE_BROKER_GATEWAY_ENABLE).
	EmbeddedGateway bool
	// ServesHTTP reports whether the Service exposes port 8080: the gateway,
	// the web applications, the brokers with an embedded gateway, and
	// connectors.
	ServesHTTP bool
	// ServesGRPC reports whether the Service exposes port 26500: the gateway
	// and the brokers with an embedded gateway.
	ServesGRPC bool
	// ConditionType is the condition that the process reports, one of the
	// v1.Condition<X>Ready constants.
	ConditionType string
}

// webApp is one of the three web applications with the profile that serves
// it.
type webApp struct {
	component string
	profile   string
	condition string
	mode      func(Effective) v1.ComponentMode
}

// webApps lists the web applications in their stable order.
var webApps = []webApp{
	{ComponentOperate, camundaconfig.ProfileOperate, v1.ConditionOperateReady, Effective.OperateMode},
	{ComponentTasklist, camundaconfig.ProfileTasklist, v1.ConditionTasklistReady, Effective.TasklistMode},
	{ComponentAdmin, camundaconfig.ProfileAdmin, v1.ConditionAdminReady, Effective.AdminMode},
}

// Resolve maps the effective spec to its processes in a stable order: zeebe,
// gateway, operate, tasklist, admin, connectors. An embedded web
// application rides on the gateway when the gateway is standalone, otherwise
// on zeebe; the host carries the application's profile.
func Resolve(e Effective) []Process {
	gatewayEmbedded := e.GatewayMode() == v1.ComponentModeEmbedded

	hostProfiles := []string{}
	standalone := []Process{}
	for _, app := range webApps {
		if app.mode(e) == v1.ComponentModeStandalone {
			standalone = append(standalone, Process{
				Component:     app.component,
				Kind:          ProcessDeployment,
				Replicas:      e.Replicas(app.component),
				Profiles:      profiles(camundaconfig.ProfileGateway, app.profile),
				ServesHTTP:    true,
				ConditionType: app.condition,
			})
			continue
		}
		hostProfiles = append(hostProfiles, app.profile)
	}

	zeebe := Process{
		Component:       ComponentZeebe,
		Kind:            ProcessStatefulSet,
		Replicas:        e.ZeebeReplicas(),
		Profiles:        profiles(camundaconfig.ProfileBroker),
		EmbeddedGateway: gatewayEmbedded,
		ServesHTTP:      gatewayEmbedded,
		ServesGRPC:      gatewayEmbedded,
		ConditionType:   v1.ConditionZeebeReady,
	}
	if gatewayEmbedded {
		zeebe.Profiles = profiles(append(hostProfiles, camundaconfig.ProfileBroker)...)
	}
	processes := []Process{zeebe}

	if !gatewayEmbedded {
		processes = append(processes, Process{
			Component:     ComponentGateway,
			Kind:          ProcessDeployment,
			Replicas:      e.Replicas(ComponentGateway),
			Profiles:      profiles(append(hostProfiles, camundaconfig.ProfileGateway)...),
			ServesHTTP:    true,
			ServesGRPC:    true,
			ConditionType: v1.ConditionGatewayReady,
		})
	}

	processes = append(processes, standalone...)

	if e.ConnectorsEnabled() {
		processes = append(processes, Process{
			Component:     ComponentConnectors,
			Kind:          ProcessDeployment,
			Replicas:      e.Replicas(ComponentConnectors),
			ServesHTTP:    true,
			ConditionType: v1.ConditionConnectorsReady,
		})
	}

	return processes
}

// profiles returns the given profiles plus consolidated-auth, sorted.
func profiles(names ...string) []string {
	all := append(slices.Clone(names), camundaconfig.ProfileConsolidatedAuth)
	slices.Sort(all)
	return all
}

// GatewayHost returns the Service name that clients (connectors) call: the
// gateway Service, or the zeebe Service when the gateway is embedded.
func GatewayHost(cluster *v1.CamundaCluster, e Effective) string {
	if e.GatewayMode() == v1.ComponentModeEmbedded {
		return WorkloadName(cluster, ComponentZeebe)
	}
	return WorkloadName(cluster, ComponentGateway)
}
