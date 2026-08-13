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

// esWithHealth returns a fixture reporting the given health.
func esWithHealth(health esv1.ElasticsearchHealth) *esv1.Elasticsearch {
	es := testObject()
	es.Status.Health = health
	return es
}

func TestDefaultConvergingStatusHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		op     concepts.ConvergingOperation
		health esv1.ElasticsearchHealth
		want   concepts.AliveConvergingStatus
	}{
		{
			"green is healthy",
			concepts.ConvergingOperationNone, esv1.ElasticsearchGreenHealth,
			concepts.AliveConvergingStatusHealthy,
		},
		{
			"green stays healthy while updating",
			concepts.ConvergingOperationUpdated, esv1.ElasticsearchGreenHealth,
			concepts.AliveConvergingStatusHealthy,
		},
		{
			"yellow on create is creating",
			concepts.ConvergingOperationCreated, esv1.ElasticsearchYellowHealth,
			concepts.AliveConvergingStatusCreating,
		},
		{
			"yellow on update is updating",
			concepts.ConvergingOperationUpdated, esv1.ElasticsearchYellowHealth,
			concepts.AliveConvergingStatusUpdating,
		},
		{
			"yellow with no change is updating",
			concepts.ConvergingOperationNone, esv1.ElasticsearchYellowHealth,
			concepts.AliveConvergingStatusUpdating,
		},
		{
			"red is failing",
			concepts.ConvergingOperationNone, esv1.ElasticsearchRedHealth,
			concepts.AliveConvergingStatusFailing,
		},
		{
			"unknown health on first apply is creating",
			concepts.ConvergingOperationCreated, esv1.ElasticsearchUnknownHealth,
			concepts.AliveConvergingStatusCreating,
		},
		{
			"unknown health on a formed cluster is failing",
			concepts.ConvergingOperationNone, esv1.ElasticsearchUnknownHealth,
			concepts.AliveConvergingStatusFailing,
		},
		{
			"unknown health on update is failing",
			concepts.ConvergingOperationUpdated, esv1.ElasticsearchUnknownHealth,
			concepts.AliveConvergingStatusFailing,
		},
		{
			"unreported health on first apply is creating",
			concepts.ConvergingOperationCreated, "",
			concepts.AliveConvergingStatusCreating,
		},
		{
			"unreported health past first apply is failing",
			concepts.ConvergingOperationNone, "",
			concepts.AliveConvergingStatusFailing,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DefaultConvergingStatusHandler(tt.op, esWithHealth(tt.health))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.Status)
			assert.NotEmpty(t, got.Reason)
		})
	}
}

func TestDefaultGraceStatusHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		health esv1.ElasticsearchHealth
		want   concepts.GraceStatus
	}{
		{"green is healthy", esv1.ElasticsearchGreenHealth, concepts.GraceStatusHealthy},
		{"yellow is degraded", esv1.ElasticsearchYellowHealth, concepts.GraceStatusDegraded},
		{"red is down", esv1.ElasticsearchRedHealth, concepts.GraceStatusDown},
		{"unknown health is down", esv1.ElasticsearchUnknownHealth, concepts.GraceStatusDown},
		{"unreported health is down", "", concepts.GraceStatusDown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DefaultGraceStatusHandler(esWithHealth(tt.health))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.Status)
			assert.NotEmpty(t, got.Reason)
		})
	}
}

func TestDefaultSuspendMutationHandlerZeroesEveryNodeSet(t *testing.T) {
	t.Parallel()

	es := testObject()
	es.Spec.NodeSets = []esv1.NodeSet{
		{Name: "default", Count: 3},
		{Name: "warm", Count: 2},
	}

	mutator := NewMutator(es)
	require.NoError(t, DefaultSuspendMutationHandler(mutator))
	require.NoError(t, mutator.Apply())

	for _, nodeSet := range es.Spec.NodeSets {
		assert.Zero(t, nodeSet.Count, "nodeSet %q", nodeSet.Name)
	}
}

func TestDefaultSuspensionStatusHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		generation         int64
		observedGeneration int64
		availableNodes     int32
		want               concepts.SuspensionStatus
	}{
		{"unpopulated status is not yet suspended", 1, 0, 0, concepts.SuspensionStatusPending},
		{"stale status is not yet suspended", 3, 2, 0, concepts.SuspensionStatusPending},
		{"no available nodes at the current generation is suspended", 3, 3, 0, concepts.SuspensionStatusSuspended},
		{"remaining nodes are suspending", 3, 3, 2, concepts.SuspensionStatusSuspending},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			es := testObject()
			es.Generation = tt.generation
			es.Status.ObservedGeneration = tt.observedGeneration
			es.Status.AvailableNodes = tt.availableNodes

			got, err := DefaultSuspensionStatusHandler(es)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.Status)
			assert.NotEmpty(t, got.Reason)
		})
	}
}
