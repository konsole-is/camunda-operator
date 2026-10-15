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

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1 "github.com/konsole-is/camunda-operator/api/v1"
)

// scheduleWithReady returns a BackupSchedule at generation 2 that the
// operator observed at observed, with the given Ready condition, or none
// when status is empty.
func scheduleWithReady(observed int64, status metav1.ConditionStatus, reason string) *apiv1.BackupSchedule {
	schedule := &apiv1.BackupSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: "orders-backups", Namespace: "team-a", Generation: 2},
		Status:     apiv1.BackupScheduleStatus{ObservedGeneration: observed},
	}
	if status != "" {
		schedule.Status.Conditions = []metav1.Condition{{Type: apiv1.ConditionReady, Status: status, Reason: reason}}
	}

	return schedule
}

func TestDefaultConvergingStatusHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		schedule *apiv1.BackupSchedule
		want     concepts.AliveConvergingStatus
	}{
		{
			name:     "Ready=True is Healthy",
			schedule: scheduleWithReady(2, metav1.ConditionTrue, string(component.Healthy)),
			want:     concepts.AliveConvergingStatusHealthy,
		},
		{
			name:     "Ready=False with reason Failing is Failing",
			schedule: scheduleWithReady(2, metav1.ConditionFalse, string(component.AliveFailing)),
			want:     concepts.AliveConvergingStatusFailing,
		},
		{
			name:     "Ready=False in progress is Updating",
			schedule: scheduleWithReady(2, metav1.ConditionFalse, string(component.AliveCreating)),
			want:     concepts.AliveConvergingStatusUpdating,
		},
		{
			name:     "Ready of an older generation is Updating",
			schedule: scheduleWithReady(1, metav1.ConditionTrue, string(component.Healthy)),
			want:     concepts.AliveConvergingStatusUpdating,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DefaultConvergingStatusHandler(concepts.ConvergingOperationUpdated, tt.schedule)
			require.NoError(t, err)

			assert.Equal(t, tt.want, got.Status)
			assert.Contains(t, got.Reason, "BackupSchedule")
		})
	}
}

func TestDefaultGraceStatusHandler(t *testing.T) {
	t.Parallel()

	healthy, err := DefaultGraceStatusHandler(scheduleWithReady(2, metav1.ConditionTrue, string(component.Healthy)))
	require.NoError(t, err)
	assert.Equal(t, concepts.GraceStatusHealthy, healthy.Status)

	down, err := DefaultGraceStatusHandler(scheduleWithReady(2, metav1.ConditionFalse, apiv1.ReasonInvalidReference))
	require.NoError(t, err)
	assert.Equal(t, concepts.GraceStatusDown, down.Status)
}

// The suspension of the
// component leaves the BackupSchedule as it is and keeps it.
func TestSuspensionLeavesTheBackupSchedule(t *testing.T) {
	t.Parallel()

	schedule := scheduleWithReady(2, metav1.ConditionTrue, string(component.Healthy))
	before := schedule.DeepCopy()
	m := NewMutator(schedule)
	require.NoError(t, DefaultSuspendMutationHandler(m))
	require.NoError(t, m.Apply())
	assert.Equal(t, before, schedule)

	status, err := DefaultSuspensionStatusHandler(schedule)
	require.NoError(t, err)
	assert.Equal(t, concepts.SuspensionStatusSuspended, status.Status)
	assert.False(t, DefaultDeleteOnSuspendHandler(schedule))
}
