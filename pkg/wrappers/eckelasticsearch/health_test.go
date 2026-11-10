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

package eckelasticsearch

import (
	"testing"

	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// esWithHealth returns a fixture that reports health and the given number
// of Ready pods.
func esWithHealth(health esv1.ElasticsearchHealth, readyPods int32) *esv1.Elasticsearch {
	es := testObject()
	es.Status.Health = health
	es.Status.AvailableNodes = readyPods
	return es
}

func TestDefaultConvergingStatusHandler(t *testing.T) {
	t.Parallel()

	threeNodes := func(es *esv1.Elasticsearch) {
		es.Spec.NodeSets = []esv1.NodeSet{{Name: "default", Count: 3}}
	}

	tests := []struct {
		name      string
		op        concepts.ConvergingOperation
		health    esv1.ElasticsearchHealth
		readyPods int32
		fixture   func(*esv1.Elasticsearch)
		want      concepts.AliveConvergingStatus
	}{
		{
			name: "green with a Ready pod is healthy",
			op:   concepts.ConvergingOperationNone, health: esv1.ElasticsearchGreenHealth, readyPods: 1,
			want: concepts.AliveConvergingStatusHealthy,
		},
		{
			name: "green stays healthy while updating",
			op:   concepts.ConvergingOperationUpdated, health: esv1.ElasticsearchGreenHealth, readyPods: 1,
			want: concepts.AliveConvergingStatusHealthy,
		},
		{
			name: "green with one of three pods Ready is healthy",
			op:   concepts.ConvergingOperationNone, health: esv1.ElasticsearchGreenHealth, readyPods: 1,
			fixture: threeNodes,
			want:    concepts.AliveConvergingStatusHealthy,
		},
		{
			name: "green with no Ready pod on create is creating",
			op:   concepts.ConvergingOperationCreated, health: esv1.ElasticsearchGreenHealth, readyPods: 0,
			want: concepts.AliveConvergingStatusCreating,
		},
		{
			name: "green with no Ready pod is updating",
			op:   concepts.ConvergingOperationNone, health: esv1.ElasticsearchGreenHealth, readyPods: 0,
			want: concepts.AliveConvergingStatusUpdating,
		},
		{
			name: "green with no Ready pod on update is updating",
			op:   concepts.ConvergingOperationUpdated, health: esv1.ElasticsearchGreenHealth, readyPods: 0,
			want: concepts.AliveConvergingStatusUpdating,
		},
		{
			name: "yellow on create is creating",
			op:   concepts.ConvergingOperationCreated, health: esv1.ElasticsearchYellowHealth, readyPods: 1,
			want: concepts.AliveConvergingStatusCreating,
		},
		{
			name: "yellow on update is updating",
			op:   concepts.ConvergingOperationUpdated, health: esv1.ElasticsearchYellowHealth, readyPods: 1,
			want: concepts.AliveConvergingStatusUpdating,
		},
		{
			name: "yellow with no change is updating",
			op:   concepts.ConvergingOperationNone, health: esv1.ElasticsearchYellowHealth, readyPods: 1,
			want: concepts.AliveConvergingStatusUpdating,
		},
		{
			name: "red is failing",
			op:   concepts.ConvergingOperationNone, health: esv1.ElasticsearchRedHealth, readyPods: 1,
			want: concepts.AliveConvergingStatusFailing,
		},
		{
			name: "unknown health on first apply is creating",
			op:   concepts.ConvergingOperationCreated, health: esv1.ElasticsearchUnknownHealth, readyPods: 1,
			want: concepts.AliveConvergingStatusCreating,
		},
		{
			name: "unknown health on a formed cluster is failing",
			op:   concepts.ConvergingOperationNone, health: esv1.ElasticsearchUnknownHealth, readyPods: 1,
			want: concepts.AliveConvergingStatusFailing,
		},
		{
			name: "unknown health on update is failing",
			op:   concepts.ConvergingOperationUpdated, health: esv1.ElasticsearchUnknownHealth, readyPods: 1,
			want: concepts.AliveConvergingStatusFailing,
		},
		{
			name: "unreported health on first apply is creating",
			op:   concepts.ConvergingOperationCreated, health: "", readyPods: 1,
			want: concepts.AliveConvergingStatusCreating,
		},
		{
			name: "unreported health past first apply is failing",
			op:   concepts.ConvergingOperationNone, health: "", readyPods: 1,
			want: concepts.AliveConvergingStatusFailing,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			es := esWithHealth(tt.health, tt.readyPods)
			if tt.fixture != nil {
				tt.fixture(es)
			}

			got, err := DefaultConvergingStatusHandler(tt.op, es)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.Status)
			assert.NotEmpty(t, got.Reason)
		})
	}
}

