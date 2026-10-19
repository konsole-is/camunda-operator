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

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/konsole-is/camunda-operator/conformance"
)

// fakeRun is an executeFunc that records its call and returns fixed results.
type fakeRun struct {
	called  bool
	cfg     conformance.Config
	results []conformance.Result
	err     error
}

func (f *fakeRun) execute(_ context.Context, cfg conformance.Config) ([]conformance.Result, error) {
	f.called, f.cfg = true, cfg
	return f.results, f.err
}

type invocation struct {
	code   int
	stdout string
	stderr string
}

func invoke(t *testing.T, fake *fakeRun, args ...string) invocation {
	t.Helper()

	var stdout, stderr bytes.Buffer
	code := run(t.Context(), args, &stdout, &stderr, fake.execute)

	return invocation{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

var sampleConfigPath = filepath.Join("testdata", "config.yaml")

func TestRunPassesTheParsedConfigToTheRun(t *testing.T) {
	fake := &fakeRun{}

	got := invoke(t, fake, "--config", sampleConfigPath, "--report", filepath.Join(t.TempDir(), "report.xml"))

	assert.Equal(t, 0, got.code, got.stderr)
	require.True(t, fake.called)
	assertSampleConfig(t, fake.cfg)
}

func TestRunWritesTheReportAndExitsOneWhenACheckFails(t *testing.T) {
	report := filepath.Join(t.TempDir(), "report.xml")
	fake := &fakeRun{results: []conformance.Result{
		{ID: "REG-ACCEPTED", Outcome: conformance.OutcomePassed},
		{ID: "REG-SCHEMA", Outcome: conformance.OutcomeFailed, Message: "the schema is not valid"},
	}}

	got := invoke(t, fake, "--config", sampleConfigPath, "--report", report)

	assert.Equal(t, 1, got.code, got.stderr)
	assert.Contains(t, got.stdout, "PASS REG-ACCEPTED")
	assert.Contains(t, got.stdout, "FAIL REG-SCHEMA: the schema is not valid")
	assert.Contains(t, got.stdout, "2 checks: 1 passed, 1 failed, 0 skipped")
	written, err := os.ReadFile(report)
	require.NoError(t, err)
	assert.Contains(t, string(written), `<testcase name="REG-SCHEMA"`)
	assert.Contains(t, string(written), "<failure")
}

func TestRunExitsZeroWhenNoCheckFails(t *testing.T) {
	fake := &fakeRun{results: []conformance.Result{
		{ID: "REG-ACCEPTED", Outcome: conformance.OutcomePassed},
		{ID: "ORDER-REGISTRATION-FIRST", Outcome: conformance.OutcomeSkipped, Message: "no Deployment"},
	}}

	got := invoke(t, fake, "--config", sampleConfigPath, "--report", filepath.Join(t.TempDir(), "report.xml"))

	assert.Equal(t, 0, got.code, got.stderr)
	assert.Contains(t, got.stdout, "SKIP ORDER-REGISTRATION-FIRST: no Deployment")
}

func TestRunRefusesARunWithoutAConfigFile(t *testing.T) {
	fake := &fakeRun{}

	got := invoke(t, fake)

	assert.Equal(t, 2, got.code)
	assert.Contains(t, got.stderr, "--config is required")
	assert.False(t, fake.called)
}

func TestRunNamesAConfigFileThatCannotBeRead(t *testing.T) {
	fake := &fakeRun{}

	got := invoke(t, fake, "--config", "missing.yaml", "--report", filepath.Join(t.TempDir(), "report.xml"))

	assert.Equal(t, 2, got.code)
	assert.Contains(t, got.stderr, "missing.yaml")
	assert.False(t, fake.called)
}

func TestRunNamesAConfigFileThatIsNotValid(t *testing.T) {
	config := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(config, []byte("registraton: echo\n"), 0o600))
	fake := &fakeRun{}

	got := invoke(t, fake, "--config", config, "--report", filepath.Join(t.TempDir(), "report.xml"))

	assert.Equal(t, 2, got.code)
	assert.Contains(t, got.stderr, config)
	assert.Contains(t, got.stderr, "registraton")
	assert.False(t, fake.called)
}

func TestRunChecksTheReportPathBeforeTheChecksRun(t *testing.T) {
	report := filepath.Join(t.TempDir(), "no-such-dir", "report.xml")
	fake := &fakeRun{}

	got := invoke(t, fake, "--config", sampleConfigPath, "--report", report)

	assert.Equal(t, 2, got.code)
	assert.Contains(t, got.stderr, report)
	assert.False(t, fake.called)
}

func TestRunStopsWhenTheRunCannotStart(t *testing.T) {
	report := filepath.Join(t.TempDir(), "report.xml")
	fake := &fakeRun{err: errors.New("conformance: the config of the run is not valid: namespace is empty")}

	got := invoke(t, fake, "--config", sampleConfigPath, "--report", report)

	assert.Equal(t, 2, got.code)
	assert.Contains(t, got.stderr, "namespace is empty")
	assert.NoFileExists(t, report)
}

func TestRunWritesTheReportAndExitsTwoWhenTheCleanupFails(t *testing.T) {
	report := filepath.Join(t.TempDir(), "report.xml")
	fake := &fakeRun{
		results: []conformance.Result{{ID: "REG-ACCEPTED", Outcome: conformance.OutcomePassed}},
		err:     errors.New("conformance: the cleanup of the run failed: the namespace is still terminating"),
	}

	got := invoke(t, fake, "--config", sampleConfigPath, "--report", report)

	assert.Equal(t, 2, got.code)
	assert.Contains(t, got.stderr, "the namespace is still terminating")
	assert.Contains(t, got.stdout, "PASS REG-ACCEPTED")
	written, err := os.ReadFile(report)
	require.NoError(t, err)
	assert.Contains(t, string(written), `<testcase name="REG-ACCEPTED"`)
}

func TestRunPrintsTheUsageForHelp(t *testing.T) {
	got := invoke(t, &fakeRun{}, "-h")

	assert.Equal(t, 0, got.code)
	assert.Contains(t, got.stderr, "--config")
	assert.Contains(t, got.stderr, "--report")
}
