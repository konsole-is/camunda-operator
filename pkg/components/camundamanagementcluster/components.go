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
// the Secrets first, because the workloads mount them, then Keycloak, whose
// operator writes the Secret that Management Identity signs in with, then one
// entry per remaining workload. This list is the extension point of the
// package.
var builders = []func(Input) ([]*component.Component, error){
	mirroredSecretComponent,
	secretsComponents,
	keycloakComponents,
	identityComponents,
	consoleComponents,
	webModelerComponents,
}

// Build renders every component of one management plane, in the order the
// builders are registered.
func Build(in Input) (Built, error) {
	var built Built
	for _, build := range builders {
		comps, err := build(in)
		if err != nil {
			return Built{}, err
		}
		built.Components = append(built.Components, comps...)
		for _, comp := range comps {
			if takesPartInReady(in, comp) {
				built.Ready = append(built.Ready, comp)
			}
		}
	}

	return built, nil
}

// takesPartInReady reports whether the Ready condition aggregates over a
// component.
//
// A gated-off component reports True with the reason Disabled, and Disabled
// outranks Healthy in the aggregate, so a management cluster that copies no
// Secret would read "Ready=True/Disabled". Each component below is built in
// every mode, so that turning it off deletes its resources; it only stays out
// of Ready while it is off.
func takesPartInReady(in Input, comp *component.Component) bool {
	switch comp.GetName() {
	case ComponentMirroredSecrets:
		return len(in.Mirrors) > 0
	case ComponentKeycloak:
		return in.Provider.Mode == ModeKeycloak
	default:
		return true
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
