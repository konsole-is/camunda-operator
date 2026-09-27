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

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestElasticsearchStorageIndexReplicas(t *testing.T) {
	cases := map[string]struct {
		nodeCount *int32
		requested *int32
		want      *int32
	}{
		"one node places no replica":      {nodeCount: new(int32(1)), want: new(int32(0))},
		"two nodes place one replica":     {nodeCount: new(int32(2)), want: new(int32(1))},
		"more nodes still default to one": {nodeCount: new(int32(5)), want: new(int32(1))},
		"the consumer setting wins": {
			nodeCount: new(int32(1)),
			requested: new(int32(2)),
			want:      new(int32(2)),
		},
		"zero is a setting of its own": {
			nodeCount: new(int32(3)),
			requested: new(int32(0)),
			want:      new(int32(0)),
		},
		"without a node count, no setting":        {},
		"without a node count, the setting stays": {requested: new(int32(1)), want: new(int32(1))},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			storage := v1.ElasticsearchStorage{NodeCount: tc.nodeCount}

			assert.Equal(t, tc.want, storage.IndexReplicas(tc.requested))
		})
	}
}

// The result is a copy: a caller that changes it leaves the setting of the
// consumer as it was.
func TestElasticsearchStorageIndexReplicasSharesNoMemory(t *testing.T) {
	requested := new(int32(2))
	storage := v1.ElasticsearchStorage{}

	*storage.IndexReplicas(requested) = 7

	assert.Equal(t, int32(2), *requested)
}

func TestElasticsearchStorageReplicasExceedNodes(t *testing.T) {
	cases := map[string]struct {
		nodeCount *int32
		replicas  int32
		want      bool
	}{
		"one replica on one node":             {nodeCount: new(int32(1)), replicas: 1, want: true},
		"no replica on one node":              {nodeCount: new(int32(1)), replicas: 0},
		"one replica on two nodes":            {nodeCount: new(int32(2)), replicas: 1},
		"two replicas on two nodes":           {nodeCount: new(int32(2)), replicas: 2, want: true},
		"an unknown node count never exceeds": {replicas: 4},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			storage := v1.ElasticsearchStorage{NodeCount: tc.nodeCount}

			assert.Equal(t, tc.want, storage.ReplicasExceedNodes(tc.replicas))
		})
	}
}
