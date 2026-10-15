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

package camundaoptimize

import (
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1 "github.com/konsole-is/camunda-operator/api/v1"
)

// optimizeWithReady returns a CamundaOptimize at generation 2 that the
// operator observed at observed, with the given Ready condition, or none
// when status is empty.
func optimizeWithReady(observed int64, status metav1.ConditionStatus, reason string) *apiv1.CamundaOptimize {
	optimize := &apiv1.CamundaOptimize{
		ObjectMeta: metav1.ObjectMeta{Name: "orders-optimize", Namespace: "team-a", Generation: 2},
		Status:     apiv1.CamundaOptimizeStatus{ObservedGeneration: observed},
	}
	if status != "" {
		optimize.Status.Conditions = []metav1.Condition{{Type: apiv1.ConditionReady, Status: status, Reason: reason}}
	}

	return optimize
}

func TestDefaultConvergingStatusHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		optimize *apiv1.CamundaOptimize
		want     concepts.AliveConvergingStatus
	}{
		{
			name:     "Ready=True is Healthy",
			optimize: optimizeWithReady(2, metav1.ConditionTrue, string(component.Healthy)),
			want:     concepts.AliveConvergingStatusHealthy,
		},
		{
			name:     "Ready=False with reason Failing is Failing",
			optimize: optimizeWithReady(2, metav1.ConditionFalse, string(component.AliveFailing)),
			want:     concepts.AliveConvergingStatusFailing,
		},
		{
			name:     "Ready=False in progress is Updating",
			optimize: optimizeWithReady(2, metav1.ConditionFalse, string(component.AliveCreating)),
			want:     concepts.AliveConvergingStatusUpdating,
		},
		{
			name:     "Ready of an older generation is Updating",
			optimize: optimizeWithReady(1, metav1.ConditionTrue, string(component.Healthy)),
			want:     concepts.AliveConvergingStatusUpdating,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DefaultConvergingStatusHandler(concepts.ConvergingOperationUpdated, tt.optimize)
			require.NoError(t, err)

			assert.Equal(t, tt.want, got.Status)
			assert.Contains(t, got.Reason, "CamundaOptimize")
		})
	}
}

func TestDefaultGraceStatusHandler(t *testing.T) {
	t.Parallel()

	healthy, err := DefaultGraceStatusHandler(optimizeWithReady(2, metav1.ConditionTrue, string(component.Healthy)))
	require.NoError(t, err)
	assert.Equal(t, concepts.GraceStatusHealthy, healthy.Status)

	down, err := DefaultGraceStatusHandler(optimizeWithReady(2, metav1.ConditionFalse, apiv1.ReasonInvalidReference))
	require.NoError(t, err)
	assert.Equal(t, concepts.GraceStatusDown, down.Status)
}

// Optimize follows the suspension of its cluster. The suspension of the
// component leaves the CamundaOptimize as it is and keeps it.
func TestSuspensionLeavesTheCamundaOptimize(t *testing.T) {
	t.Parallel()

	optimize := optimizeWithReady(2, metav1.ConditionTrue, string(component.Healthy))
	before := optimize.DeepCopy()
	m := NewMutator(optimize)
	require.NoError(t, DefaultSuspendMutationHandler(m))
	require.NoError(t, m.Apply())
	assert.Equal(t, before, optimize)

	status, err := DefaultSuspensionStatusHandler(optimize)
	require.NoError(t, err)
	assert.Equal(t, concepts.SuspensionStatusSuspended, status.Status)
	assert.False(t, DefaultDeleteOnSuspendHandler(optimize))
}
