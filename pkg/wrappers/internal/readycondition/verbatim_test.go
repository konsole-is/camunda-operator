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

package readycondition

import (
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestVerbatimConverging(t *testing.T) {
	t.Parallel()

	staleFalse := objectWith(metav1.ConditionFalse, apiv1.ReasonInvalidReference, "CamundaRelease 8-9 does not exist")
	staleFalse.ObservedGeneration = 1
	staleTrue := objectWith(metav1.ConditionTrue, string(component.Healthy), "")
	staleTrue.ObservedGeneration = 1
	staleCondition := objectWith(metav1.ConditionTrue, string(component.Healthy), "")
	staleCondition.Conditions[0].ObservedGeneration = 1

	tests := []struct {
		name   string
		op     concepts.ConvergingOperation
		object Object
		want   concepts.AliveConvergingStatus
		reason string
	}{
		{
			name:   "Ready=True is Healthy",
			op:     concepts.ConvergingOperationUpdated,
			object: objectWith(metav1.ConditionTrue, string(component.Healthy), "All resources healthy."),
			want:   concepts.AliveConvergingStatusHealthy,
			reason: "Healthy: All resources healthy.",
		},
		{
			name:   "Ready=True Suspended is Healthy",
			op:     concepts.ConvergingOperationUpdated,
			object: objectWith(metav1.ConditionTrue, string(component.Suspended), ""),
			want:   concepts.AliveConvergingStatusHealthy,
			reason: "Suspended",
		},
		{
			name: "a pre-check reason passes through unchanged",
			op:   concepts.ConvergingOperationCreated,
			object: objectWith(
				metav1.ConditionFalse,
				apiv1.ReasonInvalidReference,
				"CamundaRelease 8-9 does not exist",
			),
			want:   concepts.AliveConvergingStatusCreating,
			reason: "InvalidReference: CamundaRelease 8-9 does not exist",
		},
		{
			name:   "an in-progress reason is Updating after the first apply",
			op:     concepts.ConvergingOperationUpdated,
			object: objectWith(metav1.ConditionFalse, string(component.AliveScaling), "2 of 3 brokers"),
			want:   concepts.AliveConvergingStatusUpdating,
			reason: "Scaling: 2 of 3 brokers",
		},
		{
			name:   "reason Failing is Failing",
			op:     concepts.ConvergingOperationUpdated,
			object: objectWith(metav1.ConditionFalse, string(component.AliveFailing), "broker 0 crashes"),
			want:   concepts.AliveConvergingStatusFailing,
			reason: "Failing: broker 0 crashes",
		},
		{
			name:   "reason Degraded is Failing",
			op:     concepts.ConvergingOperationUpdated,
			object: objectWith(metav1.ConditionFalse, string(component.Degraded), ""),
			want:   concepts.AliveConvergingStatusFailing,
			reason: "Degraded",
		},
		{
			name:   "Ready=Unknown is still converging",
			op:     concepts.ConvergingOperationUpdated,
			object: objectWith(metav1.ConditionUnknown, string(component.Unknown), ""),
			want:   concepts.AliveConvergingStatusUpdating,
			reason: "Unknown",
		},
		{
			name:   "missing Ready is Creating",
			op:     concepts.ConvergingOperationCreated,
			object: objectWith("", "", ""),
			want:   concepts.AliveConvergingStatusCreating,
			reason: "ElasticsearchCluster has not reported Ready yet",
		},
		{
			name:   "Ready=True of an older generation is not Healthy",
			op:     concepts.ConvergingOperationUpdated,
			object: staleTrue,
			want:   concepts.AliveConvergingStatusUpdating,
			reason: "The operator has not observed generation 2 of the ElasticsearchCluster yet. " +
				"Its last Ready condition was Healthy",
		},
		{
			name:   "Ready=True of an older generation beside a current status is not Healthy",
			op:     concepts.ConvergingOperationUpdated,
			object: staleCondition,
			want:   concepts.AliveConvergingStatusUpdating,
			reason: "The operator has not observed generation 2 of the ElasticsearchCluster yet. " +
				"Its last Ready condition was Healthy",
		},
		{
			name:   "Ready=False of an older generation keeps its reason and message",
			op:     concepts.ConvergingOperationUpdated,
			object: staleFalse,
			want:   concepts.AliveConvergingStatusUpdating,
			reason: "The operator has not observed generation 2 of the ElasticsearchCluster yet. " +
				"Its last Ready condition was InvalidReference: CamundaRelease 8-9 does not exist",
		},
		{
			name:   "Ready=True of an older generation is Creating on the first apply",
			op:     concepts.ConvergingOperationCreated,
			object: staleTrue,
			want:   concepts.AliveConvergingStatusCreating,
			reason: "The operator has not observed generation 2 of the ElasticsearchCluster yet. " +
				"Its last Ready condition was Healthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := VerbatimConverging(tt.op, tt.object)

			assert.Equal(t, tt.want, got.Status)
			assert.Equal(t, tt.reason, got.Reason)
		})
	}
}

func TestVerbatimGrace(t *testing.T) {
	t.Parallel()

	stale := objectWith(metav1.ConditionTrue, string(component.Healthy), "")
	stale.ObservedGeneration = 1
	staleCondition := objectWith(metav1.ConditionTrue, string(component.Healthy), "")
	staleCondition.Conditions[0].ObservedGeneration = 1

	tests := []struct {
		name   string
		object Object
		want   concepts.GraceStatus
		reason string
	}{
		{
			name:   "Ready=True is Healthy",
			object: objectWith(metav1.ConditionTrue, string(component.Suspended), ""),
			want:   concepts.GraceStatusHealthy,
			reason: "Suspended",
		},
		{
			name:   "reason Degraded is Degraded",
			object: objectWith(metav1.ConditionFalse, string(component.Degraded), "1 of 3 brokers down"),
			want:   concepts.GraceStatusDegraded,
			reason: "Degraded: 1 of 3 brokers down",
		},
		{
			name:   "a pre-check reason is Down and passes through unchanged",
			object: objectWith(metav1.ConditionFalse, apiv1.ReasonMissingSecret, "Secret orders-license"),
			want:   concepts.GraceStatusDown,
			reason: "MissingSecret: Secret orders-license",
		},
		{
			name:   "missing Ready is Down",
			object: objectWith("", "", ""),
			want:   concepts.GraceStatusDown,
			reason: "ElasticsearchCluster has not reported Ready yet",
		},
		{
			name:   "Ready=True of an older generation is Down",
			object: stale,
			want:   concepts.GraceStatusDown,
		},
		{
			name:   "Ready=True of an older generation beside a current status is Down",
			object: staleCondition,
			want:   concepts.GraceStatusDown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := VerbatimGrace(tt.object)

			assert.Equal(t, tt.want, got.Status)
			if tt.reason != "" {
				assert.Equal(t, tt.reason, got.Reason)
			}
		})
	}
}
