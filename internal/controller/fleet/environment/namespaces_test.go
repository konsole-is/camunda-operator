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

package environment

import (
	. "github.com/onsi/ginkgo/v2"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
)

var _ = Describe("Environment status.namespaces", func() {
	It("reports 0 for an environment without namespaces", func() {
		env := createEnvironment(fleetv1alpha1.EnvironmentSpec{})

		expectNamespaces(env, 0)
	})

	It("counts the namespaces that carry the label of the environment, and only those", func() {
		env := createEnvironment(fleetv1alpha1.EnvironmentSpec{})
		other := createEnvironment(fleetv1alpha1.EnvironmentSpec{})

		createNamespace(map[string]string{fleetv1alpha1.LabelEnvironment: env.Name})
		createNamespace(map[string]string{fleetv1alpha1.LabelEnvironment: env.Name})
		createNamespace(map[string]string{fleetv1alpha1.LabelEnvironment: other.Name})
		createNamespace(nil)

		expectNamespaces(env, 2)
		expectNamespaces(other, 1)
	})

	It("follows a label that is added to a namespace and removed from it", func() {
		env := createEnvironment(fleetv1alpha1.EnvironmentSpec{})
		ns := createNamespace(nil)
		expectNamespaces(env, 0)

		setEnvironmentLabel(ns, env.Name)
		expectNamespaces(env, 1)

		setEnvironmentLabel(ns, "")
		expectNamespaces(env, 0)
	})
})
