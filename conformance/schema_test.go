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
	"github.com/stretchr/testify/require"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
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
	_, err := parseSchema(t.Context(), []byte(builtinShapedSchema))

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
		{
			"unknown key under items",
			`{"type":"object","properties":{"l":{"type":"array","items":{"type":"string","minLenght":3}}}}`,
			`spec.parameters.properties.l.items: unknown field "minLenght"`,
		},
		{
			"unknown key under additionalProperties",
			`{"type":"object","properties":{"m":{"type":"object","additionalProperties":{"type":"string","minLenght":3}}}}`,
			`spec.parameters.properties.m.additionalProperties: unknown field "minLenght"`,
		},
		{
			"unknown key of a validation rule",
			`{"type":"object","x-kubernetes-validations":[{"rule":"true","messageExpresion":"'x'"}]}`,
			`spec.parameters.x-kubernetes-validations[0]: unknown field "messageExpresion"`,
		},
		{
			"unknown key of a validation rule under items",
			`{"type":"object","properties":{"l":{"type":"array","items":{"type":"string",` +
				`"x-kubernetes-validations":[{"rule":"true","messageExpresion":"'x'"}]}}}}`,
			`spec.parameters.properties.l.items.x-kubernetes-validations[0]: unknown field "messageExpresion"`,
		},
		{
			"unknown key of the external docs",
			`{"type":"object","externalDocs":{"url":"https://example.com","summary":"x"}}`,
			`spec.parameters.externalDocs: unknown field "summary"`,
		},
		{
			"additionalProperties at the root",
			`{"type":"object","additionalProperties":{"type":"string"}}`,
			"spec.parameters.additionalProperties: Forbidden: must not be used at the root",
		},
		{"root that is not an object", `{"type":"string"}`, "spec.parameters.type: Invalid value"},
		{"nullable root", `{"type":"object","nullable":true}`, "spec.parameters.nullable: Forbidden"},
		{
			"validation rule that does not compile",
			`{"type":"object","properties":{"n":{"type":"integer","x-kubernetes-validations":[{"rule":"self >"}]}}}`,
			"Syntax error",
		},
		{
			"validation rule on a field that an integer does not have",
			`{"type":"object","properties":{"n":{"type":"integer","x-kubernetes-validations":[{"rule":"self.foo > 0"}]}}}`,
			"does not support field selection",
		},
		{
			"map list without keys",
			`{"type":"object","properties":{"l":{"type":"array","x-kubernetes-list-type":"map",` +
				`"items":{"type":"object","properties":{"name":{"type":"string"}}}}}}`,
			"spec.parameters.properties[l].x-kubernetes-list-map-keys: Required value",
		},
		{"unknown type", `{"type":"object","properties":{"m":{"type":"strnig"}}}`, "strnig"},
		{
			"default of the wrong type",
			`{"type":"object","properties":{"n":{"type":"integer","default":"five"}}}`,
			"spec.parameters.properties[n].default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkSchema(t.Context(), schemaSuite(t, tt.schema))

			assert.ErrorContains(t, err, "spec.parameters")
			assert.ErrorContains(t, err, tt.message)
		})
	}
}

// The marker is a schema keyword. A property and a value with the same
// name are data, and keep their meaning.
func TestSchemaKeepsAPropertyAndAValueNamedLikeTheMarker(t *testing.T) {
	property := schemaSuite(
		t,
		`{"type":"object","properties":{"x-fleet-template":{"type":"string","maxLength":2}}}`,
		Case{Name: "long", Parameters: parameters(`{"x-fleet-template":"long"}`)},
	)
	enum := schemaSuite(
		t,
		`{"type":"object","properties":{"o":{"type":"object","x-kubernetes-preserve-unknown-fields":true,`+
			`"enum":[{"x-fleet-template":1}]}}}`,
		Case{Name: "listed", Parameters: parameters(`{"o":{"x-fleet-template":1}}`)},
	)

	assert.ErrorContains(t, checkSchema(t.Context(), property), `case "long"`)
	assert.NoError(t, checkSchema(t.Context(), enum))
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

func TestSchemaChecksTheValidationRulesOfEachCase(t *testing.T) {
	s := schemaSuite(
		t,
		`{"type":"object","properties":{"n":{"type":"integer",`+
			`"x-kubernetes-validations":[{"rule":"self > 0","message":"n must be positive"}]}}}`,
		Case{Name: "positive", Parameters: parameters(`{"n":1}`)},
		Case{Name: "negative", Parameters: parameters(`{"n":-1}`)},
	)

	err := checkSchema(t.Context(), s)

	assert.ErrorContains(t, err, `case "negative"`)
	assert.ErrorContains(t, err, "n must be positive")
	assert.NotContains(t, err.Error(), `case "positive"`)
}

// 2^53 + 1 has no exact float64.
func TestParseSchemaKeepsALargeInteger(t *testing.T) {
	parsed, err := parseSchema(
		t.Context(),
		[]byte(`{"type":"object","properties":{"n":{"type":"integer","enum":[9007199254740993]}}}`),
	)

	require.NoError(t, err)
	assert.Equal(t, []apiextensions.JSON{int64(9007199254740993)}, parsed.Properties["n"].Enum)
}

func TestSchemaChecksTheListsAndEmbeddedResourcesOfEachCase(t *testing.T) {
	const schema = `{"type":"object","properties":{` +
		`"tags":{"type":"array","x-kubernetes-list-type":"set","items":{"type":"string"}},` +
		`"ports":{"type":"array","x-kubernetes-list-type":"map","x-kubernetes-list-map-keys":["name"],` +
		`"items":{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}},` +
		`"object":{"type":"object","x-kubernetes-embedded-resource":true,"x-kubernetes-preserve-unknown-fields":true}}}`
	tests := []struct {
		name       string
		parameters string
		message    string
	}{
		{"valid", `{"tags":["a","b"],"ports":[{"name":"a"},{"name":"b"}],` +
			`"object":{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"a"}}}`, ""},
		{"duplicate set element", `{"tags":["a","a"]}`, `parameters.tags[1]: Duplicate value: "a"`},
		{"duplicate map key", `{"ports":[{"name":"a"},{"name":"a"}]}`, "parameters.ports[1]: Duplicate value"},
		{
			"embedded resource without a kind",
			`{"object":{"apiVersion":"v1"}}`,
			"parameters.object.kind: Required value",
		},
		{
			"embedded resource with a name that is not a path segment",
			`{"object":{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"a/b"}}}`,
			"parameters.object.metadata.name: Invalid value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := schemaSuite(t, schema, Case{Name: "case", Parameters: parameters(tt.parameters)})

			err := checkSchema(t.Context(), s)

			if tt.message == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, `case "case"`)
			assert.ErrorContains(t, err, tt.message)
		})
	}
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
			`{"type":"object","properties":{"m":{"type":"object","additionalProperties":` +
				`{"type":"string","x-fleet-template":true,"default":"{{ .unknown }}"}}}}`,
			"spec.parameters.properties.m.additionalProperties.default",
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
	_, err := parseSchema(t.Context(), []byte(`{"type":`))

	assert.ErrorContains(t, err, "spec.parameters: unexpected end of JSON input")
}

// The schema of validRegistration passes both schema checks.
func TestSchemaChecksPassTheValidRegistration(t *testing.T) {
	s := newTestSuite(t, validConfig(), interceptor.Funcs{}, validRegistration())

	assert.NoError(t, checkSchema(t.Context(), s))
	assert.NoError(t, checkParameterTemplates(t.Context(), s))
}
