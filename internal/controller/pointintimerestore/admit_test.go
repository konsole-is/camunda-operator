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

package pointintimerestore

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// admitNow is the clock of every case below.
var admitNow = time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)

// ago returns the point d before admitNow.
func ago(d time.Duration) time.Time {
	return admitNow.Add(-d)
}

// contractWith returns a contract with a retention of seven days and the
// given windows.
func contractWith(windows ...v1.RecoveryWindow) *v1.DatabaseServerConfig {
	return &v1.DatabaseServerConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "dbsc", Namespace: "my-cluster-ns"},
		Spec: v1.DatabaseServerConfigSpec{
			PITR: &v1.PITRCapability{
				Enabled:             true,
				RetentionPeriodDays: new(int32(7)),
				Windows:             windows,
			},
		},
	}
}

// closed is a window from from ago to to ago.
func closed(from, to time.Duration) v1.RecoveryWindow {
	return v1.RecoveryWindow{From: metav1.NewTime(ago(from)), To: new(metav1.NewTime(ago(to)))}
}

// open is a window from from ago that the server still writes, lag behind the
// present.
func open(from, lag time.Duration) v1.RecoveryWindow {
	return v1.RecoveryWindow{From: metav1.NewTime(ago(from)), Lag: &metav1.Duration{Duration: lag}}
}

func TestPitrAvailableChecksTheWindows(t *testing.T) {
	t.Parallel()

	// Two archives with a gap between them: the server wrote the first from
	// five days ago to three days ago, and writes the second since two days
	// ago, six minutes behind the present.
	twoArchives := []v1.RecoveryWindow{closed(5*day, 3*day), open(2*day, 6*time.Minute)}
	failing := closed(5*day, 3*day)
	failing.UnverifiedFrom = new(metav1.NewTime(ago(4 * day)))

	tests := []struct {
		name     string
		windows  []v1.RecoveryWindow
		want     time.Time
		accepted bool
	}{
		{name: "a point inside a closed window", windows: twoArchives, want: ago(4 * day), accepted: true},
		{name: "the start of a window", windows: twoArchives, want: ago(5 * day), accepted: true},
		{name: "a point inside an open window", windows: twoArchives, want: ago(time.Hour), accepted: true},
		{name: "the end of an open window", windows: twoArchives, want: ago(6 * time.Minute), accepted: true},
		{name: "an open window without lag reaches the present", windows: []v1.RecoveryWindow{
			{From: metav1.NewTime(ago(day))},
		}, want: admitNow, accepted: true},
		{name: "a point before the first window", windows: twoArchives, want: ago(6 * day)},
		{name: "a point in the gap between two windows", windows: twoArchives, want: ago(60 * time.Hour)},
		{name: "the end of a closed window", windows: twoArchives, want: ago(3 * day)},
		{name: "a point beyond the lag of an open window", windows: twoArchives, want: ago(5 * time.Minute)},
		{name: "a point after unverifiedFrom", windows: []v1.RecoveryWindow{failing}, want: ago(3*day + time.Hour)},
		{name: "unverifiedFrom itself", windows: []v1.RecoveryWindow{failing}, want: ago(4 * day), accepted: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			failure := pitrAvailable(contractWith(tt.windows...), tt.want, admitNow)
			if tt.accepted {
				assert.Nil(t, failure)
				return
			}

			require.NotNil(t, failure)
			assert.Equal(t, v1.ReasonPitrUnavailable, failure.Reason)
			assert.Contains(t, failure.Message, tt.want.UTC().Format(time.RFC3339))
			assert.Contains(t, failure.Message, "my-cluster-ns/dbsc")
		})
	}
}

// The refusal names the windows the server can restore, so the reader picks a
// point inside one of them.
func TestPitrAvailableNamesTheReachableWindows(t *testing.T) {
	t.Parallel()

	located := closed(5*day, 3*day)
	located.Location = "s3://camunda-backups/clusters/my-cluster-db"
	failure := pitrAvailable(contractWith(located, open(2*day, 6*time.Minute)), ago(60*time.Hour), admitNow)

	require.NotNil(t, failure)
	assert.Contains(t, failure.Message, fmt.Sprintf(
		"from %s to %s at s3://camunda-backups/clusters/my-cluster-db",
		ago(5*day).Format(time.RFC3339), ago(3*day).Format(time.RFC3339),
	))
	assert.Contains(t, failure.Message, fmt.Sprintf(
		"from %s to 6m0s before now", ago(2*day).Format(time.RFC3339),
	))
}

// A contract can hold 256 windows, and the refusal goes into a condition
// message of bounded size. It names the newest windows and counts the rest.
func TestPitrAvailableBoundsTheRefusal(t *testing.T) {
	t.Parallel()

	windows := make([]v1.RecoveryWindow, 0, 256)
	for i := 256; i > 0; i-- {
		start := time.Duration(i) * 20 * time.Minute
		window := closed(start, start-10*time.Minute)
		window.Location = "s3://camunda-backups/clusters/a-server-name-of-some-length-r" + fmt.Sprint(i)
		windows = append(windows, window)
	}

	failure := pitrAvailable(contractWith(windows...), ago(25*time.Minute), admitNow)

	require.NotNil(t, failure)
	assert.Contains(t, failure.Message, "248 older windows")
	assert.Contains(t, failure.Message, "-r1")
	assert.NotContains(t, failure.Message, "-r256")
	assert.Less(t, len(failure.Message), 4096)
}

// A contract without windows keeps the rules of the retention period and the
// present.
func TestPitrAvailableWithoutWindows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		want     time.Time
		contains string
	}{
		{name: "a point inside the retention period", want: ago(time.Second)},
		{name: "the present", want: admitNow},
		{name: "a point in the future", want: admitNow.Add(time.Second), contains: "future"},
		{name: "a point older than the retention period", want: ago(8 * day), contains: "7 days"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			failure := pitrAvailable(contractWith(), tt.want, admitNow)
			if tt.contains == "" {
				assert.Nil(t, failure)
				return
			}

			require.NotNil(t, failure)
			assert.Equal(t, v1.ReasonPitrUnavailable, failure.Reason)
			assert.Contains(t, failure.Message, tt.contains)
		})
	}
}

// Windows narrow the retention period, never widen it: a point inside a window
// and older than the retention period is still refused.
func TestPitrAvailableKeepsTheRetentionPeriodWithWindows(t *testing.T) {
	t.Parallel()

	failure := pitrAvailable(contractWith(closed(10*day, 2*day)), ago(8*day), admitNow)

	require.NotNil(t, failure)
	assert.Contains(t, failure.Message, "7 days")
}
