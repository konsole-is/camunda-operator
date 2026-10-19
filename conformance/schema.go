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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsvalidation "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/listtype"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/objectmeta"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utiljson "k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
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

	parsed, err := parseSchema(ctx, reg.Spec.Parameters.Raw)
	if err != nil {
		return err
	}

	validator, _, err := validation.NewSchemaValidator(parsed)
	if err != nil {
		return fmt.Errorf("spec.parameters: %w", err)
	}

	// The schema validator does not run x-kubernetes-validations, the list
	// types, or the checks of an embedded resource.
	structural, err := schema.NewStructural(parsed)
	if err != nil {
		return fmt.Errorf("spec.parameters: %w", err)
	}
	rules := cel.NewValidator(structural, false, celconfig.PerCallLimit)

	var errs []error
	for _, tc := range s.cfg.Cases {
		value := any(map[string]any{})
		if tc.Parameters != nil && len(tc.Parameters.Raw) > 0 {
			// The API server decodes an integer as int64, as utiljson does.
			if err := utiljson.Unmarshal(tc.Parameters.Raw, &value); err != nil {
				errs = append(errs, fmt.Errorf("case %q: parameters: %w", tc.Name, err))
				continue
			}
		}

		path := field.NewPath("parameters")
		fieldErrs := validation.ValidateCustomResource(path, value, validator)
		if rules != nil {
			ruleErrs, _ := rules.Validate(ctx, path, structural, value, nil, celconfig.RuntimeCELCostBudget)
			fieldErrs = append(fieldErrs, ruleErrs...)
		}
		if object, ok := value.(map[string]any); ok {
			fieldErrs = append(fieldErrs, listtype.ValidateListSetsAndMaps(path, structural, object)...)
		}
		fieldErrs = append(fieldErrs, objectmeta.Validate(ctx, path, value, structural, false)...)
		if err := fieldErrs.ToAggregate(); err != nil {
			errs = append(errs, fmt.Errorf("case %q: %w", tc.Name, err))
		}
	}

	return errors.Join(errs...)
}

