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

package cnpgcluster

import cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"

// SetInstances records the number of PostgreSQL instances the Cluster runs.
// CloudNativePG keeps the PersistentVolumeClaim of an instance it scales
// away, so a count of zero stops the server without losing its data.
//
// The method lives beside the generated mutator because ocf scaffold wrapper
// --force rewrites mutator.go.
func (m *Mutator) SetInstances(instances int) {
	m.Edit(func(cluster *cnpgv1.Cluster) error {
		cluster.Spec.Instances = instances
		return nil
	})
}
