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

package databaseserver

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// scheduleCase is one input of the differential test between the
// baseBackupSchedule pattern and the cron parser that reads the schedule.
// pattern and parser record what each of the two answers, and why records the
// reason they differ.
//
// The rule is that the pattern accepts nothing the parser rejects. A range
// that reads downward is the one exception, because a pattern cannot compare
// the two ends of one. Every difference either way carries its reason, so a
// difference nobody wrote down fails the test.
type scheduleCase struct {
	input   string
	pattern bool
	parser  bool
	why     string
}

// schemaNode is the part of an OpenAPI schema that the pattern lookup walks.
type schemaNode struct {
	Properties map[string]schemaNode `json:"properties"`
	Pattern    string                `json:"pattern"`
}

func TestBaseBackupSchedulePattern(t *testing.T) {
	pattern := schedulePattern(
		t, "core.camunda.io_databaseservers.yaml",
		"spec", "archive", "baseBackupSchedule",
	)
	require.Equal(
		t,
		pattern,
		schedulePattern(
			t, "core.camunda.io_databaseserverpresets.yaml",
			"spec", "server", "archive", "baseBackupSchedule",
		),
		"the preset and the server must bound the schedule the same way",
	)

	// The API server matches the pattern with ECMA 262. An inline flag group
	// is RE2 syntax that ECMA has no answer for.
	require.NotContains(t, pattern, "(?i", "the pattern must hold to what ECMA 262 reads")

	re, err := regexp.Compile(pattern)
	require.NoError(t, err)

	// The field set of CloudNativePG, which reads the schedule with version 1
	// of this parser.
	parser := cron.NewParser(
		cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.DowOptional |
			cron.Descriptor,
	)

	cases := []scheduleCase{
		{input: "0 0 2 * * *", pattern: true, parser: true},
		{input: "0 0 2 * * ?", pattern: true, parser: true},
		{input: "0 */15 * * * *", pattern: true, parser: true},
		{input: "0 0 3 * * SUN", pattern: true, parser: true},
		{input: "0 0 3 * * sun", pattern: true, parser: true},
		{input: "0 0 3 * * MON-FRI", pattern: true, parser: true},
		{input: "0 0 3 1 JAN *", pattern: true, parser: true},
		{input: "0 0 3 1,15 * *", pattern: true, parser: true},
		{input: "0 0 0-23/2 * * *", pattern: true, parser: true},
		{input: "  0 0 2 * * *  ", pattern: true, parser: true},
		{input: "0 0 2 * * 0", pattern: true, parser: true},
		{input: "0 0 2 * * 6", pattern: true, parser: true},
		{input: "0 59 23 31 12 SAT", pattern: true, parser: true},
		{input: "0 0 2 */2 * *", pattern: true, parser: true},
		{input: "*/30 * * * * *", pattern: true, parser: true},
		{input: "0 0 2 * * */2", pattern: true, parser: true},
		{input: "5 4 3 2 1 0", pattern: true, parser: true},
		{input: "0 0 1-5 * * *", pattern: true, parser: true},
		{input: "0 0 2 1-31 1-12 0-6", pattern: true, parser: true},
		{input: "0,30 0 2 * * *", pattern: true, parser: true},
		{input: "0 0 2 * * SUN-SAT", pattern: true, parser: true},
		{input: "0 0 2 * * MON,WED,FRI", pattern: true, parser: true},
		{input: "0 0 2 * JAN-DEC *", pattern: true, parser: true},
		{input: "0 0 2 ? * *", pattern: true, parser: true},
		{input: "0 0 2 * * */999", pattern: true, parser: true},

		{input: "@yearly", pattern: true, parser: true},
		{input: "@annually", pattern: true, parser: true},
		{input: "@monthly", pattern: true, parser: true},
		{input: "@weekly", pattern: true, parser: true},
		{input: "@daily", pattern: true, parser: true},
		{input: "@midnight", pattern: true, parser: true},
		{input: "@hourly", pattern: true, parser: true},
		{input: "@every 1h", pattern: true, parser: true},
		{input: "@every 1.5h", pattern: true, parser: true},
		{input: "@every 30m", pattern: true, parser: true},
		{input: "@every 10s", pattern: true, parser: true},
		{input: "@every 1h30m", pattern: true, parser: true},
		{input: "@every 1.5h30m", pattern: true, parser: true},
		{input: "@every 6h", pattern: true, parser: true},
		{input: "@every 123456h", pattern: true, parser: true},
		// Both take it. The parser raises an interval under one second to one
		// second, so it names a schedule rather than a spin.
		{input: "@every 0s", pattern: true, parser: true},

		{input: "JAN 0 0 * * *", pattern: false, parser: false},
		{input: "0 0 24 * * *", pattern: false, parser: false},
		{input: "0 0 2 * * 7", pattern: false, parser: false},
		{input: "0 0 60 * * *", pattern: false, parser: false},
		{input: "0 60 2 * * *", pattern: false, parser: false},
		{input: "0 0 2 32 * *", pattern: false, parser: false},
		{input: "0 0 2 0 * *", pattern: false, parser: false},
		{input: "0 0 2 * 13 *", pattern: false, parser: false},
		{input: "0 0 2 * 0 *", pattern: false, parser: false},
		{input: "0 0 2 * * SUNDAY", pattern: false, parser: false},
		{input: "0 0 2 * * MON-", pattern: false, parser: false},
		{input: "0 0 2 * * -MON", pattern: false, parser: false},
		{input: "0 0 2 * * daily", pattern: false, parser: false},
		{input: "0 0 2 * * * 2026", pattern: false, parser: false},
		{input: "0 0 2 * * *  extra", pattern: false, parser: false},
		{input: "", pattern: false, parser: false},
		{input: "@reboot", pattern: false, parser: false},
		{input: "@Daily", pattern: false, parser: false},
		{input: "@EVERY 1h", pattern: false, parser: false},
		{input: "@every", pattern: false, parser: false},
		{input: "@every 5", pattern: false, parser: false},
		// The two inputs the bounds exist for. The duration overflows and the
		// step is too long for an int, so CloudNativePG stops taking base
		// backups on a schedule that admission let through before.
		{input: "@every 999999999999999999999999h", pattern: false, parser: false},
		{input: "0 0 2 * * */99999999999999999999", pattern: false, parser: false},

		{
			input: "0 2 * * *", pattern: false, parser: true,
			why: "five fields: CloudNativePG reads the first as seconds, so it runs at another time",
		},
		{
			input: "@every -1h", pattern: false, parser: true,
			why: "a negative interval is no interval, and the parser raises it to one second",
		},
		{
			input: "0 0 2 * * */1000", pattern: false, parser: true,
			why: "a step takes three digits, and a step above the range of a field yields its start",
		},
		{
			input: "@every 1234567h", pattern: false, parser: true,
			why: "an @every number takes six digits, well under the overflow of the duration parser",
		},
		{
			input: "@every 100000000s", pattern: false, parser: true,
			why: "an @every number takes six digits, well under the overflow of the duration parser",
		},
		{
			input: "@every 0.0000001h", pattern: false, parser: true,
			why: "the fraction of an @every number takes six digits",
		},
		{
			input: "0 0 2 * * FRI-MON", pattern: true, parser: false,
			why: "a pattern cannot compare the two ends of a range, so CloudNativePG refuses this one",
		},
		{
			input: "0 0 23-1 * * *", pattern: true, parser: false,
			why: "a pattern cannot compare the two ends of a range, so CloudNativePG refuses this one",
		},
	}

	require.GreaterOrEqual(t, len(cases), 60)

	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			_, err := parser.Parse(c.input)
			assert.Equal(t, c.parser, err == nil, "the parser answered: %v", err)
			assert.Equal(t, c.pattern, re.MatchString(c.input), "the pattern")

			if c.pattern == c.parser {
				assert.Empty(t, c.why, "the two agree, so there is nothing to write down")

				return
			}

			assert.NotEmpty(t, c.why, "the two differ and nobody wrote down why")
		})
	}
}

// schedulePattern reads the baseBackupSchedule pattern out of a generated CRD,
// under the property names that lead to the archive block. The shipped schema
// is what admission enforces, so the test reads that rather than the marker.
func schedulePattern(t *testing.T, file string, path ...string) string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "config", "crd", "bases", file))
	require.NoError(t, err)

	var crd struct {
		Spec struct {
			Versions []struct {
				Schema struct {
					OpenAPIV3Schema schemaNode `json:"openAPIV3Schema"`
				} `json:"schema"`
			} `json:"versions"`
		} `json:"spec"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &crd))
	require.NotEmpty(t, crd.Spec.Versions)

	node := crd.Spec.Versions[0].Schema.OpenAPIV3Schema
	for _, key := range path {
		next, ok := node.Properties[key]
		require.True(t, ok, "no property %q in %s", key, file)
		node = next
	}
	require.NotEmpty(t, node.Pattern, "no pattern in %s", file)

	return node.Pattern
}
