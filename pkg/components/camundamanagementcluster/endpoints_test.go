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
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/credentials"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/keycloak"
)

// fixtureWebSocketsRootURL is a websockets URL without a path, on a host of
// its own.
const fixtureWebSocketsRootURL = "https://modeler-ws.example.com"

// plane is one combination of the fields that decide the routes.
type plane struct {
	mode                ProviderMode
	console, webModeler bool
	// paths puts every app under a path of one host, as the claim
	// controller does. Without it, every URL is the root of its host.
	paths bool
}

func (p plane) String() string {
	return fmt.Sprintf("%s/console=%t/webModeler=%t/paths=%t", p.mode, p.console, p.webModeler, p.paths)
}

// everyPlane returns the 24 combinations of the three modes, Console on or
// off, Web Modeler on or off, and URLs with or without a path.
func everyPlane() []plane {
	all := make([]plane, 0, 24)
	for _, mode := range []ProviderMode{ModeKeycloak, ModeExternalKeycloak, ModeOIDC} {
		for _, console := range []bool{false, true} {
			for _, webModeler := range []bool{false, true} {
				for _, paths := range []bool{false, true} {
					all = append(all, plane{mode, console, webModeler, paths})
				}
			}
		}
	}
	return all
}

// planeInput returns the render input of the management plane my-management
// in the given combination.
func planeInput(t *testing.T, p plane) Input {
	t.Helper()

	identityURL, consoleURL := fixtureExternal, fixtureConsoleURL
	modelerURL, websocketsURL := fixtureWebModelerURL, fixtureWebSocketsRootURL
	if p.paths {
		identityURL, consoleURL = fixtureIdentityPathURL, fixtureConsolePathURL
		modelerURL, websocketsURL = fixtureWebModelerPathURL, fixtureWebSocketsPathURL
	}

	mutate := func(in *Input) {
		in.Cluster.Spec.Identity.ExternalURL = identityURL
		if p.console {
			withConsole(in, consoleURL)
		}
		if p.webModeler {
			withWebModeler(in.Cluster)
			in.Cluster.Spec.WebModeler.ExternalURL = modelerURL
			in.Cluster.Spec.WebModeler.WebsocketsExternalURL = websocketsURL
			withWebModelerClients(in.Platform)
			in.Databases.WebModeler = webModelerDatabase()
			in.Pusher = PusherCredentials{
				Key:    credentials.Password{Value: fixturePusherKey},
				Secret: credentials.Password{Value: fixturePusherSecret},
			}
		}
	}
	if p.mode == ModeOIDC {
		return newInput(t, mutate)
	}
	return newKeycloakInput(t, p.mode == ModeKeycloak, mutate)
}

// wantEndpoints spells out the routes of my-management in one combination,
// in the sorted order that a reader of status.endpoints sees.
func wantEndpoints(p plane) []v1.Endpoint {
	path := func(prefix string) string {
		if p.paths {
			return prefix
		}
		return "/"
	}
	route := func(name, service string, port int32, prefix string) v1.Endpoint {
		return v1.Endpoint{
			Name: name, Service: service, Port: port, PathPrefix: prefix, Protocol: v1.EndpointProtocolHTTP,
		}
	}

	var want []v1.Endpoint
	if p.console {
		want = append(want, route("console", "my-management-console", 80, path("/console")))
	}
	want = append(want, route("identity", "my-management-identity", 80, path("/identity")))
	if p.mode == ModeKeycloak {
		want = append(want, route("keycloak", "my-management-keycloak-service", 8080, "/auth"))
	}
	if p.webModeler {
		want = append(
			want,
			route("web-modeler", "my-management-web-modeler-restapi", 80, path("/modeler")),
			route("web-modeler-websockets", "my-management-web-modeler-websockets", 80, path("/modeler-ws")),
		)
	}
	return want
}