// parseSchema returns raw as a schema that the API server accepts as the
// root schema of a CRD, with x-fleet-template as the one extra keyword. Each
// error names its field under spec.parameters.
func parseSchema(ctx context.Context, raw []byte) (*apiextensions.JSONSchemaProps, error) {
	// utiljson keeps an integer as int64, where encoding/json rounds it.
	var tree any
	if err := utiljson.Unmarshal(raw, &tree); err != nil {
		return nil, fmt.Errorf("spec.parameters: %w", err)
	}

	// The decode drops an unknown keyword, so it is found first.
	var errs []error
	walkSchemas("spec.parameters", tree, func(path string, node map[string]any) {
		errs = append(errs, unknownFields(path, node, schemaKeywords, fleetTemplateKey)...)
		delete(node, fleetTemplateKey)

		rules, _ := node["x-kubernetes-validations"].([]any)
		for i, rule := range rules {
			if fields, ok := rule.(map[string]any); ok {
				rulePath := fmt.Sprintf("%s.x-kubernetes-validations[%d]", path, i)
				errs = append(errs, unknownFields(rulePath, fields, ruleKeywords)...)
			}
		}

		if docs, ok := node["externalDocs"].(map[string]any); ok {
			errs = append(errs, unknownFields(path+".externalDocs", docs, externalDocsKeywords)...)
		}
	})
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	stripped, err := json.Marshal(tree)
	if err != nil {
		return nil, fmt.Errorf("spec.parameters: %w", err)
	}

	var v1Schema apiextensionsv1.JSONSchemaProps
	if err := json.Unmarshal(stripped, &v1Schema); err != nil {
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

	if err := validateAsCRDField(ctx, &out); err != nil {
		return nil, err
	}

	// The CRD validation sees the schema as a field, so the rules of a root
	// schema run here.
	structural, err := schema.NewStructural(&out)
	if err != nil {
		return nil, fmt.Errorf("spec.parameters: %w", err)
	}

	root := field.NewPath("spec", "parameters")
	rootErrs := schema.ValidateStructural(root, structural)
	if out.Nullable {
		rootErrs = append(rootErrs, field.Forbidden(root.Child("nullable"), "nullable cannot be true at the root"))
	}
	if err := rootErrs.ToAggregate(); err != nil {
		return nil, err
	}

	return &out, nil
}

// The keywords of a schema, of a validation rule, and of external docs.
var (
	schemaKeywords       = jsonFields[apiextensionsv1.JSONSchemaProps]()
	ruleKeywords         = jsonFields[apiextensionsv1.ValidationRule]()
	externalDocsKeywords = jsonFields[apiextensionsv1.ExternalDocumentation]()
)

func jsonFields[T any]() map[string]bool {
	names := map[string]bool{}
	for f := range reflect.TypeFor[T]().Fields() {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		names[name] = true
	}

	return names
}

// unknownFields returns an error for each key of object that is not in
// known or extra.
func unknownFields(path string, object map[string]any, known map[string]bool, extra ...string) []error {
	var errs []error
	for _, key := range slices.Sorted(maps.Keys(object)) {
		if !known[key] && !slices.Contains(extra, key) {
			errs = append(errs, fmt.Errorf("%s: unknown field %q", path, key))
		}
	}

	return errs
}

// walkSchemas calls visit for node and for each schema under it, with its
// path. It walks only the keywords that hold schemas, so a default, an
// enum, or a property name is never read as a schema.
func walkSchemas(path string, node any, visit func(path string, node map[string]any)) {
	schemaNode, ok := node.(map[string]any)
	if !ok {
		return
	}

	visit(path, schemaNode)

	for _, keyword := range []string{"properties", "patternProperties", "definitions", "dependencies"} {
		children, _ := schemaNode[keyword].(map[string]any)
		for _, name := range slices.Sorted(maps.Keys(children)) {
			walkSchemas(path+"."+keyword+"."+name, children[name], visit)
		}
	}

	for _, keyword := range []string{"items", "allOf", "anyOf", "oneOf"} {
		if list, ok := schemaNode[keyword].([]any); ok {
			for i, item := range list {
				walkSchemas(fmt.Sprintf("%s.%s[%d]", path, keyword, i), item, visit)
			}
			continue
		}
		walkSchemas(path+"."+keyword, schemaNode[keyword], visit)
	}

	for _, keyword := range []string{"additionalProperties", "additionalItems", "not"} {
		walkSchemas(path+"."+keyword, schemaNode[keyword], visit)
	}
}

// crdFieldPath is where validateAsCRDField puts the schema in its CRD.
const crdFieldPath = "spec.validation.openAPIV3Schema.properties[parameters]"

// validateAsCRDField runs the validation of the API server on a CRD whose
// one field has the schema props.
func validateAsCRDField(ctx context.Context, props *apiextensions.JSONSchemaProps) error {
	crd := &apiextensions.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "parameters.conformance.plugins.camunda.io"},
		Spec: apiextensions.CustomResourceDefinitionSpec{
			Group: "conformance.plugins.camunda.io",
			Names: apiextensions.CustomResourceDefinitionNames{
				Plural:   "parameters",
				Singular: "parameter",
				Kind:     "Parameter",
				ListKind: "ParameterList",
			},
			Scope:    apiextensions.ClusterScoped,
			Versions: []apiextensions.CustomResourceDefinitionVersion{{Name: "v1", Served: true, Storage: true}},
			Validation: &apiextensions.CustomResourceValidation{
				OpenAPIV3Schema: &apiextensions.JSONSchemaProps{
					Type:       "object",
					Properties: map[string]apiextensions.JSONSchemaProps{"parameters": *props},
				},
			},
			PreserveUnknownFields: new(false),
		},
		Status: apiextensions.CustomResourceDefinitionStatus{StoredVersions: []string{"v1"}},
	}

	fieldErrs := apiextensionsvalidation.ValidateCustomResourceDefinition(ctx, crd)
	errs := make([]error, 0, len(fieldErrs))
	for _, fieldErr := range fieldErrs {
		fieldErr.Field = strings.Replace(fieldErr.Field, crdFieldPath, "spec.parameters", 1)
		errs = append(errs, fieldErr)
	}

	return errors.Join(errs...)
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

// templateDefaultErrors checks the default of each schema under tree that
// is marked x-fleet-template.
func templateDefaultErrors(path string, tree any) []error {
	var errs []error
	walkSchemas(path, tree, func(path string, node map[string]any) {
		marked, _ := node[fleetTemplateKey].(bool)
		def, ok := node["default"]
		if !marked || !ok {
			return
		}

		raw, err := json.Marshal(def)
		if err == nil {
			err = checkJSONTemplates(path+".default", raw, templatePlaceholders)
		}
		errs = append(errs, err)
	})

	return errs
}
