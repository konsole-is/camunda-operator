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

package camundaoptimize

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// registeredMutations are the mutations of every component, in registration
// order.
var registeredMutations = []string{
	MutationResources,
	MutationSchedulingConstraints,
	MutationPodMetadata,
	MutationExtraEnv,
	MutationExtraEnvFrom,
}

// A component without overrides registers every mutation and fires none: the
// preview is the base Deployment only.
func TestMutationsAreGatedOffWithoutOverrides(t *testing.T) {
	t.Parallel()

	in := fixtureMinimal(t)
	comps, err := Build(in)
	require.NoError(t, err)

	for _, comp := range comps {
		assert.Equal(t, registeredMutations, comp.RegisteredMutations(), comp.GetName())

		firing, err := comp.FiringSet()
		require.NoError(t, err)
		assert.Empty(t, firing, comp.GetName())

		template := previewedPodTemplate(t, previewObjects(t, comp))
		assert.Empty(t, template.Spec.Containers[0].Resources, comp.GetName())
		assert.Nil(t, template.Spec.Affinity, comp.GetName())
		assert.Empty(t, template.Spec.Tolerations, comp.GetName())
		assert.Empty(t, template.Spec.Containers[0].EnvFrom, comp.GetName())
		assert.Equal(t, discoveryLabels(in, comp.GetName()), template.Labels, comp.GetName())
		assert.Equal(
			t,
			map[string]string{ConfigHashAnnotation: ConfigHash(in, comp.GetName())},
			template.Annotations,
			comp.GetName(),
		)
	}
}

// Each override fires only on the component whose block sets it.
func TestMutationsFireOnTheirOwnComponent(t *testing.T) {
	t.Parallel()

	comps, err := Build(fixtureRealistic(t))
	require.NoError(t, err)
	firing := map[string][]string{}
	for _, comp := range comps {
		set, err := comp.FiringSet()
		require.NoError(t, err)
		firing[comp.GetName()] = set
	}

	assert.Equal(
		t,
		[]string{MutationResources, MutationPodMetadata, MutationExtraEnv, MutationExtraEnvFrom},
		firing[ComponentWebapp],
	)
	assert.Equal(
		t,
		[]string{MutationResources, MutationSchedulingConstraints},
		firing[ComponentImporter],
	)
}

// The overrides land on the previewed workload, and a user pod label never
// overrides a discovery label.
func TestMutationsApplyTheOverrides(t *testing.T) {
	t.Parallel()

	in := fixtureRealistic(t)
	in.Optimize.Spec.Webapp.PodLabels["camunda.io/component"] = "not-optimize"
	comps, err := Build(in)
	require.NoError(t, err)

	webapp := previewedPodTemplate(t, previewObjects(t, comps[0]))
	assert.Equal(t, "optimize-webapp", webapp.Labels["camunda.io/component"])
	assert.Equal(t, "platform", webapp.Labels["team"])
	assert.Equal(t, "true", webapp.Annotations["prometheus.io/scrape"])
	assert.NotEmpty(t, webapp.Annotations[ConfigHashAnnotation])
	assert.Equal(
		t,
		resource.MustParse("1Gi"),
		webapp.Spec.Containers[0].Resources.Requests[corev1.ResourceMemory],
	)
	assert.Contains(
		t,
		webapp.Spec.Containers[0].Env,
		corev1.EnvVar{Name: "OPTIMIZE_JAVA_OPTS", Value: "-Xmx2048m"},
	)
	require.Len(t, webapp.Spec.Containers[0].EnvFrom, 1)
	assert.Equal(t, "optimize-extra", webapp.Spec.Containers[0].EnvFrom[0].ConfigMapRef.Name)

	importer := previewedPodTemplate(t, previewObjects(t, comps[1]))
	require.Len(t, importer.Spec.Tolerations, 1)
	assert.Equal(t, "camunda", importer.Spec.Tolerations[0].Key)
}

// An extraEnv entry with the name of a rendered variable replaces it, so a
// user can override a setting the operator renders.
func TestExtraEnvOverridesARenderedVariable(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Optimize.Spec.Webapp = &v1.WorkloadSpec{
			ExtraEnv: []corev1.EnvVar{{Name: envZeebePartitionCount, Value: "9"}},
		}
	})
	comps, err := Build(in)
	require.NoError(t, err)

	template := previewedPodTemplate(t, previewObjects(t, comps[0]))
	assert.Equal(t, "9", envValueNamed(t, template.Spec.Containers[0].Env, envZeebePartitionCount))
	assert.Len(t, filterEnv(template.Spec.Containers[0].Env, envZeebePartitionCount), 1)
}

// filterEnv returns every entry with the given name.
func filterEnv(env []corev1.EnvVar, name string) []corev1.EnvVar {
	var found []corev1.EnvVar
	for _, e := range env {
		if e.Name == name {
			found = append(found, e)
		}
	}

	return found
}
