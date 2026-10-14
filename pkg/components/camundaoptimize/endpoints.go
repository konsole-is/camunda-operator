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
	"github.com/konsole-is/camunda-operator/pkg/externalurl"
)

// Endpoints returns the routes of an Optimize for status.endpoints: one
// entry that takes the path of spec.externalUrl, or / for a URL without one,
// to the webapp Service. The webapp serves under that path. The importer
// serves no browser traffic and has no route.
//
// The route ignores a suspension. It describes the Service, and the Service
// stays while the pods are at zero.
func Endpoints(o *v1.CamundaOptimize) []v1.Endpoint {
	return []v1.Endpoint{{
		Name:       v1.EndpointOptimize,
		Service:    WorkloadName(o, ComponentWebapp),
		Port:       PortHTTP,
		PathPrefix: externalurl.RoutePrefix(o.Spec.ExternalURL),
		Protocol:   v1.EndpointProtocolHTTP,
	}}
}
