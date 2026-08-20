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
	"time"

	"github.com/robfig/cron/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// maxMissedTriggers bounds the walk over triggers that fired while
	// nothing consumed them. A minutely schedule reaches the bound after
	// roughly one week of downtime. Past it, dueTrigger restarts inside
	// backlogWindow instead of enumerating the backlog.
	maxMissedTriggers = 10000
	// backlogWindow is how far back the restarted walk begins. Every cron
	// expression that can overflow maxMissedTriggers fires well inside this
	// window, so the window holds the latest trigger.
	backlogWindow = 366 * 24 * time.Hour
	// maxWindowTriggers bounds the restarted walk. A five-field cron fires
	// at most once per minute, and backlogWindow holds fewer minutes than
	// this, so the restarted walk always completes.
	maxWindowTriggers = 600000
)

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
// maxMissedTriggers is not enumerated: the walk restarts just inside
// backlogWindow, which holds the latest trigger, so an absurd gap costs one
// bounded walk instead of stalling the schedule forever.
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

	due, next, done := walkTriggers(sched, base, now, maxMissedTriggers)
	if !done {
		// A backlog can only overflow when the cron fires often, and a cron
		// that fires often fires inside the window. A cron sparse enough to
		// fire outside it cannot overflow within any realistic gap.
		if recent := now.Add(-backlogWindow); recent.After(base) {
			base = recent
		}
		due, next, _ = walkTriggers(sched, base, now, maxWindowTriggers)
	}

	return due, next
}

// walkTriggers returns the latest trigger of sched in (base, now], or nil
// when there is none, the first trigger after now, and whether the walk
// completed within limit steps. An incomplete walk reports no trigger.
func walkTriggers(
	sched cron.Schedule,
	base time.Time,
	now time.Time,
	limit int,
) (*time.Time, time.Time, bool) {
	var due *time.Time
	next := sched.Next(base)
	for i := 0; !next.After(now); i++ {
		if i == limit {
			return nil, sched.Next(now), false
		}
		trigger := next
		due = &trigger
		next = sched.Next(next)
	}

	return due, next, true
}
