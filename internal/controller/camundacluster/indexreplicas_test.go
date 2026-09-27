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

package camundacluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/tools/events"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
)

func TestRecordUnplaceableReplicas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		storage   components.Storage
		requested *int32
		want      []string
	}{
		{
			name: "a count the nodes cannot place records a warning",
			storage: components.Storage{
				Type:          v1.SecondaryStorageTypeElasticsearch,
				Elasticsearch: &v1.ElasticsearchStorage{NodeCount: new(int32(1))},
			},
			requested: new(int32(1)),
			want: []string{
				"Warning IndexReplicasExceedNodes indexReplicas 1 needs 2 Elasticsearch nodes, " +
					"but the storage contract names 1. The indices stay yellow",
			},
		},
		{
			name: "the default count always fits the nodes",
			storage: components.Storage{
				Type:          v1.SecondaryStorageTypeElasticsearch,
				Elasticsearch: &v1.ElasticsearchStorage{NodeCount: new(int32(1))},
			},
		},
		{
			name: "a count that fits the nodes records nothing",
			storage: components.Storage{
				Type:          v1.SecondaryStorageTypeElasticsearch,
				Elasticsearch: &v1.ElasticsearchStorage{NodeCount: new(int32(3))},
			},
			requested: new(int32(2)),
		},
		{
			name: "a contract without a node count records nothing",
			storage: components.Storage{
				Type:          v1.SecondaryStorageTypeElasticsearch,
				Elasticsearch: &v1.ElasticsearchStorage{},
			},
			requested: new(int32(5)),
		},
		{
			name:      "a relational storage records nothing",
			storage:   components.Storage{Type: v1.SecondaryStorageTypeRDBMS},
			requested: new(int32(5)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := events.NewFakeRecorder(4)
			r := &CamundaClusterReconciler{EventRecorder: recorder}

			in := components.Input{Storage: tt.storage}
			in.Effective.IndexReplicas = tt.requested

			r.recordUnplaceableReplicas(&v1.CamundaCluster{}, in)
			close(recorder.Events)

			got := make([]string, 0, len(recorder.Events))
			for e := range recorder.Events {
				got = append(got, e)
			}
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}
