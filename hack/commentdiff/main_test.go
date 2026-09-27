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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func src(body string) []byte { return []byte("package p\n\n" + body) }

func TestDiffFileListsUserFacingGrowthWithoutFlag(t *testing.T) {
	old := src("type Spec struct {\n\t// Replicas is the count.\n\tReplicas int\n}\n")
	cur := src(`type Spec struct {
	// Replicas is the count.
	// When unset, the default applies.
	Replicas int
}
`)

	got := diffFile("api/v1/spec_types.go", old, cur)

	require.Len(t, got, 1)
	assert.False(t, got[0].fatal)
	assert.Contains(t, got[0].text, "GREW from 1 to 2 lines (user-facing")
}

func TestDiffFile(t *testing.T) {
	tests := []struct {
		name      string
		old, cur  string
		wantFatal bool
		wantText  string // a substring of the one finding; "" means no finding
	}{
		{
			name: "unchanged doc",
			old: `// f returns one.
func f() int { return 1 }
`,
			cur: `// f returns one.
func f() int { return 1 }
`,
			wantText: "",
		},
		{
			name: "doc grew on the same signature",
			old: `// f returns one.
func f() int {
	return 1
}
`,
			cur: `// f returns one.
// It returns one because both kinds need it.
func f() int {
	return 1
}
`,
			wantFatal: true,
			wantText:  "GREW from 1 to 2",
		},
		{
			name: "rename keeps the base doc length",
			old: `// notReady describes Ready.
// It returns empty when Ready.
func notReady(c int) string {
	return ""
}
`,
			cur: `// cannotStart describes why.
// Both kinds need a binding.
// RDBMS also needs Ready.
// ES needs a repository instead.
// So a degraded cluster is backed up.
func cannotStart(c int, kind string) string {
	return ""
}
`,
			wantFatal: true,
			wantText:  "GREW from 2 to 5 lines (was func notReady)",
		},
		{
			name: "a new result does not excuse a longer doc",
			old: `// f returns the note.
func f(c int) string {
	return ""
}
`,
			cur: `// f returns the note.
// An error means the check failed.
func f(c int) (string, error) {
	return "", nil
}
`,
			wantFatal: true,
			wantText:  "GREW from 1 to 2",
		},
		{
			name: "a new parameter does not excuse a longer doc",
			old: `// f returns the note.
func f(c int) string {
	return ""
}
`,
			cur: `// f returns the note.
// It reads the cluster.
func f(ctx context.Context, c int) string {
	return ""
}
`,
			wantFatal: true,
			wantText:  "GREW from 1 to 2",
		},
		{
			name: "added doc longer than its body",
			old:  "",
			cur: `// f does a thing.
// It does it twice.
// Then once more.
func f() {
	g()
}
`,
			wantFatal: true,
			wantText:  "LONGER than its 1-line body",
		},
		{
			name: "two-line doc over a one-line body is the normal shape",
			old:  "",
			cur: `// f calls g.
// It never fails.
func f() {
	g()
}
`,
			wantText: "added (2 lines)",
		},
		{
			name: "new inline comment is listed, not flagged",
			old: `func f() {
	g()
}
`,
			cur: `func f() {
	// g must run first.
	g()
}
`,
			wantText: "comment: g must run first.",
		},
		{
			name: "kubebuilder marker is not a comment",
			old: `type T struct{}
`,
			cur: `// +kubebuilder:object:root=true

type T struct{}
`,
			wantText: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var old []byte
			if tt.old != "" {
				old = src(tt.old)
			}
			got := diffFile("p.go", old, src(tt.cur))
			if tt.wantText == "" {
				assert.Empty(t, got)
				return
			}
			require.Len(t, got, 1)
			assert.Contains(t, got[0].text, tt.wantText)
			assert.Equal(t, tt.wantFatal, got[0].fatal)
		})
	}
}
