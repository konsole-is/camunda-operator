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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// generatedValues ends like the values.yaml that the kubebuilder helm plugin
// (v4.16) writes.
const generatedValues = `manager:
  replicas: 1

## Network policies for controlling traffic flow.
##
networkPolicy:
  enabled: false
`

func TestAppendValuesAddsTheFleetManagerBlock(t *testing.T) {
	t.Parallel()

	out, err := appendValues(generatedValues, "ghcr.io/konsole-is/camunda-operator-fleet-manager", "0.4.0")
	require.NoError(t, err)

	var values struct {
		NetworkPolicy map[string]any `json:"networkPolicy"`
		FleetManager  struct {
			Enabled bool `json:"enabled"`
			Image   struct {
				Repository string `json:"repository"`
				Tag        string `json:"tag"`
				PullPolicy string `json:"pullPolicy"`
			} `json:"image"`
			Resources map[string]any `json:"resources"`
			Settings  map[string]any `json:"settings"`
		} `json:"fleetManager"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(out), &values))

	assert.Equal(t, map[string]any{"enabled": false}, values.NetworkPolicy, "the generated values stay")
	assert.False(t, values.FleetManager.Enabled, "the app is off by default")
	assert.Equal(t, "ghcr.io/konsole-is/camunda-operator-fleet-manager", values.FleetManager.Image.Repository)
	assert.Equal(t, "0.4.0", values.FleetManager.Image.Tag)
	assert.Equal(t, "IfNotPresent", values.FleetManager.Image.PullPolicy)
	assert.NotEmpty(t, values.FleetManager.Resources)
	assert.Equal(t, map[string]any{
		"kubernetesClusterName": "",
		"git": map[string]any{
			"repository": "", "branch": "", "path": "", "credentialsSecretRef": "",
		},
		"secretStore": map[string]any{"kind": "", "name": ""},
		"prometheus":  map[string]any{"url": "", "credentialsSecretRef": ""},
	}, values.FleetManager.Settings, "the settings keys of contract 6.1")
}

func TestAppendValuesWritesANumericTagAsAString(t *testing.T) {
	t.Parallel()

	out, err := appendValues(generatedValues, "registry/fleet", "8")
	require.NoError(t, err)

	assert.Contains(t, out, "    tag: \"8\"\n")
}

func TestAppendValuesFailsWhenTheBlockExists(t *testing.T) {
	t.Parallel()

	once, err := appendValues(generatedValues, "registry/fleet", "1")
	require.NoError(t, err)

	_, err = appendValues(once, "registry/fleet", "1")
	require.ErrorContains(t, err, "fleetManager")
}

func TestCopyTemplatesReplacesTheTargetDirectory(t *testing.T) {
	t.Parallel()

	source, target := t.TempDir(), filepath.Join(t.TempDir(), "fleet-manager")
	require.NoError(t, os.WriteFile(filepath.Join(source, "deployment.yaml"), []byte("kind: Deployment\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(source, "_helpers.tpl"), []byte("{{/* */}}\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(source, "notes.md"), []byte("not a template\n"), 0o600))
	require.NoError(t, os.MkdirAll(target, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(target, "removed.yaml"), []byte("kind: Old\n"), 0o600))

	copied, err := copyTemplates(source, target)
	require.NoError(t, err)

	assert.Equal(t, 2, copied)
	entries, err := os.ReadDir(target)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.ElementsMatch(
		t,
		[]string{"deployment.yaml", "_helpers.tpl"},
		names,
		"a template deleted from the source does not stay",
	)
	content, err := os.ReadFile(filepath.Join(target, "deployment.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "kind: Deployment\n", string(content))
}

func TestCopyTemplatesFailsWithoutTemplates(t *testing.T) {
	t.Parallel()

	_, err := copyTemplates(t.TempDir(), filepath.Join(t.TempDir(), "fleet-manager"))

	require.ErrorContains(t, err, "no .yaml or .tpl file")
}

func TestRunRejectsAnImageWithoutTag(t *testing.T) {
	t.Parallel()

	err := run(t.TempDir(), t.TempDir(), "ghcr.io/konsole-is/camunda-operator-fleet-manager")

	require.ErrorContains(t, err, "-image")
}
