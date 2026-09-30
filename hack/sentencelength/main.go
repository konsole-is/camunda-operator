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

// Command sentencelength reports the sentences of the user docs that break
// the simple-english rules: a procedural sentence over 20 words, a
// descriptive sentence over 25 words, a semicolon, and the modals "would",
// "could" and "may".
//
// Usage: sentencelength [dir]
//
// The directory defaults to docs. The command reads every .md file below it,
// except docs/superpowers/ and docs/crds/TEMPLATE.md. It prints one line per
// finding, then the count for each page and the total. It exits with status 1
// when it reports a finding.
//
// What it reads:
//   - Paragraphs, list items, admonition bodies, blockquotes, and each table
//     cell as a separate text.
//   - It skips front matter, fenced code blocks, HTML comments, also inside a
//     line, headings, table separator rows, HTML lines, admonition title
//     lines, and link reference definitions.
//
// How it counts (ASD-STE100 rules 8.4 to 8.7):
//   - A sentence ends at ".", "!" or "?" before a space or the end of the
//     text, and at the end of the text. A colon that ends a text is the end
//     of a sentence too, so a list lead-in is counted alone.
//   - Inline code, URLs, text in parentheses, and text in double quotes count
//     as one word and are not checked for semicolons or modals. A link counts
//     as the words of its text. A hyphenated word counts as one word. A token
//     without a letter or a digit is no word.
//
// How it classifies a sentence:
//   - A sentence in an ordered list item is procedural.
//   - A sentence that starts with an imperative verb from the list below is
//     procedural. So is a sentence whose first clause starts with a condition
//     word (If, When, Before, After, To, Once, Unless) and whose words after
//     the first comma start with such a verb.
//   - Every other sentence is descriptive.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
)

const (
	procedural  = 20
	descriptive = 25
)

// finding is one sentence over its limit, or one semicolon or modal.
type finding struct {
	file  string
	line  int
	words int
	limit int
	// banned holds the semicolon or modal, and is empty for a long sentence.
	banned string
	// start holds the first words of the sentence, so a reader finds it on a
	// line that holds more than one sentence.
	start string
}

// text is one unit of prose: a paragraph, a list item, or a table cell.
type text struct {
	// lines holds the prose lines and lineNos the file line of each one.
	// A table cell has one line.
	lines   []string
	lineNos []int
	ordered bool
}

var (
	comment       = regexp.MustCompile(`(?s)<!--.*?(?:-->|\z)`)
	quoteMarkers  = regexp.MustCompile(`^\s*(?:>\s?)+`)
	orderedItem   = regexp.MustCompile(`^\s*\d+[.)]\s+`)
	unorderedItem = regexp.MustCompile(`^\s*[-*+]\s+`)
	tableSep      = regexp.MustCompile(`^\s*\|?[\s:|-]+\|?\s*$`)
	linkRefDef    = regexp.MustCompile(`^\s*\[[^\]]+\]:\s`)
	inlineCode    = regexp.MustCompile("`[^`]*`")
	link          = regexp.MustCompile(`!?\[([^\]]*)\]\((?:[^()]|\([^()]*\))*\)`)
	refLink       = regexp.MustCompile(`\[([^\]]*)\]\[[^\]]*\]`)
	autolink      = regexp.MustCompile(`<[A-Za-z][A-Za-z0-9+.-]*:[^\s<>]*>|<[^\s<>@]+@[^\s<>]+>`)
	bareURL       = regexp.MustCompile(`https?://[^\s<>]*[^\s<>.,:;!?)]`)
	quoted        = regexp.MustCompile(`"[^"]*"|“[^”]*”`)
	parens        = regexp.MustCompile(`\([^()]*\)`)
	banned        = regexp.MustCompile(`;|(?i:\b(?:would|could|may)(?:n['’]t)?\b)`)
)

var imperatives = map[string]bool{
	"add": true, "apply": true, "ask": true, "build": true, "change": true, "check": true,
	"choose": true, "configure": true, "copy": true, "create": true, "delete": true,
	"deploy": true, "disable": true, "do": true, "download": true, "edit": true,
	"enable": true, "expect": true, "find": true, "fix": true, "follow": true,
	"get": true, "give": true, "grant": true, "increase": true, "inspect": true,
	"install": true, "keep": true, "label": true, "leave": true, "let": true,
	"list": true, "look": true, "make": true, "move": true, "name": true,
	"never": true, "open": true, "pass": true, "patch": true, "pick": true,
	"point": true, "put": true, "raise": true, "read": true, "remove": true,
	"replace": true, "restore": true, "resume": true, "rotate": true, "run": true,
	"scale": true, "see": true, "select": true, "set": true, "start": true,
	"stop": true, "store": true, "suspend": true, "turn": true, "update": true,
	"upgrade": true, "use": true, "wait": true, "watch": true, "write": true,
}

