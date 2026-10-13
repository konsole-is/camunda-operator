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
	"strings"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaconfig"
)

// Endpoints returns the routes of a cluster for status.endpoints, sorted by
// name: one entry for each path, with the Service and the port that serve
// it. The default, rest, and grpc entries name the Service of the process
// that runs the gateway: the gateway Service, or the client Service of the
// brokers when the gateway is embedded. A web application names its own
// Service when it runs standalone, otherwise the same Service. The
// connectors entry is present only while connectors are enabled.
//
// The routes ignore spec.suspend. They describe Services, and the Services
// stay while the pods are at zero.
func Endpoints(cluster *v1.CamundaCluster, e Effective) []v1.Endpoint {
	processes := Resolve(e)
	serving := func(serves func(Process) bool) string {
		for _, p := range processes {
			if p.Enabled && serves(p) {
				return routeService(cluster, p)
			}
		}
		// Resolve gives the gateway and each web application to exactly one
		// enabled process.
		return ""
	}
	hosts := func(profile string) func(Process) bool {
		return func(p Process) bool { return slices.Contains(p.Profiles, profile) }
	}
	gateway := serving(func(p Process) bool { return p.ServesGRPC })

	endpoints := []v1.Endpoint{
		httpEndpoint(v1.EndpointDefault, gateway, "/"),
		httpEndpoint(v1.EndpointREST, gateway, "/v2"),
		httpEndpoint(v1.EndpointOperate, serving(hosts(camundaconfig.ProfileOperate)), "/operate"),
		httpEndpoint(v1.EndpointTasklist, serving(hosts(camundaconfig.ProfileTasklist)), "/tasklist"),
		httpEndpoint(v1.EndpointAdmin, serving(hosts(camundaconfig.ProfileAdmin)), "/admin"),
		{Name: v1.EndpointGRPC, Service: gateway, Port: PortGRPC, Protocol: v1.EndpointProtocolGRPC},
	}
	if e.ConnectorsEnabled() {
		// The operator sets no context path on connectors, so Camunda serves
		// the inbound webhooks under /inbound/<webhook ID>.
		endpoints = append(
			endpoints,
			httpEndpoint(v1.EndpointConnectors, WorkloadName(cluster, ComponentConnectors), "/inbound"),
		)
	}

	slices.SortFunc(endpoints, func(a, b v1.Endpoint) int { return strings.Compare(a.Name, b.Name) })

	return endpoints
}

// routeService returns the Service that a route to the process names. For
// the brokers it is the client Service, not the headless broker Service that
// GatewayHost names, because a route must reach ready brokers only.
func routeService(cluster *v1.CamundaCluster, p Process) string {
	if p.Kind == ProcessStatefulSet {
		return ZeebeClientServiceName(cluster)
	}

	return WorkloadName(cluster, p.Component)
}

// httpEndpoint returns an HTTP route on the HTTP port of a Service.
func httpEndpoint(name, service, pathPrefix string) v1.Endpoint {
	return v1.Endpoint{
		Name:       name,
		Service:    service,
		Port:       PortHTTP,
		PathPrefix: pathPrefix,
		Protocol:   v1.EndpointProtocolHTTP,
	}
}
