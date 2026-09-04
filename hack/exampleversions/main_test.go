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
)

const releaseManifest = `apiVersion: core.camunda.io/v1
kind: CamundaRelease
metadata:
  name: camunda-8-9
spec:
  # renovate: datasource=docker depName=camunda/camunda
  version: "8.9.18"
  elasticsearch:
    # renovate: datasource=docker depName=docker.elastic.co/elasticsearch/elasticsearch
    version: "9.2.8"
  databaseServer:
    version: "17"
`

const releasePage = "# Release\n\n```sh\nkubectl apply -k config/example/releases\n```\n\n" +
	"```yaml\nspec:\n  version: \"8.9.18\"\n```\n"

const matrixEntry = `# renovate: datasource=docker depName=camunda/camunda
CAMUNDA_VERSION=8.9.18
# renovate: datasource=docker depName=docker.elastic.co/elasticsearch/elasticsearch
ELASTICSEARCH_VERSION=9.2.8
POSTGRES_VERSION=17
E2E_LABELS=
`

var testPins = []pin{
	{
		file:   "camunda-release.yaml",
		path:   "spec.version",
		env:    "CAMUNDA_VERSION",
		marker: "datasource=docker depName=camunda/camunda",
	},
	{
		file:   "camunda-release.yaml",
		path:   "spec.elasticsearch.version",
		env:    "ELASTICSEARCH_VERSION",
		marker: elasticsearchMarker,
	},
	{file: "camunda-release.yaml", path: "spec.databaseServer.version", env: "POSTGRES_VERSION"},
	{file: "README.md", path: "spec.version", env: "CAMUNDA_VERSION"},
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		page     string
		matrix   string
		problems []string
	}{
		{
			name: "every pin stands on the matrix version",
		},
		{
			name:   "a version that the matrix raised alone",
			matrix: "CAMUNDA_VERSION=8.9.19\nELASTICSEARCH_VERSION=9.2.8\nPOSTGRES_VERSION=17\n",
			problems: []string{
				`camunda-release.yaml:7: spec.version is "8.9.18", CAMUNDA_VERSION is "8.9.19"`,
				`README.md:9: spec.version is "8.9.18", CAMUNDA_VERSION is "8.9.19"`,
			},
		},
		{
			name:     "a version of a page that the manifest left behind",
			page:     "```yaml\nspec:\n  version: \"8.9.17\"\n```\n",
			problems: []string{`README.md:3: spec.version is "8.9.17", CAMUNDA_VERSION is "8.9.18"`},
		},
		{
			name: "a tracked version that lost its marker",
			manifest: "apiVersion: core.camunda.io/v1\nspec:\n  version: \"8.9.18\"\n" +
				"  elasticsearch:\n    # renovate: " + elasticsearchMarker + "\n    version: \"9.2.8\"\n" +
				"  databaseServer:\n    version: \"17\"\n",
			problems: []string{
				"camunda-release.yaml:3: spec.version has no Renovate marker. " +
					"Add `# renovate: datasource=docker depName=camunda/camunda` above it",
			},
		},
		{
			name: "a marker on a version the table says Renovate leaves alone",
			manifest: releaseManifest[:len(releaseManifest)-len("    version: \"17\"\n")] +
				"    # renovate: datasource=docker depName=postgres\n    version: \"17\"\n",
			problems: []string{
				"camunda-release.yaml:13: spec.databaseServer.version carries a Renovate marker. " +
					"Add the marker to its entry in hack/exampleversions",
			},
		},
		{
			name:     "a version that no entry holds",
			manifest: releaseManifest + "  connectors:\n    version: \"8.9.9\"\n",
			problems: []string{
				"camunda-release.yaml:14: spec.connectors.version is in no entry of hack/exampleversions. " +
					"Add one that names the variable it follows",
			},
		},
		{
			name:     "a version inside a sequence",
			manifest: releaseManifest + "  images:\n    - version: \"8.9.18\"\n",
			problems: []string{
				"camunda-release.yaml:14: spec.images.version is in no entry of hack/exampleversions. " +
					"Add one that names the variable it follows",
			},
		},
		{
			name:     "a variable the matrix entry does not hold",
			matrix:   "CAMUNDA_VERSION=8.9.18\nELASTICSEARCH_VERSION=9.2.8\n",
			problems: []string{"camunda-release.yaml:12: the matrix entry holds no POSTGRES_VERSION"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, filepath.Join(dir, "camunda-release.yaml"), or(test.manifest, releaseManifest))
			write(t, filepath.Join(dir, "README.md"), or(test.page, releasePage))

			matrix := filepath.Join(dir, "8.9.env")
			write(t, matrix, or(test.matrix, matrixEntry))

			problems, err := check(dir, matrix, testPins)
			require.NoError(t, err)

			for i, problem := range problems {
				problems[i] = trimDir(problem, dir)
			}
			assert.Equal(t, test.problems, problems)
		})
	}
}

func TestCheckMissingFile(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "README.md"), releasePage)

	matrix := filepath.Join(dir, "8.9.env")
	write(t, matrix, matrixEntry)

	problems, err := check(dir, matrix, testPins)
	require.NoError(t, err)

	assert.Equal(t, []string{
		filepath.Join(dir, "camunda-release.yaml") + ": no such file. An entry expects spec.version in it",
		filepath.Join(dir, "camunda-release.yaml") +
			": no such file. An entry expects spec.elasticsearch.version in it",
		filepath.Join(dir, "camunda-release.yaml") +
			": no such file. An entry expects spec.databaseServer.version in it",
	}, problems)
}

func TestCheckRepeatedPath(t *testing.T) {
	dir := t.TempDir()
	write(
		t,
		filepath.Join(dir, "camunda-release.yaml"),
		releaseManifest+"---\napiVersion: core.camunda.io/v1\nkind: CamundaRelease\nspec:\n"+
			"  # renovate: datasource=docker depName=camunda/camunda\n  version: \"8.9.18\"\n",
	)
	write(t, filepath.Join(dir, "README.md"), releasePage)

	matrix := filepath.Join(dir, "8.9.env")
	write(t, matrix, matrixEntry)

	problems, err := check(dir, matrix, testPins)
	require.NoError(t, err)

	assert.Equal(
		t,
		[]string{filepath.Join(dir, "camunda-release.yaml") + ": spec.version stands 2 times. It has to stand once"},
		problems,
	)
}

// TestExampleInventories runs the check the lint gate runs, against the files
// of this repository.
func TestExampleInventories(t *testing.T) {
	problems, err := check(
		filepath.Join("..", "..", "config", "example"),
		filepath.Join("..", "..", "test", "e2e", "matrix", "8.9.env"),
		pins,
	)
	require.NoError(t, err)
	assert.Empty(t, problems)
}

func write(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func or(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func trimDir(problem, dir string) string {
	return problem[len(dir)+1:]
}
