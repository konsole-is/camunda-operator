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

// Command apireference corrects the API reference that crd-ref-docs writes,
// so that the docs site shows it as the GoDoc says. It is a step of
// `make api-docs`, not a hand edit.
//
// It escapes each angle bracket outside a code span, so that a name pattern
// such as <name>-camunda shows on the page and the browser does not read it
// as an HTML tag. GoDoc keeps the plain brackets. It writes the underlying
// type of a named map type, such as map[string]ParameterPolicy, in the form
// that crd-ref-docs uses for a map field, with a link to the value type. Then
// it fails when a link of the page points at an anchor that no heading has.
//
// Usage: apireference [page]
//
// page defaults to docs/crds/api-reference.md.
package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// lineBreak is the only HTML that crd-ref-docs writes into the page: it
// stands for a new line of the GoDoc in a table cell.
const lineBreak = "<br />"

func main() {
	path := "docs/crds/api-reference.md"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	if err := run(path); err != nil {
		fmt.Fprintln(os.Stderr, "apireference:", err)
		os.Exit(1)
	}
}

func run(path string) error {
	page, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	out, err := process(string(page))
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	return os.WriteFile(path, []byte(out), 0o600)
}

// process returns the corrected page. It fails when a link points at an
// anchor that no heading of the page has. A page that process returned comes
// back unchanged.
func process(page string) (string, error) {
	page = rewriteMapLinks(page)
	page = escapeAngles(page)
	if err := checkAnchors(page); err != nil {
		return "", err
	}

	return page, nil
}

// mapLink matches the link that crd-ref-docs writes for the underlying type
// of a named map type. It takes the anchor from the whole map type, which no
// heading has.
var mapLink = regexp.MustCompile(`\[map\[(\w+)\](\w+)\]\(#[^)]*\)`)

// rewriteMapLinks writes each map link as the type of a map field:
// object (keys:string, values:[ParameterPolicy](#parameterpolicy)).
func rewriteMapLinks(page string) string {
	return mapLink.ReplaceAllStringFunc(page, func(link string) string {
		m := mapLink.FindStringSubmatch(link)
		return fmt.Sprintf("object (keys:%s, values:[%s](#%s))", m[1], m[2], strings.ToLower(m[2]))
	})
}

// escapeAngles writes each < and > outside a code span and outside a line
// break as an HTML entity.
func escapeAngles(page string) string {
	lines := strings.Split(page, "\n")
	for i, line := range lines {
		lines[i] = escapeLine(line)
	}

	return strings.Join(lines, "\n")
}

// escapeLine treats each line on its own, because crd-ref-docs writes each
// table cell on one line. It escapes the angle brackets of a code span that
// crosses a line.
func escapeLine(line string) string {
	var b strings.Builder
	for line != "" {
		i := strings.IndexAny(line, "`<>")
		if i < 0 {
			b.WriteString(line)
			break
		}
		b.WriteString(line[:i])
		line = line[i:]

		switch {
		case line[0] == '`':
			n := codeSpanLen(line)
			b.WriteString(line[:n])
			line = line[n:]
		case strings.HasPrefix(line, lineBreak):
			b.WriteString(lineBreak)
			line = line[len(lineBreak):]
		case line[0] == '<':
			b.WriteString("&lt;")
			line = line[1:]
		default:
			b.WriteString("&gt;")
			line = line[1:]
		}
	}

	return b.String()
}

// codeSpanLen returns the length of the code span at the start of s, which
// starts with a backtick. As in CommonMark, a span ends at the next run of
// the same number of backticks. A run with no such end is plain text, and
// codeSpanLen returns the length of the run.
func codeSpanLen(s string) int {
	open := backtickRun(s)
	for i := open; i < len(s); {
		if s[i] != '`' {
			i++
			continue
		}
		run := backtickRun(s[i:])
		if run == open {
			return i + run
		}
		i += run
	}

	return open
}

func backtickRun(s string) int {
	n := 0
	for n < len(s) && s[n] == '`' {
		n++
	}

	return n
}

var (
	heading   = regexp.MustCompile(`(?m)^#{1,6} (.+)$`)
	localLink = regexp.MustCompile(`\]\(#([^)]*)\)`)
	nonWord   = regexp.MustCompile(`[^\w\s-]`)
	separator = regexp.MustCompile(`[-\s]+`)
)

// checkAnchors fails when a link of the page points at an anchor that no
// heading has. It makes the anchors as the toc extension of MkDocs does: the
// slug of the heading text, with _1, _2 and so on after a repeated slug.
func checkAnchors(page string) error {
	anchors := map[string]bool{}
	seen := map[string]int{}
	for _, m := range heading.FindAllStringSubmatch(page, -1) {
		slug := slugify(m[1])
		if n := seen[slug]; n > 0 {
			anchors[fmt.Sprintf("%s_%d", slug, n)] = true
		} else {
			anchors[slug] = true
		}
		seen[slug]++
	}

	var missing []string
	for _, m := range localLink.FindAllStringSubmatch(page, -1) {
		if !anchors[m[1]] {
			missing = append(missing, "#"+m[1])
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("links point at anchors that no heading has: %s", strings.Join(missing, ", "))
	}

	return nil
}

// slugify is the default slugify of Python-Markdown for ASCII text.
func slugify(text string) string {
	slug := strings.ToLower(strings.TrimSpace(nonWord.ReplaceAllString(text, "")))
	return separator.ReplaceAllString(slug, "-")
}
