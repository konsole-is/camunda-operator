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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The named cases of the placeholder rule, shared with the renderer of the
// plugins repository.
func TestCheckTemplateAllowsOnlyContractFieldReferences(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		allowed placeholders
		message string // empty: passes
	}{
		{"plain text", "arn:aws:s3:::bucket", grantPlaceholders, ""},
		{"namespace", "{{ .namespace }}", grantPlaceholders, ""},
		{"claim", "{{ .claim }}", grantPlaceholders, ""},
		{"request", "{{ .request }}", grantPlaceholders, ""},
		{"service account", "{{ .serviceAccount }}", grantPlaceholders, ""},
		{"parameter", "{{ .parameters.bucket }}", grantPlaceholders, ""},
		{"output", "arn:aws:s3:::{{ .outputs.bucketName }}/*", grantPlaceholders, ""},
		{"pods in a metric query", `{pod=~"{{ .pods }}"}`, metricPlaceholders, ""},
		{"parameter in a metric query", "{{ .parameters.x }}", metricPlaceholders, ""},
		{"output in a metric query", "{{ .outputs.y }}", metricPlaceholders, ""},
		{"pods in a grant", "{{ .pods }}", grantPlaceholders, "pods"},
		{"parameter in a template parameter", "{{ .parameters.x }}", parameterPlaceholders, ""},
		{
			"output in a template parameter",
			"{{ .outputs.y }}",
			parameterPlaceholders,
			"{{.outputs.y}}: the placeholder .outputs.y is not allowed, " +
				"an x-fleet-template parameter is rendered before the plugin writes the outputs",
		},
		{"unknown name", "{{ .unknown }}", grantPlaceholders, "the placeholder .unknown is not in the contract"},
		{"map without a key", "{{ .parameters }}", grantPlaceholders, "parameters"},
		{"map with two keys", "{{ .parameters.a.b }}", grantPlaceholders, "parameters.a.b"},
		{"field with a key", "{{ .namespace.x }}", grantPlaceholders, "namespace.x"},
		{"if", "{{ if .namespace }}x{{ end }}", grantPlaceholders, "if is not allowed"},
		{"range", "{{ range .parameters.x }}{{ end }}", grantPlaceholders, "range is not allowed"},
		{"with", "{{ with .namespace }}{{ end }}", grantPlaceholders, "with is not allowed"},
		{"template", `{{ template "x" }}`, grantPlaceholders, "template is not allowed"},
		{"pipeline", `{{ .namespace | printf "%s" }}`, grantPlaceholders, "pipeline is not allowed"},
		{"function call", `{{ printf "%s" .namespace }}`, grantPlaceholders, "function call printf is not allowed"},
		{"function without arguments", `{{ now }}`, grantPlaceholders, "function call now is not allowed"},
		{"variable", "{{ $x := .namespace }}", grantPlaceholders, "variable is not allowed"},
		{"dot", "{{ . }}", grantPlaceholders, "not allowed"},
		{"constant", `{{ "text" }}`, grantPlaceholders, "not allowed"},
		{"define", `{{ define "x" }}{{ end }}`, grantPlaceholders, "define is not allowed"},
		{"define named template", `{{ define "template" }}{{ .unknown }}{{ end }}`, grantPlaceholders, "define"},
		{
			"empty define named template",
			`{{ .namespace }}{{ define "template" }}{{ end }}`,
			grantPlaceholders,
			"define",
		},
		{"empty define named a", `{{ .namespace }}{{ define "a" }}{{ end }}`, grantPlaceholders, "define"},
		{"empty define named b", `{{ .namespace }}{{ define "b" }}{{ end }}`, grantPlaceholders, "define"},
		{"block", `{{ block "x" . }}{{ end }}`, grantPlaceholders, "not allowed"},
		{"comment", "{{/* note */}}", grantPlaceholders, "comment is not allowed"},
		{"parse error", "{{ .namespace", grantPlaceholders, "unclosed action"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkTemplate(tt.text, tt.allowed)

			if tt.message == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.message)
		})
	}
}

func TestCheckTemplateNamesEveryBadAction(t *testing.T) {
	err := checkTemplate("{{ .unknown }}-{{ .pods }}", grantPlaceholders)

	assert.ErrorContains(t, err, ".unknown")
	assert.ErrorContains(t, err, ".pods")
}

func TestStringLeavesWalksNestedObjectsAndArraysInKeyOrder(t *testing.T) {
	raw := []byte(`{"Resource":["a","{{ .outputs.x }}"],"Effect":"Allow","{{ .key }}":1,"Condition":{"k":"v"}}`)

	leaves, err := stringLeaves("spec.grantTemplates[0].template", raw)

	want := []jsonLeaf{
		{"spec.grantTemplates[0].template.Condition.k", "v"},
		{"spec.grantTemplates[0].template.Effect", "Allow"},
		{"spec.grantTemplates[0].template.Resource[0]", "a"},
		{"spec.grantTemplates[0].template.Resource[1]", "{{ .outputs.x }}"},
	}
	require.NoError(t, err)
	assert.Equal(t, want, leaves)
}

func TestStringLeavesOfAStringIsTheStringAtThePath(t *testing.T) {
	leaves, err := stringLeaves("spec.parameters.properties.policy.default", []byte(`"{{ .namespace }}"`))

	require.NoError(t, err)
	assert.Equal(t, []jsonLeaf{{"spec.parameters.properties.policy.default", "{{ .namespace }}"}}, leaves)
}

