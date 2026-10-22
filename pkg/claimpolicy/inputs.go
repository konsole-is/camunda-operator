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

package claimpolicy

import (
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// InputValues are the resolved inputs of one request.
type InputValues struct {
	// Values holds the value of each input, by input name.
	Values map[string]string `json:"values,omitempty"`
	// Waiting are the sections, sorted and distinct, whose outputs an input
	// waits for.
	Waiting []string `json:"waiting,omitempty"`
}

// Inputs resolves the inputs of registration from the outputs of the
// requests of the claim. An input reads its output from the request of the
// first selection of its section, in claim order. It reads the output only
// while that request reports OutputsReady=True for its current generation.
// Until then, and while the request or its output does not exist, the
// section is in Waiting. An input whose section has no selection is left
// out and does not wait.
func Inputs(
	registration *pluginsv1alpha1.PluginRegistration,
	selections []fleetv1alpha1.PluginSelection,
	requests []pluginsv1alpha1.PluginRequest,
) InputValues {
	var values InputValues
	for _, input := range registration.Spec.Inputs {
		section, output, _ := strings.Cut(input.From, "/")
		i := slices.IndexFunc(selections, func(s fleetv1alpha1.PluginSelection) bool { return s.Section == section })
		if i < 0 {
			continue
		}

		value, ok := readyOutputs(requests, section, selections[i].Registration)[output]
		if !ok {
			values.Waiting = append(values.Waiting, section)
			continue
		}

		if values.Values == nil {
			values.Values = map[string]string{}
		}
		values.Values[input.Name] = value
	}

	slices.Sort(values.Waiting)
	values.Waiting = slices.Compact(values.Waiting)

	return values
}

// readyOutputs returns the outputs of the request of section and
// registration only while the request reports OutputsReady=True for its
// current generation. Otherwise it returns nil.
func readyOutputs(requests []pluginsv1alpha1.PluginRequest, section, registration string) map[string]string {
	for _, r := range requests {
		if r.Spec.Section != section || r.Spec.Registration != registration {
			continue
		}

		ready := meta.FindStatusCondition(r.Status.Conditions, pluginsv1alpha1.ConditionOutputsReady)
		if ready == nil || ready.Status != metav1.ConditionTrue || ready.ObservedGeneration != r.Generation {
			return nil
		}
		return r.Status.Outputs
	}

	return nil
}

// WaitingReason returns the guard reason of a claim resource that waits for
// the outputs of sections, or "" when sections is empty.
func WaitingReason(sections []string) string {
	if len(sections) == 0 {
		return ""
	}

	return "waiting for the outputs of " + strings.Join(sections, ", ")
}
