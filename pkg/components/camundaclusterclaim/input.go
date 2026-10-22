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

package camundaclusterclaim

import (
	"github.com/sourcehawk/operator-component-framework/pkg/component"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/claimpolicy"
)

// BuiltinStorage names the built-in storage registrations whose pods the
// cloud identity also trusts, and the suffix that each adds to the claim
// name in the name of the resource it writes.
type BuiltinStorage struct {
	ElasticsearchRegistration  string
	ElasticsearchSuffix        string
	DatabaseServerRegistration string
	DatabaseServerSuffix       string
}

// Input is what the components of one cluster claim read. The controller
// fills it after its precheck passed.
type Input struct {
	Claim *fleetv1alpha1.CamundaClusterClaim
	// ReleaseRef and PlatformConfigRef are the claim values, else the
	// defaults of the Environment. Empty leaves the field unset.
	ReleaseRef        string
	PlatformConfigRef string
	// Order holds spec.plugins of the claim in the order of
	// claimpolicy.Order.
	Order         []fleetv1alpha1.PluginSelection
	Parameters    []claimpolicy.RequestParameters
	Sections      []pluginsv1alpha1.PluginSection
	Registrations []pluginsv1alpha1.PluginRegistration
	// Requests are the PluginRequests of the claim, with their outputs.
	Requests []pluginsv1alpha1.PluginRequest
	// Live is the CamundaCluster of the claim. Nil before it exists. A slot
	// field whose output is missing keeps its value.
	Live    *v1.CamundaCluster
	Storage BuiltinStorage
}

// Components are the components of a cluster claim, in reconcile order.
type Components struct {
	Requests *component.Component
	Cluster  *component.Component
}
