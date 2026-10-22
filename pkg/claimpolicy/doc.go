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

// Package claimpolicy holds the policy rules that both claim controllers
// apply, as pure functions: the environment of a namespace, the snapshot of
// the policy, the allow-lists, the order and the inputs of the requests, and
// the fields that the slots fill. No function reads the API.
//
// Every refusal is a *conditions.PreCheckFailure with a reason of the fleet
// group or v1.ReasonInvalidReference. Any other error means that a value of a
// registration or a plugin is malformed.
//
// The table tests read their cases from test/fixtures/claimpolicy. The fleet
// manager runs its own policy code against the cases of six of its folders.
package claimpolicy

import (
	"fmt"

	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

func refuse(reason, format string, args ...any) *conditions.PreCheckFailure {
	return &conditions.PreCheckFailure{Reason: reason, Message: fmt.Sprintf(format, args...)}
}
