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

package merge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mergeRulesPage = `# Kind

Intro.

## Merge rules

The baseline is the preset.

| Field | Merge behavior |
| --- | --- |
| ` + "`a`, `b`" + ` | Replaced. |
| ` + "`c` (top-level and `d.c`)" + ` | Never merged. |

Text after the table.

| Other | Table |
| --- | --- |
| ` + "`x`" + ` | Not a rule. |

## Changes

| Field | Behavior |
| --- | --- |
| ` + "`y`" + ` | Not a rule. |
`

func TestParseMergeRulesReturnsTheFirstCellOfEachRow(t *testing.T) {
	t.Parallel()

	rules, err := parseMergeRules([]byte(mergeRulesPage))
	require.NoError(t, err)
	assert.Equal(t, []string{"`a`, `b`", "`c` (top-level and `d.c`)"}, rules)
}

func TestParseMergeRulesRefusesAPageWithoutTheTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		page string
	}{
		{"no section", "# Kind\n\n## Changes\n\n| a | b |\n| --- | --- |\n| `x` | y |\n"},
		{"section without a table", "# Kind\n\n## Merge rules\n\nProse only.\n\n## Changes\n"},
		{"table without rows", "# Kind\n\n## Merge rules\n\n| Field | Merge behavior |\n| --- | --- |\n\n## Changes\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseMergeRules([]byte(tt.page))
			assert.Error(t, err)
		})
	}
}

func TestRuleCoverage(t *testing.T) {
	t.Parallel()

	rules := []string{"`a`", "`b`"}
	fixtures := []fixture{
		{Rule: "`a`", Path: "/x/a-one.json"},
		{Rule: "`a`", Path: "/x/a-two.json"},
		{Rule: "`gone`", Path: "/x/stale.json"},
	}

	uncovered, unknown := ruleCoverage(rules, fixtures)
	assert.Equal(t, []string{"`b`"}, uncovered, "a row without a case is reported")
	assert.Equal(t, []string{"stale.json"}, unknown, "a case that names no row is reported")
}

func TestRuleCoverageOfACompleteSetIsEmpty(t *testing.T) {
	t.Parallel()

	uncovered, unknown := ruleCoverage(
		[]string{"`a`"},
		[]fixture{{Rule: "`a`", Path: "/x/a.json"}},
	)
	assert.Empty(t, uncovered)
	assert.Empty(t, unknown)
}

func TestLoadFixturesReadsEveryCaseInFileNameOrder(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for name, body := range map[string]string{
		"b.json": `{"schemaVersion": 1, "name": "second", "rule": "r", "spec": {}}`,
		"a.json": `{"schemaVersion": 1, "name": "first", "rule": "r", "spec": {}}`,
		"README": `not a case`,
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600))
	}

	fixtures, err := loadFixtures(dir)
	require.NoError(t, err)
	require.Len(t, fixtures, 2)
	assert.Equal(t, "first", fixtures[0].Name)
	assert.Equal(t, filepath.Join(dir, "a.json"), fixtures[0].Path)
	assert.Equal(t, "second", fixtures[1].Name)
}

func TestLoadFixturesRefusesAnUnknownTopLevelField(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	body := `{"schemaVersion": 1, "name": "n", "rule": "r", "spec": {}, "expected": {}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.json"), []byte(body), 0o600))

	_, err := loadFixtures(dir)
	assert.ErrorContains(t, err, "a.json")
}

type testLayer struct {
	ClientID string `json:"clientId,omitempty"`
}

func TestDecodeLayer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    *testLayer
		wantErr bool
	}{
		{"absent layer is nil", "", nil, false},
		{"null layer is nil", "null", nil, false},
		{"empty layer is not nil", "{}", &testLayer{}, false},
		{"known field", `{"clientId": "a"}`, &testLayer{ClientID: "a"}, false},
		{"field name with the wrong case", `{"clientID": "a"}`, nil, true},
		{"unknown field", `{"clientSecret": "a"}`, nil, true},
		{"duplicate field", `{"clientId": "a", "clientId": "b"}`, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := decodeLayer[testLayer](json.RawMessage(tt.raw))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWriteFixtureWritesStableBytes(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "a.json")
	f := fixture{
		SchemaVersion: schemaVersion,
		Name:          "a case",
		Rule:          "`scheduling` (top-level, per component, and `backup.dump.scheduling`) <x>",
		Spec:          json.RawMessage(`{"b":1,   "a":2}`),
		Merged:        json.RawMessage(`{"a":2}`),
		Path:          path,
	}

	require.NoError(t, writeFixture(f))
	first, err := os.ReadFile(path)
	require.NoError(t, err)

	reloaded, err := loadFixtures(filepath.Dir(path))
	require.NoError(t, err)
	require.Len(t, reloaded, 1)
	require.NoError(t, writeFixture(reloaded[0]))
	second, err := os.ReadFile(path)
	require.NoError(t, err)

	assert.Equal(t, string(first), string(second), "a second write changes nothing")
	assert.Equal(t, `{
  "schemaVersion": 1,
  "name": "a case",
  "rule": "`+"`scheduling` (top-level, per component, and `backup.dump.scheduling`) <x>"+`",
  "spec": {
    "b": 1,
    "a": 2
  },
  "merged": {
    "a": 2
  }
}
`, string(first), "fixed order, two-space indent, no HTML escaping, final newline")
}
