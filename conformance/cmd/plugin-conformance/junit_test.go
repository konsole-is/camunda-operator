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
	"encoding/xml"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/konsole-is/camunda-operator/conformance"
)

var update = flag.Bool("update", false, "write testdata/junit.golden.xml from the results of the test")

// failureMessage holds the characters that XML must escape, and a line break.
const failureMessage = "case \"default\": parameters.retentionDays: Invalid value: 0: must be >= 1\n" +
	"case \"orphan\": parameters: Required value & <none> given"

func goldenResults() []conformance.Result {
	return []conformance.Result{
		{ID: "REG-ACCEPTED", Outcome: conformance.OutcomePassed, Duration: 1250 * time.Millisecond},
		{
			ID:       "REG-SCHEMA",
			Outcome:  conformance.OutcomeFailed,
			Message:  failureMessage,
			Duration: 3250 * time.Millisecond,
		},
		{
			ID:      "ORDER-REGISTRATION-FIRST",
			Outcome: conformance.OutcomeSkipped,
			Message: "skipped by the config: the plugin has no Deployment",
		},
	}
}

func TestWriteJUnitMatchesTheGolden(t *testing.T) {
	var got bytes.Buffer
	require.NoError(t, writeJUnit(&got, "s3-backup-bucket", goldenResults()))

	golden := filepath.Join("testdata", "junit.golden.xml")
	if *update {
		require.NoError(t, os.WriteFile(golden, got.Bytes(), 0o644))
	}

	want, err := os.ReadFile(golden)
	require.NoError(t, err)
	assert.Equal(t, string(want), got.String())
}

func TestWriteJUnitWritesXMLThatParsesBack(t *testing.T) {
	type message struct {
		Message string `xml:"message,attr"`
		Text    string `xml:",chardata"`
	}
	type testcase struct {
		Name    string   `xml:"name,attr"`
		Failure *message `xml:"failure"`
		Skipped *message `xml:"skipped"`
	}
	type testsuite struct {
		Tests     int        `xml:"tests,attr"`
		Failures  int        `xml:"failures,attr"`
		Skipped   int        `xml:"skipped,attr"`
		TestCases []testcase `xml:"testcase"`
	}
	var report struct {
		Suites []testsuite `xml:"testsuite"`
	}

	var out bytes.Buffer
	require.NoError(t, writeJUnit(&out, "s3-backup-bucket", goldenResults()))
	require.NoError(t, xml.Unmarshal(out.Bytes(), &report))

	require.Len(t, report.Suites, 1)
	suite := report.Suites[0]
	assert.Equal(t, 3, suite.Tests)
	assert.Equal(t, 1, suite.Failures)
	assert.Equal(t, 1, suite.Skipped)
	require.Len(t, suite.TestCases, 3)
	assert.Equal(t, "REG-ACCEPTED", suite.TestCases[0].Name)
	assert.Equal(t, "REG-SCHEMA", suite.TestCases[1].Name)
	assert.Equal(t, "ORDER-REGISTRATION-FIRST", suite.TestCases[2].Name)
	assert.Nil(t, suite.TestCases[0].Failure)
	assert.Nil(t, suite.TestCases[0].Skipped)
	require.NotNil(t, suite.TestCases[1].Failure)
	assert.Equal(t, failureMessage, suite.TestCases[1].Failure.Message)
	assert.Equal(t, failureMessage, suite.TestCases[1].Failure.Text)
	require.NotNil(t, suite.TestCases[2].Skipped)
	assert.Equal(t, "skipped by the config: the plugin has no Deployment", suite.TestCases[2].Skipped.Message)
}
