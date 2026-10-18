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

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

func TestInputsFixtures(t *testing.T) {
	t.Parallel()

	type input struct {
		Registration *pluginsv1alpha1.PluginRegistration `json:"registration"`
		Selections   []fleetv1alpha1.PluginSelection     `json:"selections,omitempty"`
		Requests     []pluginsv1alpha1.PluginRequest     `json:"requests,omitempty"`
	}

	runFixtures(t, "inputs", func(in input) (any, error) {
		return Inputs(in.Registration, in.Selections, in.Requests), nil
	})
}
