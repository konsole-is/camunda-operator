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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// Every restore kind embeds RestoreProgress with json:",inline", so the fields
// of the shared struct keep the names they had on the status itself.
// controller-gen flattens the same way encoding/json does, so a status that
// nests them here nests them in the CRD schema too.
func TestRestoreProgressStaysInlineInTheStatus(t *testing.T) {
	t.Parallel()

	status := v1.PointInTimeRestoreStatus{
		Phase: v1.PointInTimeRestoreCompleted,
		RestoreProgress: v1.RestoreProgress{
			Brokers:            3,
			PrimaryJobNames:    []string{"r-pitr-0", "r-pitr-1", "r-pitr-2"},
			ObservedGeneration: 7,
		},
	}

	payload, err := json.Marshal(status)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(payload, &raw))
	assert.Equal(t, float64(3), raw["brokers"])
	assert.Equal(t, float64(7), raw["observedGeneration"])
	assert.NotContains(t, raw, "restoreProgress")
}

// A restore never leaves Completed or Failed. Every other phase is work in
// progress, so a controller that asks Terminal keeps reconciling it.
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

	assert.Equal(t, "PointInTimeRestore", (&v1.PointInTimeRestore{}).GetKind())
}

// The component framework stages conditions through the returned pointer and
// records the generation through the setter. Both reach the promoted fields of
// the embedded RestoreProgress.
func TestRestoreKindsCarryTheFrameworkStatusHooks(t *testing.T) {
	t.Parallel()

	condition := metav1.Condition{Type: v1.ConditionReady, Reason: v1.ReasonProgressing}

	pitr := &v1.PointInTimeRestore{}
	*pitr.GetStatusConditions() = append(*pitr.GetStatusConditions(), condition)
	pitr.SetObservedGeneration(9)
	require.Len(t, pitr.Status.Conditions, 1)
	assert.Equal(t, int64(9), pitr.Status.ObservedGeneration)
}

// The reasons a restore reports are API surface: users match them with
// kubectl wait, and the operators above import this module to gate on them.
func TestSharedRestoreReasons(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "ClusterNotSuspended", v1.ReasonClusterNotSuspended)
	assert.Equal(t, "ClusterClaimed", v1.ReasonClusterClaimed)
	assert.Equal(t, "IncompatibleTarget", v1.ReasonIncompatibleTarget)
}

func TestPointInTimeRestoreReasonsAreStable(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "PitrUnavailable", v1.ReasonPitrUnavailable)
	assert.Equal(t, "SharedServer", v1.ReasonSharedServer)
	assert.Equal(t, "DatabaseNotRestored", v1.ReasonDatabaseNotRestored)
}

// Both logical restore kinds share one phase vocabulary, because their phase
// values are the same. The two kinds arrive after this type does.
func TestLogicalRestorePhasesAreStable(t *testing.T) {
	t.Parallel()

	assert.Equal(t, v1.LogicalRestorePhase("Pending"), v1.LogicalRestorePending)
	assert.Equal(
		t, v1.LogicalRestorePhase("ValidatingCompatibility"), v1.LogicalRestoreValidatingCompatibility,
	)
	assert.Equal(
		t,
		v1.LogicalRestorePhase("RestoringSecondaryStorage"),
		v1.LogicalRestoreRestoringSecondaryStorage,
	)
	assert.Equal(
		t, v1.LogicalRestorePhase("RestoringPrimaryStorage"), v1.LogicalRestoreRestoringPrimaryStorage,
	)
	assert.Equal(t, v1.LogicalRestorePhase("Completed"), v1.LogicalRestoreCompleted)
	assert.Equal(t, v1.LogicalRestorePhase("Failed"), v1.LogicalRestoreFailed)
}
