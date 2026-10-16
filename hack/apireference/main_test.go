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

func TestProcessEscapesNamePatternsOutsideCodeSpans(t *testing.T) {
	cases := map[string]struct {
		in   string
		want string
	}{
		"name pattern": {
			in:   "| `serviceAccount` _string_ | The name <name>-camunda belongs to the operator. |",
			want: "| `serviceAccount` _string_ | The name &lt;name&gt;-camunda belongs to the operator. |",
		},
		"pattern with a space": {
			in:   "It creates it with the name <server name>-postgres.",
			want: "It creates it with the name &lt;server name&gt;-postgres.",
		},
		"capitalized patterns": {
			in:   "The message reads <Reason>: <Message>.",
			want: "The message reads &lt;Reason&gt;: &lt;Message&gt;.",
		},
		"path of patterns": {
			in:   "in the form<br /><section>/<output>.",
			want: "in the form<br />&lt;section&gt;/&lt;output&gt;.",
		},
		"version range": {
			in:   `for example ">=0.3.0 <1.0.0".`,
			want: `for example "&gt;=0.3.0 &lt;1.0.0".`,
		},
		"line breaks stay HTML": {
			in:   "first<br />second",
			want: "first<br />second",
		},
		"code span stays as written": {
			in:   "Set `<name>-camunda` or <name>.",
			want: "Set `<name>-camunda` or &lt;name&gt;.",
		},
		"double backtick span holds a backtick": {
			in:   "``a ` <b>`` and <c>",
			want: "``a ` <b>`` and &lt;c&gt;",
		},
		"backtick with no end is text": {
			in:   "a ` <b>",
			want: "a ` &lt;b&gt;",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := process(tc.in)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestProcessLinksTheValueTypeOfANamedMapType(t *testing.T) {
	page := "#### ParameterPolicies\n\n" +
		"_Underlying type:_ _[map[string]ParameterPolicy](#map[string]parameterpolicy)_\n\n" +
		"#### ParameterPolicy\n"

	got, err := process(page)

	require.NoError(t, err)
	assert.Contains(t, got, "_Underlying type:_ _object (keys:string, values:[ParameterPolicy](#parameterpolicy))_")
}

func TestProcessFailsOnALinkWithNoHeading(t *testing.T) {
	page := "#### Known\n\nSee [Known](#known) and [Gone](#gone).\n"

	_, err := process(page)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "#gone")
	assert.NotContains(t, err.Error(), "#known")
}

func TestProcessFindsTheAnchorsThatMkDocsWrites(t *testing.T) {
	page := "## core.camunda.io/v1\n\n### Resource Types\n\n### Resource Types\n\n" +
		"[a](#corecamundaiov1) [b](#resource-types) [c](#resource-types_1)\n"

	_, err := process(page)

	assert.NoError(t, err)
}

func TestProcessLeavesItsOwnOutputUnchanged(t *testing.T) {
	page := "#### ParameterPolicy\n\n" +
		"| `a` _[map[string]ParameterPolicy](#x)_ | The name <name>-camunda. |\n"

	once, err := process(page)
	require.NoError(t, err)
	twice, err := process(once)
	require.NoError(t, err)

	assert.Equal(t, once, twice)
}

func TestRunRewritesThePageInPlace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api-reference.md")
	require.NoError(t, os.WriteFile(path, []byte("The name <name>-camunda.\n"), 0o600))

	require.NoError(t, run(path))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "The name &lt;name&gt;-camunda.\n", string(got))
}

// The drift check of the Lint job only proves that the committed page is what
// make api-docs writes. This test also fails when make api-docs stops running
// this step.
func TestCommittedPageNeedsNoCorrection(t *testing.T) {
	page, err := os.ReadFile(filepath.Join("..", "..", "docs", "crds", "api-reference.md"))
	require.NoError(t, err)

	got, err := process(string(page))

	require.NoError(t, err)
	assert.Equal(t, string(page), got)
}