var conditions = map[string]bool{
	"if": true, "when": true, "before": true, "after": true, "to": true, "once": true, "unless": true,
}

var abbreviations = map[string]bool{"e.g.": true, "i.e.": true, "etc.": true, "vs.": true}

func main() {
	flag.Parse()

	root := "docs"
	if flag.NArg() > 0 {
		root = flag.Arg(0)
	}

	findings, err := checkDir(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	perFile := map[string]int{}
	for _, f := range findings {
		if f.banned != "" {
			fmt.Printf("%s:%d: %q: %s ...\n", f.file, f.line, f.banned, f.start)
		} else {
			fmt.Printf("%s:%d: %d/%d words: %s ...\n", f.file, f.line, f.words, f.limit, f.start)
		}
		perFile[f.file]++
	}

	files := slices.Sorted(maps.Keys(perFile))
	if len(files) > 0 {
		fmt.Println()
	}
	for _, f := range files {
		fmt.Printf("%4d %s\n", perFile[f], f)
	}
	fmt.Printf("%4d total\n", len(findings))

	if len(findings) > 0 {
		os.Exit(1)
	}
}

var skippedPages = map[string]bool{
	filepath.Join("crds", "TEMPLATE.md"): true,
}

func checkDir(root string) ([]finding, error) {
	var findings []finding

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() && rel == "superpowers" {
			return filepath.SkipDir
		}
		if d.IsDir() || filepath.Ext(path) != ".md" || skippedPages[rel] {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}

		for _, f := range checkPage(string(content)) {
			f.file = path
			findings = append(findings, f)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", root, err)
	}

	return findings, nil
}

// checkPage returns the findings of one Markdown page, without a file name.
func checkPage(content string) []finding {
	ts := texts(content)
	findings := make([]finding, 0, len(ts))
	for _, t := range ts {
		findings = append(findings, checkText(t)...)
	}

	return findings
}

func texts(content string) []text {
	var (
		out     []text
		cur     *text
		fence   string
		inFront bool
	)
	flush := func() {
		if cur != nil && len(cur.lines) > 0 {
			out = append(out, *cur)
		}
		cur = nil
	}
	appendLine := func(line string, n int) {
		if cur == nil {
			cur = &text{}
		}
		cur.lines = append(cur.lines, line)
		cur.lineNos = append(cur.lineNos, n)
	}

	// A comment becomes the line breaks it held, so the line numbers stay
	// valid and a comment on its own lines still ends a text.
	content = comment.ReplaceAllStringFunc(content, func(c string) string {
		return strings.Repeat("\n", strings.Count(c, "\n"))
	})

	for i, line := range strings.Split(content, "\n") {
		lineNo := i + 1
		line = quoteMarkers.ReplaceAllString(line, "")
		trimmed := strings.TrimSpace(line)

		switch {
		case lineNo == 1 && trimmed == "---":
			inFront = true
			continue
		case inFront:
			if trimmed == "---" {
				inFront = false
			}
			continue
		case fence != "":
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
			continue
		}

		switch {
		case strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~"):
			flush()
			fence = trimmed[:3]
		case trimmed == "":
			flush()
		case strings.HasPrefix(trimmed, "#"),
			strings.HasPrefix(trimmed, "<") && !startsWithAutolink(trimmed),
			strings.HasPrefix(trimmed, "!!!"),
			strings.HasPrefix(trimmed, "???"),
			linkRefDef.MatchString(line):
			flush()
		case strings.HasPrefix(trimmed, "|"):
			flush()
			if tableSep.MatchString(trimmed) {
				continue
			}
			for _, cell := range tableCells(trimmed) {
				out = append(out, text{lines: []string{cell}, lineNos: []int{lineNo}})
			}
		case orderedItem.MatchString(line):
			flush()
			appendLine(orderedItem.ReplaceAllString(line, ""), lineNo)
			cur.ordered = true
		case unorderedItem.MatchString(line):
			flush()
			appendLine(unorderedItem.ReplaceAllString(line, ""), lineNo)
		default:
			appendLine(trimmed, lineNo)
		}
	}
	flush()

	return out
}

func startsWithAutolink(s string) bool {
	loc := autolink.FindStringIndex(s)
	return loc != nil && loc[0] == 0
}

// tableCells splits a table row on the pipes outside inline code.
func tableCells(row string) []string {
	var (
		cells  []string
		cell   strings.Builder
		inCode bool
	)
	for i, r := range row {
		switch {
		case r == '`':
			inCode = !inCode
			cell.WriteRune(r)
		case r == '|' && !inCode && (i == 0 || row[i-1] != '\\'):
			cells = append(cells, strings.TrimSpace(cell.String()))
			cell.Reset()
		default:
			cell.WriteRune(r)
		}
	}
	cells = append(cells, strings.TrimSpace(cell.String()))

	return slices.DeleteFunc(cells, func(c string) bool { return c == "" })
}

func checkText(t text) []finding {
	joined := strings.Join(t.lines, " ")
	starts := make([]int, len(t.lines))
	offset := 0
	for i, l := range t.lines {
		starts[i] = offset
		offset += len(l) + 1
	}

	lineAt := func(offset int) int {
		return t.lineNos[sort.SearchInts(starts, offset+1)-1]
	}

	masked := mask(joined)

	var findings []finding
	for _, s := range sentences(masked) {
		sentence := masked[s[0]:s[1]]
		start := firstWords(joined[s[0]:s[1]], 6)

		words := strings.FieldsFunc(sentence, unicode.IsSpace)
		words = slices.DeleteFunc(words, func(w string) bool { return !strings.ContainsFunc(w, isWordRune) })

		limit := descriptive
		if t.ordered || isProcedural(words) {
			limit = procedural
		}

		if len(words) > limit {
			findings = append(findings, finding{line: lineAt(s[0]), words: len(words), limit: limit, start: start})
		}

		for _, m := range banned.FindAllStringIndex(sentence, -1) {
			findings = append(findings, finding{
				line:   lineAt(s[0] + m[0]),
				banned: strings.ToLower(sentence[m[0]:m[1]]),
				start:  start,
			})
		}
	}

	return findings
}

func firstWords(s string, n int) string {
	fields := strings.Fields(s)
	return strings.Join(fields[:min(n, len(fields))], " ")
}

// mask rewrites the spans that count as one word into runs of "x" of the same
// byte length, and blanks link targets, so offsets stay valid.
func mask(s string) string {
	b := []byte(s)
	fill := func(re *regexp.Regexp, keepGroup bool) {
		for _, m := range re.FindAllSubmatchIndex(b, -1) {
			for i := m[0]; i < m[1]; i++ {
				switch {
				case keepGroup && i >= m[2] && i < m[3]:
				case keepGroup:
					b[i] = ' '
				default:
					b[i] = 'x'
				}
			}
		}
	}

	fill(inlineCode, false)
	fill(link, true)
	fill(refLink, true)
	fill(autolink, false)
	fill(bareURL, false)
	fill(quoted, false)
	for parens.Match(b) {
		fill(parens, false)
	}

	return string(b)
}

// sentences returns the [start, end) byte ranges of the sentences of s.
func sentences(s string) [][2]int {
	var (
		out   [][2]int
		start = 0
	)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '.' && c != '!' && c != '?' {
			continue
		}

		end := i + 1
		for end < len(s) && strings.IndexByte(`*_)"'`, s[end]) >= 0 {
			end++
		}
		if end < len(s) && s[end] != ' ' {
			continue
		}

		if c == '.' {
			wordStart := strings.LastIndexByte(s[:i], ' ') + 1
			if abbreviations[strings.ToLower(s[wordStart:i+1])] {
				continue
			}
		}

		out = append(out, [2]int{start, end})
		start = end
		i = end - 1
	}
	if strings.TrimSpace(s[start:]) != "" {
		out = append(out, [2]int{start, len(s)})
	}

	return out
}

func isProcedural(words []string) bool {
	if len(words) == 0 {
		return false
	}
	if imperatives[bare(words[0])] {
		return true
	}
	if !conditions[bare(words[0])] {
		return false
	}

	for i, w := range words[:len(words)-1] {
		if strings.HasSuffix(w, ",") {
			return imperatives[bare(words[i+1])]
		}
	}

	return false
}

// bare lowercases a word and strips the Markdown emphasis and punctuation around it.
func bare(w string) string {
	return strings.ToLower(strings.TrimFunc(w, func(r rune) bool { return !isWordRune(r) }))
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
