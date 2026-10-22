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

package claimrequests

import (
	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/pkg/claimpolicy"
)

// Input is what the requests component of one claim reads. The claim
// controller fills it after its precheck passed.
type Input struct {
	// ClaimKind is written to spec.claimRef.kind.
	ClaimKind pluginsv1alpha1.ClaimKind
	// TargetKind is written to spec.target.kind, for example CamundaCluster.
	TargetKind string
	// ConditionType is the condition of the component.
	ConditionType string
	// Selections are spec.plugins of the claim, in claim order.
	Selections []fleetv1alpha1.PluginSelection
	// Order holds Selections in the order of claimpolicy.Order.
	Order []fleetv1alpha1.PluginSelection
	// Parameters are the parameters of each request, Resolution.Requests of
	// claimpolicy.Resolve. A caller may change one before it builds.
	Parameters []claimpolicy.RequestParameters
	// Sections and Registrations hold at least each one that a selection
	// names.
	Sections      []pluginsv1alpha1.PluginSection
	Registrations []pluginsv1alpha1.PluginRegistration
	// Requests are the PluginRequests of the claim as the precheck read them.
	Requests []pluginsv1alpha1.PluginRequest
	// AdditionalServiceAccounts holds target.additionalServiceAccounts by
	// section name.
	AdditionalServiceAccounts map[string][]string
	// DeletionPolicy is the effective policy: the claim value, else the
	// snapshot default.
	DeletionPolicy pluginsv1alpha1.DeletionPolicy
	// ReleaseRef is the release that the claim resolved. A request whose
	// registration declares a top-level string parameter releaseRef gets it
	// as that parameter, over any other value. Empty sets nothing.
	ReleaseRef string
}
