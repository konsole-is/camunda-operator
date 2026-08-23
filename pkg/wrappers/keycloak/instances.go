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

package keycloak

// SetInstances records a mutation that sets the number of Keycloak pods.
//
// It is recorded through Edit, so it survives a regeneration of mutator.go.
func (m *Mutator) SetInstances(instances int32) {
	m.Edit(func(kc *Keycloak) error {
		kc.Spec.Instances = &instances

		return nil
	})
}
