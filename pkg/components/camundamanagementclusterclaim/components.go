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

package camundamanagementclusterclaim

import (
	"fmt"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/pkg/claimpolicy"
	"github.com/konsole-is/camunda-operator/pkg/components/claimrequests"
)

// Build returns the components of the claim of in. A selection whose section
// or registration is not in in is an error, and so is an output that does
// not fit its field.
func Build(in Input) (Components, error) {
	requests, err := claimrequests.Component(in.Claim, RequestsInput(in))
	if err != nil {
		return Components{}, fmt.Errorf("building the requests component: %w", err)
	}

	plane, err := managementClusterComponent(in)
	if err != nil {
		return Components{}, fmt.Errorf("building the management cluster component: %w", err)
	}

	return Components{Requests: requests, ManagementCluster: plane}, nil
}

// RequestsInput returns the input of the requests component of the claim of
// in. claimrequests.Statuses reads it too. A plane claim has no release and
// trusts no other ServiceAccount.
func RequestsInput(in Input) claimrequests.Input {
	return claimrequests.Input{
		ClaimKind:      pluginsv1alpha1.ClaimKindCamundaManagementClusterClaim,
		TargetKind:     claimpolicy.KindCamundaManagementCluster,
		ConditionType:  fleetv1alpha1.ConditionPlaneRequestsReady,
		Selections:     in.Claim.Spec.Plugins,
		Order:          in.Order,
		Parameters:     in.Parameters,
		Sections:       in.Sections,
		Registrations:  in.Registrations,
		Requests:       in.Requests,
		DeletionPolicy: DeletionPolicy(in.Claim),
	}
}

// DeletionPolicy returns the deletion policy of the requests of claim: its
// own policy, else the default of its snapshot, else Delete.
func DeletionPolicy(claim *fleetv1alpha1.CamundaManagementClusterClaim) pluginsv1alpha1.DeletionPolicy {
	return claimpolicy.DeletionPolicy(claim.Spec.DeletionPolicy, claim.Status.ResolvedPolicy)
}

// PlatformConfigRef returns the platform config of the plane of claim: the
// claim value, else the platformConfigRef of environment. It is empty when
// neither sets it. environment may be nil.
func PlatformConfigRef(
	claim *fleetv1alpha1.CamundaManagementClusterClaim,
	environment *fleetv1alpha1.Environment,
) string {
	if claim.Spec.PlatformConfigRef != "" || environment == nil {
		return claim.Spec.PlatformConfigRef
	}

	return environment.Spec.PlatformConfigRef
}
