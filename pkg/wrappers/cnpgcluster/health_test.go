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

package cnpgcluster

import (
	"testing"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// clusterWith returns a Cluster that asks for instances and reports phase
// with ready ready instance pods.
func clusterWith(instances int, phase string, ready int) *cnpgv1.Cluster {
	return &cnpgv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-object", Namespace: "test-ns"},
		Spec:       cnpgv1.ClusterSpec{Instances: instances},
		Status:     cnpgv1.ClusterStatus{Phase: phase, ReadyInstances: ready},
	}
}

func TestDefaultConvergingStatusHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		op             concepts.ConvergingOperation
		cluster        *cnpgv1.Cluster
		expected       concepts.AliveConvergingStatus
		reasonContains string
	}{
		{
			name:           "healthy phase with every instance ready",
			op:             concepts.ConvergingOperationUpdated,
			cluster:        clusterWith(3, cnpgv1.PhaseHealthy, 3),
			expected:       concepts.AliveConvergingStatusHealthy,
			reasonContains: "3 of 3 instances are ready",
		},
		{
			name:           "healthy phase while an instance is still missing",
			op:             concepts.ConvergingOperationUpdated,
			cluster:        clusterWith(3, cnpgv1.PhaseHealthy, 2),
			expected:       concepts.AliveConvergingStatusUpdating,
			reasonContains: "2 of 3 instances ready",
		},
		{
			name:           "more ready instances than the spec wants during a scale-down",
			op:             concepts.ConvergingOperationUpdated,
			cluster:        clusterWith(2, cnpgv1.PhaseHealthy, 3),
			expected:       concepts.AliveConvergingStatusUpdating,
			reasonContains: "3 of 2 instances ready",
		},
		{
			name:           "unrecoverable cluster",
			op:             concepts.ConvergingOperationUpdated,
			cluster:        clusterWith(3, cnpgv1.PhaseUnrecoverable, 0),
			expected:       concepts.AliveConvergingStatusFailing,
			reasonContains: cnpgv1.PhaseUnrecoverable,
		},
		{
			name:           "invalid definition",
			op:             concepts.ConvergingOperationCreated,
			cluster:        clusterWith(3, cnpgv1.PhaseDefinitionInvalid, 0),
			expected:       concepts.AliveConvergingStatusFailing,
			reasonContains: cnpgv1.PhaseDefinitionInvalid,
		},
		{
			name:           "waiting for a user action",
			op:             concepts.ConvergingOperationUpdated,
			cluster:        clusterWith(3, cnpgv1.PhaseWaitingForUser, 3),
			expected:       concepts.AliveConvergingStatusFailing,
			reasonContains: cnpgv1.PhaseWaitingForUser,
		},
		{
			name:           "failover in progress is not a failure",
			op:             concepts.ConvergingOperationUpdated,
			cluster:        clusterWith(3, cnpgv1.PhaseFailOver, 2),
			expected:       concepts.AliveConvergingStatusUpdating,
			reasonContains: cnpgv1.PhaseFailOver,
		},
		{
			name:           "no phase on the first apply",
			op:             concepts.ConvergingOperationCreated,
			cluster:        clusterWith(3, "", 0),
			expected:       concepts.AliveConvergingStatusCreating,
			reasonContains: "has not reported a phase yet",
		},
		{
			name:           "no phase past the first apply",
			op:             concepts.ConvergingOperationUpdated,
			cluster:        clusterWith(3, "", 0),
			expected:       concepts.AliveConvergingStatusUpdating,
			reasonContains: "no phase yet",
		},
		{
			name:           "creating a replica",
			op:             concepts.ConvergingOperationCreated,
			cluster:        clusterWith(3, cnpgv1.PhaseCreatingReplica, 1),
			expected:       concepts.AliveConvergingStatusUpdating,
			reasonContains: cnpgv1.PhaseCreatingReplica,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			status, err := DefaultConvergingStatusHandler(tt.op, tt.cluster)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, status.Status)
			assert.Contains(t, status.Reason, tt.reasonContains)
		})
	}
}

func TestDefaultGraceStatusHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		cluster        *cnpgv1.Cluster
		expected       concepts.GraceStatus
		reasonContains string
	}{
		{
			name:           "every instance ready",
			cluster:        clusterWith(3, cnpgv1.PhaseHealthy, 3),
			expected:       concepts.GraceStatusHealthy,
			reasonContains: "3 of 3 instances are ready",
		},
		{
			name:           "more ready instances than the spec wants during a scale-down",
			cluster:        clusterWith(2, cnpgv1.PhaseHealthy, 3),
			expected:       concepts.GraceStatusDegraded,
			reasonContains: "3 of 2 instances are ready",
		},
		{
			name:           "waiting for a user action while every instance is ready",
			cluster:        clusterWith(3, cnpgv1.PhaseWaitingForUser, 3),
			expected:       concepts.GraceStatusDegraded,
			reasonContains: cnpgv1.PhaseWaitingForUser,
		},
		{
			name:           "some instances ready",
			cluster:        clusterWith(3, cnpgv1.PhaseCreatingReplica, 1),
			expected:       concepts.GraceStatusDegraded,
			reasonContains: "1 of 3 instances are ready",
		},
		{
			name:           "no instance ready",
			cluster:        clusterWith(3, cnpgv1.PhaseUnrecoverable, 0),
			expected:       concepts.GraceStatusDown,
			reasonContains: cnpgv1.PhaseUnrecoverable,
		},
		{
			name:           "no instance ready and no phase reported",
			cluster:        clusterWith(3, "", 0),
			expected:       concepts.GraceStatusDown,
			reasonContains: "no phase yet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			status, err := DefaultGraceStatusHandler(tt.cluster)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, status.Status)
			assert.Contains(t, status.Reason, tt.reasonContains)
		})
	}
}
