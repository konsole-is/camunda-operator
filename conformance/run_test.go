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
	"time"

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

func TestExecuteReportsEachOutcomeInRegistryOrder(t *testing.T) {
	var ran []CheckID
	record := func(id CheckID, err error) func(context.Context, *suite) error {
		return func(context.Context, *suite) error {
			ran = append(ran, id)
			return err
		}
	}

	cfg := validConfig()
	cfg.Skip = []Skip{{ID: skippableID, Reason: "the plugin has no Deployment"}}
	registry := []Check{
		{ID: mandatoryID, Description: "A check that passes.", run: record(mandatoryID, nil)},
		{
			ID:          failingID,
			Description: "A check that fails.",
			run:         record(failingID, errors.New("the plugin broke the contract")),
		},
		{
			ID:          runtimeSkipID,
			Description: "A check that skips itself.",
			run:         record(runtimeSkipID, SkipCheck("the registration has no grant templates")),
		},
		{ID: skippableID, Description: "A skippable check.", Skippable: true, run: record(skippableID, nil)},
	}

	results, err := execute(t.Context(), cfg, registry, fakeConnect)
	require.NoError(t, err)

	require.Len(t, results, 4)
	assert.Equal(t, Result{ID: mandatoryID, Outcome: OutcomePassed}, withoutDuration(results[0]))
	assert.Equal(
		t,
		Result{ID: failingID, Outcome: OutcomeFailed, Message: "the plugin broke the contract"},
		withoutDuration(results[1]),
	)
	assert.Equal(
		t,
		Result{ID: runtimeSkipID, Outcome: OutcomeSkipped, Message: "the registration has no grant templates"},
		withoutDuration(results[2]),
	)
	assert.Equal(
		t,
		Result{
			ID:      skippableID,
			Outcome: OutcomeSkipped,
			Message: "skipped by the config: the plugin has no Deployment",
		},
		results[3],
	)
	assert.Equal(t, []CheckID{mandatoryID, failingID, runtimeSkipID}, ran, "a check that the config skips does not run")
}

func TestExecuteGivesEachCheckTheConfigWithTheTimeoutDefaults(t *testing.T) {
	var ready time.Duration
	registry := []Check{{ID: mandatoryID, Description: "A check.", run: func(_ context.Context, s *suite) error {
		ready = s.cfg.Timeouts.Ready
		return nil
	}}}

	_, err := execute(t.Context(), validConfig(), registry, fakeConnect)
	require.NoError(t, err)

	assert.Equal(t, 10*time.Minute, ready)
}

func TestExecuteRefusesAnInvalidConfigAndRunsNoCheck(t *testing.T) {
	ran := false
	cfg := validConfig()
	cfg.Skip = []Skip{{ID: mandatoryID, Reason: "too slow"}}
	registry := []Check{{ID: mandatoryID, Description: "A check.", run: func(context.Context, *suite) error {
		ran = true
		return nil
	}}}

	results, err := execute(t.Context(), cfg, registry, fakeConnect)

	assert.ErrorContains(t, err, "conformance: the config of the run is not valid")
	assert.ErrorContains(t, err, `skip "TEST-MANDATORY" names a mandatory check`)
	assert.Nil(t, results)
	assert.False(t, ran)
}

func TestExecuteRunsNoCheckWhenItCannotConnect(t *testing.T) {
	ran := false
	registry := []Check{{ID: mandatoryID, Description: "A check.", run: func(context.Context, *suite) error {
		ran = true
		return nil
	}}}
	noKubeconfig := func(context.Context, *rest.Config) (*rest.Config, client.Client, error) {
		return nil, nil, errors.New("no kubeconfig")
	}

	results, err := execute(t.Context(), validConfig(), registry, noKubeconfig)

	assert.ErrorContains(t, err, "conformance: connecting to the cluster: no kubeconfig")
	assert.Nil(t, results)
	assert.False(t, ran)
}

func TestExecuteRunsTheCleanupsOfTheRunAfterTheLastCheck(t *testing.T) {
	var order []string
	registry := []Check{
		{ID: mandatoryID, Description: "A check with a cleanup.", run: func(_ context.Context, s *suite) error {
			s.onClose(func(context.Context) error {
				order = append(order, "cleanup")
				return nil
			})
			return nil
		}},
		{ID: failingID, Description: "The last check.", run: func(context.Context, *suite) error {
			order = append(order, "last check")
			return nil
		}},
	}

	results, err := execute(t.Context(), validConfig(), registry, fakeConnect)

	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, []string{"last check", "cleanup"}, order)
}

func TestExecuteRunsTheCleanupsOfAStoppedRun(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	var cleanupErr error
	cleanedUp := false
	registry := []Check{
		{ID: mandatoryID, Description: "A check that stops the run.", run: func(_ context.Context, s *suite) error {
			s.onClose(func(ctx context.Context) error {
				cleanedUp, cleanupErr = true, ctx.Err()
				return nil
			})
			cancel()
			return nil
		}},
	}

	_, err := execute(ctx, validConfig(), registry, fakeConnect)

	require.NoError(t, err)
	assert.True(t, cleanedUp)
	assert.NoError(t, cleanupErr, "the cleanup gets a context that the stop did not cancel")
}

func TestExecuteReturnsTheResultsAndTheErrorOfAFailedCleanup(t *testing.T) {
	registry := []Check{
		{ID: mandatoryID, Description: "A check with a cleanup.", run: func(_ context.Context, s *suite) error {
			s.onClose(func(context.Context) error { return errors.New("the namespace is still terminating") })
			return nil
		}},
	}

	results, err := execute(t.Context(), validConfig(), registry, fakeConnect)

	assert.ErrorContains(t, err, "conformance: the cleanup of the run failed: the namespace is still terminating")
	require.Len(t, results, 1)
	assert.Equal(t, OutcomePassed, results[0].Outcome)
}

func TestExecuteFailsTheChecksThatAStoppedRunDidNotStart(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	ran := false
	registry := []Check{
		{ID: mandatoryID, Description: "A check that stops the run.", run: func(context.Context, *suite) error {
			cancel()
			return nil
		}},
		{ID: failingID, Description: "A check after the stop.", run: func(context.Context, *suite) error {
			ran = true
			return nil
		}},
	}

	results, err := execute(ctx, validConfig(), registry, fakeConnect)
	require.NoError(t, err)

	require.Len(t, results, 2)
	assert.Equal(t, OutcomePassed, results[0].Outcome)
	assert.Equal(t, OutcomeFailed, results[1].Outcome)
	assert.Contains(t, results[1].Message, "the run stopped before this check started")
	assert.Contains(t, results[1].Message, context.Canceled.Error())
	assert.False(t, ran)
}

// withoutDuration returns r with a zero Duration, because a check that runs
// takes real time.
func withoutDuration(r Result) Result {
	r.Duration = 0
	return r
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
		return Check{
			ID:          "TEST-CLEANUP",
			Description: "A check that registers a cleanup.",
			run: func(_ context.Context, s *suite) error {
				s.onClose(func(context.Context) error {
					fmt.Println("cleanup ran")
					return err
				})
				return nil
			},
		}
	}

	return map[string]helperRun{
		"cleanup": {
			cfg: validConfig(),
			registry: []Check{
				registerCleanup(nil),
				{ID: mandatoryID, Description: "A mandatory check.", run: logRan(mandatoryID)},
			},
			connect: fakeConnect,
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
