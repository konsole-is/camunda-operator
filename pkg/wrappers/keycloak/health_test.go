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

package keycloak

import (
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withConditions returns the fixture with the given conditions reported.
func withConditions(conditions ...KeycloakCondition) *Keycloak {
	kc := testObject()
	kc.Status.Conditions = conditions

	return kc
}

// The Keycloak Operator reports Ready=False for both a rolling update and a
// broken Keycloak, so only HasErrors can mark a failure. Everything else is
// still converging.
func TestDefaultConvergingStatusHandlerMapsTheConditions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		kc     *Keycloak
		op     concepts.ConvergingOperation
		status concepts.AliveConvergingStatus
		reason string
	}{
		{
			name:   "ready",
			kc:     withConditions(KeycloakCondition{Type: ConditionReady, Status: conditionTrue}),
			op:     concepts.ConvergingOperationUpdated,
			status: concepts.AliveConvergingStatusHealthy,
			reason: "Keycloak is ready",
		},
		{
			name: "errors",
			kc: withConditions(
				KeycloakCondition{Type: ConditionReady, Status: conditionTrue},
				KeycloakCondition{
					Type: ConditionHasErrors, Status: conditionTrue, Message: "database unreachable",
				},
			),
			op:     concepts.ConvergingOperationUpdated,
			status: concepts.AliveConvergingStatusFailing,
			reason: "Keycloak reports errors: database unreachable",
		},
		{
			name:   "first apply",
			kc:     testObject(),
			op:     concepts.ConvergingOperationCreated,
			status: concepts.AliveConvergingStatusCreating,
			reason: "Keycloak has not reported readiness yet",
		},
		{
			name: "rolling update",
			kc: withConditions(KeycloakCondition{
				Type: ConditionReady, Status: "False", Message: "waiting for 1 pod",
			}),
			op:     concepts.ConvergingOperationUpdated,
			status: concepts.AliveConvergingStatusUpdating,
			reason: "Keycloak is not ready: waiting for 1 pod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DefaultConvergingStatusHandler(tt.op, tt.kc)

			require.NoError(t, err)
			assert.Equal(t, tt.status, got.Status)
			assert.Equal(t, tt.reason, got.Reason)
		})
	}
}

// A Keycloak serves requests or it does not, so the grace handler never
// reports the degraded state that a partly available cluster would.
func TestDefaultGraceStatusHandlerIsHealthyOrDown(t *testing.T) {
	t.Parallel()

	healthy, err := DefaultGraceStatusHandler(
		withConditions(KeycloakCondition{Type: ConditionReady, Status: conditionTrue}),
	)
	require.NoError(t, err)
	assert.Equal(t, concepts.GraceStatusHealthy, healthy.Status)

	down, err := DefaultGraceStatusHandler(
		withConditions(KeycloakCondition{Type: ConditionReady, Status: "False"}),
	)
	require.NoError(t, err)
	assert.Equal(t, concepts.GraceStatusDown, down.Status)
}

// Convergence and grace read the same object in one reconcile, so a Keycloak
// that reports Ready and HasErrors together must not converge as Failing and
// grade as Healthy.
func TestDefaultGraceStatusHandlerAgreesWithConvergenceOnErrors(t *testing.T) {
	t.Parallel()

	kc := withConditions(
		KeycloakCondition{Type: ConditionReady, Status: conditionTrue},
		KeycloakCondition{
			Type: ConditionHasErrors, Status: conditionTrue, Message: "database unreachable",
		},
	)

	converging, err := DefaultConvergingStatusHandler(concepts.ConvergingOperationUpdated, kc)
	require.NoError(t, err)
	assert.Equal(t, concepts.AliveConvergingStatusFailing, converging.Status)

	grace, err := DefaultGraceStatusHandler(kc)
	require.NoError(t, err)
	assert.Equal(t, concepts.GraceStatusDown, grace.Status)
	assert.Equal(t, "Keycloak reports errors: database unreachable", grace.Reason)
}

// Suspension scales the Keycloak to zero and keeps the custom resource. Every
// realm, client, and user lives in PostgreSQL, so a resume brings the same
// server back.
func TestSuspensionScalesToZeroAndKeepsTheResource(t *testing.T) {
	t.Parallel()

	res, err := NewBuilder(testObject()).Build()
	require.NoError(t, err)
	require.NoError(t, res.Suspend())

	current := testObject()
	current.Spec.Instances = new(int32(2))
	require.NoError(t, res.Mutate(current))

	require.NotNil(t, current.Spec.Instances)
	assert.Equal(t, int32(0), *current.Spec.Instances)
	assert.False(t, res.DeleteOnSuspend())
}

// The component may only report a suspended Keycloak once no pod is left, so
// a scale-down that the Keycloak Operator has not acted on yet is still
// suspending.
func TestDefaultSuspensionStatusHandlerWaitsForThePods(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(kc *Keycloak)
		status concepts.SuspensionStatus
	}{
		{
			name:   "not scaled down",
			mutate: func(kc *Keycloak) { kc.Spec.Instances = new(int32(1)) },
			status: concepts.SuspensionStatusPending,
		},
		{
			name: "generation not observed",
			mutate: func(kc *Keycloak) {
				kc.Generation = 4
				kc.Status.ObservedGeneration = 3
			},
			status: concepts.SuspensionStatusSuspending,
		},
		{
			name:   "pods still running",
			mutate: func(kc *Keycloak) { kc.Status.Instances = 1 },
			status: concepts.SuspensionStatusSuspending,
		},
		{
			name:   "no pods left",
			mutate: func(_ *Keycloak) {},
			status: concepts.SuspensionStatusSuspended,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			kc := testObject()
			kc.Spec.Instances = new(int32(0))
			tt.mutate(kc)

			got, err := DefaultSuspensionStatusHandler(kc)

			require.NoError(t, err)
			assert.Equal(t, tt.status, got.Status)
		})
	}
}
