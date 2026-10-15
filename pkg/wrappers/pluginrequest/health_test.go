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

package pluginrequest

import (
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// requestWithOutputs returns a PluginRequest of section elasticsearch at
// generation 2, with an OutputsReady condition of the given status for the
// given generation, or none when status is empty.
func requestWithOutputs(status metav1.ConditionStatus, generation int64) *pluginsv1alpha1.PluginRequest {
	request := &pluginsv1alpha1.PluginRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "orders-elasticsearch", Namespace: "team-a", Generation: 2},
		Spec:       pluginsv1alpha1.PluginRequestSpec{Section: "elasticsearch"},
	}
	if status != "" {
		request.Status.Conditions = []metav1.Condition{{
			Type:               pluginsv1alpha1.ConditionOutputsReady,
			Status:             status,
			Reason:             "Provisioning",
			Message:            "the index template is pending",
			ObservedGeneration: generation,
		}}
	}

	return request
}

func TestDefaultOperationalStatusHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		request        *pluginsv1alpha1.PluginRequest
		want           concepts.OperationalStatus
		reasonContains string
	}{
		{
			name:           "OutputsReady=True of the current generation is Operational",
			request:        requestWithOutputs(metav1.ConditionTrue, 2),
			want:           concepts.OperationalStatusOperational,
			reasonContains: "The outputs of section elasticsearch are ready",
		},
		{
			name:           "OutputsReady=False is pending with the reason of the plugin",
			request:        requestWithOutputs(metav1.ConditionFalse, 2),
			want:           concepts.OperationalStatusPending,
			reasonContains: "(Provisioning): the index template is pending",
		},
		{
			name:           "OutputsReady=True of an older generation is pending",
			request:        requestWithOutputs(metav1.ConditionTrue, 1),
			want:           concepts.OperationalStatusPending,
			reasonContains: "for generation 2",
		},
		{
			name:           "no OutputsReady is pending",
			request:        requestWithOutputs("", 0),
			want:           concepts.OperationalStatusPending,
			reasonContains: "has not reported the outputs of section elasticsearch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DefaultOperationalStatusHandler(concepts.ConvergingOperationUpdated, tt.request)
			require.NoError(t, err)

			assert.Equal(t, tt.want, got.Status)
			assert.Contains(t, got.Reason, tt.reasonContains)
		})
	}
}

// Grace never reports Healthy where the operational handler is pending.
func TestDefaultGraceStatusHandler(t *testing.T) {
	t.Parallel()

	ready, err := DefaultGraceStatusHandler(requestWithOutputs(metav1.ConditionTrue, 2))
	require.NoError(t, err)
	assert.Equal(t, concepts.GraceStatusHealthy, ready.Status)

	for _, request := range []*pluginsv1alpha1.PluginRequest{
		requestWithOutputs(metav1.ConditionFalse, 2),
		requestWithOutputs(metav1.ConditionTrue, 1),
		requestWithOutputs("", 0),
	} {
		got, err := DefaultGraceStatusHandler(request)
		require.NoError(t, err)
		assert.Equal(t, concepts.GraceStatusDown, got.Status)
	}
}

// A claim never suspends its requests. The suspension of the component
// leaves the PluginRequest as it is and keeps it.
func TestSuspensionLeavesThePluginRequest(t *testing.T) {
	t.Parallel()

	request := requestWithOutputs(metav1.ConditionTrue, 2)
	before := request.DeepCopy()
	m := NewMutator(request)
	require.NoError(t, DefaultSuspendMutationHandler(m))
	require.NoError(t, m.Apply())
	assert.Equal(t, before, request)

	status, err := DefaultSuspensionStatusHandler(request)
	require.NoError(t, err)
	assert.Equal(t, concepts.SuspensionStatusSuspended, status.Status)
	assert.False(t, DefaultDeleteOnSuspendHandler(request))
}
