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

package camundamanagementcluster

import (
	"slices"
	"strings"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/externalurl"
)

// Endpoints returns the routes of a management plane for status.endpoints,
// sorted by name: one entry for each app that the spec runs, with the
// Service and the port that serve it. The path prefix of an app is the path
// of its external URL, or / for a URL without one, because each app serves
// under that path. Keycloak serves under /auth and runs only in the
// keycloak mode.
//
// The routes ignore spec.suspend. They describe Services, and the Services
// stay while the pods are at zero.
func Endpoints(mc *v1.CamundaManagementCluster) []v1.Endpoint {
	endpoints := []v1.Endpoint{
		httpEndpoint(
			v1.EndpointIdentity,
			IdentityName(mc),
			IdentityServicePortHTTP,
			externalurl.RoutePrefix(mc.Spec.Identity.ExternalURL),
		),
	}
	if Mode(mc) == ModeKeycloak {
		endpoints = append(
			endpoints,
			httpEndpoint(v1.EndpointKeycloak, KeycloakServiceName(mc), KeycloakServicePort, keycloakBasePath),
		)
	}
	if console := mc.Spec.Console; console != nil {
		endpoints = append(
			endpoints,
			httpEndpoint(
				v1.EndpointConsole,
				ConsoleName(mc),
				ConsoleServicePortHTTP,
				externalurl.RoutePrefix(console.ExternalURL),
			),
		)
	}
	if modeler := mc.Spec.WebModeler; modeler != nil {
		endpoints = append(
			endpoints,
			httpEndpoint(
				v1.EndpointWebModeler,
				WebModelerRestapiName(mc),
				WebModelerRestapiServicePortHTTP,
				externalurl.RoutePrefix(modeler.ExternalURL),
			),
			httpEndpoint(
				v1.EndpointWebModelerWebsockets,
				WebModelerWebsocketsName(mc),
				WebModelerWebsocketsServicePortHTTP,
				externalurl.RoutePrefix(modeler.WebsocketsExternalURL),
			),
		)
	}

	slices.SortFunc(endpoints, func(a, b v1.Endpoint) int { return strings.Compare(a.Name, b.Name) })

	return endpoints
}

// httpEndpoint returns an HTTP route on a port of a Service.
func httpEndpoint(name, service string, port int32, pathPrefix string) v1.Endpoint {
	return v1.Endpoint{
		Name:       name,
		Service:    service,
		Port:       port,
		PathPrefix: pathPrefix,
		Protocol:   v1.EndpointProtocolHTTP,
	}
}
