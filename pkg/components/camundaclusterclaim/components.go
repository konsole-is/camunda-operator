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

	cluster, err := clusterComponent(in)
	if err != nil {
		return Components{}, fmt.Errorf("building the cluster component: %w", err)
	}

	attachments, err := attachmentsComponent(in)
	if err != nil {
		return Components{}, fmt.Errorf("building the attachments component: %w", err)
	}

	return Components{Requests: requests, Cluster: cluster, Attachments: attachments}, nil
}

// RequestsInput returns the input of the requests component of the claim of
// in. claimrequests.Statuses reads it too.
func RequestsInput(in Input) claimrequests.Input {
	requests := claimrequests.Input{
		ClaimKind:      pluginsv1alpha1.ClaimKindCamundaClusterClaim,
		TargetKind:     claimpolicy.KindCamundaCluster,
		ConditionType:  fleetv1alpha1.ConditionRequestsReady,
		Selections:     in.Claim.Spec.Plugins,
		Order:          in.Order,
		Parameters:     in.Parameters,
		Sections:       in.Sections,
		Registrations:  in.Registrations,
		Requests:       in.Requests,
		DeletionPolicy: DeletionPolicy(in.Claim),
		ReleaseRef:     in.ReleaseRef,
	}

	accounts := AdditionalServiceAccounts(in.Claim.Name, in.Claim.Spec.Plugins, in.Storage)
	if len(accounts) == 0 {
		return requests
	}

	for _, s := range in.Claim.Spec.Plugins {
		for _, section := range in.Sections {
			if section.Name == s.Section && section.Spec.Slot == pluginsv1alpha1.SlotWorkloadIdentity {
				requests.AdditionalServiceAccounts = map[string][]string{s.Section: accounts}
			}
		}
	}

	return requests
}
