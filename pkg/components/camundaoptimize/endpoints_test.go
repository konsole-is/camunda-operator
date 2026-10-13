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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// The operator serves Optimize at the root of its host, so one route takes
// every path to the webapp. The importer serves no browser traffic.
func TestEndpointsRouteEveryPathToTheWebapp(t *testing.T) {
	t.Parallel()

	o := &v1.CamundaOptimize{}
	o.Name = "my-optimize"

	want := []v1.Endpoint{{
		Name:       "optimize",
		Service:    "my-optimize-webapp",
		Port:       8090,
		PathPrefix: "/",
		Protocol:   v1.EndpointProtocolHTTP,
	}}
	assert.Equal(t, want, Endpoints(o))
}

// A long name truncates in the Service name. The route must name the Service
// that the render produces, with a port it carries, and that Service must
// select the webapp pods, or the route answers nothing.
func TestEndpointsOfALongNameNameTheRenderedWebappService(t *testing.T) {
	t.Parallel()

	for _, length := range []int{63, 100, 253} {
		in := newInput(t, func(in *Input) {
			in.Optimize.Name = strings.Repeat("o", length)
		})
		comps, err := Build(in)
		require.NoError(t, err)

		rendered := map[string]*corev1.Service{}
		for _, comp := range comps {
			for _, obj := range previewObjects(t, comp) {
				if svc, ok := obj.(*corev1.Service); ok {
					rendered[svc.Name] = svc
				}
			}
		}

		endpoints := Endpoints(in.Optimize)
		require.Len(t, endpoints, 1, "%d", length)
		route := endpoints[0]
		assert.Empty(t, validation.IsDNS1123Label(route.Service), "%d: %s", length, route.Service)

		svc, ok := rendered[route.Service]
		require.True(t, ok, "%d: %s is not a rendered Service", length, route.Service)
		assert.Equal(t, ComponentWebapp, svc.Spec.Selector[labels.ComponentKey], "%d", length)

		ports := make([]int32, 0, len(svc.Spec.Ports))
		for _, port := range svc.Spec.Ports {
			ports = append(ports, port.Port)
		}
		assert.Contains(t, ports, route.Port, "%d", length)
	}
}
