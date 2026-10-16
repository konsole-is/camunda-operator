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

package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// header and footer are the Helm actions that the kubebuilder helm plugin
// writes around each CRD template.
const header = `{{- if .Values.crd.enabled }}
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    {{- if .Values.crd.keep }}
    "helm.sh/resource-policy": keep
    {{- end }}
    controller-gen.kubebuilder.io/version: v0.20.1
  name: sections.plugins.camunda.io
`

const footer = `{{- end }}
`

const spec = `spec:
  group: plugins.camunda.io
  names:
    kind: Section
    plural: sections
  scope: Cluster
  versions:
  - additionalPrinterColumns:
    - description: The label of the section.
      jsonPath: .spec.label
      name: Label
      type: string
    name: v1alpha1
    schema:
      openAPIV3Schema:
        description: Section is a section of the console.
        properties:
          spec:
            description: |-
              Spec holds the section. A placeholder such as
              {{ "{{ .namespace }}" }} is read by the console.
            properties:
              description:
                description: Description is the text under the label.
                maxLength: 200
                type: string
              mode:
                default: Open
                description: Mode is how the section opens.
                enum:
                - Open
                - Closed
                type: string
              meta:
                additionalProperties:
                  description: A value of the map.
                  type: string
                default:
                  description: kept, because a default is not a schema
                description: Meta holds extra data.
                type: object
              steps:
                description: Steps are run in order.
                items:
                  description: A step.
                  anyOf:
                  - description: A string step.
                    type: string
                  - description: A number step.
                    type: integer
                type: array
              template:
                description: Template is free-form.
                pattern: '^{{ "{{" }}'
                type: object
                x-kubernetes-preserve-unknown-fields: true
            required:
            - description
            type: object
            x-kubernetes-validations:
            - message: description must not be empty
              rule: self.description.size() > 0
        type: object
    served: true
    storage: true
`

const template = header + spec + footer

func TestStripTemplateRemovesEachSchemaDescription(t *testing.T) {
	t.Parallel()

	out, removed, err := stripTemplate(template)
	require.NoError(t, err)
	assert.Equal(t, 11, removed)

	schema := parsedSchema(t, out)
	assert.NotContains(t, schema, "description")

	specSchema := schema["properties"].(map[string]any)["spec"].(map[string]any)
	assert.NotContains(t, specSchema, "description")

	steps := specSchema["properties"].(map[string]any)["steps"].(map[string]any)
	items := steps["items"].(map[string]any)
	assert.NotContains(t, items, "description")
	for _, option := range items["anyOf"].([]any) {
		assert.NotContains(t, option, "description")
	}

	meta := specSchema["properties"].(map[string]any)["meta"].(map[string]any)
	assert.NotContains(t, meta["additionalProperties"], "description")
}

func TestStripTemplateKeepsAPropertyNamedDescription(t *testing.T) {
	t.Parallel()

	out, _, err := stripTemplate(template)
	require.NoError(t, err)

	specSchema := parsedSchema(t, out)["properties"].(map[string]any)["spec"].(map[string]any)
	assert.Equal(
		t,
		map[string]any{"maxLength": float64(200), "type": "string"},
		specSchema["properties"].(map[string]any)["description"],
	)
	assert.Equal(t, []any{"description"}, specSchema["required"])
}

func TestStripTemplateKeepsWhatIsNotADescription(t *testing.T) {
	t.Parallel()

	out, _, err := stripTemplate(template)
	require.NoError(t, err)

	crd := parsed(t, out)
	version := crd["spec"].(map[string]any)["versions"].([]any)[0].(map[string]any)
	assert.Equal(
		t,
		[]any{map[string]any{
			"description": "The label of the section.",
			"jsonPath":    ".spec.label",
			"name":        "Label",
			"type":        "string",
		}},
		version["additionalPrinterColumns"],
	)

	specSchema := parsedSchema(t, out)["properties"].(map[string]any)["spec"].(map[string]any)
	assert.Equal(
		t,
		[]any{map[string]any{"message": "description must not be empty", "rule": "self.description.size() > 0"}},
		specSchema["x-kubernetes-validations"],
	)

	properties := specSchema["properties"].(map[string]any)
	assert.Equal(
		t,
		map[string]any{"default": "Open", "enum": []any{"Open", "Closed"}, "type": "string"},
		properties["mode"],
	)
	assert.Equal(
		t,
		map[string]any{"description": "kept, because a default is not a schema"},
		properties["meta"].(map[string]any)["default"],
	)
	assert.Equal(t, true, properties["template"].(map[string]any)["x-kubernetes-preserve-unknown-fields"])
}

func TestStripTemplateKeepsTheHelmActionsAroundTheSpec(t *testing.T) {
	t.Parallel()

	out, _, err := stripTemplate(template)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(out, header+"spec:\n"), "the header changed:\n%s", out)
	assert.True(t, strings.HasSuffix(out, "\n"+footer), "the footer changed:\n%s", out)
}

func TestStripTemplateKeepsAHelmLiteralOutsideADescription(t *testing.T) {
	t.Parallel()

	out, _, err := stripTemplate(template)
	require.NoError(t, err)

	assert.Contains(t, out, `pattern: '^{{ "{{" }}'`)
	assert.NotContains(t, out, `{{ "{{ .namespace }}" }}`)
	assert.NotContains(t, out, placeholderPrefix)
}

func TestStripTemplateFailsOnAnotherHelmAction(t *testing.T) {
	t.Parallel()

	in := strings.Replace(template, "scope: Cluster", "scope: {{ .Values.scope }}", 1)
	_, _, err := stripTemplate(in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Helm action")
}

func TestStripTemplateFailsWithoutASpec(t *testing.T) {
	t.Parallel()

	_, _, err := stripTemplate(header + footer)
	require.Error(t, err)
}

func TestStripTemplateFailsWithoutAVersionSchema(t *testing.T) {
	t.Parallel()

	in := header + "spec:\n  versions:\n  - name: v1alpha1\n" + footer
	_, _, err := stripTemplate(in)
	require.Error(t, err)
}

// parsed reads the CRD of a stripped template, with the Helm action lines
// removed so that YAML can read it.
func parsed(t *testing.T, out string) map[string]any {
	t.Helper()

	var kept []string
	for line := range strings.SplitSeq(out, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "{{-") {
			kept = append(kept, line)
		}
	}

	var crd map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(strings.Join(kept, "\n")), &crd))

	return crd
}

// parsedSchema returns the openAPIV3Schema of the first version.
func parsedSchema(t *testing.T, out string) map[string]any {
	t.Helper()

	version := parsed(t, out)["spec"].(map[string]any)["versions"].([]any)[0].(map[string]any)

	return version["schema"].(map[string]any)["openAPIV3Schema"].(map[string]any)
}
