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

package database

import (
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1 "github.com/konsole-is/camunda-operator/api/v1"
)

// databaseWithReady returns an observed Database with the given Ready
// condition, or none when status is empty.
func databaseWithReady(status metav1.ConditionStatus, reason string) *apiv1.Database {
	db := &apiv1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: "team-a", Generation: 1},
		Status:     apiv1.DatabaseStatus{ObservedGeneration: 1},
	}
	if status != "" {
		db.Status.Conditions = []metav1.Condition{{Type: apiv1.ConditionReady, Status: status, Reason: reason}}
	}

	return db
}

func TestDefaultOperationalStatusHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		database *apiv1.Database
		want     concepts.OperationalStatus
	}{
		{
			name:     "Ready=True is Operational",
			database: databaseWithReady(metav1.ConditionTrue, "Healthy"),
			want:     concepts.OperationalStatusOperational,
		},
		{
			name:     "Ready=False with reason Failing is failing",
			database: databaseWithReady(metav1.ConditionFalse, "Failing"),
			want:     concepts.OperationalStatusFailing,
		},
		{
			name:     "Ready=False before the server publishes is pending",
			database: databaseWithReady(metav1.ConditionFalse, apiv1.ReasonServerIdentityUnknown),
			want:     concepts.OperationalStatusPending,
		},
		{
			name:     "missing Ready is pending",
			database: databaseWithReady("", ""),
			want:     concepts.OperationalStatusPending,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DefaultOperationalStatusHandler(concepts.ConvergingOperationCreated, tt.database)
			require.NoError(t, err)

			assert.Equal(t, tt.want, got.Status)
			assert.Contains(t, got.Reason, "Database")
		})
	}
}

func TestDefaultGraceStatusHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		database *apiv1.Database
		want     concepts.GraceStatus
	}{
		{
			name:     "Ready=True is Healthy",
			database: databaseWithReady(metav1.ConditionTrue, "Healthy"),
			want:     concepts.GraceStatusHealthy,
		},
		{
			name:     "Ready=False is Down",
			database: databaseWithReady(metav1.ConditionFalse, apiv1.ReasonConnectionFailed),
			want:     concepts.GraceStatusDown,
		},
		{
			name:     "missing Ready is Down",
			database: databaseWithReady("", ""),
			want:     concepts.GraceStatusDown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DefaultGraceStatusHandler(tt.database)
			require.NoError(t, err)

			assert.Equal(t, tt.want, got.Status)
		})
	}
}

// A Database runs no pods, so suspension leaves it as it is and keeps it.
func TestSuspensionLeavesTheDatabase(t *testing.T) {
	t.Parallel()

	res, err := NewBuilder(databaseWithReady("", "")).
		WithMutation(Mutation{Name: "suspend", Mutate: DefaultSuspendMutationHandler}).
		Build()
	require.NoError(t, err)

	obj, err := res.Preview()
	require.NoError(t, err)

	assert.Equal(t, databaseWithReady("", "").Spec, obj.(*apiv1.Database).Spec)

	status, err := DefaultSuspensionStatusHandler(databaseWithReady("", ""))
	require.NoError(t, err)

	assert.Equal(t, concepts.SuspensionStatusSuspended, status.Status)
	assert.NotContains(t, status.Reason, "Scaffolded")
	assert.False(t, DefaultDeleteOnSuspendHandler(databaseWithReady("", "")))
}
