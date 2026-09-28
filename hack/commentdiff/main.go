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

// Command commentdiff lists the Go comments that changed between a base ref
// and the working tree, and flags the two shapes that review rounds grow: a
// doc comment that got longer than it was at the base, and a doc comment of
// three or more lines that is longer than the body it documents. A flag is
// not a verdict. It exits 1 so that each flagged comment gets evaluated.
// The docs of schema types and fields under api/ become CRD descriptions
// that users read, so they are listed but never flagged.
//
// Usage: go run ./hack/commentdiff [base]
//
// base defaults to origin/main. The comparison starts at the merge base of
// base and HEAD, so pass the head of the previous review round to see only
// the comments of this round.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"math"
	"os"
	"os/exec"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// minFlaggedDoc is the doc length from which a doc longer than its body is
// flagged. A one- or two-line doc over a one-line body is the normal shape.
const minFlaggedDoc = 3

type decl struct {
	name    string
	short   string // the bare name, which a rename or a new receiver keeps
	line    int
	docLen  int
	docText string
	bodyLen int
}

type change struct {
	path     string // empty when the working tree does not have the file
	basePath string // empty when base does not have the file
}

type comment struct {
	text string
	line int
}

type finding struct {
	file  string
	line  int
	text  string
	fatal bool
}

func main() {
	base := "origin/main"
	if len(os.Args) > 1 {
		base = os.Args[1]
	}
	findings, err := run(base)
	if err != nil {
		fail("%v", err)
	}

	fatal := 0
	for _, f := range findings {
		marker := "  "
		if f.fatal {
			marker = "✗ "
			fatal++
		}
		fmt.Printf("%s%s:%d: %s\n", marker, f.file, f.line, f.text)
	}
	fmt.Printf("\n%d changed comments, %d flagged (✗) for evaluation\n", len(findings), fatal)
	if fatal > 0 {
		os.Exit(1)
	}
}

func run(base string) ([]finding, error) {
	mergeBase, err := git("merge-base", base, "HEAD")
	if err != nil {
		return nil, fmt.Errorf("find the merge base of %s and HEAD: %w", base, err)
	}
	mergeBase = strings.TrimSpace(mergeBase)

	changes, err := changedFiles(mergeBase)
	if err != nil {
		return nil, err
	}

	// A file that git sees as deleted can be the base of an added file that
	// changed too much for its rename detection.
	var deletedSrcs [][]byte
	for _, c := range changes {
		if c.path != "" || strings.Contains(c.basePath, "zz_generated") {
			continue
		}
		src, err := git("show", mergeBase+":"+c.basePath)
		if err != nil {
			return nil, fmt.Errorf("read %s at %s: %w", c.basePath, mergeBase, err)
		}
		deletedSrcs = append(deletedSrcs, []byte(src))
	}

	var findings []finding
	for _, c := range changes {
		if c.path == "" || strings.Contains(c.path, "zz_generated") {
			continue
		}
		var candidates [][]byte
		if c.basePath == "" {
			candidates = deletedSrcs
		}
		found, err := compare(mergeBase, c, candidates)
		if err != nil {
			return nil, err
		}
		findings = append(findings, found...)
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].file != findings[j].file {
			return findings[i].file < findings[j].file
		}
		return findings[i].line < findings[j].line
	})
	return findings, nil
}

// changedFiles lists the Go files that differ from base in the working tree, untracked files included.
func changedFiles(base string) ([]change, error) {
	diff, err := git("diff", "-z", "--name-status", "-M", "--diff-filter=AMRD", base, "--", "*.go")
	if err != nil {
		return nil, fmt.Errorf("list changed Go files: %w", err)
	}

	var changes []change
	fields := strings.Split(strings.TrimSuffix(diff, "\x00"), "\x00")
	for i := 0; i+1 < len(fields); i += 2 {
		switch status := fields[i]; {
		case strings.HasPrefix(status, "R") && i+2 < len(fields):
			changes = append(changes, change{path: fields[i+2], basePath: fields[i+1]})
			i++
		case strings.HasPrefix(status, "A"):
			changes = append(changes, change{path: fields[i+1]})
		case strings.HasPrefix(status, "D"):
			changes = append(changes, change{basePath: fields[i+1]})
		default:
			changes = append(changes, change{path: fields[i+1], basePath: fields[i+1]})
		}
	}

	untracked, err := git("ls-files", "-z", "--others", "--exclude-standard", "--", "*.go")
	if err != nil {
		return nil, fmt.Errorf("list untracked Go files: %w", err)
	}
	for path := range strings.SplitSeq(untracked, "\x00") {
		if path != "" {
			changes = append(changes, change{path: path})
		}
	}
	return changes, nil
}

