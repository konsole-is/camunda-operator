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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestEncodeOpensWithTheGeneratedLine(t *testing.T) {
	out, err := encode([]unstructured.Unstructured{object("v1", "Namespace", "", "team-a", "u")})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(out), generatedHeader+"\n"))
}

func TestWriteOutputsRemovesOnlyStaleGeneratedFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600))
	}
	write("scenario.yaml", "schemaVersion: 1\n")
	write("observe.yaml", "logs: []\n")
	write("removed-state.yaml", generatedHeader+"\n---\n")

	ns := object("v1", "Namespace", "", "team-a", "u")
	require.NoError(t, writeOutputs(dir, map[string][]unstructured.Unstructured{
		"objects.yaml": {ns},
	}))

	assert.FileExists(t, filepath.Join(dir, "scenario.yaml"))
	assert.FileExists(t, filepath.Join(dir, "observe.yaml"))
	assert.FileExists(t, filepath.Join(dir, "objects.yaml"))
	assert.NoFileExists(t, filepath.Join(dir, "removed-state.yaml"))
}
