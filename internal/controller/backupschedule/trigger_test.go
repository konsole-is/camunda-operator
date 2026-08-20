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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// reference is the time every parse in these tests probes from.
var reference = time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)

func TestParseCron(t *testing.T) {
	t.Run("evaluates in UTC whatever the local zone is", func(t *testing.T) {
		sched, err := parseCron("0 2 * * *", reference)
		require.NoError(t, err)

		// 01:00 UTC on a day. The next 02:00 in UTC is one hour later. A
		// schedule bound to a local zone east of UTC would answer another
		// day.
		at := time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC)
		assert.Equal(
			t,
			time.Date(2026, 8, 20, 2, 0, 0, 0, time.UTC),
			sched.Next(at).UTC(),
		)
	})

	t.Run("rejects an out-of-range field that the schema pattern admits", func(t *testing.T) {
		_, err := parseCron("99 * * * *", reference)
		assert.Error(t, err)
	})

	t.Run("rejects a descriptor", func(t *testing.T) {
		_, err := parseCron("@hourly", reference)
		assert.Error(t, err)
	})

	t.Run("rejects a date that never comes", func(t *testing.T) {
		// The fields are all in range, so the parser accepts it and the
		// schema pattern admits it, but February has no 31st. Walking it
		// would search five years per step and never find a trigger.
		_, err := parseCron("0 0 31 2 *", reference)
		assert.Error(t, err)
	})

	t.Run("accepts a rare date that does come", func(t *testing.T) {
		_, err := parseCron("0 0 29 2 *", reference)
		assert.NoError(t, err)
	})
}

func TestDueTrigger(t *testing.T) {
	sched, err := parseCron("0 2 * * *", reference)
	require.NoError(t, err)
	created := time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC)
	trigger := func(day int) time.Time { return time.Date(2026, 8, day, 2, 0, 0, 0, time.UTC) }

	t.Run("nothing is due before the first trigger after creation", func(t *testing.T) {
		due, next := dueTrigger(sched, nil, created, created.Add(30*time.Minute))
		assert.Nil(t, due)
		assert.Equal(t, trigger(20), next.UTC())
	})

	t.Run("the first trigger after creation comes due", func(t *testing.T) {
		due, next := dueTrigger(sched, nil, created, trigger(20).Add(time.Minute))
		require.NotNil(t, due)
		assert.Equal(t, trigger(20), due.UTC())
		assert.Equal(t, trigger(21), next.UTC())
	})

	t.Run("triggers step from the last consumed trigger", func(t *testing.T) {
		last := metav1.NewTime(trigger(20))
		due, next := dueTrigger(sched, &last, created, trigger(20).Add(time.Hour))
		assert.Nil(t, due)
		assert.Equal(t, trigger(21), next.UTC())

		due, _ = dueTrigger(sched, &last, created, trigger(21))
		require.NotNil(t, due)
		assert.Equal(t, trigger(21), due.UTC())
	})

	t.Run("only the latest of many missed triggers is due", func(t *testing.T) {
		last := metav1.NewTime(trigger(20))
		due, next := dueTrigger(sched, &last, created, trigger(25).Add(time.Minute))
		require.NotNil(t, due)
		assert.Equal(t, trigger(25), due.UTC())
		assert.Equal(t, trigger(26), next.UTC())
	})

	t.Run("a pathological backlog still yields the latest trigger", func(t *testing.T) {
		minutely, err := parseCron("* * * * *", reference)
		require.NoError(t, err)

		// Two years of missed minutes. The full walk is too large, but the
		// latest trigger must still come due, or the schedule would re-walk
		// the same backlog forever and never fire again.
		now := created.AddDate(2, 0, 0).Add(30 * time.Second)
		last := metav1.NewTime(created)
		due, next := dueTrigger(minutely, &last, created, now)
		require.NotNil(t, due)
		assert.Equal(t, now.Truncate(time.Minute), due.UTC())
		assert.True(t, next.After(now))
	})
}

// exhaustingSchedule fires count times from base at one hour apart and then
// answers the zero time, the way robfig/cron reports "no occurrence within
// five years". A walk must never take that zero time for a trigger.
type exhaustingSchedule struct {
	base  time.Time
	count int
}

func (s exhaustingSchedule) Next(from time.Time) time.Time {
	last := s.base.Add(time.Duration(s.count) * time.Hour)
	if !from.Before(last) {
		return time.Time{}
	}

	return from.Add(time.Hour).Truncate(time.Hour)
}

func TestDueTriggerExhaustedSchedule(t *testing.T) {
	base := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)

	t.Run("stops at the last real trigger instead of taking the zero time", func(t *testing.T) {
		sched := exhaustingSchedule{base: base, count: 3}
		due, _ := dueTrigger(sched, nil, base, base.Add(24*time.Hour))
		require.NotNil(t, due)
		assert.Equal(t, base.Add(3*time.Hour), due.UTC())
	})

	t.Run("reports no trigger when the schedule is exhausted at once", func(t *testing.T) {
		sched := exhaustingSchedule{base: base, count: 0}
		due, _ := dueTrigger(sched, nil, base, base.Add(24*time.Hour))
		assert.Nil(t, due)
	})
}
