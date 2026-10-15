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

package camundamanagementcluster

import (
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1 "github.com/konsole-is/camunda-operator/api/v1"
)

// planeWithReady returns a CamundaManagementCluster at generation 2 that the
// operator observed at observed, with the given Ready condition, or none
// when status is empty.
func planeWithReady(
	observed int64,
	status metav1.ConditionStatus,
	reason, message string,
) *apiv1.CamundaManagementCluster {
	plane := &apiv1.CamundaManagementCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "plane", Namespace: "team-a", Generation: 2},
		Status:     apiv1.CamundaManagementClusterStatus{ObservedGeneration: observed},
	}
	if status != "" {
		plane.Status.Conditions = []metav1.Condition{{
			Type: apiv1.ConditionReady, Status: status, Reason: reason, Message: message,
		}}
	}

	return plane
}

func TestDefaultConvergingStatusHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		plane       *apiv1.CamundaManagementCluster
		wantHealthy bool
		reason      string
	}{
		{
			name:        "Ready=True of the current generation is healthy",
			plane:       planeWithReady(2, metav1.ConditionTrue, "Healthy", ""),
			wantHealthy: true,
		},
		{
			name:        "Ready=True Suspended of the current generation is healthy",
			plane:       planeWithReady(2, metav1.ConditionTrue, string(component.Suspended), ""),
			wantHealthy: true,
		},
		{
			name:  "Ready=True of an older generation is not healthy",
			plane: planeWithReady(1, metav1.ConditionTrue, "Healthy", ""),
		},
		{
			name: "Ready=False carries the reason and the message unchanged",
			plane: planeWithReady(
				2, metav1.ConditionFalse, apiv1.ReasonInvalidReference, "ManagementAuthConfig plane does not exist",
			),
			reason: "InvalidReference: ManagementAuthConfig plane does not exist",
		},
		{
			name:   "Ready=False with reason Failing carries it unchanged",
			plane:  planeWithReady(2, metav1.ConditionFalse, string(component.AliveFailing), "broker 0 crashes"),
			reason: "Failing: broker 0 crashes",
		},
		{
			name:   "missing Ready is not healthy",
			plane:  planeWithReady(2, "", "", ""),
			reason: "CamundaManagementCluster has not reported Ready yet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DefaultConvergingStatusHandler(concepts.ConvergingOperationUpdated, tt.plane)
			require.NoError(t, err)

			assert.Equal(t, tt.wantHealthy, got.Status == concepts.AliveConvergingStatusHealthy)
			if tt.reason != "" {
				assert.Equal(t, tt.reason, got.Reason)
			}
		})
	}
}

func TestDefaultGraceStatusHandler(t *testing.T) {
	t.Parallel()

	healthy, err := DefaultGraceStatusHandler(planeWithReady(2, metav1.ConditionTrue, "Healthy", ""))
	require.NoError(t, err)
	assert.Equal(t, concepts.GraceStatusHealthy, healthy.Status)

	stale, err := DefaultGraceStatusHandler(planeWithReady(1, metav1.ConditionTrue, "Healthy", ""))
	require.NoError(t, err)
	assert.Equal(t, concepts.GraceStatusDown, stale.Status)

	refused, err := DefaultGraceStatusHandler(
		planeWithReady(2, metav1.ConditionFalse, apiv1.ReasonMissingSecret, "Secret orders-license"),
	)
	require.NoError(t, err)
	assert.Equal(t, concepts.GraceStatusDown, refused.Status)
	assert.Equal(t, "MissingSecret: Secret orders-license", refused.Reason)
}

// The claim suspends a CamundaManagementCluster through spec.suspend. The suspension
// of the component leaves the CamundaManagementCluster as it is and keeps it.
func TestSuspensionLeavesTheCamundaManagementCluster(t *testing.T) {
	t.Parallel()

	plane := planeWithReady(2, metav1.ConditionTrue, "Healthy", "")
	before := plane.DeepCopy()
	m := NewMutator(plane)
	require.NoError(t, DefaultSuspendMutationHandler(m))
	require.NoError(t, m.Apply())
	assert.Equal(t, before, plane)

	status, err := DefaultSuspensionStatusHandler(plane)
	require.NoError(t, err)
	assert.Equal(t, concepts.SuspensionStatusSuspended, status.Status)
	assert.False(t, DefaultDeleteOnSuspendHandler(plane))
}
