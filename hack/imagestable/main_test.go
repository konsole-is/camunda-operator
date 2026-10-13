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
)

// entry89 holds every variable of test/e2e/matrix/8.9.env, with its values
// of 2026-10-11.
const entry89 = `# renovate: datasource=docker depName=camunda/camunda
CAMUNDA_VERSION=8.9.23
CAMUNDA_CONNECTORS_VERSION=8.9.14
CAMUNDA_OPTIMIZE_VERSION=8.9.23
ELASTICSEARCH_VERSION=9.2.8
CAMUNDA_IDENTITY_VERSION=8.9.10
CAMUNDA_CONSOLE_VERSION=8.9.122
CAMUNDA_WEB_MODELER_VERSION=8.9.10
KEYCLOAK_VERSION=26.6.4
CNPG_VERSION=1.30.1
BARMAN_PLUGIN_VERSION=0.14.0
POSTGRES_VERSION=17
E2E_LABELS=
`

func TestBuild(t *testing.T) {
	want89 := map[string]entry{
		"camunda":                {Repository: "camunda/camunda", Version: "8.9.23"},
		"connectors":             {Repository: "camunda/connectors-bundle", Version: "8.9.14"},
		"optimize":               {Repository: "camunda/optimize", Version: "8.9.23"},
		"identity":               {Repository: "camunda/identity", Version: "8.9.10"},
		"console":                {Repository: "camunda/console", Version: "8.9.122"},
		"web-modeler-restapi":    {Repository: "camunda/web-modeler-restapi", Version: "8.9.10"},
		"web-modeler-websockets": {Repository: "camunda/web-modeler-websockets", Version: "8.9.10"},
		"keycloak":               {Repository: "camunda/keycloak", TagPrefix: "quay-optimized-", Version: "26.6.4"},
		"postgres":               {Repository: "ghcr.io/cloudnative-pg/postgresql", Version: "17"},
		"elasticsearch":          {Version: "9.2.8"},
	}
	entry810 := strings.ReplaceAll(entry89, "=8.9.", "=8.10.")

	tests := []struct {
		name    string
		files   map[string]string
		want    map[string]map[string]entry
		wantErr string
	}{
		{
			name:  "every variable of an entry lands on its image",
			files: map[string]string{"8.9.env": entry89, "README.md": "not an entry"},
			want:  map[string]map[string]entry{"8.9": want89},
		},
		{
			name:  "the Web Modeler images move to the Hub repositories from 8.10",
			files: map[string]string{"8.9.env": entry89, "8.10.env": entry810},
			want: map[string]map[string]entry{
				"8.9": want89,
				"8.10": func() map[string]entry {
					want := map[string]entry{}
					for image, e := range want89 {
						e.Version = strings.Replace(e.Version, "8.9.", "8.10.", 1)
						want[image] = e
					}
					want["web-modeler-restapi"] = entry{Repository: "camunda/hub", Version: "8.10.10"}
					want["web-modeler-websockets"] = entry{Repository: "camunda/hub-websockets", Version: "8.10.10"}
					return want
				}(),
			},
		},
		{
			name:    "a variable that maps to no image fails the run",
			files:   map[string]string{"8.9.env": entry89 + "CAMUNDA_TASKLIST_VERSION=8.9.1\n"},
			wantErr: "CAMUNDA_TASKLIST_VERSION",
		},
		{
			name:    "an image variable that the entry does not hold fails the run",
			files:   map[string]string{"8.9.env": strings.Replace(entry89, "KEYCLOAK_VERSION=26.6.4\n", "", 1)},
			wantErr: "holds no KEYCLOAK_VERSION",
		},
		{
			name:    "an empty image variable fails the run",
			files:   map[string]string{"8.9.env": strings.Replace(entry89, "=26.6.4", "=", 1)},
			wantErr: "holds no KEYCLOAK_VERSION",
		},
		{
			name:    "an entry whose name is not a minor fails the run",
			files:   map[string]string{"8.9.env": entry89, "latest.env": entry89},
			wantErr: "latest.env",
		},
		{
			name:    "a directory without an entry fails the run",
			files:   map[string]string{"README.md": "not an entry"},
			wantErr: "holds no matrix entry",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range test.files {
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
			}

			got, err := build(dir)

			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, table{SchemaVersion: 1, Minors: test.want}, got)
		})
	}
}

// TestEncode pins the bytes that the app reads and that the Lint drift check
// compares: sorted keys, two spaces, no repository for a version-only row, no
// tagPrefix where the tag is the bare version, and a final newline.
func TestEncode(t *testing.T) {
	raw, err := encode(table{
		SchemaVersion: 1,
		Minors: map[string]map[string]entry{
			"8.9": {
				"keycloak":      {Repository: "camunda/keycloak", TagPrefix: "quay-optimized-", Version: "26.6.4"},
				"elasticsearch": {Version: "9.2.8"},
				"camunda":       {Repository: "camunda/camunda", Version: "8.9.23"},
			},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, `{
  "schemaVersion": 1,
  "minors": {
    "8.9": {
      "camunda": {
        "repository": "camunda/camunda",
        "version": "8.9.23"
      },
      "elasticsearch": {
        "version": "9.2.8"
      },
      "keycloak": {
        "repository": "camunda/keycloak",
        "tagPrefix": "quay-optimized-",
        "version": "26.6.4"
      }
    }
  }
}
`, string(raw))
}