// Every combination gets its full table, sorted by name. Only the keycloak
// mode runs Keycloak, so only it routes /auth. Each app routes the path of
// its URL, or / for a URL at the root of its host.
func TestEndpointsOfEveryPlane(t *testing.T) {
	t.Parallel()

	for _, p := range everyPlane() {
		t.Run(p.String(), func(t *testing.T) {
			t.Parallel()

			in := planeInput(t, p)
			assert.Equal(t, wantEndpoints(p), Endpoints(in.Cluster))
		})
	}
}

// An app serves under the path of its URL, without a trailing slash, so the
// route matches that path and nothing else.
func TestEndpointsTakeThePathTheAppServesUnder(t *testing.T) {
	t.Parallel()

	for url, want := range map[string]string{
		"https://camunda.example.com/console/":        "/console",
		"https://example.com/camunda/console":         "/camunda/console",
		"https://console.example.com/":                "/",
		"https://console.example.com":                 "/",
		"https://camunda.example.com/console?tab=all": "/console",
	} {
		t.Run(url, func(t *testing.T) {
			t.Parallel()

			mc := newCluster(func(mc *v1.CamundaManagementCluster) {
				mc.Spec.Console = &v1.ConsoleSpec{Version: fixtureVersion, ExternalURL: url}
			})
			routes := map[string]v1.Endpoint{}
			for _, endpoint := range Endpoints(mc) {
				routes[endpoint.Name] = endpoint
			}
			assert.Equal(t, want, routes[v1.EndpointConsole].PathPrefix)
		})
	}
}

// A route to a Service that the plane does not render, or to a port that
// the Service does not carry, answers nothing. This keeps the table and the
// components together, and the names unique for the list map. The Keycloak
// Operator, not this operator, creates the Service of a Keycloak: it names
// it <Keycloak>-service and publishes spec.http.httpPort.
func TestEveryEndpointNamesARenderedServicePort(t *testing.T) {
	t.Parallel()

	inputs := map[string]Input{}
	for _, p := range everyPlane() {
		inputs[p.String()] = planeInput(t, p)
	}
	// A long name is truncated in every derived name. The routes must name
	// the truncated Services.
	long := planeInput(t, plane{mode: ModeKeycloak, console: true, webModeler: true, paths: true})
	long.Cluster.Name = strings.Repeat("a", 63)
	inputs["long-name"] = long

	for name, in := range inputs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			built, err := Build(in)
			require.NoError(t, err)

			rendered := map[string][]int32{}
			for _, comp := range built.Components {
				objects, err := comp.Preview()
				require.NoError(t, err)
				for _, obj := range objects {
					switch o := obj.(type) {
					case *corev1.Service:
						for _, port := range o.Spec.Ports {
							rendered[o.Name] = append(rendered[o.Name], port.Port)
						}
					case *keycloak.Keycloak:
						rendered[o.Name+"-service"] = append(rendered[o.Name+"-service"], *o.Spec.HTTP.HTTPPort)
					}
				}
			}

			seen := map[string]bool{}
			for _, endpoint := range Endpoints(in.Cluster) {
				assert.False(t, seen[endpoint.Name], "two entries are named %s", endpoint.Name)
				seen[endpoint.Name] = true
				assert.Contains(
					t, rendered[endpoint.Service], endpoint.Port,
					"%s names %s:%d, which the render does not produce", endpoint.Name, endpoint.Service, endpoint.Port,
				)
			}
		})
	}
}

// The routes describe the Services, which stay while the pods are at zero,
// so a suspension changes no entry.
func TestEndpointsIgnoreSuspension(t *testing.T) {
	t.Parallel()

	p := plane{mode: ModeKeycloak, console: true, webModeler: true, paths: true}
	running := planeInput(t, p)
	suspended := planeInput(t, p)
	suspended.Cluster.Spec.Suspend = true

	assert.Equal(t, Endpoints(running.Cluster), Endpoints(suspended.Cluster))
}
