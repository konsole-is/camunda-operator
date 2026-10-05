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

package logicalbackup_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
)

func TestClusterConvergedRequiresReadyAtTheCurrentGeneration(t *testing.T) {
	ready := func(status metav1.ConditionStatus, generation int64) []metav1.Condition {
		return []metav1.Condition{{
			Type: v1.ConditionReady, Status: status, Reason: v1.ReasonHealthy, ObservedGeneration: generation,
		}}
	}
	tests := []struct {
		name       string
		observed   int64
		conditions []metav1.Condition
		converged  bool
		message    string
	}{
		{
			name:       "Ready at the current generation",
			observed:   2,
			conditions: ready(metav1.ConditionTrue, 2),
			converged:  true,
		},
		{name: "the operator has not observed the spec", observed: 1, conditions: ready(metav1.ConditionTrue, 2)},
		{name: "Ready describes an older generation", observed: 2, conditions: ready(metav1.ConditionTrue, 1)},
		{name: "the cluster is not Ready", observed: 2, conditions: ready(metav1.ConditionFalse, 2)},
		{name: "the cluster reports no Ready", observed: 2, message: "Ready absent"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cc", Generation: 2}}
			cluster.Status.ObservedGeneration = tt.observed
			cluster.Status.Conditions = tt.conditions

			failure := logicalbackup.ClusterConverged(cluster)
			if tt.converged {
				assert.Nil(t, failure)
				return
			}
			require.NotNil(t, failure)
			assert.Equal(t, v1.ReasonProgressing, failure.Reason, "a rollout ends on its own: a wait")
			assert.Contains(t, failure.Message, "has not converged")
			assert.Contains(t, failure.Message, tt.message)
		})
	}
}
