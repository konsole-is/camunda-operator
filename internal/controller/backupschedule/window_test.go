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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
)

func TestParseISODuration(t *testing.T) {
	cases := map[string]time.Duration{
		"P7D":        7 * 24 * time.Hour,
		"PT12H":      12 * time.Hour,
		"P1DT6H":     30 * time.Hour,
		"PT15M":      15 * time.Minute,
		"PT1H30M":    90 * time.Minute,
		"PT0.5S":     500 * time.Millisecond,
		"P2DT3H4M5S": 51*time.Hour + 4*time.Minute + 5*time.Second,
	}
	for in, want := range cases {
		got, err := parseISODuration(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}

	for _, in := range []string{"", "P", "PT", "7D", "P1W", "P1M", "P1Y", "P7DT"} {
		_, err := parseISODuration(in)
		assert.Error(t, err, in)
	}
}

func TestTriggerInterval(t *testing.T) {
	daily, err := parseCron("0 2 * * *")
	require.NoError(t, err)

	from := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, 24*time.Hour, triggerInterval(daily, from))
}

func TestRetentionWindowWarning(t *testing.T) {
	policy := func(window, cleanup string) components.PrimaryStorageBackupPolicy {
		return components.PrimaryStorageBackupPolicy{
			RetentionWindow: window, CleanupSchedule: cleanup,
		}
	}

	t.Run("warns when the retained dumps outlive the window", func(t *testing.T) {
		message := retentionWindowWarning(200, 24*time.Hour, policy("P7D", "PT1H"))
		assert.Contains(t, message, "P7D")
		assert.Contains(t, message, "200")
	})

	t.Run("is silent when the retained dumps fit the window", func(t *testing.T) {
		assert.Empty(t, retentionWindowWarning(7, 24*time.Hour, policy("P7D", "PT1H")))
	})

	t.Run("is silent when nothing prunes the primary-storage backups", func(t *testing.T) {
		assert.Empty(
			t,
			retentionWindowWarning(200, 24*time.Hour, policy("P7D", components.ScheduleNone)),
		)
	})

	t.Run("an absurd retained count cannot overflow the warning away", func(t *testing.T) {
		// A leap-year-only cron makes the interval about eight years. The
		// product with a huge count overflows int64; the warning must still
		// fire instead of comparing a wrapped negative lifetime.
		interval := 8 * 366 * 24 * time.Hour
		message := retentionWindowWarning(1<<31-1, interval, policy("P7D", "PT1H"))
		assert.NotEmpty(t, message)
	})

	t.Run("is silent on a window it cannot parse", func(t *testing.T) {
		assert.Empty(t, retentionWindowWarning(200, 24*time.Hour, policy("P1W", "PT1H")))
	})
}
