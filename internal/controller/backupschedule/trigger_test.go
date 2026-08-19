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

func TestParseCron(t *testing.T) {
	t.Run("evaluates in UTC whatever the local zone is", func(t *testing.T) {
		sched, err := parseCron("0 2 * * *")
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
		_, err := parseCron("99 * * * *")
		assert.Error(t, err)
	})

	t.Run("rejects a descriptor", func(t *testing.T) {
		_, err := parseCron("@hourly")
		assert.Error(t, err)
	})
}

func TestDueTrigger(t *testing.T) {
	sched, err := parseCron("0 2 * * *")
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

	t.Run("a pathological backlog stays bounded and skips to the future", func(t *testing.T) {
		minutely, err := parseCron("* * * * *")
		require.NoError(t, err)

		// Two years of missed minutes. The walk gives up and no trigger is
		// due; the next one is in the future, so the schedule recovers.
		now := created.AddDate(2, 0, 0)
		last := metav1.NewTime(created)
		due, next := dueTrigger(minutely, &last, created, now)
		assert.Nil(t, due)
		assert.True(t, next.After(now))
	})
}
