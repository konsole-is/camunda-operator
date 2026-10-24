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

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// DecodeAnnotations returns the ServiceAccount annotations of the input
// serviceAccountAnnotations of inputs, or nil when the input is absent or
// empty. A value that is not a JSON object of strings returns a
// *conditions.PreCheckFailure with reason InvalidParameters.
func DecodeAnnotations(inputs map[string]string) (map[string]string, error) {
	raw := inputs[pluginstartup.OutputServiceAccountAnnotations]
	if raw == "" {
		return nil, nil
	}

	var annotations map[string]*string
	invalid := &conditions.PreCheckFailure{
		Reason:  pluginsv1alpha1.ReasonInvalidParameters,
		Message: "spec.inputs." + pluginstartup.OutputServiceAccountAnnotations + " is not a JSON object of strings",
	}
	if err := json.Unmarshal([]byte(raw), &annotations); err != nil || annotations == nil {
		return nil, invalid
	}

	decoded := make(map[string]string, len(annotations))
	for key, value := range annotations {
		if value == nil {
			return nil, invalid
		}
		decoded[key] = *value
	}

	return decoded, nil
}
