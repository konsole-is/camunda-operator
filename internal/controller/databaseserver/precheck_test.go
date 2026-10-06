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

package databaseserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The recovery answers on the contract, so a rollback that cut over keeps it
// while it is unanswered, also when the cluster it cut over to is taken. The
// reconcile reads that cluster twice, and an owner that takes it between the
// two reads must not cost the server the contract it answers on.
func TestContractWithdrawalReason(t *testing.T) {
	t.Parallel()

	const taken = "taken by another owner"
	completed := metav1.Now()

	tests := []struct {
		name     string
		cluster  string
		recovery *v1.DatabaseServerRecoveryStatus
		expected string
	}{
		{
			name:     "no rollback",
			cluster:  "camunda",
			expected: taken,
		},
		{
			name:     "a rollback that has not cut over",
			cluster:  "camunda",
			recovery: &v1.DatabaseServerRecoveryStatus{Cluster: "camunda-r1"},
			expected: taken,
		},
		{
			name:    "a rollback that cut over",
			cluster: "camunda-r1",
			recovery: &v1.DatabaseServerRecoveryStatus{
				Cluster:         "camunda-r1",
				PreviousCluster: "camunda",
			},
			expected: "",
		},
		{
			name:    "a rollback that is answered",
			cluster: "camunda-r1",
			recovery: &v1.DatabaseServerRecoveryStatus{
				Cluster:     "camunda-r1",
				CompletedAt: &completed,
			},
			expected: taken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := &v1.DatabaseServer{
				ObjectMeta: metav1.ObjectMeta{Name: "camunda", Namespace: "camunda-ns"},
				Status:     v1.DatabaseServerStatus{Cluster: tt.cluster, Recovery: tt.recovery},
			}

			assert.Equal(t, tt.expected, contractWithdrawalReason(server, derivedCluster{taken: taken}))
		})
	}
}
