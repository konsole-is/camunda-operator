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

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// componentCond builds a component condition with the given ocf status.
func componentCond(
	condType string,
	status metav1.ConditionStatus,
	reason component.Status,
	message string,
) metav1.Condition {
	return metav1.Condition{Type: condType, Status: status, Reason: string(reason), Message: message}
}

func TestAggregate(t *testing.T) {
	tests := []struct {
		name        string
		components  []metav1.Condition
		wantStatus  metav1.ConditionStatus
		wantReason  string
		wantMessage string
	}{
		{
			name:        "no components is Unknown",
			wantStatus:  metav1.ConditionFalse,
			wantReason:  string(component.Unknown),
			wantMessage: "No component has reported yet",
		},
		{
			name: "all healthy mirrors a healthy component",
			components: []metav1.Condition{
				componentCond("CredentialsReady", metav1.ConditionTrue, component.Healthy, "Component is healthy."),
				componentCond("ElasticsearchReady", metav1.ConditionTrue, component.Healthy, "Component is healthy."),
			},
			wantStatus:  metav1.ConditionTrue,
			wantReason:  v1.ReasonHealthy,
			wantMessage: "CredentialsReady: Component is healthy.",
		},
		{
			name: "a converging component outranks a healthy one",
			components: []metav1.Condition{
				componentCond("CredentialsReady", metav1.ConditionTrue, component.Healthy, "Component is healthy."),
				componentCond(
					"ElasticsearchReady",
					metav1.ConditionFalse,
					component.AliveCreating,
					"yellow while converging",
				),
			},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  string(component.AliveCreating),
			wantMessage: "ElasticsearchReady: yellow while converging",
		},
		{
			name: "down outranks degraded outranks creating",
			components: []metav1.Condition{
				componentCond("A", metav1.ConditionFalse, component.AliveCreating, "creating"),
				componentCond("B", metav1.ConditionFalse, component.Down, "red"),
				componentCond("C", metav1.ConditionFalse, component.Degraded, "yellow"),
			},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  string(component.Down),
			wantMessage: "B: red",
		},
		{
			name: "a suspended component is mirrored as True",
			components: []metav1.Condition{
				componentCond("CredentialsReady", metav1.ConditionTrue, component.Healthy, "Component is healthy."),
				componentCond(
					"ElasticsearchReady",
					metav1.ConditionTrue,
					component.Suspended,
					"node set scaled to zero",
				),
			},
			wantStatus:  metav1.ConditionTrue,
			wantReason:  string(component.Suspended),
			wantMessage: "ElasticsearchReady: node set scaled to zero",
		},
		{
			name: "suspending is not yet suspended",
			components: []metav1.Condition{
				componentCond("ElasticsearchReady", metav1.ConditionFalse, component.Suspending, "scaling down"),
			},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  string(component.Suspending),
			wantMessage: "ElasticsearchReady: scaling down",
		},
		{
			name: "an error outranks everything",
			components: []metav1.Condition{
				componentCond("A", metav1.ConditionFalse, component.Down, "red"),
				componentCond("B", metav1.ConditionFalse, component.Error, "apply failed"),
			},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  string(component.Error),
			wantMessage: "B: apply failed",
		},
		{
			name: "the first component wins a tie",
			components: []metav1.Condition{
				componentCond("A", metav1.ConditionFalse, component.AliveCreating, "first"),
				componentCond("B", metav1.ConditionFalse, component.AliveCreating, "second"),
			},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  string(component.AliveCreating),
			wantMessage: "A: first",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := Aggregate(tt.components, 4)

			assert.Equal(t, v1.ConditionReady, cond.Type)
			assert.Equal(t, tt.wantStatus, cond.Status)
			assert.Equal(t, tt.wantReason, cond.Reason)
			assert.Equal(t, tt.wantMessage, cond.Message)
			assert.Equal(t, int64(4), cond.ObservedGeneration)
		})
	}
}
