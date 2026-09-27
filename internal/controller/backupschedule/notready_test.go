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

package backupschedule

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestNotReady(t *testing.T) {
	withReady := func(status metav1.ConditionStatus, reason string) *v1.CamundaCluster {
		return &v1.CamundaCluster{Status: v1.CamundaClusterStatus{Conditions: []metav1.Condition{{
			Type:   v1.ConditionReady,
			Status: status,
			Reason: reason,
		}}}}
	}

	tests := []struct {
		name    string
		cluster *v1.CamundaCluster
		want    string
	}{
		{
			name:    "a ready cluster is not skipped",
			cluster: withReady(metav1.ConditionTrue, v1.ReasonHealthy),
			want:    "",
		},
		{
			name:    "a failed cluster names its Ready reason",
			cluster: withReady(metav1.ConditionFalse, v1.ReasonInvalidReference),
			want:    "Ready is False with reason InvalidReference",
		},
		{
			name:    "an unknown Ready is not ready",
			cluster: withReady(metav1.ConditionUnknown, v1.ReasonProgressing),
			want:    "Ready is Unknown with reason Progressing",
		},
		{
			name:    "a cluster without a Ready condition is not ready",
			cluster: &v1.CamundaCluster{},
			want:    "it reports no Ready condition yet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, notReady(tt.cluster))
		})
	}
}
