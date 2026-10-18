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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// fleetTemplateKey marks a parameter whose default is a template. The
// contract adds it to OpenAPI v3.
const fleetTemplateKey = "x-fleet-template"

func checkSchema(ctx context.Context, s *suite) error {
	reg, err := s.registration(ctx)
	if err != nil {
		return err
	}

	if reg.Spec.Parameters == nil || len(reg.Spec.Parameters.Raw) == 0 {
		var errs []error
		for _, tc := range s.cfg.Cases {
			if tc.Parameters != nil && len(tc.Parameters.Raw) > 0 {
				errs = append(errs, fmt.Errorf("spec.parameters is empty, and case %q has parameters", tc.Name))
			}
		}
		return errors.Join(errs...)
	}

	parsed, err := parseSchema(reg.Spec.Parameters.Raw)
	if err != nil {
		return err
	}

	validator, _, err := validation.NewSchemaValidator(parsed)
	if err != nil {
		return fmt.Errorf("spec.parameters: %w", err)
	}

	var errs []error
	for _, tc := range s.cfg.Cases {
		value := any(map[string]any{})
		if tc.Parameters != nil && len(tc.Parameters.Raw) > 0 {
			if err := json.Unmarshal(tc.Parameters.Raw, &value); err != nil {
				errs = append(errs, fmt.Errorf("case %q: parameters: %w", tc.Name, err))
				continue
			}
		}

		fieldErrs := validation.ValidateCustomResource(field.NewPath("parameters"), value, validator)
		if err := fieldErrs.ToAggregate(); err != nil {
			errs = append(errs, fmt.Errorf("case %q: %w", tc.Name, err))
		}
	}

	return errors.Join(errs...)
}

// parseSchema returns raw as a schema that the API server accepts for a
// CRD: no unknown keys besides x-fleet-template, and structural. Each error
// names its field under spec.parameters.
func parseSchema(raw []byte) (*apiextensions.JSONSchemaProps, error) {
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, fmt.Errorf("spec.parameters: %w", err)
	}

	// A strict decode refuses the key of the contract.
	stripped, err := json.Marshal(withoutKey(tree, fleetTemplateKey))
	if err != nil {
		return nil, fmt.Errorf("spec.parameters: %w", err)
	}

	var v1Schema apiextensionsv1.JSONSchemaProps
	decoder := json.NewDecoder(bytes.NewReader(stripped))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&v1Schema); err != nil {
		return nil, fmt.Errorf("spec.parameters: %w", err)
	}

	var out apiextensions.JSONSchemaProps
	if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(
		&v1Schema,
		&out,
		nil,
	); err != nil {
		return nil, fmt.Errorf("spec.parameters: %w", err)
	}

	structural, err := schema.NewStructural(&out)
	if err != nil {
		return nil, fmt.Errorf("spec.parameters: %w", err)
	}

	if err := schema.ValidateStructural(field.NewPath("spec", "parameters"), structural).ToAggregate(); err != nil {
		return nil, err
	}

	return &out, nil
}

// withoutKey returns value with each object key named key removed, at any
// depth. It changes value in place.
func withoutKey(value any, key string) any {
	switch v := value.(type) {
	case map[string]any:
		delete(v, key)
		for k, child := range v {
			v[k] = withoutKey(child, key)
		}
	case []any:
		for i, child := range v {
			v[i] = withoutKey(child, key)
		}
	}

	return value
}

func checkParameterTemplates(ctx context.Context, s *suite) error {
	reg, err := s.registration(ctx)
	if err != nil {
		return err
	}

	if reg.Spec.Parameters == nil || len(reg.Spec.Parameters.Raw) == 0 {
		return nil
	}

	var tree any
	if err := json.Unmarshal(reg.Spec.Parameters.Raw, &tree); err != nil {
		return fmt.Errorf("spec.parameters: %w", err)
	}

	return errors.Join(templateDefaultErrors("spec.parameters", tree)...)
}

// templateDefaultErrors checks the default of each schema under node that
// is marked x-fleet-template. It walks only the keywords that hold schemas,
// so a default value or an enum is never read as a schema.
func templateDefaultErrors(path string, node any) []error {
	schemaNode, ok := node.(map[string]any)
	if !ok {
		return nil
	}

	var errs []error
	if marked, _ := schemaNode[fleetTemplateKey].(bool); marked {
		if def, ok := schemaNode["default"]; ok {
			raw, err := json.Marshal(def)
			if err == nil {
				err = checkJSONTemplates(path+".default", raw, templatePlaceholders)
			}
			errs = append(errs, err)
		}
	}

	for _, keyword := range []string{"properties", "patternProperties"} {
		children, _ := schemaNode[keyword].(map[string]any)
		for _, name := range slices.Sorted(maps.Keys(children)) {
			errs = append(errs, templateDefaultErrors(path+"."+keyword+"."+name, children[name])...)
		}
	}

	for _, keyword := range []string{"items", "allOf", "anyOf", "oneOf"} {
		switch child := schemaNode[keyword].(type) {
		case []any:
			for i, item := range child {
				errs = append(errs, templateDefaultErrors(fmt.Sprintf("%s.%s[%d]", path, keyword, i), item)...)
			}
		default:
			errs = append(errs, templateDefaultErrors(path+"."+keyword, child)...)
		}
	}

	for _, keyword := range []string{"additionalProperties", "not"} {
		errs = append(errs, templateDefaultErrors(path+"."+keyword, schemaNode[keyword])...)
	}

	return errs
}
