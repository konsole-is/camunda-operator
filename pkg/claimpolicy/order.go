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

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// Order returns the selections in the order in which their requests are
// written: each selection comes after every selection of a section that one
// of its inputs reads from. Selections without such a link keep the claim
// order. An optional input whose section has no selection adds no link.
//
// It refuses, in this order: a selected registration that does not exist
// (InvalidReference); a required input whose section has no selection, in
// claim order and input order (MissingInput); inputs that form a cycle
// (WiringCycle). The message of a cycle names each section on a cycle, in
// name order, and no section that only reads from one.
func Order(
	selections []fleetv1alpha1.PluginSelection,
	registrations []pluginsv1alpha1.PluginRegistration,
) ([]fleetv1alpha1.PluginSelection, error) {
	selected := make([]*pluginsv1alpha1.PluginRegistration, len(selections))
	for i, s := range selections {
		registration, err := registrationOf(s.Registration, registrations)
		if err != nil {
			return nil, err
		}
		selected[i] = registration
	}

	if err := checkRequiredInputs(selections, selected); err != nil {
		return nil, err
	}

	edges := inputEdges(selections, selected)
	if cycle := cycleSections(selections, edges); len(cycle) > 0 {
		return nil, refuse(
			fleetv1alpha1.ReasonWiringCycle,
			"the inputs of these sections form a cycle: %s",
			strings.Join(cycle, ", "),
		)
	}

	return topologicalOrder(selections, edges), nil
}

func checkRequiredInputs(
	selections []fleetv1alpha1.PluginSelection,
	registrations []*pluginsv1alpha1.PluginRegistration,
) error {
	for i, s := range selections {
		for _, input := range registrations[i].Spec.Inputs {
			section := inputSection(input.From)
			if !input.Required || hasSelection(selections, section) {
				continue
			}
			return refuse(
				fleetv1alpha1.ReasonMissingInput,
				"registration %q of section %q needs the input %q from %s, and the claim selects nothing in section %q",
				s.Registration,
				s.Section,
				input.Name,
				input.From,
				section,
			)
		}
	}

	return nil
}

// inputSection returns the section of the input source <section>/<output>.
func inputSection(from string) string {
	section, _, _ := strings.Cut(from, "/")
	return section
}

func hasSelection(selections []fleetv1alpha1.PluginSelection, section string) bool {
	return slices.ContainsFunc(selections, func(s fleetv1alpha1.PluginSelection) bool { return s.Section == section })
}

// inputEdges returns, for each claim index, the claim indexes of the
// selections that read from it.
func inputEdges(
	selections []fleetv1alpha1.PluginSelection,
	registrations []*pluginsv1alpha1.PluginRegistration,
) [][]int {
	edges := make([][]int, len(selections))
	for to := range selections {
		for _, input := range registrations[to].Spec.Inputs {
			section := inputSection(input.From)
			for from, s := range selections {
				if s.Section == section {
					edges[from] = append(edges[from], to)
				}
			}
		}
	}

	return edges
}

// cycleSections finds the strongly connected components with Tarjan's
// algorithm and returns the sorted, distinct sections of the components that
// are cycles.
func cycleSections(selections []fleetv1alpha1.PluginSelection, edges [][]int) []string {
	index := make([]int, len(selections))
	low := make([]int, len(selections))
	onStack := make([]bool, len(selections))
	for i := range index {
		index[i] = -1
	}

	var stack []int
	var sections []string
	next := 0

	var visit func(v int)
	visit = func(v int) {
		index[v], low[v] = next, next
		next++
		stack = append(stack, v)
		onStack[v] = true

		for _, w := range edges[v] {
			switch {
			case index[w] < 0:
				visit(w)
				low[v] = min(low[v], low[w])
			case onStack[w]:
				low[v] = min(low[v], index[w])
			}
		}

		if low[v] != index[v] {
			return
		}

		var component []int
		for {
			w := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[w] = false
			component = append(component, w)
			if w == v {
				break
			}
		}

		if len(component) > 1 || slices.Contains(edges[v], v) {
			for _, w := range component {
				sections = append(sections, selections[w].Section)
			}
		}
	}

	for v := range selections {
		if index[v] < 0 {
			visit(v)
		}
	}

	slices.Sort(sections)
	return slices.Compact(sections)
}

// topologicalOrder runs Kahn's algorithm on a graph without cycles. Among the
// selections that are ready at the same time, the first in the claim comes
// first, so that the order is stable.
func topologicalOrder(selections []fleetv1alpha1.PluginSelection, edges [][]int) []fleetv1alpha1.PluginSelection {
	inDegree := make([]int, len(selections))
	for _, targets := range edges {
		for _, to := range targets {
			inDegree[to]++
		}
	}

	ordered := make([]fleetv1alpha1.PluginSelection, 0, len(selections))
	for len(ordered) < len(selections) {
		v := slices.Index(inDegree, 0)
		// -1 marks a selection that is in the order already.
		inDegree[v] = -1
		ordered = append(ordered, selections[v])
		for _, to := range edges[v] {
			inDegree[to]--
		}
	}

	return ordered
}
