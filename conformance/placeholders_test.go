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
		{"plain text", "arn:aws:s3:::bucket", templatePlaceholders, ""},
		{"namespace", "{{ .namespace }}", templatePlaceholders, ""},
		{"claim", "{{ .claim }}", templatePlaceholders, ""},
		{"request", "{{ .request }}", templatePlaceholders, ""},
		{"service account", "{{ .serviceAccount }}", templatePlaceholders, ""},
		{"parameter", "{{ .parameters.bucket }}", templatePlaceholders, ""},
		{"output", "arn:aws:s3:::{{ .outputs.bucketName }}/*", templatePlaceholders, ""},
		{"pods in a metric query", `{pod=~"{{ .pods }}"}`, metricPlaceholders, ""},
		{"parameter in a metric query", "{{ .parameters.x }}", metricPlaceholders, ""},
		{"output in a metric query", "{{ .outputs.y }}", metricPlaceholders, ""},
		{"pods in a grant", "{{ .pods }}", templatePlaceholders, "pods"},
		{"unknown name", "{{ .unknown }}", templatePlaceholders, "the placeholder .unknown is not in the contract"},
		{"map without a key", "{{ .parameters }}", templatePlaceholders, "parameters"},
		{"map with two keys", "{{ .parameters.a.b }}", templatePlaceholders, "parameters.a.b"},
		{"field with a key", "{{ .namespace.x }}", templatePlaceholders, "namespace.x"},
		{"if", "{{ if .namespace }}x{{ end }}", templatePlaceholders, "if is not allowed"},
		{"range", "{{ range .parameters.x }}{{ end }}", templatePlaceholders, "range is not allowed"},
		{"with", "{{ with .namespace }}{{ end }}", templatePlaceholders, "with is not allowed"},
		{"template", `{{ template "x" }}`, templatePlaceholders, "template is not allowed"},
		{"pipeline", `{{ .namespace | printf "%s" }}`, templatePlaceholders, "pipeline is not allowed"},
		{"function call", `{{ printf "%s" .namespace }}`, templatePlaceholders, "function call printf is not allowed"},
		{"function without arguments", `{{ now }}`, templatePlaceholders, "function call now is not allowed"},
		{"variable", "{{ $x := .namespace }}", templatePlaceholders, "variable is not allowed"},
		{"dot", "{{ . }}", templatePlaceholders, "not allowed"},
		{"constant", `{{ "text" }}`, templatePlaceholders, "not allowed"},
		{"define", `{{ define "x" }}{{ end }}`, templatePlaceholders, "define is not allowed"},
		{"comment", "{{/* note */}}", templatePlaceholders, "comment is not allowed"},
		{"parse error", "{{ .namespace", templatePlaceholders, "unclosed action"},
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
	err := checkTemplate("{{ .unknown }}-{{ .pods }}", templatePlaceholders)

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

	err := checkJSONTemplates("spec.grantTemplates[0].template", raw, templatePlaceholders)

	assert.ErrorContains(t, err, "spec.grantTemplates[0].template.Resource")
	assert.ErrorContains(t, err, "unknown")
	assert.ErrorContains(t, err, "spec.grantTemplates[0].template.Action[0]")
}

func TestCheckJSONTemplatesPassesOnlyContractPlaceholders(t *testing.T) {
	raw := []byte(`{"Resource":"arn:aws:s3:::{{ .outputs.value }}","Effect":"Allow"}`)

	assert.NoError(t, checkJSONTemplates("spec.grantTemplates[0].template", raw, templatePlaceholders))
}
