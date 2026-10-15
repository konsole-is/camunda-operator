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

package elasticsearchcluster

import (
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1 "github.com/konsole-is/camunda-operator/api/v1"
)

// clusterWithReady returns an observed ElasticsearchCluster with the given Ready
// condition, or none when status is empty.
func clusterWithReady(status metav1.ConditionStatus, reason string) *apiv1.ElasticsearchCluster {
	esc := &apiv1.ElasticsearchCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "orders-es", Namespace: "team-a", Generation: 1},
		Status:     apiv1.ElasticsearchClusterStatus{ObservedGeneration: 1},
	}
	if status != "" {
		esc.Status.Conditions = []metav1.Condition{{
			Type: apiv1.ConditionReady, Status: status, Reason: reason, ObservedGeneration: 1,
		}}
	}

	return esc
}

func TestDefaultConvergingStatusHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cluster *apiv1.ElasticsearchCluster
		want    concepts.AliveConvergingStatus
	}{
		{
			name:    "Ready=True is Healthy",
			cluster: clusterWithReady(metav1.ConditionTrue, "Healthy"),
			want:    concepts.AliveConvergingStatusHealthy,
		},
		{
			name:    "Ready=False with reason Failing is Failing",
			cluster: clusterWithReady(metav1.ConditionFalse, "Failing"),
			want:    concepts.AliveConvergingStatusFailing,
		},
		{
			name:    "Ready=False while it starts is Creating",
			cluster: clusterWithReady(metav1.ConditionFalse, "Creating"),
			want:    concepts.AliveConvergingStatusCreating,
		},
		{
			name:    "missing Ready is Creating",
			cluster: clusterWithReady("", ""),
			want:    concepts.AliveConvergingStatusCreating,
		},
		{
			name:    "Ready=True Suspended is not Healthy",
			cluster: clusterWithReady(metav1.ConditionTrue, "Suspended"),
			want:    concepts.AliveConvergingStatusCreating,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DefaultConvergingStatusHandler(concepts.ConvergingOperationCreated, tt.cluster)
			require.NoError(t, err)

			assert.Equal(t, tt.want, got.Status)
			assert.Contains(t, got.Reason, "ElasticsearchCluster")
		})
	}
}

func TestDefaultGraceStatusHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cluster *apiv1.ElasticsearchCluster
		want    concepts.GraceStatus
	}{
		{
			name:    "Ready=True is Healthy",
			cluster: clusterWithReady(metav1.ConditionTrue, "Healthy"),
			want:    concepts.GraceStatusHealthy,
		},
		{
			name:    "Ready=False Degraded is Degraded",
			cluster: clusterWithReady(metav1.ConditionFalse, "Degraded"),
			want:    concepts.GraceStatusDegraded,
		},
		{
			name:    "Ready=False Failing is Down",
			cluster: clusterWithReady(metav1.ConditionFalse, "Failing"),
			want:    concepts.GraceStatusDown,
		},
		{
			name:    "missing Ready is Down",
			cluster: clusterWithReady("", ""),
			want:    concepts.GraceStatusDown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DefaultGraceStatusHandler(tt.cluster)
			require.NoError(t, err)

			assert.Equal(t, tt.want, got.Status)
		})
	}
}
