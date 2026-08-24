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
// with ready instances ready.
func clusterWith(instances int, phase string, ready int) *cnpgv1.Cluster {
	return &cnpgv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-object", Namespace: "test-ns"},
		Spec:       cnpgv1.ClusterSpec{Instances: instances},
		Status:     cnpgv1.ClusterStatus{Phase: phase, Instances: ready, ReadyInstances: ready},
	}
}

func TestDefaultConvergingStatusHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		op       concepts.ConvergingOperation
		cluster  *cnpgv1.Cluster
		expected concepts.AliveConvergingStatus
	}{
		{
			name:     "healthy phase with every instance ready",
			op:       concepts.ConvergingOperationUpdated,
			cluster:  clusterWith(3, cnpgv1.PhaseHealthy, 3),
			expected: concepts.AliveConvergingStatusHealthy,
		},
		{
			name:     "healthy phase while an instance is still missing",
			op:       concepts.ConvergingOperationUpdated,
			cluster:  clusterWith(3, cnpgv1.PhaseHealthy, 2),
			expected: concepts.AliveConvergingStatusUpdating,
		},
		{
			name:     "unrecoverable cluster",
			op:       concepts.ConvergingOperationUpdated,
			cluster:  clusterWith(3, cnpgv1.PhaseUnrecoverable, 0),
			expected: concepts.AliveConvergingStatusFailing,
		},
		{
			name:     "invalid definition",
			op:       concepts.ConvergingOperationCreated,
			cluster:  clusterWith(3, cnpgv1.PhaseDefinitionInvalid, 0),
			expected: concepts.AliveConvergingStatusFailing,
		},
		{
			name:     "failover in progress is not a failure",
			op:       concepts.ConvergingOperationUpdated,
			cluster:  clusterWith(3, cnpgv1.PhaseFailOver, 2),
			expected: concepts.AliveConvergingStatusUpdating,
		},
		{
			name:     "no phase on the first apply",
			op:       concepts.ConvergingOperationCreated,
			cluster:  clusterWith(3, "", 0),
			expected: concepts.AliveConvergingStatusCreating,
		},
		{
			name:     "no phase past the first apply",
			op:       concepts.ConvergingOperationUpdated,
			cluster:  clusterWith(3, "", 0),
			expected: concepts.AliveConvergingStatusUpdating,
		},
		{
			name:     "creating a replica",
			op:       concepts.ConvergingOperationCreated,
			cluster:  clusterWith(3, cnpgv1.PhaseCreatingReplica, 1),
			expected: concepts.AliveConvergingStatusUpdating,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			status, err := DefaultConvergingStatusHandler(tt.op, tt.cluster)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, status.Status)
			assert.NotEmpty(t, status.Reason)
		})
	}
}

func TestDefaultGraceStatusHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cluster  *cnpgv1.Cluster
		expected concepts.GraceStatus
	}{
		{
			name:     "every instance ready",
			cluster:  clusterWith(3, cnpgv1.PhaseHealthy, 3),
			expected: concepts.GraceStatusHealthy,
		},
		{
			name:     "some instances ready",
			cluster:  clusterWith(3, cnpgv1.PhaseCreatingReplica, 1),
			expected: concepts.GraceStatusDegraded,
		},
		{
			name:     "no instance ready",
			cluster:  clusterWith(3, cnpgv1.PhaseUnrecoverable, 0),
			expected: concepts.GraceStatusDown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			status, err := DefaultGraceStatusHandler(tt.cluster)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, status.Status)
			assert.NotEmpty(t, status.Reason)
		})
	}
}

func TestDefaultSuspensionStatusHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cluster  *cnpgv1.Cluster
		expected concepts.SuspensionStatus
	}{
		{
			name:     "instances still running",
			cluster:  clusterWith(0, cnpgv1.PhaseHealthy, 2),
			expected: concepts.SuspensionStatusSuspending,
		},
		{
			name:     "scaled to zero",
			cluster:  clusterWith(0, cnpgv1.PhaseHealthy, 0),
			expected: concepts.SuspensionStatusSuspended,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			status, err := DefaultSuspensionStatusHandler(tt.cluster)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, status.Status)
			assert.NotEmpty(t, status.Reason)
		})
	}
}

// TestDefaultSuspendMutationHandlerScalesToZero proves that suspension asks
// CloudNativePG for zero instances, which removes the pods and keeps the
// volume claims.
func TestDefaultSuspendMutationHandlerScalesToZero(t *testing.T) {
	t.Parallel()

	cluster := clusterWith(3, cnpgv1.PhaseHealthy, 3)
	mutator := NewMutator(cluster)

	require.NoError(t, DefaultSuspendMutationHandler(mutator))
	require.NoError(t, mutator.Apply())

	assert.Equal(t, 0, cluster.Spec.Instances)
}

// TestDefaultDeleteOnSuspendHandlerKeepsTheCluster proves that suspension
// never deletes the Cluster, so the volumes of the server outlive it.
func TestDefaultDeleteOnSuspendHandlerKeepsTheCluster(t *testing.T) {
	t.Parallel()

	assert.False(t, DefaultDeleteOnSuspendHandler(clusterWith(3, cnpgv1.PhaseHealthy, 3)))
}
