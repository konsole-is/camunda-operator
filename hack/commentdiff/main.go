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
// The docs under api/ become CRD descriptions that users read, so they are
// listed but never flagged.
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
	"os"
	"os/exec"
	"sort"
	"strings"
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
	mergeBase, err := git("merge-base", base, "HEAD")
	if err != nil {
		fail("find the merge base of %s and HEAD: %v", base, err)
	}
	mergeBase = strings.TrimSpace(mergeBase)

	names, err := git("diff", "--name-only", "--diff-filter=AMR", mergeBase, "--", "*.go")
	if err != nil {
		fail("list changed Go files: %v", err)
	}

	var findings []finding
	for file := range strings.FieldsSeq(names) {
		if strings.Contains(file, "zz_generated") {
			continue
		}
		findings = append(findings, compare(mergeBase, file)...)
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].file != findings[j].file {
			return findings[i].file < findings[j].file
		}
		return findings[i].line < findings[j].line
	})

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

// compare reports the comments of file that are new or changed since base.
func compare(base, file string) []finding {
	newSrc, err := os.ReadFile(file)
	if err != nil {
		return nil // deleted in the working tree
	}
	oldSrc, _ := git("show", base+":"+file) // empty for an added file

	return diffFile(file, []byte(oldSrc), newSrc)
}

// diffFile reports every doc comment and every free-standing comment in
// newSrc that is not in oldSrc. An empty oldSrc is a file added since base.
func diffFile(file string, oldSrc, newSrc []byte) []finding {
	newDecls, newFree := scan(file, newSrc)
	oldDecls, oldFree := scan(file, oldSrc)

	userFacing := strings.HasPrefix(file, "api/")
	var out []finding
	renamed := pairRenames(oldDecls, newDecls)
	for key, d := range newDecls {
		old, existed := oldDecls[key]
		if !existed {
			old, existed = renamed[key]
		}
		if existed && old.docText == d.docText {
			continue
		}
		if d.docLen == 0 {
			continue
		}
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
		if userFacing && f.fatal {
			f.fatal = false
			f.text += " (user-facing: judge with writing-operator-docs)"
		}
		out = append(out, f)
	}
	for text, line := range newFree {
		if _, ok := oldFree[text]; ok {
			continue
		}
		out = append(out, finding{file: file, line: line, text: "comment: " + firstLine(text)})
	}
	return out
}

// pairRenames maps each new declaration that has no match at base to the
// base declaration it most likely replaced: the one with the same bare name,
// or else the removed one of the same kind whose doc sat closest to it.
func pairRenames(old, cur map[string]decl) map[string]decl {
	var removed []decl
	for key, d := range old {
		if _, ok := cur[key]; !ok && d.docLen > 0 {
			removed = append(removed, d)
		}
	}
	paired := map[string]decl{}
	used := map[string]bool{}
	for key, d := range cur {
		if _, ok := old[key]; ok {
			continue
		}
		best, dist := decl{}, -1
		for _, r := range removed {
			if used[r.name] || kind(r.name) != kind(d.name) {
				continue
			}
			gap := abs(r.line - d.line)
			if r.short == d.short {
				gap = -1
			}
			if dist == -1 || gap < dist {
				best, dist = r, gap
			}
		}
		if best.name != "" && dist <= maxRenameGap {
			used[best.name] = true
			paired[key] = best
		}
	}
	return paired
}

// maxRenameGap is how far, in lines, a removed doc may sit from an added one
// and still count as the same declaration under a new name.
const maxRenameGap = 60

func kind(key string) string { return strings.SplitN(key, " ", 2)[0] }

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// scan returns the doc comments keyed by declaration, and the text of every
// comment group that is not a doc comment, mapped to its line.
func scan(file string, src []byte) (map[string]decl, map[string]int) {
	decls := map[string]decl{}
	free := map[string]int{}
	if len(bytes.TrimSpace(src)) == 0 {
		return decls, free
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, parser.ParseComments)
	if err != nil {
		return decls, free
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

	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			body := 0
			if d.Body != nil {
				body = fset.Position(d.Body.Rbrace).Line - fset.Position(d.Body.Lbrace).Line - 1
			}
			add(funcKey(d), d.Name.Name, d.Doc, body)
		case *ast.GenDecl:
			add(genKey(d), "", d.Doc, 0)
			for _, s := range d.Specs {
				switch s := s.(type) {
				case *ast.TypeSpec:
					add("type "+s.Name.Name, s.Name.Name, s.Doc, 0)
					if st, ok := s.Type.(*ast.StructType); ok {
						for _, fld := range st.Fields.List {
							for _, n := range fld.Names {
								add("field "+s.Name.Name+"."+n.Name, n.Name, fld.Doc, 0)
							}
						}
					}
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
		if text == "" || strings.HasPrefix(text, "+") {
			continue // kubebuilder markers and build tags
		}
		free[text] = fset.Position(cg.Pos()).Line
	}
	return decls, free
}

func funcKey(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return "func " + d.Name.Name
	}
	var recv bytes.Buffer
	switch t := d.Recv.List[0].Type.(type) {
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			recv.WriteString(id.Name)
		}
	case *ast.Ident:
		recv.WriteString(t.Name)
	}
	return "func (" + recv.String() + ") " + d.Name.Name
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
