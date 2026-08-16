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
	"strings"
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

// componentName derives a component name from condType for these tests only.
// The real components pick their own names, and this rule does not reproduce
// every one of them: storage-contract reports StorageContractReady. It exists
// so that the name always differs from the condition type, because the
// aggregate message names the component and never the condition type.
func componentName(condType string) string {
	return strings.ToLower(strings.TrimSuffix(condType, "Ready"))
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
			WithName(componentName(sc.condType)).
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
			wantMessage: "No components were aggregated.",
		},
		{
			name:        "a component that has not reported is Unknown",
			staged:      []staged{{condType: "BindingsReady"}},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  string(component.Unknown),
			wantMessage: "bindings: Component has not been reconciled yet.",
		},
		{
			name: "every component True is Ready True",
			staged: []staged{
				{"CredentialsReady", metav1.ConditionTrue, component.Healthy, "Component is healthy."},
				{"ElasticsearchReady", metav1.ConditionTrue, component.Healthy, "Component is healthy."},
			},
			wantStatus:  metav1.ConditionTrue,
			wantReason:  v1.ReasonHealthy,
			wantMessage: "credentials: Component is healthy.",
		},
		{
			name: "a converging component gives the reason and makes Ready False",
			staged: []staged{
				{"CredentialsReady", metav1.ConditionTrue, component.Healthy, "Component is healthy."},
				{"ElasticsearchReady", metav1.ConditionFalse, component.AliveCreating, "yellow while converging"},
			},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  string(component.AliveCreating),
			wantMessage: "elasticsearch: yellow while converging",
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
			wantMessage: "b: red",
		},
		{
			name: "a suspended component next to a healthy one is True",
			staged: []staged{
				{"CredentialsReady", metav1.ConditionTrue, component.Healthy, "Component is healthy."},
				{"ElasticsearchReady", metav1.ConditionTrue, component.Suspended, "node set scaled to zero"},
			},
			wantStatus:  metav1.ConditionTrue,
			wantReason:  string(component.Suspended),
			wantMessage: "elasticsearch: node set scaled to zero",
		},
		{
			// Suspended has a higher framework priority than a failing
			// component, so a status taken from the highest priority alone
			// would report the cluster ready here.
			name: "a suspended component never hides a failing one",
			staged: []staged{
				{"CredentialsReady", metav1.ConditionTrue, component.Suspended, "node set scaled to zero"},
				{
					"ElasticsearchReady",
					metav1.ConditionFalse,
					component.AliveFailing,
					"red for longer than the grace period",
				},
			},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  string(component.AliveFailing),
			wantMessage: "elasticsearch: red for longer than the grace period",
		},
		{
			name: "suspending is not yet suspended",
			staged: []staged{
				{"ElasticsearchReady", metav1.ConditionFalse, component.Suspending, "scaling down"},
			},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  string(component.Suspending),
			wantMessage: "elasticsearch: scaling down",
		},
		{
			name: "an error outranks everything",
			staged: []staged{
				{"A", metav1.ConditionFalse, component.Down, "red"},
				{"B", metav1.ConditionFalse, component.Error, "apply failed"},
			},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  string(component.Error),
			wantMessage: "b: apply failed",
		},
		{
			name: "the first component wins a tie",
			staged: []staged{
				{"A", metav1.ConditionFalse, component.AliveCreating, "first"},
				{"B", metav1.ConditionFalse, component.AliveCreating, "second"},
			},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  string(component.AliveCreating),
			wantMessage: "a: first",
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
