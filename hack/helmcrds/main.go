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

// Command helmcrds removes the field descriptions from the CRD templates of
// the chart. Helm stores each release in one Secret, and the API server
// limits a Secret to 1 MiB of data. The release holds every CRD twice, once
// as a chart template and once in the rendered manifest, and the descriptions
// are most of the bytes of a CRD. Without them the release fits.
//
// It removes the description keyword of each schema node under
// spec.versions[].schema.openAPIV3Schema and keeps everything else: a
// property named description, validations, CEL rules and their messages,
// enums, defaults, and printer columns. config/crd/bases and dist/install.yaml
// keep their descriptions. It is a step of `make helm-generate`, after the
// plugin writes templates/crd, not a hand edit. A template whose spec holds a
// Helm action fails, so a plugin upgrade that templates the schema breaks
// generation visibly.
//
// Usage: helmcrds [chart-dir]
//
// chart-dir defaults to dist/chart.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// subschemaKeywords are the keywords whose value is a schema, or a list of
// schemas.
var subschemaKeywords = []string{"items", "additionalProperties", "additionalItems", "not", "allOf", "anyOf", "oneOf"}

// subschemaMapKeywords are the keywords whose value maps a name to a schema.
var subschemaMapKeywords = []string{"properties", "patternProperties", "definitions", "dependencies"}

// helmLiteral matches a Helm action that prints a string literal.
var helmLiteral = regexp.MustCompile(`\{\{ "(?:[^"\\]|\\.)*" \}\}`)

// placeholderPrefix starts each placeholder of hideLiterals. A placeholder is
// one word, so the YAML encoder never breaks it across lines, and it ends in
// "end", so no placeholder is the start of another.
const placeholderPrefix = "helmcrdsliteral"

func main() {
	dir := "dist/chart"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if err := run(dir); err != nil {
		fmt.Fprintln(os.Stderr, "helmcrds:", err)
		os.Exit(1)
	}
}

func run(dir string) error {
	paths, err := filepath.Glob(filepath.Join(dir, "templates", "crd", "*.yaml"))
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no CRD templates in %s", filepath.Join(dir, "templates", "crd"))
	}

	removed := 0
	for _, path := range paths {
		template, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		stripped, n, err := stripTemplate(string(template))
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

		if err := os.WriteFile(path, []byte(stripped), 0o600); err != nil {
			return err
		}
		removed += n
	}
	fmt.Printf("helmcrds: removed %d field descriptions from %d CRD templates\n", removed, len(paths))

	return nil
}

// stripTemplate returns the CRD template without the descriptions of its
// schema nodes, and the number it removed. The Helm actions around the spec
// stay as they are. It fails when the template has no top-level spec, or when
// the spec holds a Helm action.
func stripTemplate(template string) (string, int, error) {
	lines := strings.Split(template, "\n")

	start := slices.Index(lines, "spec:")
	if start < 0 {
		return "", 0, errors.New("no top-level spec line")
	}
	// The spec ends at the next line that is not indented, such as the
	// {{- end }} of the crd.enabled toggle.
	end := start + 1
	for end < len(lines) && (lines[end] == "" || strings.HasPrefix(lines[end], " ")) {
		end++
	}
	spec, literals, err := hideLiterals(strings.Join(lines[start:end], "\n"))
	if err != nil {
		return "", 0, err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(spec), &doc); err != nil {
		return "", 0, fmt.Errorf("parsing the spec: %w", err)
	}
	removed, err := stripVersions(&doc)
	if err != nil {
		return "", 0, err
	}

	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return "", 0, fmt.Errorf("writing the spec: %w", err)
	}
	if err := enc.Close(); err != nil {
		return "", 0, fmt.Errorf("writing the spec: %w", err)
	}

	stripped := literals.Replace(strings.TrimSuffix(out.String(), "\n"))
	result := slices.Concat(lines[:start], []string{stripped}, lines[end:])

	return strings.Join(result, "\n"), removed, nil
}

// hideLiterals replaces each Helm string literal in the spec, such as
// {{ "{{ .namespace }}" }}, with a placeholder that YAML reads as plain text.
// The plugin writes such a literal for each {{ in a CRD text. It returns a
// replacer that puts the literals back. It fails when the spec holds any other
// Helm action, or already holds a placeholder.
func hideLiterals(spec string) (string, *strings.Replacer, error) {
	if strings.Contains(spec, placeholderPrefix) {
		return "", nil, fmt.Errorf("the spec already holds the placeholder %q", placeholderPrefix)
	}

	var restore []string
	hidden := helmLiteral.ReplaceAllStringFunc(spec, func(literal string) string {
		placeholder := placeholderPrefix + strconv.Itoa(len(restore)/2) + "end"
		restore = append(restore, placeholder, literal)
		return placeholder
	})
	if strings.Contains(hidden, "{{") {
		return "", nil, errors.New("the spec holds a Helm action that is not a string literal")
	}

	return hidden, strings.NewReplacer(restore...), nil
}

// stripVersions removes the descriptions from the schema of each version in
// the spec document.
func stripVersions(doc *yaml.Node) (int, error) {
	versions := lookup(lookup(doc.Content[0], "spec"), "versions")
	if versions == nil || versions.Kind != yaml.SequenceNode {
		return 0, errors.New("no spec.versions list")
	}

	removed := 0
	for _, version := range versions.Content {
		schema := lookup(lookup(version, "schema"), "openAPIV3Schema")
		if schema == nil {
			return 0, errors.New("a version has no schema.openAPIV3Schema")
		}
		removed += stripSchema(schema)
	}

	return removed, nil
}

// stripSchema removes the description keyword of the schema node and of each
// schema below it. A value that is not a schema, such as a default or an
// enum, keeps its content, also when it holds a key named description.
func stripSchema(schema *yaml.Node) int {
	if schema.Kind != yaml.MappingNode {
		return 0
	}

	removed := 0
	for i := 0; i < len(schema.Content); {
		key, value := schema.Content[i].Value, schema.Content[i+1]
		switch {
		case key == "description":
			schema.Content = slices.Delete(schema.Content, i, i+2)
			removed++
			continue
		case slices.Contains(subschemaKeywords, key):
			removed += stripSchemaOrList(value)
		case slices.Contains(subschemaMapKeywords, key) && value.Kind == yaml.MappingNode:
			for j := 1; j < len(value.Content); j += 2 {
				removed += stripSchemaOrList(value.Content[j])
			}
		}
		i += 2
	}

	return removed
}

// stripSchemaOrList strips a schema, or each schema of a list. A boolean, as
// in additionalProperties: false, or a list of names, as in dependencies, has
// nothing to strip.
func stripSchemaOrList(node *yaml.Node) int {
	if node.Kind != yaml.SequenceNode {
		return stripSchema(node)
	}

	removed := 0
	for _, item := range node.Content {
		removed += stripSchema(item)
	}

	return removed
}

// lookup returns the value of key in a mapping node, or nil.
func lookup(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}

	return nil
}
