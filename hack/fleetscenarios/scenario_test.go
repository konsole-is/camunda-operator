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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeScenario writes body to a scenario.yaml in a new directory and
// returns its path.
func writeScenario(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "scenario.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

// validScenario is the smallest input that readScenario accepts.
const validScenario = `schemaVersion: 1
description: One namespace.
seed:
  - apiVersion: v1
    kind: Namespace
    metadata:
      name: team-a
`

func TestReadScenarioReadsAValidInput(t *testing.T) {
	s, err := readScenario(writeScenario(t, validScenario))
	require.NoError(t, err)
	assert.Equal(t, "One namespace.", s.Description)
	assert.Len(t, s.Seed, 1)
}

func TestReadScenarioRefusesAnUnknownSchemaVersion(t *testing.T) {
	body := strings.Replace(validScenario, "schemaVersion: 1", "schemaVersion: 2\nnewKey: x", 1)
	_, err := readScenario(writeScenario(t, body))
	require.Error(t, err)
	assert.ErrorContains(t, err, "schemaVersion 2 is not supported")
	assert.ErrorContains(t, err, "reads schemaVersion 1")
}

func TestReadScenarioRefusesAnUnknownKey(t *testing.T) {
	_, err := readScenario(writeScenario(t, validScenario+"overlay: []\n"))
	assert.ErrorContains(t, err, "overlay")
}

func TestReadScenarioRefusesAnOverlayWithoutAReason(t *testing.T) {
	body := validScenario + `overlays:
  - object: {apiVersion: v1, kind: Namespace, name: team-a}
    status: {phase: Active}
`
	_, err := readScenario(writeScenario(t, body))
	assert.ErrorContains(t, err, "overlay 1 of the scenario has no reason")
}

func TestReadScenarioRefusesAStateNamedLikeAnOutput(t *testing.T) {
	body := validScenario + `states:
  - name: objects
    description: Clashes with objects.yaml.
`
	_, err := readScenario(writeScenario(t, body))
	assert.ErrorContains(t, err, `state 1 has the name "objects", which is an output file`)
}

func TestReadScenarioRefusesTwoStatesWithOneName(t *testing.T) {
	body := validScenario + `states:
  - name: later
    description: First.
  - name: later
    description: Second.
`
	_, err := readScenario(writeScenario(t, body))
	assert.ErrorContains(t, err, `two states have the name "later"`)
}

func TestReadScenarioRefusesAJobStepWithReadyReplicas(t *testing.T) {
	body := validScenario + `workload:
  - kind: Job
    namespace: team-a
    name: backup
    jobOutcome: Complete
    readyReplicas: 1
`
	_, err := readScenario(writeScenario(t, body))
	assert.ErrorContains(
		t, err,
		"workload step 1 of the scenario: a Job step needs jobOutcome Complete or Failed, and no readyReplicas",
	)
}

func TestReadScenarioRefusesAStatefulSetStepWithoutReadyReplicas(t *testing.T) {
	body := validScenario + `states:
  - name: later
    description: A step without a count.
    workload:
      - kind: StatefulSet
        namespace: team-a
        name: orders-zeebe
`
	_, err := readScenario(writeScenario(t, body))
	assert.ErrorContains(
		t, err,
		`workload step 1 of state "later": a StatefulSet step needs readyReplicas of 0 or more, and no jobOutcome`,
	)
}

func TestReadScenarioRefusesAnEmptySeed(t *testing.T) {
	_, err := readScenario(writeScenario(t, "schemaVersion: 1\ndescription: Nothing.\n"))
	assert.ErrorContains(t, err, "seed is empty")
}

func TestReadScenarioReadsEveryCheckedInScenario(t *testing.T) {
	t.Skip("the scenarios land in Task 8")

	paths, err := filepath.Glob("../../fleet-manager/e2e/scenarios/*/scenario.yaml")
	require.NoError(t, err)
	require.NotEmpty(t, paths)

	for _, path := range paths {
		_, err := readScenario(path)
		assert.NoError(t, err, path)
	}
}
