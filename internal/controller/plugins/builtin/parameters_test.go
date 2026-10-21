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

package builtin

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

func TestCheckParameters(t *testing.T) {
	tests := []struct {
		name         string
		registration string
		parameters   string   // "" means absent
		wantFields   []string // each one must be in the message. nil means pass
	}{
		{"a name passes", pluginstartup.RegistrationExistingObjectStorage, `{"name":"my-bucket"}`, nil},
		{"absent parameters", pluginstartup.RegistrationExistingObjectStorage, "", []string{"spec.parameters.name"}},
		{"an empty object", pluginstartup.RegistrationExistingSecondaryStorage, `{}`, []string{"spec.parameters.name"}},
		{
			"a name of the wrong type", pluginstartup.RegistrationExistingObjectStorage,
			`{"name":3}`, []string{"spec.parameters.name"},
		},
		{
			"an empty name",
			pluginstartup.RegistrationExistingObjectStorage,
			`{"name":""}`,
			[]string{"spec.parameters.name"},
		},
		{"no database config name passes", pluginstartup.RegistrationExistingDatabaseConfigs, `{}`, nil},
		{
			"two fields fail together", pluginstartup.RegistrationExistingDatabaseConfigs,
			`{"keycloak":1,"identity":""}`, []string{"spec.parameters.keycloak", "spec.parameters.identity"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var parameters *apiextensionsv1.JSON
			if tt.parameters != "" {
				parameters = &apiextensionsv1.JSON{Raw: []byte(tt.parameters)}
			}

			err := checkParameters(t, schemaOf(t, tt.registration), parameters)
			if tt.wantFields == nil {
				assert.NoError(t, err)
				return
			}

			var failure *conditions.PreCheckFailure
			require.ErrorAs(t, err, &failure)
			assert.Equal(t, pluginsv1alpha1.ReasonInvalidParameters, failure.Reason)
			for _, f := range tt.wantFields {
				assert.Contains(t, failure.Message, f)
			}
		})
	}
}

func TestCheckParametersRunsTheValidationRulesOfTheSchema(t *testing.T) {
	schema := &apiextensionsv1.JSON{Raw: []byte(`{
  "type": "object",
  "properties": {"min": {"type": "integer"}, "max": {"type": "integer"}},
  "x-kubernetes-validations": [
    {"rule": "self.min <= self.max", "message": "min must not be above max"},
    {"rule": "self.max < 100"}
  ]
}`)}

	tests := []struct {
		name       string
		parameters string
		want       string // in the message. "" means pass
	}{
		{"parameters that keep the rules pass", `{"min":1,"max":2}`, ""},
		{"a rule with a message", `{"min":3,"max":2}`, "min must not be above max"},
		{"a rule without a message", `{"min":1,"max":200}`, "self.max < 100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkParameters(t, schema, &apiextensionsv1.JSON{Raw: []byte(tt.parameters)})
			if tt.want == "" {
				assert.NoError(t, err)
				return
			}

			var failure *conditions.PreCheckFailure
			require.ErrorAs(t, err, &failure)
			assert.Equal(t, pluginsv1alpha1.ReasonInvalidParameters, failure.Reason)
			assert.Contains(t, failure.Message, "spec.parameters")
			assert.Contains(t, failure.Message, tt.want)
		})
	}
}

func TestCheckParametersChecksNullParametersAsAnEmptyObject(t *testing.T) {
	schema := &apiextensionsv1.JSON{Raw: []byte(
		`{"type":"object","nullable":true,"required":["message"],"properties":{"message":{"type":"string"}}}`,
	)}

	err := checkParameters(t, schema, &apiextensionsv1.JSON{Raw: []byte(`null`)})

	var failure *conditions.PreCheckFailure
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, pluginsv1alpha1.ReasonInvalidParameters, failure.Reason)
	assert.Contains(t, failure.Message, "spec.parameters.message")
}

func TestNewParameterSchemaRefusesASchemaThatItCannotRead(t *testing.T) {
	for name, raw := range map[string]*apiextensionsv1.JSON{
		"not JSON":  {Raw: []byte(`{"type":`)},
		"no schema": nil,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := newParameterSchema(raw)

			require.Error(t, err)
			var failure *conditions.PreCheckFailure
			assert.False(t, errors.As(err, &failure), "a broken schema is not the fault of the request")
		})
	}
}

func checkParameters(t *testing.T, schema, parameters *apiextensionsv1.JSON) error {
	p, err := newParameterSchema(schema)
	require.NoError(t, err)

	return p.check(t.Context(), parameters)
}
