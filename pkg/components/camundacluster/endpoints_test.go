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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// topology is one combination of the fields that decide the routes.
type topology struct {
	gateway, operate, tasklist, admin v1.ComponentMode
	connectors                        bool
}

func (tp topology) String() string {
	return fmt.Sprintf(
		"gateway=%s/operate=%s/tasklist=%s/admin=%s/connectors=%t",
		tp.gateway, tp.operate, tp.tasklist, tp.admin, tp.connectors,
	)
}

// everyTopology returns the 32 combinations of the gateway mode, the three
// web application modes, and connectors on or off.
func everyTopology() []topology {
	modes := []v1.ComponentMode{v1.ComponentModeStandalone, v1.ComponentModeEmbedded}

	var all []topology
	for _, gateway := range modes {
		for _, operate := range modes {
			for _, tasklist := range modes {
				for _, admin := range modes {
					for _, connectors := range []bool{false, true} {
						all = append(all, topology{gateway, operate, tasklist, admin, connectors})
					}
				}
			}
		}
	}
	return all
}

// topologyInput returns the minimal cluster my-cluster in the given topology.
func topologyInput(t *testing.T, tp topology) Input {
	t.Helper()
	return newInput(t, func(in *Input) {
		in.Cluster.Spec.Gateway = &v1.GatewaySpec{Mode: tp.gateway}
		in.Cluster.Spec.Operate = &v1.WebAppSpec{Mode: tp.operate}
		in.Cluster.Spec.Tasklist = &v1.WebAppSpec{Mode: tp.tasklist}
		in.Cluster.Spec.Admin = &v1.WebAppSpec{Mode: tp.admin}
		in.Cluster.Spec.Connectors = &v1.ConnectorsSpec{Enabled: new(tp.connectors), Version: "8.9.7"}
		in.Effective = NewEffective(in.Cluster.Spec)
	})
}

// wantEndpoints spells out the table of #575 for one topology of my-cluster,
// in the sorted order that a reader of status.endpoints sees.
func wantEndpoints(tp topology) []v1.Endpoint {
	host := "my-cluster-gateway"
	if tp.gateway == v1.ComponentModeEmbedded {
		host = "my-cluster-zeebe-client"
	}
	app := func(name string, mode v1.ComponentMode) v1.Endpoint {
		service := host
		if mode == v1.ComponentModeStandalone {
			service = "my-cluster-" + name
		}
		return v1.Endpoint{
			Name: name, Service: service, Port: 8080, PathPrefix: "/" + name, Protocol: v1.EndpointProtocolHTTP,
		}
	}

	want := []v1.Endpoint{app("admin", tp.admin)}
	if tp.connectors {
		want = append(want, v1.Endpoint{
			Name: "connectors", Service: "my-cluster-connectors", Port: 8080, PathPrefix: "/inbound",
			Protocol: v1.EndpointProtocolHTTP,
		})
	}
	return append(
		want,
		v1.Endpoint{Name: "default", Service: host, Port: 8080, PathPrefix: "/", Protocol: v1.EndpointProtocolHTTP},
		v1.Endpoint{Name: "grpc", Service: host, Port: 26500, Protocol: v1.EndpointProtocolGRPC},
		app("operate", tp.operate),
		v1.Endpoint{Name: "rest", Service: host, Port: 8080, PathPrefix: "/v2", Protocol: v1.EndpointProtocolHTTP},
		app("tasklist", tp.tasklist),
	)
}

// The 8.9 default: a standalone gateway serves every path, and connectors
// do not run.
func TestEndpointsOfTheDefaultTopology(t *testing.T) {
	t.Parallel()

	in := fixtureMinimal(t)
	assert.Equal(t, []v1.Endpoint{
		{Name: "admin", Service: "my-cluster-gateway", Port: 8080, PathPrefix: "/admin", Protocol: v1.EndpointProtocolHTTP},
		{Name: "default", Service: "my-cluster-gateway", Port: 8080, PathPrefix: "/", Protocol: v1.EndpointProtocolHTTP},
		{Name: "grpc", Service: "my-cluster-gateway", Port: 26500, Protocol: v1.EndpointProtocolGRPC},
		{Name: "operate", Service: "my-cluster-gateway", Port: 8080, PathPrefix: "/operate", Protocol: v1.EndpointProtocolHTTP},
		{Name: "rest", Service: "my-cluster-gateway", Port: 8080, PathPrefix: "/v2", Protocol: v1.EndpointProtocolHTTP},
		{Name: "tasklist", Service: "my-cluster-gateway", Port: 8080, PathPrefix: "/tasklist", Protocol: v1.EndpointProtocolHTTP},
	}, Endpoints(in.Cluster, in.Effective))
}

// Every topology gets the full table, sorted by name. A standalone web
// application gets its own Service, an embedded one the Service of the
// process that runs the gateway.
func TestEndpointsOfEveryTopology(t *testing.T) {
	t.Parallel()

	for _, tp := range everyTopology() {
		t.Run(tp.String(), func(t *testing.T) {
			t.Parallel()

			in := topologyInput(t, tp)
			assert.Equal(t, wantEndpoints(tp), Endpoints(in.Cluster, in.Effective))
		})
	}
}

// A route to a Service that the cluster does not render, or to a port that
// the Service does not carry, answers nothing. This keeps the table and the
// components together, and the names unique for the list map.
func TestEveryEndpointNamesARenderedServicePort(t *testing.T) {
	t.Parallel()

	for _, tp := range everyTopology() {
		t.Run(tp.String(), func(t *testing.T) {
			t.Parallel()

			in := topologyInput(t, tp)
			comps, err := Build(in)
			require.NoError(t, err)

			rendered := map[string][]int32{}
			for _, comp := range enabledComponents(comps) {
				for _, obj := range previewObjects(t, comp) {
					if svc, ok := obj.(*corev1.Service); ok {
						for _, port := range svc.Spec.Ports {
							rendered[svc.Name] = append(rendered[svc.Name], port.Port)
						}
					}
				}
			}

			seen := map[string]bool{}
			for _, endpoint := range Endpoints(in.Cluster, in.Effective) {
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

	tp := topology{
		gateway: v1.ComponentModeEmbedded, operate: v1.ComponentModeStandalone,
		tasklist: v1.ComponentModeEmbedded, admin: v1.ComponentModeEmbedded, connectors: true,
	}
	running := topologyInput(t, tp)
	suspended := topologyInput(t, tp)
	suspended.Effective.Suspend = true

	assert.Equal(
		t,
		Endpoints(running.Cluster, running.Effective),
		Endpoints(suspended.Cluster, suspended.Effective),
	)
}
