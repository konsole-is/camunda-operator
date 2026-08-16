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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// workloadMutationNames are the mutations of every process, in registration
// order; the brokers add VolumeRetention.
var workloadMutationNames = []string{
	MutationResources, MutationSchedulingConstraints, MutationPodMetadata, MutationServiceAccount,
}

// A process without overrides registers every mutation and fires none: the
// preview is the base workload only.
func TestMutationsAreGatedOffWithoutOverrides(t *testing.T) {
	t.Parallel()

	in := fixtureMinimal(t)
	comps, err := Build(in)
	require.NoError(t, err)

	for _, pc := range comps {
		want := workloadMutationNames
		if pc.Process.Kind == ProcessStatefulSet {
			want = append(append([]string{}, workloadMutationNames...), MutationVolumeRetention)
		}
		assert.Equal(t, want, pc.Component.RegisteredMutations(), pc.Process.Component)

		firing, err := pc.Component.FiringSet()
		require.NoError(t, err)
		assert.Empty(t, firing, pc.Process.Component)

		template := previewedPodTemplate(t, previewObjects(t, pc.Component))
		assert.Empty(t, template.Spec.Containers[0].Resources, pc.Process.Component)
		assert.Nil(t, template.Spec.Affinity, pc.Process.Component)
		assert.Empty(t, template.Spec.Tolerations, pc.Process.Component)
		assert.Empty(t, template.Spec.ServiceAccountName, pc.Process.Component)
		assert.Equal(t, discoveryLabels(in.Cluster, pc.Process.Component), template.Labels, pc.Process.Component)
		assert.Equal(t, []string{ConfigHashAnnotation}, keysOf(template.Annotations), pc.Process.Component)
	}
}

func keysOf(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Every override of the default fixture fires on the process it applies to,
// and a discovery label wins over a user label with the same key.
func TestMutationsFireOnOverrides(t *testing.T) {
	t.Parallel()

	in := fixtureDefault(t)
	comps, err := Build(in)
	require.NoError(t, err)

	byName := map[string]ProcessComponent{}
	for _, pc := range comps {
		byName[pc.Process.Component] = pc
	}

	zeebe, err := byName[ComponentZeebe].Component.FiringSet()
	require.NoError(t, err)
	assert.Equal(
		t,
		[]string{MutationResources, MutationSchedulingConstraints, MutationPodMetadata, MutationServiceAccount},
		zeebe,
	)

	// The gateway inherits the preset resources and the cluster pod labels
	// but has no scheduling of its own and the cluster sets none.
	gateway, err := byName[ComponentGateway].Component.FiringSet()
	require.NoError(t, err)
	assert.Equal(t, []string{MutationResources, MutationPodMetadata, MutationServiceAccount}, gateway)

	template := previewedPodTemplate(t, previewObjects(t, byName[ComponentZeebe].Component))
	assert.Equal(t, "my-cluster-camunda", template.Spec.ServiceAccountName)
	assert.Len(t, template.Spec.Tolerations, 1)
	assert.Equal(t, "platform", template.Labels["team"])
	assert.Equal(t, "zeebe", template.Labels["camunda.io/component"])
	assert.Equal(t, "true", template.Annotations["prometheus.io/scrape"])
	assert.NotEmpty(t, template.Annotations[ConfigHashAnnotation])
}

// VolumeRetention fires only for a Retain policy and switches the applied
// StatefulSet policy to Retain on deletion.
func TestVolumeRetentionMutation(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Cluster.Spec.Zeebe = &v1.ZeebeSpec{
			PersistentVolumeClaimRetentionPolicy: &v1.PersistentVolumeClaimRetentionPolicy{
				WhenDeleted: v1.RetainPersistentVolumeClaimRetentionPolicyType,
			},
		}
		in.Effective = NewEffective(in.Cluster.Spec)
	})
	comps, err := Build(in)
	require.NoError(t, err)

	firing, err := comps[0].Component.FiringSet()
	require.NoError(t, err)
	assert.Equal(t, []string{MutationVolumeRetention}, firing)

	for _, obj := range previewObjects(t, comps[0].Component) {
		if sts, ok := obj.(*appsv1.StatefulSet); ok {
			assert.Equal(
				t,
				appsv1.RetainPersistentVolumeClaimRetentionPolicyType,
				sts.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted,
			)
			assert.Equal(
				t,
				appsv1.RetainPersistentVolumeClaimRetentionPolicyType,
				sts.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled,
			)
		}
	}
}
