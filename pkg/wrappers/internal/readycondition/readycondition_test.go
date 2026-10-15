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

// objectWith returns an Object at generation 2 that the operator observed,
// with the given Ready condition, or none when status is empty.
func objectWith(status metav1.ConditionStatus, reason, message string) Object {
	o := Object{Kind: "ElasticsearchCluster", Generation: 2, ObservedGeneration: 2}
	if status != "" {
		o.Conditions = []metav1.Condition{{
			Type: apiv1.ConditionReady, Status: status, Reason: reason, Message: message, ObservedGeneration: 2,
		}}
	}

	return o
}

func TestConverging(t *testing.T) {
	t.Parallel()

	stale := objectWith(metav1.ConditionTrue, string(component.Healthy), "")
	stale.ObservedGeneration = 1

	tests := []struct {
		name           string
		op             concepts.ConvergingOperation
		object         Object
		want           concepts.AliveConvergingStatus
		reasonContains string
	}{
		{
			name:           "Ready=True is Healthy",
			op:             concepts.ConvergingOperationUpdated,
			object:         objectWith(metav1.ConditionTrue, string(component.Healthy), ""),
			want:           concepts.AliveConvergingStatusHealthy,
			reasonContains: "ElasticsearchCluster is ready",
		},
		{
			name:           "missing Ready is Creating on the first apply",
			op:             concepts.ConvergingOperationCreated,
			object:         objectWith("", "", ""),
			want:           concepts.AliveConvergingStatusCreating,
			reasonContains: "has not reported Ready yet",
		},
		{
			name:   "missing Ready is Updating afterwards",
			op:     concepts.ConvergingOperationUpdated,
			object: objectWith("", "", ""),
			want:   concepts.AliveConvergingStatusUpdating,
		},
		{
			name:   "Ready=Unknown is still converging",
			op:     concepts.ConvergingOperationUpdated,
			object: objectWith(metav1.ConditionUnknown, string(component.Unknown), ""),
			want:   concepts.AliveConvergingStatusUpdating,
		},
		{
			name:           "Ready=False with an in-progress reason is still converging",
			op:             concepts.ConvergingOperationUpdated,
			object:         objectWith(metav1.ConditionFalse, string(component.AliveScaling), "2 of 3 nodes"),
			want:           concepts.AliveConvergingStatusUpdating,
			reasonContains: "is not ready (Scaling): 2 of 3 nodes",
		},
		{
			name: "pre-check reason is still converging",
			op:   concepts.ConvergingOperationCreated,
			object: objectWith(
				metav1.ConditionFalse, apiv1.ReasonMissingSecret, "Secret orders-es-es-elastic-user",
			),
			want:           concepts.AliveConvergingStatusCreating,
			reasonContains: "(MissingSecret): Secret orders-es-es-elastic-user",
		},
		{
			name:           "Ready=False with reason Failing is Failing",
			op:             concepts.ConvergingOperationUpdated,
			object:         objectWith(metav1.ConditionFalse, string(component.AliveFailing), "red health"),
			want:           concepts.AliveConvergingStatusFailing,
			reasonContains: "(Failing): red health",
		},
		{
			name:   "Ready=False with reason Down is Failing",
			op:     concepts.ConvergingOperationUpdated,
			object: objectWith(metav1.ConditionFalse, string(component.Down), ""),
			want:   concepts.AliveConvergingStatusFailing,
		},
		{
			name:   "Ready=False with reason Degraded is Failing",
			op:     concepts.ConvergingOperationUpdated,
			object: objectWith(metav1.ConditionFalse, string(component.Degraded), ""),
			want:   concepts.AliveConvergingStatusFailing,
		},
		{
			name:   "Ready=False with reason Error is Failing",
			op:     concepts.ConvergingOperationUpdated,
			object: objectWith(metav1.ConditionFalse, string(component.Error), ""),
			want:   concepts.AliveConvergingStatusFailing,
		},
		{
			name:   "Ready=False with reason ArchiveFailing is Failing",
			op:     concepts.ConvergingOperationUpdated,
			object: objectWith(metav1.ConditionFalse, apiv1.ReasonArchiveFailing, ""),
			want:   concepts.AliveConvergingStatusFailing,
		},
		{
			name:           "suspended is not serving",
			op:             concepts.ConvergingOperationUpdated,
			object:         objectWith(metav1.ConditionTrue, string(component.Suspended), ""),
			want:           concepts.AliveConvergingStatusUpdating,
			reasonContains: "is suspended",
		},
		{
			name:           "Ready of an older generation is Updating",
			op:             concepts.ConvergingOperationUpdated,
			object:         stale,
			want:           concepts.AliveConvergingStatusUpdating,
			reasonContains: "has not observed generation 2 of the ElasticsearchCluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Converging(tt.op, tt.object)

			assert.Equal(t, tt.want, got.Status)
			assert.Contains(t, got.Reason, tt.reasonContains)
		})
	}
}

