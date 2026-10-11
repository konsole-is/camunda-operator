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

package matrix

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "8.9.env")
	entry := "# renovate: datasource=docker depName=camunda/camunda\n" +
		"CAMUNDA_VERSION=8.9.23\n" +
		"\n" +
		"  KEYCLOAK_VERSION = 26.6.4  \n" +
		"not a variable\n" +
		"E2E_LABELS=\n"
	require.NoError(t, os.WriteFile(path, []byte(entry), 0o600))

	values, err := Read(path)

	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"CAMUNDA_VERSION":  "8.9.23",
		"KEYCLOAK_VERSION": "26.6.4",
		"E2E_LABELS":       "",
	}, values)
}

func TestReadAMissingFile(t *testing.T) {
	_, err := Read(filepath.Join(t.TempDir(), "8.9.env"))

	assert.ErrorContains(t, err, "reading the matrix entry")
}
