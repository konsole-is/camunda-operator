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

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// fieldSchema is the type of one field, as fieldschema.ts writes it.
type fieldSchema struct {
	Type        string       `json:"type,omitempty"`
	Enum        []string     `json:"enum,omitempty"`
	Minimum     *float64     `json:"minimum,omitempty"`
	Maximum     *float64     `json:"maximum,omitempty"`
	Format      string       `json:"format,omitempty"`
	IntOrString bool         `json:"intOrString,omitempty"`
	Items       *fieldSchema `json:"items,omitempty"`
}

// schemaSet is the content of fieldschema.ts.
type schemaSet struct {
	Fields map[string]map[string]fieldSchema // by root, then path
	Full   map[string]json.RawMessage        // by full schema name
}

// fullSchemaNames are the fields whose whole schema the YAML editor checks.
var fullSchemaNames = []string{"extraEnvFrom", "nodeAffinity", "podAffinity"}

// walkSchema adds to set an entry for each property of struct t that s
// describes, by the path of JSON names under root. It goes into a property
// only when its Go type, without pointers, is a struct under the package
// path local: a slice, a map, a scalar, and a type outside local are one
// entry each. The items of an entry are the type of the items of a list.
// For a property named in fullSchemaNames it also keeps the whole property
// schema without descriptions, once for each name.
//
// It returns an error for a property name with a dot, a property that the
// Go type does not have, an enum value that is not a string, and a second
// full schema of one name that differs from the first.
func walkSchema(
	set *schemaSet,
	root string,
	t reflect.Type,
	s *apiextensionsv1.JSONSchemaProps,
	local string,
) error {
	if set.Fields == nil {
		set.Fields = map[string]map[string]fieldSchema{}
	}

	if set.Full == nil {
		set.Full = map[string]json.RawMessage{}
	}

	if set.Fields[root] == nil {
		set.Fields[root] = map[string]fieldSchema{}
	}

	return walkSchemaStruct(set, root, "", t, s.Properties, local)
}

// walkSchemaStruct adds to set the properties of struct t under prefix, and
// the properties of each field whose type is a struct under local.
func walkSchemaStruct(
	set *schemaSet,
	root, prefix string,
	t reflect.Type,
	props map[string]apiextensionsv1.JSONSchemaProps,
	local string,
) error {
	fields := jsonFields(t)
	for _, name := range slices.Sorted(maps.Keys(props)) {
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}

		// The readers in the app split a path on its dots.
		if strings.Contains(name, ".") {
			return fmt.Errorf("%s: %s has a dot in its name", root, path)
		}

		fieldType, ok := fields[name]
		if !ok {
			return fmt.Errorf("%s: %s is in the CRD schema and not in the Go type %s", root, path, t.Name())
		}

		prop := props[name]
		entry, err := shallowSchema(prop)
		if err != nil {
			return fmt.Errorf("%s: %s: %w", root, path, err)
		}
		set.Fields[root][path] = entry

		if slices.Contains(fullSchemaNames, name) {
			if err := keepFullSchema(set, name, prop); err != nil {
				return fmt.Errorf("%s: %s: %w", root, path, err)
			}
		}

		next := fieldType
		for next.Kind() == reflect.Pointer {
			next = next.Elem()
		}

		if next.Kind() != reflect.Struct || !inModule(next.PkgPath(), local) {
			continue
		}

		if err := walkSchemaStruct(set, root, path, next, prop.Properties, local); err != nil {
			return err
		}
	}

	return nil
}

// shallowSchema returns the type of p without its properties.
func shallowSchema(p apiextensionsv1.JSONSchemaProps) (fieldSchema, error) {
	s := fieldSchema{
		Type:        p.Type,
		Minimum:     p.Minimum,
		Maximum:     p.Maximum,
		Format:      p.Format,
		IntOrString: p.XIntOrString,
	}

	for _, v := range p.Enum {
		var value string
		if err := json.Unmarshal(v.Raw, &value); err != nil {
			return fieldSchema{}, fmt.Errorf("the enum value %s is not a string", v.Raw)
		}
		s.Enum = append(s.Enum, value)
	}

	if p.Items != nil && p.Items.Schema != nil {
		items, err := shallowSchema(*p.Items.Schema)
		if err != nil {
			return fieldSchema{}, fmt.Errorf("items: %w", err)
		}
		s.Items = &items
	}

	return s, nil
}

// keepFullSchema stores p without descriptions as the full schema of name.
// It refuses a schema that differs from the one stored for name.
func keepFullSchema(set *schemaSet, name string, p apiextensionsv1.JSONSchemaProps) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("encoding the schema: %w", err)
	}

	var schema any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return fmt.Errorf("decoding the schema: %w", err)
	}

	// encoding/json sorts the keys of a map, so equal schemas give equal bytes.
	canonical, err := json.Marshal(withoutDescriptions(schema))
	if err != nil {
		return fmt.Errorf("encoding the schema: %w", err)
	}

	if kept, ok := set.Full[name]; ok && !bytes.Equal(kept, canonical) {
		return fmt.Errorf("the schema differs from the schema of an earlier field %s", name)
	}
	set.Full[name] = canonical

	return nil
}

// withoutDescriptions returns v with each description keyword removed, at
// every depth. A property named description has an object value, so it stays.
func withoutDescriptions(v any) any {
	switch v := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, value := range v {
			if _, text := value.(string); key == "description" && text {
				continue
			}
			out[key] = withoutDescriptions(value)
		}

		return out
	case []any:
		out := make([]any, len(v))
		for i, value := range v {
			out[i] = withoutDescriptions(value)
		}

		return out
	default:
		return v
	}
}
