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
			name: "a comment inside a line is skipped",
			page: "The " + strings.Repeat("word ", 20) + "<!-- " + words(10) + " --> end.",
		},
		{
			name: "a comment that opens inside a line is skipped to its end",
			page: "The " + strings.Repeat("word ", 20) + "<!-- " + words(10) + "\n" + words(10) + " -->",
		},
		{
			name: "a blockquote is read without its markers",
			page: "> " + words(26),
			want: []finding{{line: 1, words: 26, limit: 25}},
		},
		{
			name: "an empty quote line ends a text",
			page: "> The " + strings.Repeat("word ", 15) + "\n>\n> " + strings.Repeat("word ", 15) + "end.",
		},
		{
			name: "a line longer than a scanner buffer is read",
			page: strings.Repeat("Short one. ", 200000) + words(26),
			want: []finding{{line: 1, words: 26, limit: 25}},
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
			name: "a semicolon is reported",
			page: "The pod starts; the job ends.",
			want: []finding{{line: 1, banned: ";"}},
		},
		{
			name: "each modal is reported at its line",
			page: "The pod would start.\nThe job could end. It May fail.",
			want: []finding{{line: 1, banned: "would"}, {line: 2, banned: "could"}, {line: 2, banned: "may"}},
		},
		{
			name: "a contracted modal is reported",
			page: "The pod couldn't start. The job wouldn’t end.",
			want: []finding{{line: 1, banned: "couldn't"}, {line: 1, banned: "wouldn’t"}},
		},
		{
			name: "a modal inside a longer word is not reported",
			page: "The pod shoulder mayhem couldron ends.",
		},
		{
			name: "a semicolon or a modal in code is not reported",
			page: "The `a; b` value and `may` flag end.\n\n```\nx; it would, could, or may fail\n```\n",
		},
		{
			name: "a semicolon in a link target is not reported",
			page: "See [the page](https://example.com/a;b) now.",
		},
		{
			name: "a modal in a link target with parentheses is not reported",
			page: "See [the page](https://example.com/(a)/may) now.",
		},
		{
			name: "a semicolon or a modal in an autolink or a bare URL is not reported",
			page: "See <https://example.com/a;b> and https://example.com/may-release/x;y now.",
		},
		{
			name: "a semicolon right after a bare URL is reported",
			page: "See https://example.com/a; the pod starts.",
			want: []finding{{line: 1, banned: ";"}},
		},
		{
			name: "a line that starts with an autolink is read",
			page: "<https://example.com> would fail.",
			want: []finding{{line: 1, banned: "would"}},
		},
		{
			name: "a modal in an email autolink is not reported",
			page: "Write to <may@example.com> now.",
		},
		{
			name: "a period after a bare URL ends the sentence",
			page: "The " + strings.Repeat("word ", 22) + "https://example.com/a. The pod starts now.",
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
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

func TestCheckPageNamesTheSentence(t *testing.T) {
	got := checkPage("Short one. " + words(26))
	require.Len(t, got, 1)
	assert.Equal(t, "The word word word word word", got[0].start)
}

func TestCheckDirSkipsUnpublishedAndGeneratedPages(t *testing.T) {
	root := t.TempDir()
	long := words(30)

	pages := []string{
		"index.md",
		"superpowers/plan.md",
		"crds/TEMPLATE.md",
		"crds/api-reference.md",
		"crds/kind.md",
		"notes.txt",
	}
	for _, rel := range pages {
		path := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(long), 0o600))
	}

	findings, err := checkDir(root)
	require.NoError(t, err)

	files := make([]string, 0, len(findings))
	for _, f := range findings {
		files = append(files, f.file)
	}
	assert.ElementsMatch(t, []string{filepath.Join(root, "crds/kind.md"), filepath.Join(root, "index.md")}, files)
}
