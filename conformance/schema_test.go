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

package conformance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// builtinShapedSchema is a copy of the Elasticsearch parameter schema of the
// built-in registrations of the operator, which this module cannot import.
const builtinShapedSchema = `{
  "type": "object",
  "required": ["preset"],
  "properties": {
    "preset": {"type": "string", "minLength": 1, "description": "The name of the ElasticsearchClusterPreset."},
    "overrides": {
      "type": "object",
      "description": "The values that replace the values of the preset.",
      "properties": {
        "replicas": {"type": "integer", "format": "int32", "minimum": 1, "description": "The number of nodes."},
        "resources": {"type": "object", "x-kubernetes-preserve-unknown-fields": true, "description": "CPU and memory."},
        "storageSize": {"type": "string", "description": "The volume size of each node."},
        "scheduling": {"type": "object", "x-kubernetes-preserve-unknown-fields": true, "description": "Where."},
        "podLabels": {"type": "object", "additionalProperties": {"type": "string"}, "description": "The pod labels."}
      }
    }
  }
}`

// schemaSuite returns the suite of a run with cases on a fake cluster that
// holds validRegistration with the parameter schema schema. An empty schema
// leaves spec.parameters unset.
func schemaSuite(t *testing.T, schema string, cases ...Case) *suite {
	t.Helper()

	reg := validRegistration()
	reg.Spec.Parameters = nil
	if schema != "" {
		reg.Spec.Parameters = &apiextensionsv1.JSON{Raw: []byte(schema)}
	}

	cfg := validConfig()
	if len(cases) > 0 {
		cfg.Cases = cases
	}

	return newTestSuite(t, cfg, interceptor.Funcs{}, reg)
}

func parameters(raw string) *apiextensionsv1.JSON {
	return &apiextensionsv1.JSON{Raw: []byte(raw)}
}

func TestSchemaAcceptsABuiltinShapedSchema(t *testing.T) {
	_, err := parseSchema([]byte(builtinShapedSchema))

	assert.NoError(t, err)
}

func TestSchemaAcceptsTheFleetTemplateExtension(t *testing.T) {
	s := schemaSuite(
		t,
		`{"type":"object","properties":{"policy":{"type":"string","x-fleet-template":true,"default":"{{ .namespace }}"}}}`,
	)

	assert.NoError(t, checkSchema(t.Context(), s))
}

func TestSchemaRefusesAnInvalidSchema(t *testing.T) {
	tests := []struct {
		name    string
		schema  string
		message string
	}{
		{"unknown key", `{"type":"object","tpye":"string"}`, `unknown field "tpye"`},
		{
			"property without a type",
			`{"type":"object","properties":{"message":{"description":"x"}}}`,
			"spec.parameters.properties[message].type",
		},
		{"reference", `{"type":"object","properties":{"x":{"$ref":"#/definitions/x"}}}`, "$ref"},
		{"type of the wrong kind", `{"type":5}`, "type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkSchema(t.Context(), schemaSuite(t, tt.schema))

			assert.ErrorContains(t, err, "spec.parameters")
			assert.ErrorContains(t, err, tt.message)
		})
	}
}

func TestSchemaChecksEveryCaseAgainstTheSchema(t *testing.T) {
	s := schemaSuite(
		t,
		`{"type":"object","properties":{"message":{"type":"string"}}}`,
		Case{Name: "good", Parameters: parameters(`{"message":"hi"}`)},
		Case{Name: "bad", Parameters: parameters(`{"message":5}`)},
	)

	err := checkSchema(t.Context(), s)

	assert.ErrorContains(t, err, `case "bad"`)
	assert.ErrorContains(t, err, "parameters.message")
	assert.NotContains(t, err.Error(), `case "good"`)
}

func TestSchemaChecksACaseWithoutParametersAsAnEmptyObject(t *testing.T) {
	s := schemaSuite(t, `{"type":"object","required":["message"],"properties":{"message":{"type":"string"}}}`)

	err := checkSchema(t.Context(), s)

	assert.ErrorContains(t, err, `case "default"`)
	assert.ErrorContains(t, err, "message")
}

