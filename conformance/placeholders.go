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
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"text/template/parse"
)

// placeholders is a set of the field references that a template may hold.
// A name of fields stands alone, as .namespace. A name of maps takes one
// key, as .parameters.<name>.
type placeholders struct {
	fields map[string]bool
	maps   map[string]bool
}

// templatePlaceholders are the placeholders of a grant template and of the
// default of an x-fleet-template parameter.
var templatePlaceholders = placeholders{
	fields: map[string]bool{"namespace": true, "claim": true, "request": true, "serviceAccount": true},
	maps:   map[string]bool{"parameters": true, "outputs": true},
}

// metricPlaceholders are the placeholders of a metric query: those of a
// template, and .pods.
var metricPlaceholders = placeholders{
	fields: map[string]bool{"namespace": true, "claim": true, "request": true, "serviceAccount": true, "pods": true},
	maps:   map[string]bool{"parameters": true, "outputs": true},
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
	const name = "template"

	tree := parse.New(name)
	// Comments and unknown functions parse, so that the walk names them.
	tree.Mode = parse.ParseComments | parse.SkipFuncCheck
	treeSet := map[string]*parse.Tree{}
	if _, err := tree.Parse(text, "", "", treeSet); err != nil {
		return err
	}

	var errs []error
	for _, defined := range slices.Sorted(maps.Keys(treeSet)) {
		if defined != name {
			errs = append(errs, fmt.Errorf("define %q: define is not allowed, %s", defined, onlyFieldReferences))
		}
	}

	for _, node := range tree.Root.Nodes {
		if err := checkNode(node, allowed); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
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
