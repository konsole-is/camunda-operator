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

// Command callsplit enforces one shape for a call that spans lines. The
// arguments start on the line after the opening paren, and the closing paren
// stands on its own line:
//
//	f(a, b,            f(
//	    c, d)    →         a, b,
//	                       c, d,
//	                   )
//
// A call whose first argument is itself multi-line, for example a func
// literal, is left alone.
//
// Usage: callsplit [-check] [dir ...]
//
// Without -check the tool rewrites the files in place. With -check it prints
// every offending call as file:line and exits 1 when it found any. Directories
// default to ".". Files named zz_generated*.go and the bin and testdata
// directories are skipped.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// edit is one insertion into a source file at a byte offset.
type edit struct {
	offset int
	text   string
}

// offender is one call that the tool reports in check mode.
type offender struct {
	pos token.Position
}

func main() {
	check := flag.Bool("check", false, "report offending calls and exit 1 instead of rewriting")
	flag.Parse()

	roots := flag.Args()
	if len(roots) == 0 {
		roots = []string{"."}
	}

	files := make([]string, 0, len(roots))
	for _, root := range roots {
		found, err := goFiles(root)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		files = append(files, found...)
	}

	total := 0
	for _, path := range files {
		n, err := process(path, *check)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		total += n
	}

	if *check && total > 0 {
		fmt.Fprintf(os.Stderr, "callsplit: %d call(s) split after the first argument; run `make fmt`\n", total)
		os.Exit(1)
	}
}

// goFiles lists the Go files under root that the tool inspects.
func goFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "bin", "testdata", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasPrefix(d.Name(), "zz_generated") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files, err
}

// process inspects one file. In check mode it prints the offenders and
// returns their count. Otherwise it rewrites the file and returns the number
// of rewritten calls.
func process(path string, check bool) (int, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return 0, err
	}

	var edits []edit
	var offenders []offender
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !splitAfterFirstArg(fset, call) {
			return true
		}
		offenders = append(offenders, offender{pos: fset.Position(call.Lparen)})
		edits = append(edits, rewrite(fset, src, call)...)
		return true
	})

	if check {
		for _, o := range offenders {
			fmt.Printf("%s: call split after its first argument\n", o.pos)
		}
		return len(offenders), nil
	}

	if len(edits) == 0 {
		return 0, nil
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].offset > edits[j].offset })
	out := src
	for _, e := range edits {
		out = append(out[:e.offset:e.offset], append([]byte(e.text), out[e.offset:]...)...)
	}
	out = bytes.ReplaceAll(out, []byte(",\n\n"), []byte(",\n"))

	return len(offenders), os.WriteFile(path, out, 0o644)
}

// splitAfterFirstArg reports whether the first argument of call sits on the
// paren line, fits on that line, and a later argument starts on another line.
func splitAfterFirstArg(fset *token.FileSet, call *ast.CallExpr) bool {
	if len(call.Args) == 0 {
		return false
	}

	lparen := fset.Position(call.Lparen).Line
	first := call.Args[0]
	firstStart, firstEnd := fset.Position(first.Pos()).Line, fset.Position(first.End()).Line
	lastStart := fset.Position(call.Args[len(call.Args)-1].Pos()).Line

	return firstStart == lparen && firstEnd == firstStart && lastStart != firstStart
}

// rewrite returns the insertions that move the arguments of call onto their
// own lines and close the paren on its own line.
func rewrite(fset *token.FileSet, src []byte, call *ast.CallExpr) []edit {
	tf := fset.File(call.Lparen)
	lparen := tf.Offset(call.Lparen)
	rparen := tf.Offset(call.Rparen)
	lastEnd := tf.Offset(call.Args[len(call.Args)-1].End())

	lineStart := lparen
	for lineStart > 0 && src[lineStart-1] != '\n' {
		lineStart--
	}
	indent := ""
	for i := lineStart; i < len(src) && (src[i] == '\t' || src[i] == ' '); i++ {
		indent += string(src[i])
	}

	edits := []edit{{offset: lparen + 1, text: "\n" + indent + "\t"}}

	between := string(src[lastEnd:rparen])
	switch {
	case strings.Contains(between, "\n"):
		// The paren already closes on its own line.
	case strings.TrimSpace(between) == ",":
		edits = append(edits, edit{offset: rparen, text: "\n" + indent})
	default:
		edits = append(edits, edit{offset: rparen, text: ",\n" + indent})
	}

	return edits
}