func compare(base string, c change, deletedSrcs [][]byte) ([]finding, error) {
	newSrc, err := os.ReadFile(c.path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", c.path, err)
	}

	var oldSrc string
	if c.basePath != "" {
		if oldSrc, err = git("show", base+":"+c.basePath); err != nil {
			return nil, fmt.Errorf("read %s at %s: %w", c.basePath, base, err)
		}
	}
	return diffFile(c.path, []byte(oldSrc), newSrc, deletedSrcs...)
}

// diffFile reports every doc comment and every free-standing comment in
// newSrc that is not in oldSrc. An empty oldSrc is a file added since base.
func diffFile(file string, oldSrc, newSrc []byte, deletedSrcs ...[]byte) ([]finding, error) {
	newDecls, newFree, err := scan(file, newSrc)
	if err != nil {
		return nil, err
	}
	oldDecls, oldFree, err := scan(file, oldSrc)
	if err != nil {
		return nil, fmt.Errorf("at the base: %w", err)
	}
	for _, src := range deletedSrcs {
		decls, free, err := scan(file, src)
		if err != nil {
			return nil, fmt.Errorf("in a deleted file: %w", err)
		}
		maps.Copy(oldDecls, decls)
		oldFree = append(oldFree, free...)
	}

	schemaFile := strings.HasPrefix(file, "api/") && !strings.HasSuffix(file, "_test.go")
	var out []finding
	reported := map[int]bool{} // one doc can cover several names
	renamed := pairRenames(oldDecls, newDecls)
	for _, key := range sortedKeys(newDecls) {
		d := newDecls[key]
		old, existed := oldDecls[key]
		if !existed {
			old, existed = renamed[key]
		}
		if existed && old.docText == d.docText || d.docLen == 0 || reported[d.line] {
			continue
		}
		reported[d.line] = true
		f := finding{file: file, line: d.line}
		switch {
		case existed && d.docLen > old.docLen:
			f.text = fmt.Sprintf("doc of %s GREW from %d to %d lines", d.name, old.docLen, d.docLen)
			if old.name != d.name {
				f.text += " (was " + old.name + ")"
			}
			f.fatal = true
		case existed:
			f.text = fmt.Sprintf("doc of %s changed (%d lines, was %d)", d.name, d.docLen, old.docLen)
		default:
			f.text = fmt.Sprintf("doc of %s added (%d lines)", d.name, d.docLen)
		}
		if d.bodyLen > 0 && d.docLen >= minFlaggedDoc && d.docLen > d.bodyLen {
			f.text += fmt.Sprintf(", LONGER than its %d-line body", d.bodyLen)
			f.fatal = true
		}
		if schemaFile && f.fatal && crdDescription(newDecls, d.name) {
			f.fatal = false
			f.text += " (user-facing: judge with writing-operator-docs)"
		}
		out = append(out, f)
	}
	for _, c := range unmatched(oldFree, newFree) {
		out = append(out, finding{file: file, line: c.line, text: "comment: " + firstLine(c.text)})
	}
	return out, nil
}

