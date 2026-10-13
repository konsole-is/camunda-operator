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

package camundacluster

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The API server refuses a pod whose request is above its limit, so a
// sample cluster on top of the sample preset must merge to resources that fit.
func TestMergeSpecOfTheSamplesKeepsEachRequestWithinItsLimit(t *testing.T) {
	var cluster v1.CamundaCluster
	readSample(t, "core_v1_camundacluster.yaml", &cluster)

	var preset v1.CamundaClusterPreset
	readSample(t, "core_v1_camundaclusterpreset.yaml", &preset)
	require.Equal(t, cluster.Spec.PresetRef, preset.Name, "the cluster sample must reference the preset sample")

	merged := MergeSpec(cluster.Spec, &preset.Spec, nil)

	for name, resources := range workloadResources(merged) {
		if resources == nil {
			continue
		}
		for resourceName, limit := range resources.Limits {
			request, ok := resources.Requests[resourceName]
			if !ok {
				continue
			}
			assert.LessOrEqual(
				t,
				request.Cmp(limit),
				0,
				"%s requests %s %s, above its limit %s",
				name,
				resourceName,
				request.String(),
				limit.String(),
			)
		}
	}
}

func readSample(t *testing.T, file string, into any) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "..", "config", "samples", file))
	require.NoError(t, err)
	require.NoError(t, yaml.UnmarshalStrict(data, into), file)
}

func workloadResources(spec v1.CamundaClusterSpec) map[string]*corev1.ResourceRequirements {
	resources := map[string]*corev1.ResourceRequirements{}
	if spec.Zeebe != nil {
		resources["zeebe"] = spec.Zeebe.Resources
	}
	if spec.Gateway != nil {
		resources["gateway"] = spec.Gateway.Resources
	}
	if spec.Operate != nil {
		resources["operate"] = spec.Operate.Resources
	}
	if spec.Tasklist != nil {
		resources["tasklist"] = spec.Tasklist.Resources
	}
	if spec.Admin != nil {
		resources["admin"] = spec.Admin.Resources
	}
	if spec.Connectors != nil {
		resources["connectors"] = spec.Connectors.Resources
	}
	if spec.Backup != nil && spec.Backup.Dump != nil {
		resources["backup.dump"] = spec.Backup.Dump.Resources
	}

	return resources
}
