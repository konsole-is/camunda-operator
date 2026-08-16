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
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// staged is one component and the condition it staged on the owner.
type staged struct {
	condType string
	status   metav1.ConditionStatus
	reason   component.Status
	message  string
}

// ownerWith returns an owner that carries the given staged component
// conditions, and the matching components in the same order.
func ownerWith(t *testing.T, stagedConds ...staged) (component.OperatorCRD, []*component.Component) {
	t.Helper()

	owner := &v1.Database{}
	owner.Generation = 4
	comps := make([]*component.Component, 0, len(stagedConds))
	for _, sc := range stagedConds {
		comp, err := component.NewComponentBuilder().
			WithName(sc.condType).
			WithConditionType(component.ConditionType(sc.condType)).
			Build()
		require.NoError(t, err)
		comps = append(comps, comp)
		if sc.reason == "" {
			continue // registered but never reported
		}
		owner.Status.Conditions = append(owner.Status.Conditions, metav1.Condition{
			Type: sc.condType, Status: sc.status, Reason: string(sc.reason), Message: sc.message,
		})
	}
	return owner, comps
}

func TestAggregate(t *testing.T) {
	tests := []struct {
		name        string
		staged      []staged
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
			name:        "a component that has not reported is Unknown",
			staged:      []staged{{condType: "BindingsReady"}},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  string(component.Unknown),
			wantMessage: "BindingsReady: Component has not been reconciled yet.",
		},
		{
			name: "all healthy mirrors a healthy component",
			staged: []staged{
				{"CredentialsReady", metav1.ConditionTrue, component.Healthy, "Component is healthy."},
				{"ElasticsearchReady", metav1.ConditionTrue, component.Healthy, "Component is healthy."},
			},
			wantStatus:  metav1.ConditionTrue,
			wantReason:  v1.ReasonHealthy,
			wantMessage: "CredentialsReady: Component is healthy.",
		},
		{
			name: "a converging component outranks a healthy one",
			staged: []staged{
				{"CredentialsReady", metav1.ConditionTrue, component.Healthy, "Component is healthy."},
				{"ElasticsearchReady", metav1.ConditionFalse, component.AliveCreating, "yellow while converging"},
			},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  string(component.AliveCreating),
			wantMessage: "ElasticsearchReady: yellow while converging",
		},
		{
			name: "down outranks degraded outranks creating",
			staged: []staged{
				{"A", metav1.ConditionFalse, component.AliveCreating, "creating"},
				{"B", metav1.ConditionFalse, component.Down, "red"},
				{"C", metav1.ConditionFalse, component.Degraded, "yellow"},
			},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  string(component.Down),
			wantMessage: "B: red",
		},
		{
			name: "a suspended component is mirrored as True",
			staged: []staged{
				{"CredentialsReady", metav1.ConditionTrue, component.Healthy, "Component is healthy."},
				{"ElasticsearchReady", metav1.ConditionTrue, component.Suspended, "node set scaled to zero"},
			},
			wantStatus:  metav1.ConditionTrue,
			wantReason:  string(component.Suspended),
			wantMessage: "ElasticsearchReady: node set scaled to zero",
		},
		{
			name: "suspending is not yet suspended",
			staged: []staged{
				{"ElasticsearchReady", metav1.ConditionFalse, component.Suspending, "scaling down"},
			},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  string(component.Suspending),
			wantMessage: "ElasticsearchReady: scaling down",
		},
		{
			name: "an error outranks everything",
			staged: []staged{
				{"A", metav1.ConditionFalse, component.Down, "red"},
				{"B", metav1.ConditionFalse, component.Error, "apply failed"},
			},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  string(component.Error),
			wantMessage: "B: apply failed",
		},
		{
			name: "the first component wins a tie",
			staged: []staged{
				{"A", metav1.ConditionFalse, component.AliveCreating, "first"},
				{"B", metav1.ConditionFalse, component.AliveCreating, "second"},
			},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  string(component.AliveCreating),
			wantMessage: "A: first",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, comps := ownerWith(t, tt.staged...)

			cond := Aggregate(owner, comps...)

			assert.Equal(t, v1.ConditionReady, cond.Type)
			assert.Equal(t, tt.wantStatus, cond.Status)
			assert.Equal(t, tt.wantReason, cond.Reason)
			assert.Equal(t, tt.wantMessage, cond.Message)
			assert.Equal(t, int64(4), cond.ObservedGeneration)
		})
	}
}
