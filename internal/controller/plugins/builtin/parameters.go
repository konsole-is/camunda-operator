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


package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/listtype"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/objectmeta"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	utiljson "k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// checkParameters checks parameters against schema as the API server checks
// a custom resource, x-kubernetes-validations included. A nil parameters is
// checked as an empty object. Parameters that fail return a
// *conditions.PreCheckFailure with reason InvalidParameters, whose message
// names each field under spec.parameters. A schema that cannot be read
// returns a plain error, because it is a defect of the registration.
func checkParameters(ctx context.Context, schemaJSON, parameters *apiextensionsv1.JSON) error {
	var props apiextensionsv1.JSONSchemaProps
	if err := json.Unmarshal(schemaJSON.Raw, &props); err != nil {
		return fmt.Errorf("decoding the parameter schema: %w", err)
	}

	var internal apiextensions.JSONSchemaProps
	if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(
		&props,
		&internal,
		nil,
	); err != nil {
		return fmt.Errorf("converting the parameter schema: %w", err)
	}

	validator, _, err := validation.NewSchemaValidator(&internal)
	if err != nil {
		return fmt.Errorf("reading the parameter schema: %w", err)
	}

	// The schema validator does not run x-kubernetes-validations, the list
	// types, or the checks of an embedded resource.
	structural, err := schema.NewStructural(&internal)
	if err != nil {
		return fmt.Errorf("reading the parameter schema: %w", err)
	}

	value := any(map[string]any{})
	if parameters != nil && len(parameters.Raw) > 0 {
		// The API server decodes an integer as int64, as utiljson does.
		if err := utiljson.Unmarshal(parameters.Raw, &value); err != nil {
			return fmt.Errorf("decoding the parameters: %w", err)
		}
	}

	path := field.NewPath("spec", "parameters")
	errs := validation.ValidateCustomResource(path, value, validator)
	if rules := cel.NewValidator(structural, false, celconfig.PerCallLimit); rules != nil {
		ruleErrs, _ := rules.Validate(ctx, path, structural, value, nil, celconfig.RuntimeCELCostBudget)
		errs = append(errs, ruleErrs...)
	}
	if object, ok := value.(map[string]any); ok {
		errs = append(errs, listtype.ValidateListSetsAndMaps(path, structural, object)...)
	}
	errs = append(errs, objectmeta.Validate(ctx, path, value, structural, false)...)
	if len(errs) == 0 {
		return nil
	}

	messages := make([]string, 0, len(errs))
	for _, e := range errs {
		messages = append(messages, e.Error())
	}

	return &conditions.PreCheckFailure{
		Reason:  pluginsv1alpha1.ReasonInvalidParameters,
		Message: conditions.BoundMessage(strings.Join(messages, ". ")),
	}
}