func TestStringLeavesRefusesInvalidJSON(t *testing.T) {
	_, err := stringLeaves("spec.grantTemplates[0].template", []byte(`{"Resource":`))

	assert.ErrorContains(t, err, "spec.grantTemplates[0].template")
}

func TestCheckJSONTemplatesNamesThePathOfEachBadLeaf(t *testing.T) {
	raw := []byte(`{"Resource":"{{ .unknown }}","Action":["{{ if .x }}{{ end }}"]}`)

	err := checkJSONTemplates("spec.grantTemplates[0].template", raw, grantPlaceholders)

	assert.ErrorContains(t, err, "spec.grantTemplates[0].template.Resource")
	assert.ErrorContains(t, err, "unknown")
	assert.ErrorContains(t, err, "spec.grantTemplates[0].template.Action[0]")
}

func TestCheckJSONTemplatesPassesOnlyContractPlaceholders(t *testing.T) {
	raw := []byte(`{"Resource":"arn:aws:s3:::{{ .outputs.value }}","Effect":"Allow"}`)

	assert.NoError(t, checkJSONTemplates("spec.grantTemplates[0].template", raw, grantPlaceholders))
}

// The named cases of the placeholder rule, shared with the checker and the
// renderer of the plugins repository.
func TestRenderTemplateFillsEachContractField(t *testing.T) {
	data := templateData{
		namespace:      "team-a",
		claim:          "orders",
		request:        "orders-backup",
		serviceAccount: "orders-camunda",
		pods:           "orders-.*",
		parameters:     map[string]any{"bucket": "b1", "size": json.Number("12345678901234567890")},
		outputs:        map[string]string{"bucketName": "orders-bucket"},
	}
	tests := []struct {
		name    string
		text    string
		allowed placeholders
		want    string
	}{
		{"plain text", "arn:aws:s3:::bucket", grantPlaceholders, "arn:aws:s3:::bucket"},
		{"namespace", "{{ .namespace }}", grantPlaceholders, "team-a"},
		{"claim", "{{ .claim }}", grantPlaceholders, "orders"},
		{"request", "{{ .request }}", grantPlaceholders, "orders-backup"},
		{"service account", "{{ .serviceAccount }}", grantPlaceholders, "orders-camunda"},
		{"parameter", "{{ .parameters.bucket }}", grantPlaceholders, "b1"},
		{"large integer parameter", "{{ .parameters.size }}", grantPlaceholders, "12345678901234567890"},
		{"output", "arn:aws:s3:::{{ .outputs.bucketName }}/*", grantPlaceholders, "arn:aws:s3:::orders-bucket/*"},
		{"pods in a metric query", `{pod=~"{{ .pods }}"}`, metricPlaceholders, `{pod=~"orders-.*"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderTemplate(tt.text, tt.allowed, data)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRenderTemplateRefusesWhatTheCheckerRefuses(t *testing.T) {
	tests := []struct{ name, text, message string }{
		{"pods in a grant", "{{ .pods }}", "pods"},
		{"unknown name", "{{ .unknown }}", "unknown"},
		{"if", "{{ if .namespace }}x{{ end }}", "if is not allowed"},
		{"pipeline", `{{ .namespace | printf "%s" }}`, "pipeline is not allowed"},
		{"function call", `{{ printf "%s" .namespace }}`, "function call printf is not allowed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := renderTemplate(tt.text, grantPlaceholders, templateData{})

			assert.ErrorContains(t, err, tt.message)
		})
	}
}

func TestRenderTemplateNamesAMissingParameterOrOutput(t *testing.T) {
	_, err := renderTemplate("{{ .parameters.bucket }}", grantPlaceholders, templateData{})
	assert.ErrorContains(t, err, `parameter "bucket" has no value`)

	_, err = renderTemplate("{{ .outputs.bucketName }}", grantPlaceholders, templateData{})
	assert.ErrorContains(t, err, `output "bucketName" has no value`)
}

func TestRenderJSONTemplateRendersStringLeavesOnly(t *testing.T) {
	raw := []byte(`{"{{ .namespace }}":"{{ .namespace }}","n":3,"b":true,"list":["{{ .request }}",null]}`)

	got, err := renderJSONTemplate("template", raw, grantPlaceholders, templateData{namespace: "ns", request: "r"})

	require.NoError(t, err)
	assert.JSONEq(t, `{"{{ .namespace }}":"ns","n":3,"b":true,"list":["r",null]}`, string(got))
}

func TestRenderJSONTemplateKeepsALargeNumberExact(t *testing.T) {
	got, err := renderJSONTemplate(
		"template",
		[]byte(`{"n":12345678901234567890}`),
		grantPlaceholders,
		templateData{},
	)

	require.NoError(t, err)
	assert.Equal(t, `{"n":12345678901234567890}`, string(got))
}

func TestRenderJSONTemplateKeepsAValueInsideItsString(t *testing.T) {
	data := templateData{parameters: map[string]any{"x": `a"b\c}}<&>`}}

	got, err := renderJSONTemplate("template", []byte(`{"Sid":"{{ .parameters.x }}"}`), grantPlaceholders, data)

	require.NoError(t, err)
	assert.Equal(t, `{"Sid":"a\"b\\c}}<&>"}`, string(got))
}

func TestRenderJSONTemplateNamesThePathOfABadLeaf(t *testing.T) {
	_, err := renderJSONTemplate(
		"spec.grantTemplates[0].template",
		[]byte(`{"a":{"b":["{{ .unknown }}"]}}`),
		grantPlaceholders,
		templateData{},
	)

	assert.ErrorContains(t, err, "spec.grantTemplates[0].template.a.b[0]")
}
