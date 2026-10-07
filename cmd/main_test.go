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
	"flag"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chartReadme is the page that Artifact Hub shows for the chart. Its section
// "Manager settings" is the list of manager flags that users read.
const chartReadme = "../dist/chart/README.md"

// importedEnv maps a flag that an imported package registers to the
// environment variable that the package reads for the same setting. The
// manager does not read these through getenv, so bindFlags cannot show them.
var importedEnv = map[string]string{
	// controller-runtime reads KUBECONFIG when --kubeconfig is empty.
	"kubeconfig": "KUBECONFIG",
}

// TestManagerSettingsTable fails when the table "Manager settings" of the
// chart README and the flags of the manager differ in either direction, when
// the table and the manager differ on the environment variables, or when a
// row names an environment variable that does not set the flag of that row.
func TestManagerSettingsTable(t *testing.T) {
	rows := readSettingsTable(t, chartReadme)

	read := map[string]bool{}
	fs := flag.NewFlagSet("manager", flag.ContinueOnError)
	bindFlags(fs, func(name string) string {
		read[name] = true
		return ""
	})

	flags := map[string]bool{}
	fs.VisitAll(func(f *flag.Flag) { flags[f.Name] = true })
	// The manager parses flag.CommandLine, so a flag that an imported package
	// registers there in its init function is a manager flag too.
	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		// go test registers the test.* flags. The manager binary has none.
		if !strings.HasPrefix(f.Name, "test.") {
			flags[f.Name] = true
		}
	})

	assert.ElementsMatch(
		t,
		slices.Collect(maps.Keys(flags)),
		slices.Collect(maps.Keys(rows)),
		"the flags of the manager and the flags in %s differ", chartReadme,
	)

	envs := slices.Collect(maps.Keys(read))
	for name, env := range importedEnv {
		envs = append(envs, env)
		assert.Equal(t, env, rows[name], "the environment variable of --%s in %s", name, chartReadme)
	}

	var documented []string
	for _, env := range rows {
		if env != "" {
			documented = append(documented, env)
		}
	}

	assert.ElementsMatch(
		t,
		envs,
		documented,
		"the environment variables that the manager reads and those in %s differ, or one is on two rows",
		chartReadme,
	)

	for name, env := range rows {
		if env == "" || importedEnv[name] != "" {
			continue
		}

		assert.True(
			t,
			envSetsFlag(name, env),
			"%s lists %s for --%s, but that variable does not change the default of --%s",
			chartReadme, env, name, name,
		)
	}
}

// envSetsFlag reports whether the environment variable env changes the
// default of the manager flag name.
func envSetsFlag(name, env string) bool {
	defaultWith := func(getenv func(string) string) string {
		fs := flag.NewFlagSet("manager", flag.ContinueOnError)
		bindFlags(fs, getenv)
		if f := fs.Lookup(name); f != nil {
			return f.DefValue
		}

		return ""
	}

	// 1h parses as a string and as a duration, so it changes a flag of
	// either kind.
	set := defaultWith(func(n string) string {
		if n == env {
			return "1h"
		}

		return ""
	})

	return set != defaultWith(func(string) string { return "" })
}

// readSettingsTable returns the rows of the table under "## Manager settings"
// in the file at path, as a map from flag name, without the leading dashes,
// to the environment variable. The variable is empty for "none". It fails the
// test on a row whose flag or environment variable cell it cannot read.
func readSettingsTable(t *testing.T, path string) map[string]string {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	_, section, found := strings.Cut(string(content), "\n## Manager settings\n")
	require.True(t, found, "%s has no section \"## Manager settings\"", path)

	section, _, _ = strings.Cut(section, "\n## ")

	rows := map[string]string{}
	for line := range strings.SplitSeq(section, "\n") {
		cells := strings.Split(line, "|")
		if len(cells) < 3 || strings.Trim(strings.TrimSpace(cells[1]), ":-") == "" ||
			strings.TrimSpace(cells[1]) == "Flag" {
			continue
		}

		name, ok := codeSpan(cells[1])
		require.True(t, ok && strings.HasPrefix(name, "--"), "the flag cell of %q is not `--<name>`", line)

		env, ok := codeSpan(cells[2])
		if !ok {
			require.Equal(t, "none", strings.TrimSpace(cells[2]), "the environment variable cell of %q", line)
		}

		flagName := strings.TrimPrefix(name, "--")
		require.NotContains(t, rows, flagName, "%s lists --%s twice", path, flagName)
		rows[flagName] = env
	}

	require.NotEmpty(t, rows, "the section \"## Manager settings\" of %s has no table rows", path)

	return rows
}

// codeSpan returns the text of a cell that holds one code span and nothing
// else, and false for any other cell.
func codeSpan(cell string) (string, bool) {
	cell = strings.TrimSpace(cell)
	if len(cell) < 3 || !strings.HasPrefix(cell, "`") || !strings.HasSuffix(cell, "`") {
		return "", false
	}

	inner := cell[1 : len(cell)-1]
	if strings.Contains(inner, "`") {
		return "", false
	}

	return inner, true
}
