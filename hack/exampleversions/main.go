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

// Command exampleversions proves that the example inventories run the versions
// the end-to-end suite tests. The table below holds every version field of
// config/example. Each entry names the variable of test/e2e/matrix/<minor>.env
// that holds the same version, and the Renovate marker that raises it.
//
// The check fails when:
//   - a version and its variable differ,
//   - a version of an inventory has no entry,
//   - a version whose entry names a marker does not carry it,
//   - a version whose entry names none carries one.
//
// Usage: exampleversions -matrix <file> [dir]
//
// It reads the block style of these files: one key per line, two-space
// indentation, and no flow mapping. In a Markdown file it reads the fenced
// yaml blocks. A version inside a sequence takes the path of the key that
// holds the sequence, with version behind it, for example spec.images.version.
// No entry holds such a path, so the check fails on it instead of passing it
// unseen.
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
	"strings"
)

// pin is one version field of the example inventories.
type pin struct {
	// file is the path of the manifest below the examples directory, and path
	// is the dotted path of the field inside it.
	file string
	path string
	// env is the variable of the matrix entry that holds the same version.
	env string
	// marker is the Renovate marker that stands above the field, without its
	// leading "# renovate: ". It is empty for a version Renovate leaves alone.
	marker string
}

const (
	elasticsearchMarker = "datasource=docker depName=docker.elastic.co/elasticsearch/elasticsearch"
	keycloakMarker      = `datasource=docker depName=camunda/keycloak ` +
		`extractVersion=^quay-optimized-(?<version>\d+\.\d+\.\d+)$`
)

// document is a parsed file: the version fields it holds, and its lines, which
// the marker check reads.
type document struct {
	versions []version
	lines    []string
}

// version is one version field: the dotted path of the field, the value it
// holds, and the line it stands on, counted from one.
type version struct {
	path  string
	value string
	line  int
}

// pins is every version of the example inventories. A version that is not
// here fails the check, so a new one has to name the variable it follows.
var pins = []pin{
	{
		file:   "releases/camunda-release.yaml",
		path:   "spec.version",
		env:    "CAMUNDA_VERSION",
		marker: "datasource=docker depName=camunda/camunda",
	},
	{
		file:   "releases/camunda-release.yaml",
		path:   "spec.connectors.version",
		env:    "CAMUNDA_CONNECTORS_VERSION",
		marker: "datasource=docker depName=camunda/connectors-bundle",
	},
	{
		file:   "releases/camunda-release.yaml",
		path:   "spec.elasticsearch.version",
		env:    "ELASTICSEARCH_VERSION",
		marker: elasticsearchMarker,
	},
	{
		// The PostgreSQL major carries no marker. A new major is a
		// supported-version decision the operator makes on purpose, and
		// Renovate raises no major of a pin.
		file: "releases/camunda-release.yaml",
		path: "spec.databaseServer.version",
		env:  "POSTGRES_VERSION",
	},
	// The README of the release repeats the manifest inside a fenced block,
	// marker included, so Renovate raises the page with the manifest.
	{
		file:   "releases/README.md",
		path:   "spec.version",
		env:    "CAMUNDA_VERSION",
		marker: "datasource=docker depName=camunda/camunda",
	},
	{
		file:   "releases/README.md",
		path:   "spec.connectors.version",
		env:    "CAMUNDA_CONNECTORS_VERSION",
		marker: "datasource=docker depName=camunda/connectors-bundle",
	},
	{
		file:   "releases/README.md",
		path:   "spec.elasticsearch.version",
		env:    "ELASTICSEARCH_VERSION",
		marker: elasticsearchMarker,
	},
	{file: "releases/README.md", path: "spec.databaseServer.version", env: "POSTGRES_VERSION"},
	{
		file:   "camunda-cluster/elasticsearch/02-elasticsearch-cluster.yaml",
		path:   "spec.version",
		env:    "ELASTICSEARCH_VERSION",
		marker: elasticsearchMarker,
	},
	{
		file:   "camunda-cluster/elasticsearch/04-camunda-cluster.yaml",
		path:   "spec.version",
		env:    "CAMUNDA_VERSION",
		marker: "datasource=docker depName=camunda/camunda",
	},
	{
		file:   "camunda-management-cluster/keycloak/06-management-cluster.yaml",
		path:   "spec.identityProvider.keycloak.version",
		env:    "KEYCLOAK_VERSION",
		marker: keycloakMarker,
	},
	{
		file:   "camunda-management-cluster/keycloak/06-management-cluster.yaml",
		path:   "spec.identity.version",
		env:    "CAMUNDA_IDENTITY_VERSION",
		marker: "datasource=docker depName=camunda/identity",
	},
	{
		file:   "camunda-management-cluster/keycloak/06-management-cluster.yaml",
		path:   "spec.console.version",
		env:    "CAMUNDA_CONSOLE_VERSION",
		marker: "datasource=docker depName=camunda/console",
	},
	{
		file:   "camunda-management-cluster/keycloak/06-management-cluster.yaml",
		path:   "spec.webModeler.version",
		env:    "CAMUNDA_WEB_MODELER_VERSION",
		marker: "datasource=docker depName=camunda/web-modeler-restapi",
	},
	{
		file:   "camunda-management-cluster/keycloak/09-optimize.yaml",
		path:   "spec.version",
		env:    "CAMUNDA_OPTIMIZE_VERSION",
		marker: "datasource=docker depName=camunda/optimize",
	},
	{
		file:   "camunda-management-cluster/oidc/06-management-cluster.yaml",
		path:   "spec.identity.version",
		env:    "CAMUNDA_IDENTITY_VERSION",
		marker: "datasource=docker depName=camunda/identity",
	},
	{
		file:   "camunda-management-cluster/oidc/06-management-cluster.yaml",
		path:   "spec.console.version",
		env:    "CAMUNDA_CONSOLE_VERSION",
		marker: "datasource=docker depName=camunda/console",
	},
	{
		file:   "camunda-management-cluster/oidc/06-management-cluster.yaml",
		path:   "spec.webModeler.version",
		env:    "CAMUNDA_WEB_MODELER_VERSION",
		marker: "datasource=docker depName=camunda/web-modeler-restapi",
	},
}

