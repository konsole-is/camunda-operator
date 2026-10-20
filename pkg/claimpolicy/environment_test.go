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
	"testing"

	corev1 "k8s.io/api/core/v1"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
)

func TestEnvironmentOfFixtures(t *testing.T) {
	t.Parallel()

	type input struct {
		Namespace    *corev1.Namespace           `json:"namespace"`
		Environments []fleetv1alpha1.Environment `json:"environments,omitempty"`
	}
	type value struct {
		Environment string `json:"environment,omitempty"`
	}

	runFixtures(t, "environment-of", func(in input) (any, error) {
		env, err := EnvironmentOf(in.Namespace, in.Environments)
		if err != nil || env == nil {
			return value{}, err
		}
		return value{Environment: env.Name}, nil
	})
}
