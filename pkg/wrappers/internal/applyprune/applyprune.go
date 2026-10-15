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

// Package applyprune applies an operator kind with server-side apply and
// sends only the fields that hold a value. A typed object encodes every
// struct field, so an apply of it would own fields that its writer never
// set, and another writer could not own them.
package applyprune

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewClient wraps c so that a server-side apply of a T sends the content
// that Prune returns. It decodes the answer of the API server back into the
// T, so the caller reads the applied object as it does without the wrapper.
// Every other call passes through unchanged.
func NewClient[T client.Object](c client.Client) client.Client {
	return &pruningClient[T]{Client: c}
}

type pruningClient[T client.Object] struct {
	client.Client
}

// Patch sends the pruned content of a server-side apply of a T, and passes
// every other patch through.
func (c *pruningClient[T]) Patch(
	ctx context.Context,
	obj client.Object,
	patch client.Patch,
	opts ...client.PatchOption,
) error {
	if _, ok := obj.(T); !ok || patch.Type() != types.ApplyPatchType {
		return c.Client.Patch(ctx, obj, patch, opts...)
	}

	content, err := Prune(obj)
	if err != nil {
		return err
	}

	if err := c.Client.Patch(ctx, content, patch, opts...); err != nil {
		return fmt.Errorf("applying %T %q: %w", obj, obj.GetName(), err)
	}

	return decodeInto(content, obj)
}

// decodeInto replaces obj with the typed form of content.
func decodeInto(content *unstructured.Unstructured, obj client.Object) error {
	applied := reflect.New(reflect.TypeOf(obj).Elem())
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(content.Object, applied.Interface()); err != nil {
		return fmt.Errorf("decoding applied %T %q: %w", obj, obj.GetName(), err)
	}

	reflect.ValueOf(obj).Elem().Set(applied.Elem())

	return nil
}

// Prune returns the content that a server-side apply of obj sends: obj
// without its status and without every value that holds nothing. A value
// holds nothing when it is null, an empty string, an empty list, an empty
// map, or a struct whose fields all hold nothing. A set pointer holds a
// value even when it points to an empty struct, as in
// identityProvider.oidc: {}. A number and a boolean always hold a value.
// An entry of a map and an item of a list stay, because a key is a value.
// Content that the Go type does not describe field by field, such as
// free-form JSON, stays as it is.
func Prune(obj client.Object) (*unstructured.Unstructured, error) {
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, fmt.Errorf("converting %T %q: %w", obj, obj.GetName(), err)
	}

	delete(content, "status")
	pruneStruct(content, reflect.TypeOf(obj).Elem())

	return &unstructured.Unstructured{Object: content}, nil
}

// pruneStruct deletes from content, the encoding of a struct of type t,
// every field that holds nothing. A key that names no field of t stays.
func pruneStruct(content map[string]any, t reflect.Type) {
	fields := jsonFields(t)
	for key, value := range content {
		field, known := fields[key]
		if known && !prune(value, field) {
			delete(content, key)
		}
	}
}

// jsonFields maps the JSON names of the fields of struct t to their types.
// The fields of an embedded struct without a name, such as TypeMeta, count
// as fields of t.
func jsonFields(t reflect.Type) map[string]reflect.Type {
	fields := map[string]reflect.Type{}
	for f := range t.Fields() {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		switch {
		case name == "-" || !f.IsExported():
			continue
		case name == "" && f.Anonymous:
			maps.Copy(fields, jsonFields(f.Type))

			continue
		case name == "":
			name = f.Name
		}

		fields[name] = f.Type
	}

	return fields
}

// prune deletes the empty values inside value, the encoding of a value of
// type t, and reports whether value still holds something.
func prune(value any, t reflect.Type) bool {
	set := t.Kind() == reflect.Pointer
	if set {
		t = t.Elem()
	}

	switch v := value.(type) {
	case nil:
		return false
	case string:
		return set || v != ""
	case map[string]any:
		switch t.Kind() {
		case reflect.Struct:
			pruneStruct(v, t)
		case reflect.Map:
			for _, entry := range v {
				prune(entry, t.Elem())
			}
		}

		return set || len(v) > 0
	case []any:
		if t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
			for _, item := range v {
				prune(item, t.Elem())
			}
		}

		return set || len(v) > 0
	}

	return true
}
