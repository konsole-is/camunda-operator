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

package conformance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"text/template"
	"text/template/parse"
)

// placeholders is a set of the field references that a template may hold.
// A name of fields stands alone, as .namespace. A name of maps takes one
// key, as .parameters.<name>. refused maps a name that is not allowed to the
// reason.
type placeholders struct {
	fields  map[string]bool
	maps    map[string]bool
	refused map[string]string
}

// grantPlaceholders are the placeholders of a grant template.
var grantPlaceholders = placeholders{
	fields: map[string]bool{"namespace": true, "claim": true, "request": true, "serviceAccount": true},
	maps:   map[string]bool{"parameters": true, "outputs": true},
}

// parameterPlaceholders are the placeholders of an x-fleet-template
// parameter.
var parameterPlaceholders = placeholders{
	fields: map[string]bool{"namespace": true, "claim": true, "request": true, "serviceAccount": true},
	maps:   map[string]bool{"parameters": true},
	refused: map[string]string{
		"outputs": "an x-fleet-template parameter is rendered before the plugin writes the outputs",
	},
}

// metricPlaceholders are the placeholders of a metric query: those of a
// grant template, and .pods.
var metricPlaceholders = placeholders{
	fields: map[string]bool{"namespace": true, "claim": true, "request": true, "serviceAccount": true, "pods": true},
	maps:   map[string]bool{"parameters": true, "outputs": true},
}

// templateData holds the values of the placeholders for one request.
type templateData struct {
	namespace      string
	claim          string
	request        string
	serviceAccount string
	pods           string
	parameters     map[string]any
	outputs        map[string]string
}

// jsonLeaf is one string value of a JSON document, with its field path.
type jsonLeaf struct {
	path  string
	value string
}

// checkJSONTemplates runs checkTemplate on each string value of raw, at any
// depth. Each error names the path of its value under path. An object key
// is never a template.
func checkJSONTemplates(path string, raw []byte, allowed placeholders) error {
	leaves, err := stringLeaves(path, raw)
	if err != nil {
		return err
	}

	var errs []error
	for _, leaf := range leaves {
		if err := checkTemplate(leaf.value, allowed); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", leaf.path, err))
		}
	}

	return errors.Join(errs...)
}

// stringLeaves returns each string value of raw with its path: <path>.<key>
// in an object, <path>[<i>] in an array, and path itself when raw is a
// string. The keys of an object come in sorted order.
func stringLeaves(path string, raw []byte) ([]jsonLeaf, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return appendLeaves(nil, path, value), nil
}

func appendLeaves(leaves []jsonLeaf, path string, value any) []jsonLeaf {
	switch v := value.(type) {
	case string:
		leaves = append(leaves, jsonLeaf{path: path, value: v})
	case []any:
		for i, item := range v {
			leaves = appendLeaves(leaves, fmt.Sprintf("%s[%d]", path, i), item)
		}
	case map[string]any:
		for _, key := range slices.Sorted(maps.Keys(v)) {
			leaves = appendLeaves(leaves, path+"."+key, v[key])
		}
	}

	return leaves
}

