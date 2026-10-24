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

package claimrequests

import (
	"encoding/json"
	"fmt"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// CheckParameterObjects refuses with NotAllowed the first selection whose
// parameters are not a JSON object. The claim schemas accept any JSON value
// there. The refusal is a *conditions.PreCheckFailure.
func CheckParameterObjects(selections []fleetv1alpha1.PluginSelection) error {
	for _, s := range selections {
		if s.Parameters == nil {
			continue
		}

		var object map[string]json.RawMessage
		if json.Unmarshal(s.Parameters.Raw, &object) != nil {
			return &conditions.PreCheckFailure{
				Reason:  fleetv1alpha1.ReasonNotAllowed,
				Message: fmt.Sprintf("the parameters of the selection of section %q are not a JSON object", s.Section),
			}
		}
	}

	return nil
}
