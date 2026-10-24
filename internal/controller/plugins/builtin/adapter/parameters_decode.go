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

package adapter

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	sigsjson "sigs.k8s.io/json"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// DecodeFields decodes each value of values into the target of its key in
// targets, matched by exact name. A key without a target, or a value that
// does not decode, returns a *conditions.PreCheckFailure that names prefix
// and the key.
func DecodeFields(values map[string]json.RawMessage, prefix string, targets map[string]any) error {
	for _, key := range slices.Sorted(maps.Keys(values)) {
		target, ok := targets[key]
		if !ok {
			return InvalidParameter(prefix+key, fmt.Errorf("unknown field %q", key))
		}
		if err := DecodeStrict(&apiextensionsv1.JSON{Raw: values[key]}, target); err != nil {
			return InvalidParameter(prefix+key, err)
		}
	}

	return nil
}

// DecodeStrict decodes raw into target and refuses an unknown field and a
// field twice. Nil or empty raw decodes nothing.
func DecodeStrict(raw *apiextensionsv1.JSON, target any) error {
	if raw == nil || len(raw.Raw) == 0 {
		return nil
	}

	strict, err := sigsjson.UnmarshalStrict(raw.Raw, target)
	if err != nil {
		return err
	}

	return errors.Join(strict...)
}

// InvalidParameter returns the failure with reason InvalidParameters that
// says field does not decode, with err.
func InvalidParameter(field string, err error) *conditions.PreCheckFailure {
	return &conditions.PreCheckFailure{
		Reason:  pluginsv1alpha1.ReasonInvalidParameters,
		Message: field + " does not decode: " + err.Error(),
	}
}
