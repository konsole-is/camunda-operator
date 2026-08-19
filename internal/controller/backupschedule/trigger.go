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
	"github.com/robfig/cron/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"time"
)

// maxMissedTriggers bounds the walk over triggers that fired while nothing
// consumed them. A minutely schedule reaches the bound after roughly one week
// of downtime. Past it, dueTrigger stops walking and skips the backlog.
const maxMissedTriggers = 10000

// cronParser accepts the five standard cron fields and nothing else: no
// seconds field, no @descriptors. The schema pattern of spec.schedule is the
// first gate; this parser is the authority on the field values.
var cronParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
)

// parseCron parses a five-field cron expression, evaluated in UTC. The
// CRON_TZ prefix pins the zone; the schema pattern admits no such prefix in
// spec.schedule itself, so the pin cannot be overridden.
func parseCron(spec string) (cron.Schedule, error) {
	return cronParser.Parse("CRON_TZ=UTC " + spec)
}

// dueTrigger returns the trigger of sched that is due at now, or nil when
// none is, and the first trigger after now. Triggers step from the last
// consumed trigger, or from created for a schedule that never consumed one.
// When more than one trigger passed unconsumed, only the latest one is due;
// a schedule does not catch up on a backlog. A backlog beyond
// maxMissedTriggers is skipped entirely, so an absurd gap cannot stall the
// reconcile.
func dueTrigger(
	sched cron.Schedule,
	last *metav1.Time,
	created time.Time,
	now time.Time,
) (*time.Time, time.Time) {
	base := created
	if last != nil {
		base = last.Time
	}

	var due *time.Time
	next := sched.Next(base)
	for i := 0; !next.After(now); i++ {
		if i == maxMissedTriggers {
			return nil, sched.Next(now)
		}
		trigger := next
		due = &trigger
		next = sched.Next(next)
	}

	return due, next
}
