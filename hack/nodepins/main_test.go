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

// files is the content of the four pinned files below a root, by path.
type files map[string]string

func agreeingFiles() files {
	return files{
		"fleet-manager/.node-version": "26.13.1\n",
		"fleet-manager/package.json":  `{"name": "fleet-manager", "engines": {"node": "26.13.1"}}`,
		"fleet-manager/Dockerfile": "# The build stage.\nFROM node:26.13.1-trixie-slim AS build\n" +
			"FROM gcr.io/distroless/nodejs26-debian13:nonroot\n",
		".tool-versions": "golang 1.27.4\nnodejs 26.13.1\nkind 0.33.0\n",
	}
}

func writeFiles(t *testing.T, content files) string {
	t.Helper()
	root := t.TempDir()
	for path, text := range content {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, path), []byte(text), 0o644))
	}
	return root
}

func TestCheckPassesWhenAllPinsNameOneRelease(t *testing.T) {
	assert.NoError(t, check(writeFiles(t, agreeingFiles())))
}

func TestCheckPassesOnTheRepository(t *testing.T) {
	assert.NoError(t, check(filepath.Join("..", "..")))
}

func TestCheckNamesEveryPinWhenOneDiffers(t *testing.T) {
	content := agreeingFiles()
	content["fleet-manager/package.json"] = `{"engines": {"node": "26.13.0"}}`

	err := check(writeFiles(t, content))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "fleet-manager/.node-version release: 26.13.1")
	assert.Contains(t, err.Error(), "fleet-manager/package.json engines.node: 26.13.0")
	assert.Contains(t, err.Error(), "fleet-manager/Dockerfile build stage: 26.13.1")
	assert.Contains(t, err.Error(), ".tool-versions nodejs: 26.13.1")
}

func TestCheckFailsWhenAPinNamesNoRelease(t *testing.T) {
	tests := map[string]struct {
		path    string
		content string
		want    string
	}{
		"no build stage": {
			path:    "fleet-manager/Dockerfile",
			content: "FROM gcr.io/distroless/nodejs26-debian13:nonroot\n",
			want:    "fleet-manager/Dockerfile build stage: (none)",
		},
		"a build stage without a variant, which Renovate does not read": {
			path:    "fleet-manager/Dockerfile",
			content: "FROM node:26.13.1 AS build\n",
			want:    "fleet-manager/Dockerfile build stage: (none)",
		},
		"no nodejs line": {
			path:    ".tool-versions",
			content: "golang 1.27.4\n",
			want:    ".tool-versions nodejs: (none)",
		},
		"no engines": {
			path:    "fleet-manager/package.json",
			content: `{"name": "fleet-manager"}`,
			want:    "fleet-manager/package.json engines.node: (none)",
		},
		"an empty .node-version": {
			path:    "fleet-manager/.node-version",
			content: "\n",
			want:    "fleet-manager/.node-version release: (none)",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			content := agreeingFiles()
			content[tc.path] = tc.content

			err := check(writeFiles(t, content))

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestCheckFailsWhenAFileIsMissing(t *testing.T) {
	content := agreeingFiles()
	delete(content, ".tool-versions")

	err := check(writeFiles(t, content))

	require.Error(t, err)
	assert.Contains(t, err.Error(), ".tool-versions")
}

func TestCheckFailsOnPackageJSONThatIsNotJSON(t *testing.T) {
	content := agreeingFiles()
	content["fleet-manager/package.json"] = "engines: node"

	err := check(writeFiles(t, content))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "fleet-manager/package.json engines.node")
}
