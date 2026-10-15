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

package conformance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	mandatoryID CheckID = "TEST-MANDATORY"
	skippableID CheckID = "TEST-SKIPPABLE"
)

// testRegistry holds one mandatory and one skippable check. Each check logs
// a line with its subtest name and the Ready timeout it got, so a test can
// see which checks ran and with which config.
var testRegistry = []Check{
	{ID: mandatoryID, Description: "A mandatory check.", run: logRan},
	{ID: skippableID, Description: "A skippable check.", Skippable: true, run: logRan},
}

func logRan(t *testing.T, s *suite) {
	t.Logf("check ran: %s, ready timeout %s", t.Name(), s.cfg.Timeouts.Ready)
}

func TestPlanRunsEveryCheckInRegistryOrder(t *testing.T) {
	steps, err := plan(validConfig(), testRegistry)
	require.NoError(t, err)

	require.Len(t, steps, 2)
	assert.Equal(t, mandatoryID, steps[0].check.ID)
	assert.Equal(t, skippableID, steps[1].check.ID)
	assert.Empty(t, steps[0].skipReason)
	assert.Empty(t, steps[1].skipReason)
}

func TestPlanKeepsTheReasonOfASkippableCheck(t *testing.T) {
	cfg := validConfig()
	cfg.Skip = []Skip{{ID: skippableID, Reason: "the plugin has no Deployment"}}

	steps, err := plan(cfg, testRegistry)
	require.NoError(t, err)

	require.Len(t, steps, 2)
	assert.Empty(t, steps[0].skipReason)
	assert.Equal(t, "the plugin has no Deployment", steps[1].skipReason)
}

func TestPlanRefusesInvalidSkips(t *testing.T) {
	tests := []struct {
		name    string
		skip    []Skip
		message string
	}{
		{
			"mandatory check",
			[]Skip{{ID: mandatoryID, Reason: "too slow"}},
			`skip "TEST-MANDATORY" names a mandatory check`,
		},
		{
			"unknown ID",
			[]Skip{{ID: "test-skippable", Reason: "typo"}},
			`skip "test-skippable" names no check`,
		},
		{"empty reason", []Skip{{ID: skippableID}}, `skip "TEST-SKIPPABLE" has no reason`},
		{"blank reason", []Skip{{ID: skippableID, Reason: "  "}}, `skip "TEST-SKIPPABLE" has no reason`},
		{
			"same check twice",
			[]Skip{{ID: skippableID, Reason: "one"}, {ID: skippableID, Reason: "two"}},
			`check "TEST-SKIPPABLE" is skipped more than once`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Skip = tt.skip

			steps, err := plan(cfg, testRegistry)

			assert.ErrorContains(t, err, tt.message)
			assert.Nil(t, steps)
		})
	}
}

func TestPlanReportsEveryError(t *testing.T) {
	cfg := validConfig()
	cfg.Namespace = ""
	cfg.Skip = []Skip{{ID: mandatoryID, Reason: "too slow"}}

	_, err := plan(cfg, testRegistry)

	assert.ErrorContains(t, err, "namespace is empty")
	assert.ErrorContains(t, err, `skip "TEST-MANDATORY" names a mandatory check`)
}
