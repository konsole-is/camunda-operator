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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// Statuses returns status.requests: one item for each selection, in claim
// order, with the status of the OutputsReady and Ready conditions of its
// request as in.Requests holds it. A request that is missing or has no such
// condition leaves the status empty. A selection whose section is not in
// in.Sections gets the name of a request of a One section.
func Statuses(claim client.Object, in Input) []fleetv1alpha1.RequestStatus {
	statuses := make([]fleetv1alpha1.RequestStatus, 0, len(in.Selections))
	for _, s := range in.Selections {
		status := fleetv1alpha1.RequestStatus{
			Section:      s.Section,
			Registration: s.Registration,
			Name:         Name(claim.GetName(), s, modeOf(in.Sections, s.Section)),
		}
		if request := find(in.Requests, status.Name); request != nil {
			status.OutputsReady = conditionStatus(request, pluginsv1alpha1.ConditionOutputsReady)
			status.Ready = conditionStatus(request, v1.ConditionReady)
		}
		statuses = append(statuses, status)
	}

	return statuses
}

func conditionStatus(request *pluginsv1alpha1.PluginRequest, conditionType string) metav1.ConditionStatus {
	if c := meta.FindStatusCondition(request.Status.Conditions, conditionType); c != nil {
		return c.Status
	}

	return ""
}
