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

package grace

import (
	"flag"
	"testing"
	"time"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestBindFlags(t *testing.T) {
	tests := []struct {
		name          string
		env           map[string]string
		args          []string
		wantWorkload  time.Duration
		wantDatastore time.Duration
	}{
		{
			name:          "defaults without flags or environment",
			wantWorkload:  DefaultWorkload,
			wantDatastore: DefaultDatastore,
		},
		{
			name:          "environment variables set the defaults",
			env:           map[string]string{WorkloadEnv: "5m", DatastoreEnv: "1h"},
			wantWorkload:  5 * time.Minute,
			wantDatastore: time.Hour,
		},
		{
			name:          "flags win over the environment",
			env:           map[string]string{WorkloadEnv: "5m", DatastoreEnv: "1h"},
			args:          []string{"--" + WorkloadFlag + "=2m", "--" + DatastoreFlag + "=0"},
			wantWorkload:  2 * time.Minute,
			wantDatastore: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := flag.NewFlagSet("manager", flag.ContinueOnError)
			var p Periods

			require.NoError(t, p.BindFlags(fs, func(name string) string { return tt.env[name] }))
			require.NoError(t, fs.Parse(tt.args))

			assert.Equal(t, tt.wantWorkload, p.Workload)
			assert.Equal(t, tt.wantDatastore, p.Datastore)
			assert.NoError(t, p.Validate())
		})
	}
}

func TestBindFlagsRejectsAnEnvironmentValueThatIsNoDuration(t *testing.T) {
	for _, name := range []string{WorkloadEnv, DatastoreEnv} {
		t.Run(name, func(t *testing.T) {
			var p Periods
			err := p.BindFlags(
				flag.NewFlagSet("manager", flag.ContinueOnError),
				func(key string) string {
					if key == name {
						return "fifteen minutes"
					}
					return ""
				},
			)

			require.Error(t, err)
			assert.Contains(t, err.Error(), name)
		})
	}
}

func TestValidateRejectsANegativePeriod(t *testing.T) {
	assert.ErrorContains(t, Periods{Workload: -time.Second}.Validate(), WorkloadFlag)
	assert.ErrorContains(t, Periods{Datastore: -time.Second}.Validate(), DatastoreFlag)
}

func TestRemaining(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	period := 10 * time.Minute

	tests := []struct {
		name   string
		status metav1.ConditionStatus
		reason component.Status
		since  time.Duration
		want   time.Duration
	}{
		{
			name: "a progressing component waits for the rest of its period", status: metav1.ConditionFalse,
			reason: component.AliveCreating, since: 4 * time.Minute, want: 6*time.Minute + time.Second,
		},
		{
			name: "a blocked component waits too", status: metav1.ConditionFalse,
			reason: component.GuardBlocked, since: time.Minute, want: 9*time.Minute + time.Second,
		},
		{
			name: "a failing component waits too", status: metav1.ConditionFalse,
			reason: component.AliveFailing, since: 0, want: period + time.Second,
		},
		{
			name: "a ready component does not wait", status: metav1.ConditionTrue,
			reason: component.Healthy, since: time.Minute,
		},
		{
			name: "a suspended component does not wait", status: metav1.ConditionTrue,
			reason: component.Suspended, since: time.Minute,
		},
		{
			name: "a component that reports Down does not wait", status: metav1.ConditionFalse,
			reason: component.Down, since: time.Minute,
		},
		{
			name: "a component that reports Degraded does not wait", status: metav1.ConditionFalse,
			reason: component.Degraded, since: time.Minute,
		},
		{
			name: "a component behind a prerequisite does not wait", status: metav1.ConditionFalse,
			reason: component.PrerequisiteNotMet, since: time.Minute,
		},
		{
			name: "a component whose feature gate failed does not wait", status: metav1.ConditionFalse,
			reason: component.FeatureGateError, since: time.Minute,
		},
		{
			name: "a component that waits to suspend does not wait", status: metav1.ConditionFalse,
			reason: component.PendingSuspension, since: time.Minute,
		},
		{
			name: "a component that suspends does not wait", status: metav1.ConditionFalse,
			reason: component.Suspending, since: time.Minute,
		},
		{
			name: "a component past its period does not wait", status: metav1.ConditionFalse,
			reason: component.AliveUpdating, since: time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, comp := ownerWith(t, "WorkloadReady", metav1.Condition{
				Status:             tt.status,
				Reason:             string(tt.reason),
				LastTransitionTime: metav1.NewTime(now.Add(-tt.since)),
			})

			assert.Equal(t, tt.want, Remaining(owner, period, now, comp))
		})
	}
}

func TestRemainingIsZeroWithoutAPeriod(t *testing.T) {
	now := time.Now()
	owner, comp := ownerWith(t, "WorkloadReady", metav1.Condition{
		Status:             metav1.ConditionFalse,
		Reason:             string(component.AliveCreating),
		LastTransitionTime: metav1.NewTime(now),
	})

	assert.Zero(t, Remaining(owner, 0, now, comp))
}

func TestRemainingIsZeroForAComponentThatNeverReported(t *testing.T) {
	owner := &v1.Database{}
	comp, err := component.NewComponentBuilder().
		WithName("workload").
		WithConditionType("WorkloadReady").
		Build()
	require.NoError(t, err)

	assert.Zero(t, Remaining(owner, time.Minute, time.Now(), comp))
}

func TestRemainingReturnsTheSoonestOfSeveralComponents(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	owner, first := ownerWith(t, "FirstReady", metav1.Condition{
		Status:             metav1.ConditionFalse,
		Reason:             string(component.AliveCreating),
		LastTransitionTime: metav1.NewTime(now.Add(-time.Minute)),
	})
	second, err := component.NewComponentBuilder().
		WithName("second").
		WithConditionType("SecondReady").
		Build()
	require.NoError(t, err)
	owner.Status.Conditions = append(owner.Status.Conditions, metav1.Condition{
		Type:               "SecondReady",
		Status:             metav1.ConditionFalse,
		Reason:             string(component.AliveUpdating),
		LastTransitionTime: metav1.NewTime(now.Add(-8 * time.Minute)),
	})

	assert.Equal(t, 2*time.Minute+time.Second, Remaining(owner, 10*time.Minute, now, first, second))
}

func TestSooner(t *testing.T) {
	assert.Equal(t, time.Second, Sooner(0, time.Second))
	assert.Equal(t, time.Second, Sooner(time.Second, 0))
	assert.Equal(t, time.Second, Sooner(time.Minute, time.Second))
	assert.Zero(t, Sooner(0, 0))
}

// ownerWith returns an owner that carries cond under condType, and the
// component that owns that condition type.
func ownerWith(t *testing.T, condType string, cond metav1.Condition) (*v1.Database, *component.Component) {
	t.Helper()

	comp, err := component.NewComponentBuilder().
		WithName("workload").
		WithConditionType(component.ConditionType(condType)).
		Build()
	require.NoError(t, err)

	cond.Type = condType
	owner := &v1.Database{}
	owner.Status.Conditions = []metav1.Condition{cond}

	return owner, comp
}