func main() {
	matrix := flag.String("matrix", "", "the matrix entry the example inventories mirror")
	flag.Parse()

	if *matrix == "" {
		fmt.Fprintln(os.Stderr, "-matrix names the matrix entry to check against, for example test/e2e/matrix/8.9.env")
		flag.Usage()
		os.Exit(2)
	}

	examples := filepath.Join("config", "example")
	if flag.NArg() > 0 {
		examples = flag.Arg(0)
	}

	problems, err := check(examples, *matrix, pins)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	for _, problem := range problems {
		fmt.Println(problem)
	}

	if len(problems) > 0 {
		fmt.Fprintf(
			os.Stderr,
			"\n%d version pins of %s disagree with %s\n",
			len(problems),
			examples,
			*matrix,
		)
		os.Exit(1)
	}
}

// check returns one line per problem: first the versions that no entry holds,
// by file, then the entries of the table in order. It returns an error only
// when it cannot read a file it has to read.
func check(examples, matrixFile string, pins []pin) ([]string, error) {
	matrix, err := readMatrix(matrixFile)
	if err != nil {
		return nil, err
	}

	docs, err := readExamples(examples)
	if err != nil {
		return nil, err
	}

	problems := unlistedVersions(examples, docs, pins)

	for _, p := range pins {
		where := filepath.Join(examples, p.file)

		doc, ok := docs[p.file]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s: no such file. An entry expects %s in it", where, p.path))
			continue
		}

		found := doc.find(p.path)
		if len(found) != 1 {
			problems = append(
				problems,
				fmt.Sprintf("%s: %s stands %d times. It has to stand once", where, p.path, len(found)),
			)
			continue
		}

		problems = append(problems, comparePin(where, p, found[0], doc, matrix)...)
	}

	return problems, nil
}

// readMatrix reads the `NAME=value` lines of a matrix entry.
func readMatrix(path string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading the matrix entry: %w", err)
	}

	values := map[string]string{}
	for line := range strings.SplitSeq(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		name, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}

		values[strings.TrimSpace(name)] = strings.TrimSpace(value)
	}

	return values, nil
}

