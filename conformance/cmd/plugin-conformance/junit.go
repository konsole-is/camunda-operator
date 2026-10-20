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
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/konsole-is/camunda-operator/conformance"
)

type junitTestSuites struct {
	XMLName  xml.Name         `xml:"testsuites"`
	Name     string           `xml:"name,attr"`
	Tests    int              `xml:"tests,attr"`
	Failures int              `xml:"failures,attr"`
	Errors   int              `xml:"errors,attr"`
	Skipped  int              `xml:"skipped,attr"`
	Time     string           `xml:"time,attr"`
	Suites   []junitTestSuite `xml:"testsuite"`
}

type junitTestSuite struct {
	Name      string          `xml:"name,attr"`
	Tests     int             `xml:"tests,attr"`
	Failures  int             `xml:"failures,attr"`
	Errors    int             `xml:"errors,attr"`
	Skipped   int             `xml:"skipped,attr"`
	Time      string          `xml:"time,attr"`
	TestCases []junitTestCase `xml:"testcase"`
}

type junitTestCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *junitFailure `xml:"failure"`
	Skipped   *junitSkipped `xml:"skipped"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}

type junitSkipped struct {
	Message string `xml:"message,attr"`
}

// writeJUnit writes results as a JUnit report with one test suite, named
// by registration, and one test case for each result, in the order of
// results.
func writeJUnit(w io.Writer, registration string, results []conformance.Result) error {
	suite := junitTestSuite{Name: registration}
	var total time.Duration
	for _, r := range results {
		tc := junitTestCase{Name: string(r.ID), ClassName: registration, Time: seconds(r.Duration)}

		switch r.Outcome {
		case conformance.OutcomeFailed:
			// CI viewers show either the attribute or the text, so the
			// message goes in both.
			tc.Failure = &junitFailure{Message: r.Message, Text: r.Message}
			suite.Failures++
		case conformance.OutcomeSkipped:
			tc.Skipped = &junitSkipped{Message: r.Message}
			suite.Skipped++
		}

		suite.TestCases = append(suite.TestCases, tc)
		total += r.Duration
	}

	suite.Tests = len(results)
	suite.Time = seconds(total)

	report := junitTestSuites{
		Name:     "plugin-conformance",
		Tests:    suite.Tests,
		Failures: suite.Failures,
		Skipped:  suite.Skipped,
		Time:     suite.Time,
		Suites:   []junitTestSuite{suite},
	}

	out, err := xml.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding the JUnit report: %w", err)
	}

	if _, err := io.WriteString(w, xml.Header+string(out)+"\n"); err != nil {
		return fmt.Errorf("writing the JUnit report: %w", err)
	}

	return nil
}

func seconds(d time.Duration) string {
	return strconv.FormatFloat(d.Seconds(), 'f', 3, 64)
}
