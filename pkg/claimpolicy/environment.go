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
	corev1 "k8s.io/api/core/v1"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
)

// EnvironmentOf returns the Environment that the label
// fleet.camunda.io/environment of namespace names. An empty label counts as
// no label. It returns nil and no error when the namespace has no label and
// no Environment exists: the registrations then give the policy. It refuses
// with NoEnvironment a namespace without the label while an Environment
// exists, and with UnknownEnvironment a label that names no Environment.
func EnvironmentOf(
	namespace *corev1.Namespace,
	environments []fleetv1alpha1.Environment,
) (*fleetv1alpha1.Environment, error) {
	name := namespace.Labels[fleetv1alpha1.LabelEnvironment]
	if name == "" {
		if len(environments) > 0 {
			return nil, refuse(
				fleetv1alpha1.ReasonNoEnvironment,
				"namespace %q has no label %s, and an Environment exists",
				namespace.Name,
				fleetv1alpha1.LabelEnvironment,
			)
		}
		return nil, nil
	}

	for i := range environments {
		if environments[i].Name == name {
			return &environments[i], nil
		}
	}

	return nil, refuse(
		fleetv1alpha1.ReasonUnknownEnvironment,
		"namespace %q has the label %s=%s, and no Environment %q exists",
		namespace.Name,
		fleetv1alpha1.LabelEnvironment,
		name,
		name,
	)
}
