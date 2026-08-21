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
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
)

// isoDuration matches an ISO 8601 duration of days and time, the subset that
// Camunda parses and the CRD patterns admit: no week, month, or year units,
// and a fraction only on the seconds.
var isoDuration = regexp.MustCompile(
	`^P(?:([0-9]+)D)?(?:T(?:([0-9]+)H)?(?:([0-9]+)M)?(?:([0-9]+(?:\.[0-9]+)?)S)?)?$`,
)

// parseISODuration parses an ISO 8601 duration of days and time, for example
// P7D, PT12H, or P1DT6H. It rejects the week, month, and year units that
// Camunda rejects too, and a duration that names no unit at all.
func parseISODuration(s string) (time.Duration, error) {
	match := isoDuration.FindStringSubmatch(s)
	timeless := match != nil && match[2] == "" && match[3] == "" && match[4] == ""
	if match == nil || (timeless && match[1] == "") || (timeless && strings.Contains(s, "T")) {
		return 0, fmt.Errorf("%q is not an ISO 8601 duration of days and time", s)
	}

	var total time.Duration
	for i, unit := range []time.Duration{24 * time.Hour, time.Hour, time.Minute} {
		if match[i+1] == "" {
			continue
		}
		n, err := strconv.Atoi(match[i+1])
		if err != nil {
			return 0, fmt.Errorf("parsing %q: %w", s, err)
		}
		total += time.Duration(n) * unit
	}
	if match[4] != "" {
		seconds, err := strconv.ParseFloat(match[4], 64)
		if err != nil {
			return 0, fmt.Errorf("parsing %q: %w", s, err)
		}
		total += time.Duration(seconds * float64(time.Second))
	}

	return total, nil
}

// triggerInterval estimates the time between two triggers of sched: the gap
// between the next two triggers after from. An irregular schedule answers
// with whichever gap comes next, which is an estimate on purpose. The value
// feeds a warning, not a decision.
func triggerInterval(sched cron.Schedule, from time.Time) time.Duration {
	first := sched.Next(from)

	return sched.Next(first).Sub(first)
}

// retentionWindowWarning returns the warning of a schedule whose retained
// completed backups outlive the primary-storage retention window of the
// cluster, or the empty string when they fit. A database dump pairs with the
// Zeebe backups of its time, and Zeebe deletes those outside the window. So
// a dump older than the window can no longer be restored. Nothing is deleted
// while the cleanup schedule is none, so no dump outlives the window then.
// A window that does not parse warns about nothing: the CRD pattern already
// bounds the value, and this warning is advisory.
func retentionWindowWarning(
	retainedCompleted int32,
	interval time.Duration,
	policy components.PrimaryStorageBackupPolicy,
) string {
	if policy.CleanupSchedule == components.ScheduleNone {
		return ""
	}

	window, err := parseISODuration(policy.RetentionWindow)
	if err != nil {
		return ""
	}

	lifetime := time.Duration(retainedCompleted) * interval
	if interval > 0 && lifetime/interval != time.Duration(retainedCompleted) {
		// The product overflowed int64, so the true lifetime is far beyond
		// any window. The largest duration keeps the comparison honest.
		lifetime = time.Duration(math.MaxInt64)
	}
	if lifetime <= window {
		return ""
	}

	return fmt.Sprintf(
		"the schedule keeps %d completed backups at one every %s, so the oldest dump lives about %s, "+
			"longer than the primary-storage retention window %s of the cluster; "+
			"dumps older than the window can no longer be restored",
		retainedCompleted,
		interval,
		lifetime,
		policy.RetentionWindow,
	)
}
