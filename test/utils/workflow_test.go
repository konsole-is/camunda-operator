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

package utils

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/onsi/ginkgo/v2/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// e2eWorkflowPath is the workflow that runs the e2e suite, under the module
// root.
const e2eWorkflowPath = ".github/workflows/test-e2e.yml"

// e2eWorkflow is the part of the workflow these tests read: the matrix of the
// job that runs the suite.
type e2eWorkflow struct {
	Jobs struct {
		TestE2E struct {
			Strategy struct {
				Matrix struct {
					Include []e2eMatrixEntry `json:"include"`
				} `json:"matrix"`
			} `json:"strategy"`
		} `json:"test-e2e"`
	} `json:"jobs"`
}

// e2eMatrixEntry is one job of the matrix. Labels is the Ginkgo label filter
// that selects the flows of the job.
type e2eMatrixEntry struct {
	Minor  string `json:"minor"`
	Suffix string `json:"suffix"`
	Labels string `json:"labels"`
}

func (e e2eMatrixEntry) jobName() string {
	return "Camunda " + e.Minor + e.Suffix
}

func TestWorkflowLabelFiltersParse(t *testing.T) {
	for _, entry := range readE2EMatrix(t) {
		t.Run(entry.jobName(), func(t *testing.T) {
			require.NotEmpty(t, entry.Labels, "the job runs every flow without a label filter")

			_, err := types.ParseLabelFilter(entry.Labels)
			assert.NoError(t, err)
		})
	}
}

// TestWorkflowJobsClaimEveryLabel holds the split of the suite over the jobs
// of one Camunda minor. A label that no job runs, and a label that two jobs
// run, both fail here instead of passing as a green check.
func TestWorkflowJobsClaimEveryLabel(t *testing.T) {
	byMinor := map[string][]e2eMatrixEntry{}
	for _, entry := range readE2EMatrix(t) {
		byMinor[entry.Minor] = append(byMinor[entry.Minor], entry)
	}

	for _, minor := range slices.Sorted(maps.Keys(byMinor)) {
		t.Run(minor, func(t *testing.T) {
			filters := map[string]types.LabelFilter{}
			for _, entry := range byMinor[minor] {
				filter, err := types.ParseLabelFilter(entry.Labels)
				require.NoError(t, err, "job %q", entry.jobName())
				filters[entry.jobName()] = filter
			}

			for _, label := range AllLabels {
				var claimed []string
				for _, name := range slices.Sorted(maps.Keys(filters)) {
					if filters[name]([]string{label}) {
						claimed = append(claimed, name)
					}
				}

				assert.Lenf(
					t,
					claimed,
					1,
					"label %q is claimed by %d jobs (%s), want exactly one",
					label,
					len(claimed),
					strings.Join(claimed, ", "),
				)
			}
		})
	}
}

func readE2EMatrix(t *testing.T) []e2eMatrixEntry {
	t.Helper()

	root, err := ModuleRoot()
	require.NoError(t, err)

	path := filepath.Join(root, e2eWorkflowPath)
	content, err := os.ReadFile(path) // nolint:gosec // a path of this repository
	require.NoError(t, err)

	var workflow e2eWorkflow
	require.NoError(t, yaml.Unmarshal(content, &workflow), "parsing %s", path)

	entries := workflow.Jobs.TestE2E.Strategy.Matrix.Include
	require.NotEmpty(t, entries, "no matrix entries in %s", path)

	return entries
}
