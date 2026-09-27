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

// words returns a sentence of n words that starts with a descriptive subject.
func words(n int) string {
	return "The " + strings.Repeat("word ", n-2) + "ends."
}

func TestCheckPage(t *testing.T) {
	tests := []struct {
		name string
		page string
		want []finding
	}{
		{
			name: "descriptive sentence at the limit passes",
			page: words(25),
		},
		{
			name: "descriptive sentence over the limit is reported",
			page: words(26),
			want: []finding{{line: 1, words: 26, limit: 25}},
		},
		{
			name: "imperative sentence has the procedural limit",
			page: "Set " + strings.Repeat("word ", 20) + "now.",
			want: []finding{{line: 1, words: 22, limit: 20}},
		},
		{
			name: "condition before an imperative has the procedural limit",
			page: "If the cluster is down, run " + strings.Repeat("word ", 16) + "now.",
			want: []finding{{line: 1, words: 23, limit: 20}},
		},
		{
			name: "ordered list item has the procedural limit",
			page: "1. " + words(21),
			want: []finding{{line: 1, words: 21, limit: 20}},
		},
		{
			name: "sentence over two lines is reported at its first line",
			page: "Short one. The " + strings.Repeat("word ", 10) + "\n" + strings.Repeat("word ", 15) + "end.",
			want: []finding{{line: 1, words: 27, limit: 25}},
		},
		{
			name: "inline code, parentheses, and quotes count as one word",
			page: "The " + strings.Repeat("word ", 20) + "`a b c d` (e f g h) \"i j k l\" end.",
			want: nil,
		},
		{
			name: "link counts the words of its text",
			page: "The " + strings.Repeat("word ", 22) + "[one two](https://example.com/a-b-c) end.",
			want: []finding{{line: 1, words: 26, limit: 25}},
		},
		{
			name: "code fences, front matter, comments, and headings are skipped",
			page: "---\ntitle: " + words(30) + "\n---\n# " + words(30) + "\n```\n" + words(30) +
				"\n```\n<!--\n" + words(30) + "\n-->\n",
		},
		{
			name: "each table cell is its own text",
			page: "| a | b |\n| --- | --- |\n| " + words(20) + " | " + words(26) + " |",
			want: []finding{{line: 3, words: 26, limit: 25}},
		},
		{
			name: "a blank line ends a text",
			page: "The " + strings.Repeat("word ", 15) + "\n\n" + strings.Repeat("word ", 15) + "end.",
		},
		{
			name: "abbreviation does not end a sentence",
			page: "The " + strings.Repeat("word ", 12) + "e.g. " + strings.Repeat("word ", 11) + "end.",
			want: []finding{{line: 1, words: 26, limit: 25}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkPage(tt.page)
			for i := range got {
				got[i].start = ""
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCheckPageNamesTheSentence(t *testing.T) {
	got := checkPage("Short one. " + words(26))
	require.Len(t, got, 1)
	assert.Equal(t, "The word word word word word", got[0].start)
}

func TestCheckDirSkipsUnpublishedPages(t *testing.T) {
	root := t.TempDir()
	long := words(30)

	for _, rel := range []string{"index.md", "superpowers/plan.md", "crds/TEMPLATE.md", "crds/kind.md", "notes.txt"} {
		path := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(long), 0o600))
	}

	findings, err := checkDir(root)
	require.NoError(t, err)

	var files []string
	for _, f := range findings {
		files = append(files, f.file)
	}
	assert.ElementsMatch(t, []string{filepath.Join(root, "crds/kind.md"), filepath.Join(root, "index.md")}, files)
}
