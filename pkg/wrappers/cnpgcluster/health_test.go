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
// with groups volume claim groups and ready ready instance pods. The two
// counts differ while the Cluster is hibernated: CloudNativePG keeps the
// claims and removes the pods.
func clusterWith(instances int, phase string, groups, ready int) *cnpgv1.Cluster {
	return &cnpgv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-object", Namespace: "test-ns"},
		Spec:       cnpgv1.ClusterSpec{Instances: instances},
		Status:     cnpgv1.ClusterStatus{Phase: phase, Instances: groups, ReadyInstances: ready},
	}
}

// hibernated returns cluster with the hibernation condition CloudNativePG
// writes once every instance pod is gone.
func hibernated(cluster *cnpgv1.Cluster, status metav1.ConditionStatus) *cnpgv1.Cluster {
	cluster.Status.Conditions = append(cluster.Status.Conditions, metav1.Condition{
		Type:    HibernationCondition,
		Status:  status,
		Reason:  HibernationConditionReason,
		Message: "Cluster has been hibernated",
	})

	return cluster
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
			cluster:  clusterWith(3, cnpgv1.PhaseHealthy, 3, 3),
			expected: concepts.AliveConvergingStatusHealthy,
		},
		{
			name:     "healthy phase while an instance is still missing",
			op:       concepts.ConvergingOperationUpdated,
			cluster:  clusterWith(3, cnpgv1.PhaseHealthy, 2, 2),
			expected: concepts.AliveConvergingStatusUpdating,
		},
		{
			name:     "unrecoverable cluster",
			op:       concepts.ConvergingOperationUpdated,
			cluster:  clusterWith(3, cnpgv1.PhaseUnrecoverable, 0, 0),
			expected: concepts.AliveConvergingStatusFailing,
		},
		{
			name:     "invalid definition",
			op:       concepts.ConvergingOperationCreated,
			cluster:  clusterWith(3, cnpgv1.PhaseDefinitionInvalid, 0, 0),
			expected: concepts.AliveConvergingStatusFailing,
		},
		{
			name:     "waiting for a user action",
			op:       concepts.ConvergingOperationUpdated,
			cluster:  clusterWith(3, cnpgv1.PhaseWaitingForUser, 3, 3),
			expected: concepts.AliveConvergingStatusFailing,
		},
		{
			name:     "failover in progress is not a failure",
			op:       concepts.ConvergingOperationUpdated,
			cluster:  clusterWith(3, cnpgv1.PhaseFailOver, 2, 2),
			expected: concepts.AliveConvergingStatusUpdating,
		},
		{
			name:     "no phase on the first apply",
			op:       concepts.ConvergingOperationCreated,
			cluster:  clusterWith(3, "", 0, 0),
			expected: concepts.AliveConvergingStatusCreating,
		},
		{
			name:     "no phase past the first apply",
			op:       concepts.ConvergingOperationUpdated,
			cluster:  clusterWith(3, "", 0, 0),
			expected: concepts.AliveConvergingStatusUpdating,
		},
		{
			name:     "creating a replica",
			op:       concepts.ConvergingOperationCreated,
			cluster:  clusterWith(3, cnpgv1.PhaseCreatingReplica, 1, 1),
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
			cluster:  clusterWith(3, cnpgv1.PhaseHealthy, 3, 3),
			expected: concepts.GraceStatusHealthy,
		},
		{
			name:     "some instances ready",
			cluster:  clusterWith(3, cnpgv1.PhaseCreatingReplica, 1, 1),
			expected: concepts.GraceStatusDegraded,
		},
		{
			name:     "no instance ready",
			cluster:  clusterWith(3, cnpgv1.PhaseUnrecoverable, 0, 0),
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
			name:     "pods still ready and no hibernation condition yet",
			cluster:  clusterWith(2, cnpgv1.PhaseHealthy, 2, 2),
			expected: concepts.SuspensionStatusSuspending,
		},
		{
			name:     "no hibernation condition yet and no ready pod",
			cluster:  clusterWith(2, cnpgv1.PhaseHealthy, 2, 0),
			expected: concepts.SuspensionStatusSuspended,
		},
		{
			name:     "volume claim groups remain, no ready pod, hibernation True",
			cluster:  hibernated(clusterWith(2, cnpgv1.PhaseHealthy, 2, 0), metav1.ConditionTrue),
			expected: concepts.SuspensionStatusSuspended,
		},
		{
			name:     "hibernation False while every pod is ready",
			cluster:  hibernated(clusterWith(2, cnpgv1.PhaseHealthy, 2, 2), metav1.ConditionFalse),
			expected: concepts.SuspensionStatusSuspending,
		},
		{
			name:     "hibernation False while the last pod drains",
			cluster:  hibernated(clusterWith(2, cnpgv1.PhaseHealthy, 2, 1), metav1.ConditionFalse),
			expected: concepts.SuspensionStatusSuspending,
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

// TestDefaultSuspendMutationHandlerHibernates proves that suspension asks
// CloudNativePG to hibernate, which removes the pods and keeps the volume
// claims. It must not touch spec.instances: the schema puts a minimum of 1 on
// that field, so a zero would be rejected by the API server.
func TestDefaultSuspendMutationHandlerHibernates(t *testing.T) {
	t.Parallel()

	cluster := clusterWith(3, cnpgv1.PhaseHealthy, 3, 3)
	mutator := NewMutator(cluster)

	require.NoError(t, DefaultSuspendMutationHandler(mutator))
	require.NoError(t, mutator.Apply())

	assert.Equal(t, HibernationOn, cluster.Annotations[HibernationAnnotation])
	assert.Equal(t, 3, cluster.Spec.Instances)
}

// TestSetHibernationOffWritesTheOffValue proves that the resume value is
// available for a caller that has to overwrite a hand-set annotation.
func TestSetHibernationOffWritesTheOffValue(t *testing.T) {
	t.Parallel()

	cluster := clusterWith(3, cnpgv1.PhaseHealthy, 3, 3)
	mutator := NewMutator(cluster)

	mutator.SetHibernation(false)
	require.NoError(t, mutator.Apply())

	assert.Equal(t, HibernationOff, cluster.Annotations[HibernationAnnotation])
}

// TestDefaultDeleteOnSuspendHandlerKeepsTheCluster proves that suspension
// never deletes the Cluster, so the volumes of the server outlive it.
func TestDefaultDeleteOnSuspendHandlerKeepsTheCluster(t *testing.T) {
	t.Parallel()

	assert.False(t, DefaultDeleteOnSuspendHandler(clusterWith(3, cnpgv1.PhaseHealthy, 3, 3)))
}