// checkTemplate returns an error when text is not a Go template, or when an
// action of it is anything but one field reference of allowed. The error
// names each such action.
func checkTemplate(text string, allowed placeholders) error {
	tree, defines, err := parseTemplate(text)
	if err != nil {
		return err
	}

	var errs []error
	for _, name := range defines {
		errs = append(errs, fmt.Errorf("define %q: define is not allowed, %s", name, onlyFieldReferences))
	}

	for _, node := range tree.Root.Nodes {
		if err := checkNode(node, allowed); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// parseTemplate returns the main tree of text and the names of the
// templates that text defines.
func parseTemplate(text string) (*parse.Tree, []string, error) {
	var main *parse.Tree
	defines := map[string]bool{}

	// A define with the name of the main tree takes its place, or is dropped
	// when it is empty. No define has both names, so one of the two parses
	// shows it.
	for _, name := range []string{"a", "b"} {
		tree := parse.New(name)
		// Comments and unknown functions parse, so that the walk names them.
		tree.Mode = parse.ParseComments | parse.SkipFuncCheck
		treeSet := map[string]*parse.Tree{}
		if _, err := tree.Parse(text, "", "", treeSet); err != nil {
			return nil, nil, err
		}

		for defined, t := range treeSet {
			if t != tree {
				defines[defined] = true
			}
		}

		if main == nil {
			main = tree
		}
	}

	return main, slices.Sorted(maps.Keys(defines)), nil
}

const onlyFieldReferences = "only a field reference such as {{ .namespace }}"

func checkNode(node parse.Node, allowed placeholders) error {
	notAllowed := func(construct string) error {
		return fmt.Errorf("%s: %s is not allowed, %s", node, construct, onlyFieldReferences)
	}

	switch n := node.(type) {
	case *parse.TextNode:
		return nil
	case *parse.IfNode:
		return notAllowed("if")
	case *parse.RangeNode:
		return notAllowed("range")
	case *parse.WithNode:
		return notAllowed("with")
	case *parse.TemplateNode:
		return notAllowed("template")
	case *parse.CommentNode:
		return notAllowed("comment")
	case *parse.ActionNode:
		return checkAction(n, allowed, notAllowed)
	default:
		return notAllowed(fmt.Sprintf("%T", node))
	}
}

func checkAction(action *parse.ActionNode, allowed placeholders, notAllowed func(string) error) error {
	pipe := action.Pipe
	switch {
	case len(pipe.Decl) > 0:
		return notAllowed("variable")
	case len(pipe.Cmds) > 1:
		return notAllowed("pipeline")
	}

	args := pipe.Cmds[0].Args
	var field *parse.FieldNode
	switch arg := args[0].(type) {
	case *parse.IdentifierNode:
		return notAllowed("function call " + arg.Ident)
	case *parse.VariableNode:
		return notAllowed("variable")
	case *parse.DotNode:
		return notAllowed("dot")
	case *parse.FieldNode:
		field = arg
	default:
		return notAllowed("the expression " + arg.String())
	}

	if len(args) > 1 {
		return notAllowed("function call")
	}

	ident := field.Ident
	if (len(ident) == 1 && allowed.fields[ident[0]]) || (len(ident) == 2 && allowed.maps[ident[0]]) {
		return nil
	}

	dotted := "." + strings.Join(ident, ".")
	if reason := allowed.refused[ident[0]]; reason != "" {
		return fmt.Errorf("%s: the placeholder %s is not allowed, %s", action, dotted, reason)
	}
	if allowed.maps[ident[0]] {
		return fmt.Errorf(
			"%s: the placeholder %s is not in the contract, .%s takes one key, as .%s.<key>",
			action,
			dotted,
			ident[0],
			ident[0],
		)
	}

	return fmt.Errorf("%s: the placeholder %s is not in the contract", action, dotted)
}

// renderTemplate fills the placeholders of text from data. It refuses what
// checkTemplate refuses, and a parameter or an output that text references
// and data does not hold. It is the conformance copy of the renderer of
// pluginkit, and both hold the same named test cases.
func renderTemplate(text string, allowed placeholders, data templateData) (string, error) {
	if err := checkTemplate(text, allowed); err != nil {
		return "", err
	}

	tree, _, err := parseTemplate(text)
	if err != nil {
		return "", err
	}

	for _, node := range tree.Root.Nodes {
		action, ok := node.(*parse.ActionNode)
		if !ok {
			continue
		}

		field, ok := action.Pipe.Cmds[0].Args[0].(*parse.FieldNode)
		if !ok || len(field.Ident) != 2 {
			continue
		}

		name := field.Ident[1]
		if _, ok := data.parameters[name]; field.Ident[0] == "parameters" && !ok {
			return "", fmt.Errorf("parameter %q has no value", name)
		}
		if _, ok := data.outputs[name]; field.Ident[0] == "outputs" && !ok {
			return "", fmt.Errorf("output %q has no value", name)
		}
	}

	tmpl, err := template.New("").Option("missingkey=error").Parse(text)
	if err != nil {
		return "", err
	}

	var out strings.Builder
	err = tmpl.Execute(&out, map[string]any{
		"namespace":      data.namespace,
		"claim":          data.claim,
		"request":        data.request,
		"serviceAccount": data.serviceAccount,
		"pods":           data.pods,
		"parameters":     data.parameters,
		"outputs":        data.outputs,
	})
	if err != nil {
		return "", err
	}

	return out.String(), nil
}

// renderJSONTemplate runs renderTemplate on each string value of raw, at any
// depth, and returns the document as JSON. An object key is never a
// template. A rendered value stays inside its string, and a number keeps its
// exact digits. An error names the path of its value under path, as
// checkJSONTemplates does. It is the conformance copy of pluginkit
// RenderJSON.
func renderJSONTemplate(path string, raw []byte, allowed placeholders, data templateData) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var doc any
	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	rendered, err := renderLeaves(path, doc, allowed, data)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(rendered); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return bytes.TrimSuffix(out.Bytes(), []byte("\n")), nil
}

func renderLeaves(path string, value any, allowed placeholders, data templateData) (any, error) {
	switch v := value.(type) {
	case string:
		rendered, err := renderTemplate(v, allowed, data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return rendered, nil
	case []any:
		for i, item := range v {
			rendered, err := renderLeaves(fmt.Sprintf("%s[%d]", path, i), item, allowed, data)
			if err != nil {
				return nil, err
			}
			v[i] = rendered
		}
	case map[string]any:
		for _, key := range slices.Sorted(maps.Keys(v)) {
			rendered, err := renderLeaves(path+"."+key, v[key], allowed, data)
			if err != nil {
				return nil, err
			}
			v[key] = rendered
		}
	}

	return value, nil
}
