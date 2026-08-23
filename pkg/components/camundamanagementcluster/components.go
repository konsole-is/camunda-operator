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

package camundamanagementcluster

import (
	"github.com/sourcehawk/operator-component-framework/pkg/component"

	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// Built is what Build returns.
type Built struct {
	// Components is every component, in reconcile order. The controller
	// passes it to FlushStatus, which then owns the condition of each one.
	Components []*component.Component
	// Ready are the components that the Ready condition aggregates over.
	Ready []*component.Component
}

// builders render the components of the management plane, in reconcile order:
// the copied and the generated Secrets first, because the workloads mount
// them, then Keycloak, whose operator writes the Secret that Management
// Identity signs in with, then one entry per remaining workload. A builder
// reports which of its components take part in Ready, so that a component the
// spec turned off still reconciles and deletes what it left behind without
// its Disabled reason becoming the reason of the whole management plane. This
// list is the extension point of the package.
var builders = []func(Input) (Built, error){
	mirroredSecretComponent,
	secretsComponents,
	keycloakComponents,
	alwaysReady(identityComponents),
	consoleComponents,
	webModelerComponents,
}

// Build renders every component of one management plane, in the order the
// builders are registered.
func Build(in Input) (Built, error) {
	var built Built
	for _, build := range builders {
		rendered, err := build(in)
		if err != nil {
			return Built{}, err
		}
		built.Components = append(built.Components, rendered.Components...)
		built.Ready = append(built.Ready, rendered.Ready...)
	}

	return built, nil
}

// alwaysReady adapts a builder whose components all take part in Ready.
func alwaysReady(
	build func(Input) ([]*component.Component, error),
) func(Input) (Built, error) {
	return func(in Input) (Built, error) {
		comps, err := build(in)
		if err != nil {
			return Built{}, err
		}

		return Built{Components: comps, Ready: comps}, nil
	}
}

// managedLabels returns the labels of an object that the operator applies for
// a component of the management plane.
func managedLabels(in Input, comp string) map[string]string {
	return labels.Managed(labels.ManagementCluster(in.Cluster.Name), comp)
}

// discoveryLabels returns the labels of the pods and the selectors of a
// component.
func discoveryLabels(in Input, comp string) map[string]string {
	return labels.Discovery(labels.ManagementCluster(in.Cluster.Name), comp)
}