func TestOperational(t *testing.T) {
	t.Parallel()

	stale := objectWith(metav1.ConditionTrue, string(component.Healthy), "")
	stale.ObservedGeneration = 1

	tests := []struct {
		name   string
		object Object
		want   concepts.OperationalStatus
	}{
		{
			name:   "Ready=True is Operational",
			object: objectWith(metav1.ConditionTrue, string(component.Healthy), ""),
			want:   concepts.OperationalStatusOperational,
		},
		{
			name:   "missing Ready is pending",
			object: objectWith("", "", ""),
			want:   concepts.OperationalStatusPending,
		},
		{
			name:   "pre-check reason is pending",
			object: objectWith(metav1.ConditionFalse, apiv1.ReasonInvalidReference, ""),
			want:   concepts.OperationalStatusPending,
		},
		{
			name:   "in-progress reason is pending",
			object: objectWith(metav1.ConditionFalse, string(component.AliveCreating), ""),
			want:   concepts.OperationalStatusPending,
		},
		{
			name:   "reason Failing is failing",
			object: objectWith(metav1.ConditionFalse, string(component.AliveFailing), ""),
			want:   concepts.OperationalStatusFailing,
		},
		{
			name:   "reason Degraded is failing",
			object: objectWith(metav1.ConditionFalse, string(component.Degraded), ""),
			want:   concepts.OperationalStatusFailing,
		},
		{
			name:   "reason ArchiveFailing is failing",
			object: objectWith(metav1.ConditionFalse, apiv1.ReasonArchiveFailing, ""),
			want:   concepts.OperationalStatusFailing,
		},
		{
			name:   "suspended is pending",
			object: objectWith(metav1.ConditionTrue, string(component.Suspended), ""),
			want:   concepts.OperationalStatusPending,
		},
		{
			name:   "Ready of an older generation is pending",
			object: stale,
			want:   concepts.OperationalStatusPending,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, Operational(tt.object).Status)
		})
	}
}

func TestGrace(t *testing.T) {
	t.Parallel()

	stale := objectWith(metav1.ConditionTrue, string(component.Healthy), "")
	stale.ObservedGeneration = 1

	tests := []struct {
		name   string
		object Object
		want   concepts.GraceStatus
	}{
		{
			name:   "Ready=True is Healthy",
			object: objectWith(metav1.ConditionTrue, string(component.Healthy), ""),
			want:   concepts.GraceStatusHealthy,
		},
		{
			name:   "reason Degraded is Degraded",
			object: objectWith(metav1.ConditionFalse, string(component.Degraded), ""),
			want:   concepts.GraceStatusDegraded,
		},
		{
			name:   "reason Failing is Down",
			object: objectWith(metav1.ConditionFalse, string(component.AliveFailing), ""),
			want:   concepts.GraceStatusDown,
		},
		{
			name:   "pre-check reason is Down",
			object: objectWith(metav1.ConditionFalse, apiv1.ReasonInvalidReference, ""),
			want:   concepts.GraceStatusDown,
		},
		{
			name:   "missing Ready is Down",
			object: objectWith("", "", ""),
			want:   concepts.GraceStatusDown,
		},
		{
			name:   "suspended is Down",
			object: objectWith(metav1.ConditionTrue, string(component.Suspended), ""),
			want:   concepts.GraceStatusDown,
		},
		{
			name:   "Ready of an older generation is Down",
			object: stale,
			want:   concepts.GraceStatusDown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, Grace(tt.object).Status)
		})
	}
}

func TestSuspension(t *testing.T) {
	t.Parallel()

	stale := objectWith(metav1.ConditionTrue, string(component.Suspended), "")
	stale.ObservedGeneration = 1

	tests := []struct {
		name      string
		object    Object
		requested bool
		want      concepts.SuspensionStatus
	}{
		{
			name:   "not requested on the object yet is pending",
			object: objectWith(metav1.ConditionTrue, string(component.Healthy), ""),
			want:   concepts.SuspensionStatusPending,
		},
		{
			name:      "requested and not observed is suspending",
			object:    stale,
			requested: true,
			want:      concepts.SuspensionStatusSuspending,
		},
		{
			name:      "requested and still serving is suspending",
			object:    objectWith(metav1.ConditionTrue, string(component.Healthy), ""),
			requested: true,
			want:      concepts.SuspensionStatusSuspending,
		},
		{
			name:      "requested and Suspending is suspending",
			object:    objectWith(metav1.ConditionFalse, string(component.Suspending), ""),
			requested: true,
			want:      concepts.SuspensionStatusSuspending,
		},
		{
			name:      "requested and Ready=True Suspended is suspended",
			object:    objectWith(metav1.ConditionTrue, string(component.Suspended), ""),
			requested: true,
			want:      concepts.SuspensionStatusSuspended,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, Suspension(tt.object, tt.requested).Status)
		})
	}
}
