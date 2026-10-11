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

package databaseserver

import (
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"

	apiv1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/internal/readycondition"
)

// DefaultSuspendMutationHandler sets spec.suspend, which stops the
// PostgreSQL instances and keeps their data volumes.
func DefaultSuspendMutationHandler(m *Mutator) error {
	m.Edit(func(server *apiv1.DatabaseServer) error {
		server.Spec.Suspend = true

		return nil
	})

	return nil
}

// DefaultSuspensionStatusHandler reports Suspended once the applied
// DatabaseServer asks for suspension, the operator observed that
// generation, and Ready is True with the reason Suspended.
func DefaultSuspensionStatusHandler(server *apiv1.DatabaseServer) (concepts.SuspensionStatusWithReason, error) {
	return readycondition.Suspension(observed(server), server.Spec.Suspend), nil
}

// DefaultDeleteOnSuspendHandler keeps the DatabaseServer on suspension. A
// deleted DatabaseServer erases its data.
func DefaultDeleteOnSuspendHandler(_ *apiv1.DatabaseServer) bool {
	return false
}
