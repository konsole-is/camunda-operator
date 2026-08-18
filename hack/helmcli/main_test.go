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
)

// generatedValues is the shape kubebuilder's helm plugin (v4.13) writes for a
// manager with one env var.
const generatedValues = `manager:
  replicas: 1

  image:
    repository: controller
    tag: latest
    pullPolicy: IfNotPresent

  ## Arguments
  ##
  args:
    - --leader-elect

  ## Environment variables
  ##
  env:
    - name: CAMUNDA_OPERATOR_CLI_IMAGE
      value: ghcr.io/konsole-is/camunda-operator-cli:0.4.0

  ## Env overrides (--set manager.envOverrides.VAR=value)
  ## Same name in env above: this value takes precedence.
  ##
  envOverrides: {}
`

const generatedTemplate = `      containers:
      - args:
        {{- if .Values.metrics.enable }}
        - --metrics-bind-address=:{{ .Values.metrics.port }}
        {{- end }}
        - --health-probe-bind-address=:8081
        {{- range .Values.manager.args }}
        - {{ . }}
        {{- end }}
        command:
        - /manager
`

func TestRewriteValuesExposesCLIImage(t *testing.T) {
	t.Parallel()

	out, image, err := rewriteValues(generatedValues)
	require.NoError(t, err)
	assert.Equal(t, "ghcr.io/konsole-is/camunda-operator-cli:0.4.0", image)

	assert.NotContains(t, out, "CAMUNDA_OPERATOR_CLI_IMAGE")
	assert.Contains(t, out, "  env: []\n")
	assert.Contains(t, out, "  cliImage:\n    repository: ghcr.io/konsole-is/camunda-operator-cli\n    tag: 0.4.0\n")

	// The block sits with the image block, before args.
	assert.Less(t, strings.Index(out, "cliImage:"), strings.Index(out, "args:"))
	assert.Greater(t, strings.Index(out, "cliImage:"), strings.Index(out, "pullPolicy:"))
}

func TestRewriteValuesKeepsOtherEnvEntries(t *testing.T) {
	t.Parallel()

	in := strings.Replace(generatedValues,
		"      value: ghcr.io/konsole-is/camunda-operator-cli:0.4.0\n",
		"      value: ghcr.io/konsole-is/camunda-operator-cli:0.4.0\n    - name: OTHER\n      value: x\n", 1)
	out, _, err := rewriteValues(in)
	require.NoError(t, err)
	assert.Contains(t, out, "  env:\n    - name: OTHER\n      value: x\n")
	assert.NotContains(t, out, "env: []")
}

func TestRewriteValuesFailsWithoutTheEnvEntry(t *testing.T) {
	t.Parallel()

	in := strings.Replace(generatedValues, "CAMUNDA_OPERATOR_CLI_IMAGE", "SOMETHING_ELSE", 1)
	_, _, err := rewriteValues(in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CAMUNDA_OPERATOR_CLI_IMAGE")
}

func TestRewriteTemplateRendersTheFlagFromCLIImage(t *testing.T) {
	t.Parallel()

	out, err := rewriteTemplate(generatedTemplate)
	require.NoError(t, err)
	assert.Contains(t, out,
		"        - --health-probe-bind-address=:8081\n"+
			"        - --camunda-operator-cli-image={{ .Values.manager.cliImage.repository }}:"+
			"{{ .Values.manager.cliImage.tag }}\n"+
			"        {{- range .Values.manager.args }}\n")

	_, err = rewriteTemplate(out)
	require.Error(t, err, "a second pass must not add the flag twice")
}

func TestSplitImage(t *testing.T) {
	t.Parallel()

	repo, tag, err := splitImage("localhost:5000/cli:1.2.3")
	require.NoError(t, err)
	assert.Equal(t, "localhost:5000/cli", repo)
	assert.Equal(t, "1.2.3", tag)

	_, _, err = splitImage("ghcr.io/konsole-is/camunda-operator-cli")
	require.Error(t, err)
	_, _, err = splitImage("ghcr.io/x/cli@sha256:abc")
	require.Error(t, err)
	// An empty repository or tag would render "repo:" or ":tag" into the
	// chart, an invalid image, instead of failing here.
	_, _, err = splitImage("ghcr.io/x/cli:")
	require.Error(t, err)
	_, _, err = splitImage(":1.2.3")
	require.Error(t, err)
}
