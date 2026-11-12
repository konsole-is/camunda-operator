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
	"encoding/json"
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
)

func TestCheckAllowedFixtures(t *testing.T) {
	t.Parallel()

	runFixtures(t, "check-allowed", func(in AllowedInput) (any, error) { return struct{}{}, CheckAllowed(in) })
}

func TestEveryBuiltinPresetParameterHasAPresetList(t *testing.T) {
	t.Parallel()

	var withPreset []string
	for _, r := range pluginstartup.Registrations("0.0.0", pluginsv1alpha1.ServiceAccountRef{}) {
		var schema struct {
			Properties map[string]json.RawMessage `json:"properties"`
		}
		require.NoError(t, json.Unmarshal(r.Spec.Parameters.Raw, &schema), r.Name)
		if _, ok := schema.Properties[presetParameter]; ok {
			withPreset = append(withPreset, r.Name)
		}
	}

	assert.ElementsMatch(t, withPreset, slices.Collect(maps.Keys(storagePresetLists)))
}
