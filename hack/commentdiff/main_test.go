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
	"os/exec"
	"strings"
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

	got, err := diffFile("api/v1/spec_types.go", old, cur)

	require.NoError(t, err)
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
			name: "one-line body still counts as a body",
			old:  "",
			cur: `// f does a thing.
// It does it twice.
// Then once more.
func f() { g() }
`,
			wantFatal: true,
			wantText:  "LONGER than its 1-line body",
		},
		{
			name: "interface method doc grew",
			old: `type I interface {
	// M runs.
	M()
}
`,
			cur: `type I interface {
	// M runs.
	// It runs twice.
	M()
}
`,
			wantFatal: true,
			wantText:  "doc of field I.M GREW from 1 to 2",
		},
		{
			name: "changed comment that duplicates another is listed",
			old: `func f() {
	// a
	g()
	// b
	g()
}
`,
			cur: `func f() {
	// b
	g()
	// b
	g()
}
`,
			wantText: "comment: b",
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
			name: "comment that starts with a number after a plus",
			old: `func f() {
	g()
}
`,
			cur: `func f() {
	// +1 is the minimum.
	g()
}
`,
			wantText: "comment: +1 is the minimum.",
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
			got, err := diffFile("p.go", old, src(tt.cur))
			require.NoError(t, err)
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

func TestDiffFileFlagsGrownPackageDoc(t *testing.T) {
	old := []byte("// Package p does a thing.\npackage p\n")
	cur := []byte("// Package p does a thing.\n// It does it twice.\npackage p\n")

	got, err := diffFile("p.go", old, cur)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.True(t, got[0].fatal)
	assert.Contains(t, got[0].text, "doc of package p GREW from 1 to 2")
}

func TestDiffFileFailsWhenTheSourceDoesNotParse(t *testing.T) {
	_, err := diffFile("p.go", nil, src("func f( {\n"))

	assert.Error(t, err)
}

func TestPairRenamesBreaksTiesTheSameWayEachTime(t *testing.T) {
	old := map[string]decl{
		"func a": {name: "func a", short: "a", line: 10, docLen: 1},
		"func b": {name: "func b", short: "b", line: 30, docLen: 1},
	}
	cur := map[string]decl{"func c": {name: "func c", short: "c", line: 20, docLen: 1}}

	for range 50 {
		assert.Equal(t, "func a", pairRenames(old, cur)["func c"].name)
	}
}

func TestRunComparesARenamedFileWithItsBasePath(t *testing.T) {
	newRepo(t)
	writeFile(t, "a.go", "package p\n\n// f returns one.\n// It never fails.\nfunc f() int { return 1 }\n")
	gitRun(t, "add", "a.go")
	gitRun(t, "commit", "-q", "-m", "base")
	gitRun(t, "mv", "a.go", "b.go")

	got, err := run("HEAD")

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestRunScansUntrackedFiles(t *testing.T) {
	newRepo(t)
	gitRun(t, "commit", "-q", "--allow-empty", "-m", "base")
	writeFile(t, "c.go", "package p\n\n// f returns one.\nfunc f() int { return 1 }\n")

	got, err := run("HEAD")

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "c.go", got[0].file)
	assert.Contains(t, got[0].text, "doc of func f added")
}

func newRepo(t *testing.T) {
	t.Chdir(t.TempDir())
	gitRun(t, "init", "-q")
}

func writeFile(t *testing.T, name, content string) {
	require.NoError(t, os.WriteFile(name, []byte(content), 0o600))
}

func gitRun(t *testing.T, args ...string) {
	t.Helper()
	cfg := []string{
		"-c", "user.name=test",
		"-c", "user.email=test@example.com",
		"-c", "commit.gpgsign=false",
		"-c", "core.hooksPath=/dev/null",
	}
	out, err := exec.Command("git", append(cfg, args...)...).CombinedOutput()
	require.NoError(t, err, string(out))
}

func TestDiffFileReportsTheLineOfAChangedDuplicateComment(t *testing.T) {
	tests := []struct {
		name     string
		old      string
		wantLine int
	}{
		{name: "first of two changed", old: "func f() {\n\t// a\n\tg()\n\t// b\n\tg()\n}\n", wantLine: 4},
		{name: "second of two changed", old: "func f() {\n\t// b\n\tg()\n\t// a\n\tg()\n}\n", wantLine: 6},
	}
	cur := src("func f() {\n\t// b\n\tg()\n\t// b\n\tg()\n}\n")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := diffFile("p.go", src(tt.old), cur)

			require.NoError(t, err)
			require.Len(t, got, 1)
			assert.Equal(t, tt.wantLine, got[0].line)
		})
	}
}

