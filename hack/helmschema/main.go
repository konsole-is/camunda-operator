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

// Command helmschema writes values.schema.json of the Helm chart from the
// generated values.yaml. Helm validates the values against this file on
// install, upgrade, lint, and template, and Artifact Hub shows it as the values
// schema of the chart. It is a step of `make helm-generate`, after helmcli, so
// the schema follows every regeneration of values.yaml.
//
// The schema holds each key of values.yaml with the type of its default and
// the comment above it as the description. It also holds the keys that
// values.yaml shows only as a commented-out example, and every key that a
// template reads. An object of the chart rejects an unknown key, so a typo
// such as prometheus.enabeld fails. These objects accept any key:
//
//   - An object whose default is empty, such as nodeSelector.
//   - A Kubernetes type that the chart passes through, listed in openPaths.
//   - The items of a list.
//
// Usage: helmschema [chart-dir]
//
// chart-dir defaults to dist/chart.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// openPaths are the values that the chart passes to a Kubernetes field as a
// whole. The schema does not constrain their content: the Kubernetes API
// server validates it, and a key or a type that values.yaml does not show is
// valid there, such as resources.requests.ephemeral-storage or an integer
// maxSurge.
var openPaths = []string{
	"manager.podSecurityContext",
	"manager.resources",
	"manager.securityContext",
	"manager.strategy",
}

// schemaDraft is the JSON Schema dialect of the file. Helm and Artifact Hub
// both read draft-07.
const schemaDraft = "http://json-schema.org/draft-07/schema#"

// valuesRef matches a value path that a template reads, such as
// .Values.manager.image.tag.
var valuesRef = regexp.MustCompile(`\.Values((?:\.[A-Za-z_][A-Za-z0-9_]*)+)`)

// schema is the subset of JSON Schema that the generator writes. The field
// order is the order of the keys in the file.
type schema struct {
	Schema               string             `json:"$schema,omitempty"`
	Type                 string             `json:"type,omitempty"`
	Description          string             `json:"description,omitempty"`
	Properties           map[string]*schema `json:"properties,omitempty"`
	AdditionalProperties *bool              `json:"additionalProperties,omitempty"`
	Items                *schema            `json:"items,omitempty"`
}

func main() {
	dir := "dist/chart"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if err := run(dir); err != nil {
		fmt.Fprintln(os.Stderr, "helmschema:", err)
		os.Exit(1)
	}
}

func run(dir string) error {
	values, err := os.ReadFile(filepath.Join(dir, "values.yaml"))
	if err != nil {
		return err
	}

	templates, err := readTemplates(filepath.Join(dir, "templates"))
	if err != nil {
		return err
	}

	out, err := generate(string(values), templates)
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "values.schema.json")
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return err
	}
	fmt.Printf("helmschema: wrote %s\n", path)

	return nil
}

// readTemplates returns the content of each file under dir.
func readTemplates(dir string) ([]string, error) {
	var templates []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		templates = append(templates, string(content))

		return nil
	})

	return templates, err
}

// generate returns the schema document for values, the content of
// values.yaml, and templates, the content of the chart templates. The same
// input always gives the same output. It fails when values is not a YAML
// mapping, when an uncommented example is not valid YAML, and when a path in
// openPaths is not in values.
func generate(values string, templates []string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(uncommentExamples(values)), &doc); err != nil {
		return nil, fmt.Errorf("parsing values.yaml with its examples uncommented: %w", err)
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("values.yaml is not a mapping")
	}

	root := nodeSchema(doc.Content[0], "")
	root.Schema = schemaDraft

	for _, path := range openPaths {
		if lookup(root, path) == nil {
			return nil, fmt.Errorf("open path %s is not in values.yaml; update openPaths", path)
		}
	}

	for _, template := range templates {
		for _, match := range valuesRef.FindAllStringSubmatch(template, -1) {
			allow(root, strings.Split(strings.TrimPrefix(match[1], "."), "."))
		}
	}

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, err
	}

	return append(out, '\n'), nil
}

// uncommentExamples returns values with each commented-out example made into
// YAML. The kubebuilder helm plugin writes an optional value as a "##"
// description, a "##" line, and then the value commented out with "# ":
//
//	## Priority class name
//	##
//	# priorityClassName: ""
//
// A "#" comment that does not follow a "##" line at the same indentation is a
// description, such as "# Metrics server port", and stays a comment.
func uncommentExamples(values string) string {
	lines := strings.Split(values, "\n")
	example := -1 // the indentation of the example block, or -1 outside one
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		indent := len(line) - len(trimmed)

		if example >= 0 && indent == example && (trimmed == "#" || strings.HasPrefix(trimmed, "# ")) {
			lines[i] = line[:indent] + strings.TrimPrefix(strings.TrimPrefix(trimmed, "#"), " ")
			continue
		}
		example = -1
		if trimmed == "##" && i+1 < len(lines) && strings.HasPrefix(lines[i+1], line[:indent]+"# ") {
			example = indent
		}
	}

	return strings.Join(lines, "\n")
}

// nodeSchema returns the schema of a value whose default is node. path is the
// dotted path of the value, empty for the root.
func nodeSchema(node *yaml.Node, path string) *schema {
	switch node.Kind {
	case yaml.MappingNode:
		s := &schema{Type: "object"}
		if len(node.Content) == 0 || slices.Contains(openPaths, path) {
			return s
		}
		s.Properties = map[string]*schema{}
		s.AdditionalProperties = new(false)
		for i := 0; i+1 < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			child := nodeSchema(value, strings.TrimPrefix(path+"."+key.Value, "."))
			child.Description = description(key.HeadComment)
			s.Properties[key.Value] = child
		}

		return s
	case yaml.SequenceNode:
		s := &schema{Type: "array"}
		if len(node.Content) > 0 && node.Content[0].Kind == yaml.ScalarNode {
			s.Items = nodeSchema(node.Content[0], path)
		}

		return s
	case yaml.ScalarNode:
		return &schema{Type: scalarType(node.Tag)}
	default:
		return &schema{}
	}
}

// scalarType returns the JSON Schema type of a YAML scalar tag. A null has no
// type, so any value is valid.
func scalarType(tag string) string {
	switch tag {
	case "!!bool":
		return "boolean"
	case "!!int":
		return "integer"
	case "!!float":
		return "number"
	case "!!str":
		return "string"
	default:
		return ""
	}
}

// description returns the text of a YAML head comment as one line, without
// the "#" markers and the empty comment lines.
func description(comment string) string {
	var words []string
	for line := range strings.SplitSeq(comment, "\n") {
		if text := strings.TrimSpace(strings.TrimLeft(line, "#")); text != "" {
			words = append(words, text)
		}
	}

	return strings.Join(words, " ")
}

// lookup returns the schema at the dotted path, or nil.
func lookup(s *schema, path string) *schema {
	for key := range strings.SplitSeq(path, ".") {
		if s = s.Properties[key]; s == nil {
			return nil
		}
	}

	return s
}

// allow makes the schema accept the value at path. It adds the first missing
// key of the path as a value of any type to an object that rejects unknown
// keys.
func allow(s *schema, path []string) {
	for _, key := range path {
		if s.AdditionalProperties == nil || *s.AdditionalProperties {
			return
		}
		child, ok := s.Properties[key]
		if !ok {
			s.Properties[key] = &schema{}
			return
		}
		s = child
	}
}
