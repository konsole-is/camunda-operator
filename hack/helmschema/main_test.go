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
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generatedValues has the shape that the kubebuilder helm plugin (v4.16)
// writes, after helmcli.
const generatedValues = `## String to partially override chart.fullname template
##
# nameOverride: ""

## Configure the controller manager deployment
##
manager:
  replicas: 1

  image:
    repository: controller
    ## Image tag (defaults to Chart.appVersion if not set)
    ##
    # tag: ""
    pullPolicy: IfNotPresent

  ## Arguments
  ##
  args:
    - --leader-elect

  ## Image pull secrets
  ##
  # imagePullSecrets:
  #   - name: myregistrykey

  podSecurityContext:
    runAsNonRoot: true

  securityContext:
    readOnlyRootFilesystem: true

  ## Resource limits and requests
  ##
  resources:
    limits:
      cpu: 500m

  nodeSelector: {}

  tolerations: []

  ## Deployment strategy
  ##
  # strategy:
  #   type: RollingUpdate
  #   rollingUpdate:
  #     maxSurge: 25%

## Controller metrics endpoint.
## Enable to expose /metrics endpoint
##
metrics:
  enabled: true
  # Metrics server port
  port: 8443
  ratio: 0.5
`

const generatedTemplate = `spec:
  replicas: {{ .Values.manager.replicas }}
  {{- with .Values.manager.extraVolumes }}
  volumes: {{ toYaml . | nindent 4 }}
  {{- end }}
  {{- with .Values.manager.resources.claims }}
  {{- end }}
  {{- if $.Values.webhook.enabled }}
  {{- end }}
`

// property is the JSON form of one schema node, as Helm reads it.
type property struct {
	Type                 string               `json:"type"`
	Description          string               `json:"description"`
	Properties           map[string]*property `json:"properties"`
	AdditionalProperties *bool                `json:"additionalProperties"`
	Items                *property            `json:"items"`
}

// generateSchema returns the schema of generatedValues with the templates.
func generateSchema(t *testing.T, templates ...string) *property {
	t.Helper()

	out, err := generate(generatedValues, templates)
	require.NoError(t, err)

	var root property
	require.NoError(t, json.Unmarshal(out, &root))

	return &root
}

func at(t *testing.T, root *property, path string) *property {
	t.Helper()

	p := root
	for key := range strings.SplitSeq(path, ".") {
		require.NotNil(t, p.Properties[key], "no property at %s", path)
		p = p.Properties[key]
	}

	return p
}

func TestGenerateTypesEachValueByItsDefault(t *testing.T) {
	t.Parallel()

	root := generateSchema(t)

	assert.Equal(t, "object", root.Type)
	assert.Equal(t, "integer", at(t, root, "manager.replicas").Type)
	assert.Equal(t, "string", at(t, root, "manager.image.repository").Type)
	assert.Equal(t, "boolean", at(t, root, "metrics.enabled").Type)
	assert.Equal(t, "number", at(t, root, "metrics.ratio").Type)
	assert.Equal(t, "array", at(t, root, "manager.args").Type)
	require.NotNil(t, at(t, root, "manager.args").Items)
	assert.Equal(t, "string", at(t, root, "manager.args").Items.Type)
}

func TestGenerateDescribesEachValueByItsComment(t *testing.T) {
	t.Parallel()

	root := generateSchema(t)

	assert.Equal(t, "Configure the controller manager deployment", at(t, root, "manager").Description)
	assert.Equal(
		t,
		"Controller metrics endpoint. Enable to expose /metrics endpoint",
		at(t, root, "metrics").Description,
	)
	assert.Equal(t, "Metrics server port", at(t, root, "metrics.port").Description)
	assert.Empty(t, at(t, root, "manager.replicas").Description)
}

func TestGenerateRejectsUnknownKeysOfTheChart(t *testing.T) {
	t.Parallel()

	root := generateSchema(t)

	for _, path := range []string{"", "manager", "manager.image", "metrics"} {
		p := root
		if path != "" {
			p = at(t, root, path)
		}
		require.NotNil(t, p.AdditionalProperties, path)
		assert.False(t, *p.AdditionalProperties, path)
	}
}

func TestGenerateAcceptsAnyContentOfOpenValues(t *testing.T) {
	t.Parallel()

	root := generateSchema(t)

	// An empty default, a Kubernetes pass-through type, and list items.
	for _, path := range []string{
		"manager.nodeSelector",
		"manager.podSecurityContext",
		"manager.securityContext",
		"manager.resources",
		"manager.strategy",
	} {
		p := at(t, root, path)
		assert.Equal(t, "object", p.Type, path)
		assert.Nil(t, p.AdditionalProperties, path)
		assert.Empty(t, p.Properties, path)
	}
	assert.Nil(t, at(t, root, "manager.tolerations").Items)
	assert.Nil(t, at(t, root, "manager.imagePullSecrets").Items)
}

func TestGenerateIncludesCommentedOutExamples(t *testing.T) {
	t.Parallel()

	root := generateSchema(t)

	nameOverride := at(t, root, "nameOverride")
	assert.Equal(t, "string", nameOverride.Type)
	assert.Equal(t, "String to partially override chart.fullname template", nameOverride.Description)

	assert.Equal(t, "string", at(t, root, "manager.image.tag").Type)
	assert.Equal(t, "Image tag (defaults to Chart.appVersion if not set)", at(t, root, "manager.image.tag").Description)
	assert.Equal(t, "array", at(t, root, "manager.imagePullSecrets").Type)
	assert.Equal(t, "Deployment strategy", at(t, root, "manager.strategy").Description)
}

func TestGenerateKeepsDescriptiveCommentsAsComments(t *testing.T) {
	t.Parallel()

	root := generateSchema(t)

	assert.NotContains(t, at(t, root, "metrics").Properties, "Metrics server port")
	assert.Len(t, at(t, root, "metrics").Properties, 3)
}

func TestGenerateAllowsEveryValueThatATemplateReads(t *testing.T) {
	t.Parallel()

	root := generateSchema(t, generatedTemplate)

	// Not in values.yaml: any type is valid.
	extraVolumes := at(t, root, "manager.extraVolumes")
	assert.Empty(t, extraVolumes.Type)
	assert.Nil(t, extraVolumes.AdditionalProperties)
	webhook := at(t, root, "webhook")
	assert.Empty(t, webhook.Type)

	// Under an open value: the schema stays open and adds nothing.
	assert.Empty(t, at(t, root, "manager.resources").Properties)

	// In values.yaml: the type of the default stays.
	assert.Equal(t, "integer", at(t, root, "manager.replicas").Type)
}

func TestGenerateIsDeterministic(t *testing.T) {
	t.Parallel()

	first, err := generate(generatedValues, []string{generatedTemplate})
	require.NoError(t, err)
	second, err := generate(generatedValues, []string{generatedTemplate})
	require.NoError(t, err)

	assert.Equal(t, string(first), string(second))
	assert.Contains(t, string(first), `"$schema": "http://json-schema.org/draft-07/schema#"`)
}

func TestGenerateFailsWhenAnOpenPathIsMissing(t *testing.T) {
	t.Parallel()

	values := strings.Replace(generatedValues, "  resources:\n", "  limits:\n", 1)

	_, err := generate(values, nil)
	assert.ErrorContains(t, err, "manager.resources")
}

func TestGenerateFailsOnValuesThatAreNotAMapping(t *testing.T) {
	t.Parallel()

	_, err := generate("- a\n- b\n", nil)
	assert.ErrorContains(t, err, "not a mapping")

	_, err = generate("manager: [\n", nil)
	assert.Error(t, err)
}
