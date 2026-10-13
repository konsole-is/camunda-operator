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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// scenariosDir holds the checked-in page scenarios.
const scenariosDir = "../../fleet-manager/e2e/scenarios"

// readOutput decodes a generated file of a checked-in scenario.
func readOutput(t *testing.T, scenarioName, file string) []unstructured.Unstructured {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(scenariosDir, scenarioName, file))
	require.NoError(t, err)

	var objs []unstructured.Unstructured
	for _, doc := range strings.Split(string(data), "\n---\n")[1:] {
		var obj map[string]any
		require.NoError(t, yaml.Unmarshal([]byte(doc), &obj))
		objs = append(objs, unstructured.Unstructured{Object: obj})
	}
	return objs
}

func TestTheReadyScenarioHoldsAHealthyCluster(t *testing.T) {
	objs := readOutput(t, "cluster-ready", objectsFile)
	ready := condition(t, find(t, objs, "CamundaCluster", "team-a", "camundacluster-sample"), "Ready")
	assert.Equal(t, "True", ready["status"])
	assert.Equal(t, "Healthy", ready["reason"])
}

func TestTheOneBrokerScenarioHoldsOneReadyBroker(t *testing.T) {
	objs := readOutput(t, "cluster-one-broker-ready", objectsFile)

	sts := find(t, objs, "StatefulSet", "team-a", "camundacluster-sample-zeebe")
	ready, _, _ := unstructured.NestedFieldNoCopy(sts.Object, "status", "readyReplicas")
	assert.EqualValues(t, 1, ready)

	cluster := find(t, objs, "CamundaCluster", "team-a", "camundacluster-sample")
	zeebe := condition(t, cluster, "ZeebeReady")
	assert.Equal(t, "False", zeebe["status"])
	assert.Contains(t, zeebe["message"], "1/3 ready")
	assert.Equal(t, "False", condition(t, cluster, "Ready")["status"])
}
