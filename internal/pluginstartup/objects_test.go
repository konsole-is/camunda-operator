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

package pluginstartup

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

var testServiceAccount = pluginsv1alpha1.ServiceAccountRef{
	Namespace: "camunda-operator-system",
	Name:      "camunda-operator-controller-manager",
}

func TestRegistrationsCarryTheControllerAndServiceAccount(t *testing.T) {
	for _, r := range Registrations("0.3.0", testServiceAccount) {
		assert.Equal(t, pluginsv1alpha1.ControllerInfo{Name: "camunda-operator", Version: "0.3.0"}, r.Spec.Controller, r.Name)
		assert.Equal(t, testServiceAccount, r.Spec.ServiceAccount, r.Name)
		assert.Equal(t, "PluginRegistration", r.Kind, r.Name)
		assert.Equal(t, pluginsv1alpha1.GroupVersion.String(), r.APIVersion, r.Name)
	}
}

func TestEachSectionHasAtMostOneDefaultBuiltIn(t *testing.T) {
	defaults := map[string][]string{}
	for _, r := range Registrations("0.3.0", testServiceAccount) {
		if r.Spec.Mode != pluginsv1alpha1.ModeDefault {
			continue
		}
		for _, s := range r.Spec.Sections {
			defaults[s] = append(defaults[s], r.Name)
		}
	}

	assert.Equal(t, map[string][]string{
		"secondary-storage":   {"elasticsearch"},
		"management-database": {"management-database-server"},
	}, defaults)
}

func TestRegistrationsServeCoreSectionsAndListTheirOutputs(t *testing.T) {
	sections := map[string]pluginsv1alpha1.PluginSection{}
	for _, s := range Sections() {
		sections[s.Name] = s
	}

	for _, r := range Registrations("0.3.0", testServiceAccount) {
		keys := outputKeys(r.Spec.Outputs)
		for _, name := range r.Spec.Sections {
			section, ok := sections[name]
			require.True(t, ok, "%s serves %s, which is not a core section", r.Name, name)
			for _, key := range outputKeys(section.Spec.Outputs) {
				assert.Contains(t, keys, key, "%s does not list output %s of section %s", r.Name, key, name)
			}
		}
	}
}

func TestInputsNameAnOutputOfACoreSection(t *testing.T) {
	outputs := map[string]bool{}
	for _, s := range Sections() {
		for _, key := range outputKeys(s.Spec.Outputs) {
			outputs[s.Name+"/"+key] = true
		}
	}

	for _, r := range Registrations("0.3.0", testServiceAccount) {
		for _, in := range r.Spec.Inputs {
			assert.True(t, outputs[in.From], "%s input %s reads %s, which no core section declares", r.Name, in.Name, in.From)
		}
	}
}

func TestParameterSchemasAreOpenAPIAndCoverClaimEditable(t *testing.T) {
	for _, r := range Registrations("0.3.0", testServiceAccount) {
		require.NotNil(t, r.Spec.Parameters, r.Name)

		decoder := json.NewDecoder(bytes.NewReader(r.Spec.Parameters.Raw))
		decoder.DisallowUnknownFields()
		var schema apiextensionsv1.JSONSchemaProps
		require.NoError(t, decoder.Decode(&schema), r.Name)

		assert.Equal(t, "object", schema.Type, r.Name)
		for _, editable := range r.Spec.ClaimEditable {
			assert.Contains(t, schema.Properties, editable, "%s: claim-editable %s is not a parameter", r.Name, editable)
		}
	}
}

func outputKeys(outputs []pluginsv1alpha1.OutputDecl) []string {
	keys := make([]string, 0, len(outputs))
	for _, o := range outputs {
		keys = append(keys, o.Key)
	}
	return keys
}
