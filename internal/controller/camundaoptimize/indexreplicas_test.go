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
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/tools/events"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundaoptimize"
)

func TestRecordUnplaceableReplicas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		nodeCount *int32
		requested *int32
		want      []string
	}{
		{
			name:      "a count the nodes cannot place records a warning",
			nodeCount: new(int32(2)),
			requested: new(int32(2)),
			want: []string{
				"Warning IndexReplicasExceedNodes indexReplicas 2 needs 3 Elasticsearch nodes, " +
					"but the storage contract names 2. The indices stay yellow",
			},
		},
		{
			name:      "the largest count names its node count without overflow",
			nodeCount: new(int32(1)),
			requested: new(int32(math.MaxInt32)),
			want: []string{
				"Warning IndexReplicasExceedNodes indexReplicas 2147483647 needs 2147483648 Elasticsearch nodes, " +
					"but the storage contract names 1. The indices stay yellow",
			},
		},
		{
			name:      "the default count always fits the nodes",
			nodeCount: new(int32(1)),
		},
		{
			name:      "a count that fits the nodes records nothing",
			nodeCount: new(int32(2)),
			requested: new(int32(1)),
		},
		{
			name:      "a contract without a node count records nothing",
			requested: new(int32(5)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := events.NewFakeRecorder(4)
			r := &Reconciler{EventRecorder: recorder}

			optimize := &v1.CamundaOptimize{Spec: v1.CamundaOptimizeSpec{IndexReplicas: tt.requested}}
			res := resolved{
				Input: components.Input{
					Optimize: optimize,
					Storage:  v1.ElasticsearchStorage{NodeCount: tt.nodeCount},
				},
			}

			r.recordUnplaceableReplicas(optimize, res)
			close(recorder.Events)

			got := make([]string, 0, len(recorder.Events))
			for e := range recorder.Events {
				got = append(got, e)
			}
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}
