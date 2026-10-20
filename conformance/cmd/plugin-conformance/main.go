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

// Command plugin-conformance runs the conformance checks of a plugin and
// writes a JUnit report, for the CI of a plugin that is not written in Go.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/konsole-is/camunda-operator/conformance"
)

// The exit codes of the binary: no check failed, at least one check failed,
// and the run could not start, the report could not be written, or the
// cleanup of the run failed.
const (
	exitPassed = 0
	exitFailed = 1
	exitError  = 2
)

const usage = `Usage: plugin-conformance --config <file> [--report <file>]

Runs the conformance checks of a plugin and writes a JUnit report.

  --config <file>  the YAML config of the run (required)
  --report <file>  the JUnit report to write (default plugin-conformance-report.xml)

The binary reaches the cluster through KUBECONFIG, or the in-cluster
config, or ~/.kube/config.
`

// executeFunc runs the checks of a config. main passes conformance.Execute.
type executeFunc func(ctx context.Context, cfg conformance.Config) ([]conformance.Result, error)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr, conformance.Execute)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, execute executeFunc) int {
	flags := flag.NewFlagSet("plugin-conformance", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { _, _ = io.WriteString(stderr, usage) }
	configPath := flags.String("config", "", "")
	reportPath := flags.String("report", "plugin-conformance-report.xml", "")

	if err := flags.Parse(args); errors.Is(err, flag.ErrHelp) {
		return exitPassed
	} else if err != nil {
		return exitError
	}

	if *configPath == "" {
		_, _ = fmt.Fprintln(stderr, "--config is required")
		flags.Usage()
		return exitError
	}

	data, err := os.ReadFile(*configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "reading the config: %v\n", err)
		return exitError
	}

	cfg, err := parseConfig(data)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "config %s: %v\n", *configPath, err)
		return exitError
	}

	// A run can take many minutes, so a report path that cannot be written
	// stops the binary before the first check.
	report, err := os.Create(*reportPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "creating the report: %v\n", err)
		return exitError
	}

	results, runErr := execute(ctx, cfg)
	if runErr != nil && results == nil {
		_, _ = fmt.Fprintln(stderr, runErr)
		_ = report.Close()
		_ = os.Remove(*reportPath)
		return exitError
	}

	failed := printResults(stdout, results, *reportPath)

	if err := errors.Join(writeJUnit(report, cfg.Registration, results), report.Close()); err != nil {
		_, _ = fmt.Fprintf(stderr, "report %s: %v\n", *reportPath, err)
		return exitError
	}

	// The checks ran, but the run left objects in the cluster.
	if runErr != nil {
		_, _ = fmt.Fprintln(stderr, runErr)
		return exitError
	}

	if failed {
		return exitFailed
	}

	return exitPassed
}

// printResults prints one line for each result and a summary, and reports
// whether a check failed.
func printResults(w io.Writer, results []conformance.Result, reportPath string) bool {
	var passed, failed, skipped int
	for _, r := range results {
		switch r.Outcome {
		case conformance.OutcomePassed:
			passed++
			_, _ = fmt.Fprintf(w, "PASS %s\n", r.ID)
		case conformance.OutcomeFailed:
			failed++
			_, _ = fmt.Fprintf(w, "FAIL %s: %s\n", r.ID, r.Message)
		case conformance.OutcomeSkipped:
			skipped++
			_, _ = fmt.Fprintf(w, "SKIP %s: %s\n", r.ID, r.Message)
		}
	}

	_, _ = fmt.Fprintf(
		w,
		"%d checks: %d passed, %d failed, %d skipped. Report: %s\n",
		len(results),
		passed,
		failed,
		skipped,
		reportPath,
	)

	return failed > 0
}