func TestDiffFileFlagsGrownMemberDocs(t *testing.T) {
	tests := []struct {
		name     string
		old, cur string
		wantText string
	}{
		{
			name:     "embedded field",
			old:      "type T struct {\n\t// Base is shared.\n\t*Base\n}\n",
			cur:      "type T struct {\n\t// Base is shared.\n\t// It is never nil.\n\t*Base\n}\n",
			wantText: "doc of field T.Base GREW from 1 to 2",
		},
		{
			name: "field of a nested struct",
			old:  "type T struct {\n\tInner struct {\n\t\t// X is the count.\n\t\tX int\n\t}\n}\n",
			cur: "type T struct {\n\tInner struct {\n\t\t// X is the count.\n" +
				"\t\t// It is never negative.\n\t\tX int\n\t}\n}\n",
			wantText: "doc of field T.Inner.X GREW from 1 to 2",
		},
		{
			name: "method of one of two generic types",
			old: "// M runs.\nfunc (b *B[T]) M() {\n\tg()\n}\n\n" +
				"// M runs.\nfunc (a A[K, V]) M() {\n\tg()\n}\n",
			cur: "// M runs.\n// It runs twice.\nfunc (b *B[T]) M() {\n\tg()\n}\n\n" +
				"// M runs.\nfunc (a A[K, V]) M() {\n\tg()\n}\n",
			wantText: "doc of func (B) M GREW from 1 to 2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := diffFile("p.go", src(tt.old), src(tt.cur))

			require.NoError(t, err)
			require.Len(t, got, 1)
			assert.True(t, got[0].fatal)
			assert.Contains(t, got[0].text, tt.wantText)
		})
	}
}

func TestDiffFileFlagsGoDocsUnderAPI(t *testing.T) {
	tests := []struct {
		name, file string
		old, cur   string
	}{
		{
			name: "method in a types file",
			file: "api/v1/spec_types.go",
			old:  "// M runs.\nfunc (s *Spec) M() {\n\tg()\n}\n",
			cur:  "// M runs.\n// It runs twice.\nfunc (s *Spec) M() {\n\tg()\n}\n",
		},
		{
			name: "type that controller-gen does not generate",
			file: "api/v1/groupversion_info.go",
			old:  "// Builder builds.\n// +kubebuilder:object:generate=false\ntype Builder struct{}\n",
			cur:  "// Builder builds.\n// It never fails.\n// +kubebuilder:object:generate=false\ntype Builder struct{}\n",
		},
		{
			name: "type in a test file",
			file: "api/v1/spec_types_test.go",
			old:  "// fixture is a spec.\ntype fixture struct{}\n",
			cur:  "// fixture is a spec.\n// It is valid.\ntype fixture struct{}\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := diffFile(tt.file, src(tt.old), src(tt.cur))

			require.NoError(t, err)
			require.Len(t, got, 1)
			assert.True(t, got[0].fatal)
		})
	}
}

func TestDiffFilePairsARenameThatMovedFar(t *testing.T) {
	old := src("// notReady describes Ready.\nfunc notReady() {\n\tg()\n}\n")
	cur := src(strings.Repeat("var _ = 0\n", 100) +
		"// cannotStart describes why.\n// Both kinds need a binding.\nfunc cannotStart() {\n\tg()\n}\n")

	got, err := diffFile("p.go", old, cur)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.True(t, got[0].fatal)
	assert.Contains(t, got[0].text, "GREW from 1 to 2 lines (was func notReady)")
}

func TestDiffFileReportsASharedDocOnce(t *testing.T) {
	old := src("type T struct {\n\t// A and B are counts.\n\tA, B int\n}\n")
	cur := src("type T struct {\n\t// A and B are counts.\n\t// They are never negative.\n\tA, B int\n}\n")

	got, err := diffFile("p.go", old, cur)

	require.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestRunPairsARenameBelowTheGitSimilarityThreshold(t *testing.T) {
	newRepo(t)
	writeFile(t, "a.go", "package p\n\n// notReady describes Ready.\nfunc notReady() string {\n\treturn \"ready\"\n}\n")
	gitRun(t, "add", "a.go")
	gitRun(t, "commit", "-q", "-m", "base")
	require.NoError(t, os.Remove("a.go"))
	writeFile(t, "b.go", "package p\n\nimport \"fmt\"\n\n"+
		"// cannotStart describes why.\n// Both kinds need a binding.\nfunc cannotStart(kind string) string {\n"+
		"\tif kind == \"\" {\n\t\treturn fmt.Sprint(\"no kind\")\n\t}\n\treturn fmt.Sprint(kind, \" cannot start\")\n}\n")

	got, err := run("HEAD")

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "b.go", got[0].file)
	assert.Contains(t, got[0].text, "GREW from 1 to 2 lines (was func notReady)")
}
