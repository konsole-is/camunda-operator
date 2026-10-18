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
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	mandatoryID CheckID = "TEST-MANDATORY"
	skippableID CheckID = "TEST-SKIPPABLE"
)

// testRegistry holds one mandatory and one skippable check. Each check prints
// a line with its ID and the Ready timeout it got, so a test can see which
// checks ran and with which config.
var testRegistry = []Check{
	{ID: mandatoryID, Description: "A mandatory check.", run: logRan(mandatoryID)},
	{ID: skippableID, Description: "A skippable check.", Skippable: true, run: logRan(skippableID)},
}

// logRan prints to stdout, because a check has no *testing.T. The output of
// runHelper holds it.
func logRan(id CheckID) func(context.Context, *suite) error {
	return func(_ context.Context, s *suite) error {
		fmt.Printf("check ran: %s, ready timeout %s\n", id, s.cfg.Timeouts.Ready)
		return nil
	}
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

const (
	failingID     CheckID = "TEST-FAILING"
	runtimeSkipID CheckID = "TEST-RUNTIME-SKIP"
)

type helperRun struct {
	cfg      Config
	registry []Check
	connect  func(context.Context, *rest.Config) (*rest.Config, client.Client, error)
}

func fakeConnect(_ context.Context, restCfg *rest.Config) (*rest.Config, client.Client, error) {
	scheme, err := newScheme()
	if err != nil {
		return nil, nil, err
	}

	return restCfg, fake.NewClientBuilder().WithScheme(scheme).Build(), nil
}

func helperRuns() map[string]helperRun {
	skipSkippable := validConfig()
	skipSkippable.Skip = []Skip{{ID: skippableID, Reason: "the plugin has no Deployment"}}

	skipMandatory := validConfig()
	skipMandatory.Skip = []Skip{{ID: mandatoryID, Reason: "too slow"}}

	registerCleanup := func(err error) Check {
		return Check{ID: "TEST-CLEANUP", Description: "A check that registers a cleanup.", run: func(_ context.Context, s *suite) error {
			s.onClose(func(context.Context) error {
				fmt.Println("cleanup ran")
				return err
			})
			return nil
		}}
	}

	return map[string]helperRun{
		"cleanup": {
			cfg:      validConfig(),
			registry: []Check{registerCleanup(nil), {ID: mandatoryID, Description: "A mandatory check.", run: logRan(mandatoryID)}},
			connect:  fakeConnect,
		},
		"cleanup-fails": {
			cfg: validConfig(),
			registry: []Check{
				registerCleanup(errors.New("the namespace could not be deleted")),
				{ID: mandatoryID, Description: "A mandatory check.", run: logRan(mandatoryID)},
			},
			connect: fakeConnect,
		},
		"clean":          {cfg: validConfig(), registry: testRegistry, connect: fakeConnect},
		"skip-skippable": {cfg: skipSkippable, registry: testRegistry, connect: fakeConnect},
		"skip-mandatory": {cfg: skipMandatory, registry: testRegistry, connect: fakeConnect},
		"failing-check": {
			cfg: validConfig(),
			registry: []Check{
				{ID: failingID, Description: "A check that fails.", run: func(context.Context, *suite) error {
					return errors.New("the plugin broke the contract")
				}},
				{ID: mandatoryID, Description: "A mandatory check.", run: logRan(mandatoryID)},
			},
			connect: fakeConnect,
		},
		"skip-at-run-time": {
			cfg: validConfig(),
			registry: []Check{
				{
					ID:          runtimeSkipID,
					Description: "A check that skips itself.",
					run: func(context.Context, *suite) error {
						return SkipCheck("the plugin has no Deployment")
					},
				},
				{ID: mandatoryID, Description: "A mandatory check.", run: logRan(mandatoryID)},
			},
			connect: fakeConnect,
		},
		"connect-fails": {
			cfg:      validConfig(),
			registry: testRegistry,
			connect: func(context.Context, *rest.Config) (*rest.Config, client.Client, error) {
				return nil, nil, errors.New("no kubeconfig")
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

	runChecks(t, run.cfg, run.registry, run.connect)
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
	assert.NotContains(t, out, "check ran: TEST-SKIPPABLE")
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

func TestRunSkipsACheckThatReturnsSkipCheck(t *testing.T) {
	out, passed := runHelper(t, "skip-at-run-time")

	assert.True(t, passed, out)
	assert.Contains(t, out, "--- SKIP: TestRunHelper/TEST-RUNTIME-SKIP")
	assert.Contains(t, out, "the plugin has no Deployment")
	assert.Contains(t, out, "--- PASS: TestRunHelper/TEST-MANDATORY")
}

func TestRunFailsAndRunsNoCheckWhenItCannotConnect(t *testing.T) {
	out, passed := runHelper(t, "connect-fails")

	assert.False(t, passed, out)
	assert.Contains(t, out, "conformance: connecting to the cluster: no kubeconfig")
	assert.NotContains(t, out, "check ran:")
}

func TestCloseRunsEveryCleanupInReverseOrder(t *testing.T) {
	s := &suite{}
	var order []string
	s.onClose(func(context.Context) error { order = append(order, "first"); return nil })
	s.onClose(func(context.Context) error { order = append(order, "second"); return errors.New("second failed") })
	s.onClose(func(context.Context) error { order = append(order, "third"); return nil })

	err := s.close(t.Context())

	assert.Equal(t, []string{"third", "second", "first"}, order)
	assert.ErrorContains(t, err, "second failed")
}

func TestRunChecksRunsCleanupsAfterTheLastCheck(t *testing.T) {
	out, passed := runHelper(t, "cleanup")

	assert.True(t, passed, out)
	last := strings.Index(out, "check ran: TEST-MANDATORY")
	cleanup := strings.Index(out, "cleanup ran")
	assert.NotEqual(t, -1, last, out)
	assert.Greater(t, cleanup, last, "the cleanup runs after the last check")
}

func TestRunChecksFailsWhenACleanupFails(t *testing.T) {
	out, passed := runHelper(t, "cleanup-fails")

	assert.False(t, passed, out)
	assert.Contains(t, out, "conformance: the cleanup of the run failed: the namespace could not be deleted")
	assert.Contains(t, out, "--- PASS: TestRunHelper/TEST-MANDATORY")
}