// unmatched returns the comments of cur that a longest common subsequence of texts with old leaves out.
func unmatched(old, cur []comment) []comment {
	lcs := make([][]int, len(old)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(cur)+1)
	}
	for i := len(old) - 1; i >= 0; i-- {
		for j := len(cur) - 1; j >= 0; j-- {
			if old[i].text == cur[j].text {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}

	var out []comment
	i, j := 0, 0
	for j < len(cur) {
		switch {
		case i < len(old) && old[i].text == cur[j].text:
			i++
			j++
		case i < len(old) && lcs[i+1][j] > lcs[i][j+1]:
			i++
		default:
			out = append(out, cur[j])
			j++
		}
	}
	return out
}

// pairRenames maps each new declaration that has no match at base to the
// base declaration it most likely replaced: the one with the same bare name,
// or else the removed one of the same kind whose doc sat closest to it.
func pairRenames(old, cur map[string]decl) map[string]decl {
	var removed []decl
	for _, key := range sortedKeys(old) {
		if d := old[key]; d.docLen > 0 && !hasKey(cur, key) {
			removed = append(removed, d)
		}
	}
	paired := map[string]decl{}
	used := map[string]bool{}
	for _, key := range sortedKeys(cur) {
		if hasKey(old, key) {
			continue
		}
		d := cur[key]
		best, dist := decl{}, math.MaxInt
		for _, r := range removed {
			if used[r.name] || kind(r.name) != kind(d.name) {
				continue
			}
			gap := abs(r.line - d.line)
			if r.short == d.short {
				gap = -1
			}
			if gap < dist {
				best, dist = r, gap
			}
		}
		if best.name != "" {
			used[best.name] = true
			paired[key] = best
		}
	}
	return paired
}

func crdDescription(decls map[string]decl, key string) bool {
	kind, name, _ := strings.Cut(strings.TrimPrefix(key, "group "), " ")
	if kind != "type" && kind != "field" {
		return false
	}
	owner, _, _ := strings.Cut(name, ".")
	if !ast.IsExported(owner) {
		return false
	}
	ownerDoc := decls["type "+owner].docText + decls["group type "+owner].docText
	return !strings.Contains(ownerDoc, "+kubebuilder:object:generate=false")
}

func sortedKeys(m map[string]decl) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func hasKey(m map[string]decl, key string) bool {
	_, ok := m[key]
	return ok
}

func kind(key string) string { return strings.SplitN(key, " ", 2)[0] }

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// scan returns the doc comments keyed by declaration, and every comment group
// that is not a doc comment, in source order.
func scan(file string, src []byte) (map[string]decl, []comment, error) {
	decls := map[string]decl{}
	var free []comment
	if len(bytes.TrimSpace(src)) == 0 {
		return decls, free, nil
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}

	docs := map[*ast.CommentGroup]bool{}
	add := func(key, short string, doc *ast.CommentGroup, bodyLen int) {
		if doc == nil {
			return
		}
		docs[doc] = true
		decls[key] = decl{
			name:    key,
			short:   short,
			line:    fset.Position(doc.Pos()).Line,
			docLen:  lines(fset, doc),
			docText: doc.Text(),
			bodyLen: bodyLen,
		}
	}
	var addMembers func(owner string, t ast.Expr)
	addMembers = func(owner string, t ast.Expr) {
		for _, fld := range members(t) {
			names := make([]string, 0, max(1, len(fld.Names)))
			for _, n := range fld.Names {
				names = append(names, n.Name)
			}
			if len(names) == 0 {
				names = []string{typeName(fld.Type)}
			}
			for _, n := range names {
				if n == "" {
					continue
				}
				add("field "+owner+"."+n, n, fld.Doc, 0)
				addMembers(owner+"."+n, fld.Type)
			}
		}
	}

	add("package "+f.Name.Name, "", f.Doc, 0)
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			body := 0
			if d.Body != nil {
				body = max(1, fset.Position(d.Body.Rbrace).Line-fset.Position(d.Body.Lbrace).Line-1)
			}
			add(funcKey(d), d.Name.Name, d.Doc, body)
		case *ast.GenDecl:
			add(genKey(d), "", d.Doc, 0)
			for _, s := range d.Specs {
				switch s := s.(type) {
				case *ast.TypeSpec:
					add("type "+s.Name.Name, s.Name.Name, s.Doc, 0)
					addMembers(s.Name.Name, s.Type)
				case *ast.ValueSpec:
					for _, n := range s.Names {
						add("value "+n.Name, n.Name, s.Doc, 0)
					}
				}
			}
		}
	}
	for _, cg := range f.Comments {
		if docs[cg] || fset.Position(cg.Pos()).Line == 1 {
			continue
		}
		text := strings.TrimSpace(cg.Text())
		if text == "" || isMarker(text) {
			continue
		}
		free = append(free, comment{text: text, line: fset.Position(cg.Pos()).Line})
	}
	return decls, free, nil
}

// isMarker reports whether every line of text is a marker such as +optional or +kubebuilder:validation:Required.
func isMarker(text string) bool {
	for line := range strings.Lines(text) {
		r, _ := utf8.DecodeRuneInString(strings.TrimPrefix(line, "+"))
		if !strings.HasPrefix(line, "+") || !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func members(t ast.Expr) []*ast.Field {
	switch t := t.(type) {
	case *ast.StructType:
		return t.Fields.List
	case *ast.InterfaceType:
		return t.Methods.List
	case *ast.StarExpr:
		return members(t.X)
	case *ast.ArrayType:
		return members(t.Elt)
	}
	return nil
}

// typeName returns the bare name of a named type, or "" for a type that has no name.
func typeName(t ast.Expr) string {
	switch t := t.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.StarExpr:
		return typeName(t.X)
	case *ast.IndexExpr:
		return typeName(t.X)
	case *ast.IndexListExpr:
		return typeName(t.X)
	}
	return ""
}

func funcKey(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return "func " + d.Name.Name
	}
	return "func (" + typeName(d.Recv.List[0].Type) + ") " + d.Name.Name
}

func genKey(d *ast.GenDecl) string {
	if len(d.Specs) == 0 {
		return d.Tok.String()
	}
	switch s := d.Specs[0].(type) {
	case *ast.TypeSpec:
		return "group type " + s.Name.Name
	case *ast.ValueSpec:
		return "group " + d.Tok.String() + " " + s.Names[0].Name
	}
	return d.Tok.String()
}

func lines(fset *token.FileSet, cg *ast.CommentGroup) int {
	return fset.Position(cg.End()).Line - fset.Position(cg.Pos()).Line + 1
}

func firstLine(s string) string {
	if first, _, found := strings.Cut(s, "\n"); found {
		return first + " …"
	}
	return s
}

func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	return string(out), err
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "commentdiff: "+format+"\n", args...)
	os.Exit(2)
}