// readExamples parses every manifest and Markdown page below dir. The keys of
// the result are paths below dir, with forward slashes.
func readExamples(dir string) (map[string]document, error) {
	docs := map[string]document{}

	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !slices.Contains([]string{".yaml", ".md"}, filepath.Ext(path)) {
			return nil
		}

		doc, err := readDocument(path)
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		docs[filepath.ToSlash(rel)] = doc
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading the example inventories: %w", err)
	}

	return docs, nil
}

// keyLine matches a block mapping key with the value that follows it. The
// prefix takes the "- " of a sequence entry, so the first key of an entry
// counts as a key and a version below it does not pass unseen.
var keyLine = regexp.MustCompile(`^(\s*(?:-\s+)?)([A-Za-z][A-Za-z0-9_.-]*):(?:\s+(.*?))?\s*$`)

// key is one mapping key of the path down to a line, with the indentation it
// stands at.
type key struct {
	indent int
	name   string
}

func readDocument(path string) (document, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return document{}, fmt.Errorf("reading %s: %w", path, err)
	}

	doc := document{lines: strings.Split(string(raw), "\n")}
	markdown := filepath.Ext(path) == ".md"
	reading := !markdown

	// keys holds the mapping keys of the path down to the current line.
	var keys []key

	for i, line := range doc.lines {
		trimmed := strings.TrimSpace(line)

		if markdown {
			switch {
			case !reading && strings.HasPrefix(trimmed, "```yaml"):
				reading, keys = true, nil
				continue
			case reading && strings.HasPrefix(trimmed, "```"):
				reading = false
				continue
			case !reading:
				continue
			}
		}

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if trimmed == "---" {
			keys = nil
			continue
		}

		match := keyLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		indent := len(match[1])
		for len(keys) > 0 && keys[len(keys)-1].indent >= indent {
			keys = keys[:len(keys)-1]
		}
		keys = append(keys, key{indent: indent, name: match[2]})

		if match[2] != "version" {
			continue
		}

		names := make([]string, 0, len(keys))
		for _, k := range keys {
			names = append(names, k.name)
		}

		doc.versions = append(doc.versions, version{
			path:  strings.Join(names, "."),
			value: strings.Trim(match[3], `"'`),
			line:  i + 1,
		})
	}

	return doc, nil
}

func (d document) find(path string) []version {
	var found []version
	for _, v := range d.versions {
		if v.path == path {
			found = append(found, v)
		}
	}
	return found
}

// unlistedVersions reports every version of the inventories that the table
// does not hold, so a version pin cannot enter without a variable to follow.
func unlistedVersions(examples string, docs map[string]document, pins []pin) []string {
	listed := map[string]bool{}
	for _, p := range pins {
		listed[p.file+" "+p.path] = true
	}

	var problems []string
	for _, file := range slices.Sorted(maps.Keys(docs)) {
		for _, v := range docs[file].versions {
			if listed[file+" "+v.path] {
				continue
			}

			problems = append(problems, fmt.Sprintf(
				"%s:%d: %s is in no entry of hack/exampleversions. Add one that names the variable it follows",
				filepath.Join(examples, file), v.line, v.path,
			))
		}
	}

	return problems
}

func comparePin(where string, p pin, found version, doc document, matrix map[string]string) []string {
	var problems []string

	want, ok := matrix[p.env]
	switch {
	case !ok:
		problems = append(problems, fmt.Sprintf("%s:%d: the matrix entry holds no %s", where, found.line, p.env))
	case found.value != want:
		problems = append(problems, fmt.Sprintf(
			"%s:%d: %s is %q, %s is %q",
			where, found.line, p.path, found.value, p.env, want,
		))
	}

	marker := ""
	if found.line >= 2 {
		marker = strings.TrimSpace(doc.lines[found.line-2])
	}

	switch {
	case p.marker != "" && marker != "# renovate: "+p.marker:
		problems = append(problems, fmt.Sprintf(
			"%s:%d: %s has no Renovate marker. Add `# renovate: %s` above it",
			where, found.line, p.path, p.marker,
		))
	case p.marker == "" && strings.HasPrefix(marker, "# renovate:"):
		problems = append(problems, fmt.Sprintf(
			"%s:%d: %s carries a Renovate marker. Add the marker to its entry in hack/exampleversions",
			where, found.line, p.path,
		))
	}

	return problems
}
