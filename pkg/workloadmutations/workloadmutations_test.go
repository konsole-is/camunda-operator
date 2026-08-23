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

package workloadmutations_test

import (
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/deployment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/workloadmutations"
)

// theContainer is the container that the mutations of these tests edit. The
// pod carries a sidecar of another name.
const theContainer = "app"

// registeredMutations are the mutations that Mutations returns, in
// registration order.
var registeredMutations = []string{
	workloadmutations.MutationResources,
	workloadmutations.MutationSchedulingConstraints,
	workloadmutations.MutationPodMetadata,
	workloadmutations.MutationExtraEnv,
	workloadmutations.MutationExtraEnvFrom,
}

// A component without overrides registers every mutation and fires none: the
// workload keeps what it was rendered with.
func TestMutationsAreGatedOffWithoutOverrides(t *testing.T) {
	t.Parallel()

	workload := baseWorkload()
	apply(t, workload, v1.WorkloadSpec{})

	assert.Empty(t, workload.Spec.Template.Spec.Containers[0].Resources.Requests)
	assert.Nil(t, workload.Spec.Template.Spec.Affinity)
	assert.Empty(t, workload.Spec.Template.Spec.Tolerations)
	assert.Empty(t, workload.Spec.Template.Spec.Containers[0].EnvFrom)
	assert.Equal(t, map[string]string{"camunda.io/component": "rendered"}, workload.Spec.Template.Labels)
}

// The overrides land on the pod and on the named container. The container
// scope is what a golden test cannot show: a pod of one container renders the
// same whether a mutation edits every container or one, so the sidecar is what
// tells the two apart.
func TestMutationsEditTheNamedContainerOnly(t *testing.T) {
	t.Parallel()

	workload := baseWorkload()
	apply(t, workload, v1.WorkloadSpec{
		Resources: &corev1.ResourceRequirements{Requests: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		}},
		ExtraEnv: []corev1.EnvVar{{Name: "JAVA_OPTS", Value: "-Xmx2048m"}},
		ExtraEnvFrom: []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{Name: "extra"},
		}}},
		PodLabels:      map[string]string{"camunda.io/component": "user", "team": "platform"},
		PodAnnotations: map[string]string{"prometheus.io/scrape": "true"},
		Scheduling: &v1.SchedulingSpec{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{{
						MatchExpressions: []corev1.NodeSelectorRequirement{{
							Key:      "topology.kubernetes.io/zone",
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"eu-west-1a"},
						}},
					}},
				},
			},
			Tolerations: []corev1.Toleration{{Key: "camunda", Operator: corev1.TolerationOpExists}},
		},
	})

	edited, sidecar := workload.Spec.Template.Spec.Containers[0], workload.Spec.Template.Spec.Containers[1]

	assert.Equal(t, resource.MustParse("1Gi"), edited.Resources.Requests[corev1.ResourceMemory])
	assert.Contains(t, edited.Env, corev1.EnvVar{Name: "JAVA_OPTS", Value: "-Xmx2048m"})
	assert.Len(t, edited.EnvFrom, 1)

	assert.Empty(t, sidecar.Resources.Requests, "the sidecar keeps its own resources")
	assert.Empty(t, sidecar.Env, "extraEnv is not for the sidecar")
	assert.Empty(t, sidecar.EnvFrom, "extraEnvFrom is not for the sidecar")

	require.NotNil(t, workload.Spec.Template.Spec.Affinity)
	assert.NotNil(t, workload.Spec.Template.Spec.Affinity.NodeAffinity)
	require.Len(t, workload.Spec.Template.Spec.Tolerations, 1)
	assert.Equal(t, "camunda", workload.Spec.Template.Spec.Tolerations[0].Key)
	assert.Equal(t, "true", workload.Spec.Template.Annotations["prometheus.io/scrape"])
	assert.Equal(t, "platform", workload.Spec.Template.Labels["team"])
	assert.Equal(
		t,
		"rendered",
		workload.Spec.Template.Labels["camunda.io/component"],
		"a user pod label never overrides a discovery label",
	)
}

// An extraEnv entry with the name of a rendered variable replaces it, so a
// user can override a setting the operator renders.
func TestExtraEnvOverridesARenderedVariable(t *testing.T) {
	t.Parallel()

	workload := baseWorkload()
	workload.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{{Name: "LOG_LEVEL", Value: "INFO"}}
	apply(t, workload, v1.WorkloadSpec{
		ExtraEnv: []corev1.EnvVar{{Name: "LOG_LEVEL", Value: "DEBUG"}},
	})

	assert.Equal(
		t,
		[]corev1.EnvVar{{Name: "LOG_LEVEL", Value: "DEBUG"}},
		workload.Spec.Template.Spec.Containers[0].Env,
	)
}

// baseWorkload is a rendered Deployment of two containers: the one the
// mutations name and a sidecar.
func baseWorkload() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "workload", Namespace: "camunda"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"camunda.io/component": "rendered"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: theContainer}, {Name: "sidecar"}},
				},
			},
		},
	}
}

// apply layers the mutations of spec onto workload. It honours the gate of
// each mutation, the way the builder does, so a mutation whose field spec
// leaves unset never runs.
func apply(t *testing.T, workload *appsv1.Deployment, spec v1.WorkloadSpec) {
	t.Helper()

	mutations := workloadmutations.Mutations(spec, theContainer)
	mutator := deployment.NewMutator(workload)
	registered := make([]string, 0, len(mutations))
	for _, mutation := range mutations {
		gated := feature.Mutation[*deployment.Mutator](mutation)
		require.NoError(t, gated.ApplyIntent(mutator), mutation.Name)
		mutator.NextFeature()
		registered = append(registered, mutation.Name)
	}
	require.NoError(t, mutator.Apply())

	assert.Equal(t, registeredMutations, registered)
}
