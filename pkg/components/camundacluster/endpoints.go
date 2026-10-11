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
)

// Endpoints returns the routes of a cluster for status.endpoints, sorted by
// name: one entry for each path, with the Service and the port that serve
// it. The default, rest, and grpc entries name the Service of the process
// that runs the gateway: the gateway Service, or the client Service of the
// brokers when the gateway is embedded. A web application names its own
// Service when it runs standalone, otherwise the same Service. The
// connectors entry is present only while connectors run.
//
// The routes ignore spec.suspend. They describe Services, and the Services
// stay while the pods are at zero.
func Endpoints(cluster *v1.CamundaCluster, e Effective) []v1.Endpoint {
	host := routeService(cluster, e)
	webApp := func(component string, mode v1.ComponentMode) string {
		if mode == v1.ComponentModeStandalone {
			return WorkloadName(cluster, component)
		}
		return host
	}

	endpoints := []v1.Endpoint{
		httpEndpoint(v1.EndpointDefault, host, "/"),
		httpEndpoint(v1.EndpointREST, host, "/v2"),
		httpEndpoint(v1.EndpointOperate, webApp(ComponentOperate, e.OperateMode()), "/operate"),
		httpEndpoint(v1.EndpointTasklist, webApp(ComponentTasklist, e.TasklistMode()), "/tasklist"),
		httpEndpoint(v1.EndpointAdmin, webApp(ComponentAdmin, e.AdminMode()), "/admin"),
		{Name: v1.EndpointGRPC, Service: host, Port: PortGRPC, Protocol: v1.EndpointProtocolGRPC},
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

// routeService returns the Service that a route to the gateway names. With
// an embedded gateway it is the client Service, not the headless broker
// Service that GatewayHost names, because a route must reach ready brokers
// only.
func routeService(cluster *v1.CamundaCluster, e Effective) string {
	if e.GatewayMode() == v1.ComponentModeEmbedded {
		return ZeebeClientServiceName(cluster)
	}

	return WorkloadName(cluster, ComponentGateway)
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