func TestSchemaRefusesCaseParametersThatAreNotJSON(t *testing.T) {
	s := schemaSuite(
		t,
		`{"type":"object"}`,
		Case{Name: "broken", Parameters: parameters(`{"message":`)},
	)

	assert.ErrorContains(t, checkSchema(t.Context(), s), `case "broken"`)
}

func TestSchemaWithoutASchemaRefusesCaseParameters(t *testing.T) {
	withParameters := schemaSuite(t, "", Case{Name: "default", Parameters: parameters(`{"message":"hi"}`)})
	withoutParameters := schemaSuite(t, "")

	assert.ErrorContains(
		t,
		checkSchema(t.Context(), withParameters),
		`spec.parameters is empty, and case "default" has parameters`,
	)
	assert.NoError(t, checkSchema(t.Context(), withoutParameters))
}

func TestParameterTemplatesCheck(t *testing.T) {
	tests := []struct {
		name    string
		schema  string
		message string
	}{
		{"no schema", "", ""},
		{
			"no marked parameter",
			`{"type":"object","properties":{"message":{"type":"string","default":"{{ .x }}"}}}`,
			"",
		},
		{
			"marked string default",
			`{"type":"object","properties":{"policy":{"type":"string","x-fleet-template":true,` +
				`"default":"{{ .serviceAccount }}"}}}`,
			"",
		},
		{
			"marked object default",
			`{"type":"object","properties":{"policy":{"type":"object","x-fleet-template":true,` +
				`"default":{"role":"r","resource":"projects/acme/topics/{{ .namespace }}-*"}}}}`,
			"",
		},
		{
			"marked default with pods",
			`{"type":"object","properties":{"policy":{"type":"string","x-fleet-template":true,"default":"{{ .pods }}"}}}`,
			"spec.parameters.properties.policy.default: {{.pods}}",
		},
		{
			"nested marked default",
			`{"type":"object","properties":{"overrides":{"type":"object","properties":{"policy":` +
				`{"type":"string","x-fleet-template":true,"default":"{{ .unknown }}"}}}}}`,
			"spec.parameters.properties.overrides.properties.policy.default",
		},
		{
			"marked default under items",
			`{"type":"object","properties":{"list":{"type":"array","items":` +
				`{"type":"string","x-fleet-template":true,"default":"{{ .unknown }}"}}}}`,
			"spec.parameters.properties.list.items.default",
		},
		{
			"marked default under additionalProperties",
			`{"type":"object","additionalProperties":{"type":"string","x-fleet-template":true,"default":"{{ .unknown }}"}}`,
			"spec.parameters.additionalProperties.default",
		},
		{
			"marked default under allOf",
			`{"type":"object","allOf":[{"x-fleet-template":true,"default":"{{ .unknown }}"}]}`,
			"spec.parameters.allOf[0].default",
		},
		{
			"marked parameter without a default",
			`{"type":"object","properties":{"policy":{"type":"string","x-fleet-template":true}}}`,
			"",
		},
		{
			"marker inside a default",
			`{"type":"object","properties":{"policy":{"type":"object",` +
				`"default":{"x-fleet-template":true,"default":"{{ .unknown }}"}}}}`,
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkParameterTemplates(t.Context(), schemaSuite(t, tt.schema))

			if tt.message == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.message)
		})
	}
}

// The API server stores only JSON in spec.parameters, so only parseSchema
// can get text that is not JSON.
func TestParseSchemaRefusesTextThatIsNotJSON(t *testing.T) {
	_, err := parseSchema([]byte(`{"type":`))

	assert.ErrorContains(t, err, "spec.parameters: unexpected end of JSON input")
}

// The schema of validRegistration passes both schema checks.
func TestSchemaChecksPassTheValidRegistration(t *testing.T) {
	s := newTestSuite(t, validConfig(), interceptor.Funcs{}, validRegistration())

	assert.NoError(t, checkSchema(t.Context(), s))
	assert.NoError(t, checkParameterTemplates(t.Context(), s))
}
