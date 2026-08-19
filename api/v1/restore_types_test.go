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

package v1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// A restore never leaves Completed or Failed. Every other phase is work in
// progress, so a controller that asks Terminal keeps reconciling it.
func TestLogicalRestoreTerminal(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		phase v1.LogicalRestorePhase
		want  bool
	}{
		{v1.LogicalRestorePending, false},
		{v1.LogicalRestoreValidatingCompatibility, false},
		{v1.LogicalRestoreRestoringSecondaryStorage, false},
		{v1.LogicalRestoreRestoringPrimaryStorage, false},
		{v1.LogicalRestoreCompleted, true},
		{v1.LogicalRestoreFailed, true},
	} {
		restore := &v1.LogicalRestore{Status: v1.LogicalRestoreStatus{Phase: tc.phase}}
		assert.Equal(t, tc.want, restore.Terminal(), "phase %s", tc.phase)
	}
}

func TestPointInTimeRestoreTerminal(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		phase v1.PointInTimeRestorePhase
		want  bool
	}{
		{v1.PointInTimeRestorePending, false},
		{v1.PointInTimeRestoreValidatingDatabaseState, false},
		{v1.PointInTimeRestoreRestoringPrimaryStorage, false},
		{v1.PointInTimeRestoreCompleted, true},
		{v1.PointInTimeRestoreFailed, true},
	} {
		restore := &v1.PointInTimeRestore{Status: v1.PointInTimeRestoreStatus{Phase: tc.phase}}
		assert.Equal(t, tc.want, restore.Terminal(), "phase %s", tc.phase)
	}
}

func TestRestoreKindsReportTheirKind(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "LogicalRestore", (&v1.LogicalRestore{}).GetKind())
	assert.Equal(t, "PointInTimeRestore", (&v1.PointInTimeRestore{}).GetKind())
}

// The component framework stages conditions through the returned pointer and
// records the generation through the setter. Both must reach the status of
// the resource itself.
func TestRestoreKindsCarryTheFrameworkStatusHooks(t *testing.T) {
	t.Parallel()

	condition := metav1.Condition{Type: v1.ConditionReady, Reason: v1.ReasonProgressing}

	logical := &v1.LogicalRestore{}
	*logical.GetStatusConditions() = append(*logical.GetStatusConditions(), condition)
	logical.SetObservedGeneration(7)
	require.Len(t, logical.Status.Conditions, 1)
	assert.Equal(t, int64(7), logical.Status.ObservedGeneration)

	pitr := &v1.PointInTimeRestore{}
	*pitr.GetStatusConditions() = append(*pitr.GetStatusConditions(), condition)
	pitr.SetObservedGeneration(9)
	require.Len(t, pitr.Status.Conditions, 1)
	assert.Equal(t, int64(9), pitr.Status.ObservedGeneration)
}

// The reasons a restore reports are API surface: users match them with
// kubectl wait, and the operators above import this module to gate on them.
func TestRestoreReasonsAreStable(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "ClusterNotSuspended", v1.ReasonClusterNotSuspended)
	assert.Equal(t, "IncompatibleTarget", v1.ReasonIncompatibleTarget)
	assert.Equal(t, "PitrUnavailable", v1.ReasonPitrUnavailable)
	assert.Equal(t, "SharedServer", v1.ReasonSharedServer)
	assert.Equal(t, "DatabaseNotRestored", v1.ReasonDatabaseNotRestored)
}
