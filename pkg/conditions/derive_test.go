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

package conditions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDeriveReady(t *testing.T) {
	trueCond := func(condType string) metav1.Condition {
		return metav1.Condition{Type: condType, Status: metav1.ConditionTrue, Reason: "Healthy"}
	}
	falseCond := func(condType, reason, message string) metav1.Condition {
		return metav1.Condition{Type: condType, Status: metav1.ConditionFalse, Reason: reason, Message: message}
	}

	tests := []struct {
		name        string
		pre         *PreCheckFailure
		components  []metav1.Condition
		suspended   bool
		wantReason  string
		wantMessage string
	}{
		{
			name: "pre-check failure wins over everything",
			pre: &PreCheckFailure{
				Reason:  ReasonInvalidReference,
				Message: `ElasticsearchClusterPreset "standard" not found`,
			},
			components:  []metav1.Condition{falseCond("ElasticsearchReady", "Creating", "still converging")},
			suspended:   true,
			wantReason:  ReasonInvalidReference,
			wantMessage: `ElasticsearchClusterPreset "standard" not found`,
		},
		{
			name:        "suspended wins over converging components",
			components:  []metav1.Condition{falseCond("ElasticsearchReady", "Suspending", "scaling to zero")},
			suspended:   true,
			wantReason:  ReasonSuspended,
			wantMessage: "Suspended by spec.suspend",
		},
		{
			name: "a non-True component is progressing and named",
			components: []metav1.Condition{
				trueCond("CredentialsReady"),
				falseCond("ElasticsearchReady", "Creating", "Elasticsearch reports yellow health while converging"),
			},
			wantReason:  ReasonProgressing,
			wantMessage: "Waiting for ElasticsearchReady: Elasticsearch reports yellow health while converging",
		},
		{
			name: "an Unknown component counts as not ready",
			components: []metav1.Condition{
				{Type: "BindingsReady", Status: metav1.ConditionUnknown, Reason: "Unknown"},
			},
			wantReason:  ReasonProgressing,
			wantMessage: "Waiting for BindingsReady: Unknown",
		},
		{
			name: "the first non-True component in order is named",
			components: []metav1.Condition{
				falseCond("CredentialsReady", "Creating", "generating credentials"),
				falseCond("ElasticsearchReady", "Creating", "still converging"),
			},
			wantReason:  ReasonProgressing,
			wantMessage: "Waiting for CredentialsReady: generating credentials",
		},
		{
			name: "all components True is healthy",
			components: []metav1.Condition{
				trueCond("CredentialsReady"),
				trueCond("ElasticsearchReady"),
				trueCond("StorageContractReady"),
			},
			wantReason:  ReasonHealthy,
			wantMessage: "All components ready",
		},
		{
			name:        "no components is healthy",
			wantReason:  ReasonHealthy,
			wantMessage: "All components ready",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, message := DeriveReady(tt.pre, tt.components, tt.suspended)

			assert.Equal(t, tt.wantReason, reason)
			assert.Equal(t, tt.wantMessage, message)
		})
	}
}
