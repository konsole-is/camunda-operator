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
	"errors"
	"os"
	"os/exec"
	"strings"
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

// helperEnv selects the helper run of TestRunHelper in a subprocess.
const helperEnv = "CONFORMANCE_RUN_HELPER"

const failingID CheckID = "TEST-FAILING"

type helperRun struct {
	cfg      Config
	registry []Check
}

func helperRuns() map[string]helperRun {
	skipSkippable := validConfig()
	skipSkippable.Skip = []Skip{{ID: skippableID, Reason: "the plugin has no Deployment"}}

	skipMandatory := validConfig()
	skipMandatory.Skip = []Skip{{ID: mandatoryID, Reason: "too slow"}}

	return map[string]helperRun{
		"clean":          {cfg: validConfig(), registry: testRegistry},
		"skip-skippable": {cfg: skipSkippable, registry: testRegistry},
		"skip-mandatory": {cfg: skipMandatory, registry: testRegistry},
		"failing-check": {
			cfg: validConfig(),
			registry: []Check{
				{ID: failingID, Description: "A check that fails.", run: func(t *testing.T, _ *suite) {
					t.Error("the plugin broke the contract")
				}},
				{ID: mandatoryID, Description: "A mandatory check.", run: logRan},
			},
		},
	}
}

// TestRunHelper runs one helper run when the Run tests start the test binary
// with helperEnv set. In a normal test run it skips.
func TestRunHelper(t *testing.T) {
	name := os.Getenv(helperEnv)
	if name == "" {
		t.Skip("runs only as a subprocess of the Run tests")
	}

	run, ok := helperRuns()[name]
	if !ok {
		t.Errorf("no helper run %q", name)
		return
	}

	runChecks(t, run.cfg, run.registry)
}

// runHelper runs the helper run name in a subprocess and returns its verbose
// output and whether it passed.
func runHelper(t *testing.T, name string) (string, bool) {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=^TestRunHelper$", "-test.v")
	cmd.Env = append(os.Environ(), helperEnv+"="+name)
	out, err := cmd.CombinedOutput()

	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		require.NoError(t, err, "starting the test binary")
	}

	return string(out), err == nil
}

func TestRunNamesEachSubtestByCheckID(t *testing.T) {
	out, passed := runHelper(t, "clean")

	assert.True(t, passed, out)
	mandatory := strings.Index(out, "--- PASS: TestRunHelper/TEST-MANDATORY")
	skippable := strings.Index(out, "--- PASS: TestRunHelper/TEST-SKIPPABLE")
	assert.NotEqual(t, -1, mandatory, out)
	assert.NotEqual(t, -1, skippable, out)
	assert.Less(t, mandatory, skippable, "the checks run in registry order")
	assert.Contains(t, out, "ready timeout 10m0s", "a check gets the config with the timeout defaults")
}

func TestRunReportsTheReasonOfASkippedCheck(t *testing.T) {
	out, passed := runHelper(t, "skip-skippable")

	assert.True(t, passed, out)
	assert.Contains(t, out, "--- SKIP: TestRunHelper/TEST-SKIPPABLE")
	assert.Contains(t, out, "the plugin has no Deployment")
	assert.NotContains(t, out, "check ran: TestRunHelper/TEST-SKIPPABLE")
	assert.Contains(t, out, "--- PASS: TestRunHelper/TEST-MANDATORY")
}

func TestRunFailsWhenTheConfigSkipsAMandatoryCheck(t *testing.T) {
	out, passed := runHelper(t, "skip-mandatory")

	assert.False(t, passed, out)
	assert.Contains(t, out, "--- FAIL: TestRunHelper")
	assert.Contains(t, out, `skip "TEST-MANDATORY" names a mandatory check`)
	assert.NotContains(t, out, "check ran:", "no check runs with an invalid config")
}

func TestRunKeepsRunningAfterAFailedCheck(t *testing.T) {
	out, passed := runHelper(t, "failing-check")

	assert.False(t, passed, out)
	assert.Contains(t, out, "--- FAIL: TestRunHelper/TEST-FAILING")
	assert.Contains(t, out, "the plugin broke the contract")
	assert.Contains(t, out, "--- PASS: TestRunHelper/TEST-MANDATORY")
}
