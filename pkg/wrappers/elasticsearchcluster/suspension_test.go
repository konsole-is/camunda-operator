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

func TestDefaultSuspendMutationHandlerSetsSuspend(t *testing.T) {
	t.Parallel()

	res, err := NewBuilder(clusterWithReady("", "")).
		WithMutation(Mutation{Name: "suspend", Mutate: DefaultSuspendMutationHandler}).
		Build()
	require.NoError(t, err)

	obj, err := res.Preview()
	require.NoError(t, err)

	assert.True(t, obj.(*apiv1.ElasticsearchCluster).Spec.Suspend)
}

func TestDefaultSuspensionStatusHandler(t *testing.T) {
	t.Parallel()

	asked := clusterWithReady(metav1.ConditionTrue, "Healthy")
	asked.Spec.Suspend = true

	done := clusterWithReady(metav1.ConditionTrue, "Suspended")
	done.Spec.Suspend = true

	tests := []struct {
		name    string
		cluster *apiv1.ElasticsearchCluster
		want    concepts.SuspensionStatus
	}{
		{
			name:    "spec.suspend unset is pending",
			cluster: clusterWithReady(metav1.ConditionTrue, "Healthy"),
			want:    concepts.SuspensionStatusPending,
		},
		{
			name:    "spec.suspend set and still serving is suspending",
			cluster: asked,
			want:    concepts.SuspensionStatusSuspending,
		},
		{
			name:    "spec.suspend set and Ready reason Suspended is suspended",
			cluster: done,
			want:    concepts.SuspensionStatusSuspended,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DefaultSuspensionStatusHandler(tt.cluster)
			require.NoError(t, err)

			assert.Equal(t, tt.want, got.Status)
		})
	}
}

func TestDefaultDeleteOnSuspendHandlerKeepsTheCluster(t *testing.T) {
	t.Parallel()

	assert.False(t, DefaultDeleteOnSuspendHandler(clusterWithReady("", "")))
}