func TestDefaultConvergingStatusHandlerNamesTheMissingReadyPod(t *testing.T) {
	t.Parallel()

	got, err := DefaultConvergingStatusHandler(
		concepts.ConvergingOperationNone, esWithHealth(esv1.ElasticsearchGreenHealth, 0),
	)
	require.NoError(t, err)
	assert.Equal(t, "Elasticsearch reports green health, and no Elasticsearch pod is Ready yet", got.Reason)
}

func TestDefaultGraceStatusHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		health    esv1.ElasticsearchHealth
		readyPods int32
		want      concepts.GraceStatus
	}{
		{"green with a Ready pod is healthy", esv1.ElasticsearchGreenHealth, 1, concepts.GraceStatusHealthy},
		{"green with no Ready pod is down", esv1.ElasticsearchGreenHealth, 0, concepts.GraceStatusDown},
		{"yellow is degraded", esv1.ElasticsearchYellowHealth, 1, concepts.GraceStatusDegraded},
		{"red is down", esv1.ElasticsearchRedHealth, 1, concepts.GraceStatusDown},
		{"unknown health is down", esv1.ElasticsearchUnknownHealth, 1, concepts.GraceStatusDown},
		{"unreported health is down", "", 1, concepts.GraceStatusDown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DefaultGraceStatusHandler(esWithHealth(tt.health, tt.readyPods))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.Status)
			assert.NotEmpty(t, got.Reason)
		})
	}
}

func TestDefaultGraceStatusHandlerNamesTheMissingReadyPod(t *testing.T) {
	t.Parallel()

	got, err := DefaultGraceStatusHandler(esWithHealth(esv1.ElasticsearchGreenHealth, 0))
	require.NoError(t, err)
	assert.Equal(t, "Elasticsearch reports green health, and no Elasticsearch pod is Ready", got.Reason)
}

func TestDefaultSuspendMutationHandlerSetsRetainingDeletePolicy(t *testing.T) {
	t.Parallel()

	es := testObject()
	es.Spec.NodeSets = []esv1.NodeSet{{Name: "default", Count: 3}}
	es.Spec.VolumeClaimDeletePolicy = esv1.DeleteOnScaledownAndClusterDeletionPolicy

	mutator := NewMutator(es)
	require.NoError(t, DefaultSuspendMutationHandler(mutator))
	require.NoError(t, mutator.Apply())

	assert.Equal(t, esv1.DeleteOnScaledownOnlyPolicy, es.Spec.VolumeClaimDeletePolicy)
	assert.Equal(t, int32(3), es.Spec.NodeSets[0].Count, "suspension must not scale nodes away")
}

func TestDefaultSuspensionStatusHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		policy             esv1.VolumeClaimDeletePolicy
		generation         int64
		observedGeneration int64
		phase              esv1.ElasticsearchOrchestrationPhase
		want               concepts.SuspensionStatus
	}{
		{
			"a retaining policy that is not applied yet is pending",
			esv1.DeleteOnScaledownAndClusterDeletionPolicy,
			1,
			1,
			"",
			concepts.SuspensionStatusPending,
		},
		{
			"an unpopulated status is pending",
			esv1.DeleteOnScaledownOnlyPolicy,
			1,
			0,
			"",
			concepts.SuspensionStatusPending,
		},
		{"a stale status is pending", esv1.DeleteOnScaledownOnlyPolicy, 3, 2, "", concepts.SuspensionStatusPending},
		{
			"a data migration is pending",
			esv1.DeleteOnScaledownOnlyPolicy,
			3,
			3,
			esv1.ElasticsearchMigratingDataPhase,
			concepts.SuspensionStatusPending,
		},
		{
			"a retaining policy at the current generation is suspended",
			esv1.DeleteOnScaledownOnlyPolicy,
			3,
			3,
			esv1.ElasticsearchReadyPhase,
			concepts.SuspensionStatusSuspended,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			es := testObject()
			es.Spec.VolumeClaimDeletePolicy = tt.policy
			es.Generation = tt.generation
			es.Status.ObservedGeneration = tt.observedGeneration
			es.Status.Phase = tt.phase

			got, err := DefaultSuspensionStatusHandler(es)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.Status)
			assert.NotEmpty(t, got.Reason)
		})
	}
}

func TestDefaultDeleteOnSuspendHandlerDeletes(t *testing.T) {
	t.Parallel()

	assert.True(t, DefaultDeleteOnSuspendHandler(testObject()))
}
