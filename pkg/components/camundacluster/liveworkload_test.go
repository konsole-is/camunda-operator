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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestLiveZeebeWorkloadWaitsForAWorkloadThatIsNotRendered(t *testing.T) {
	s := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(s))
	cluster := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cc"}}

	workload, failure, err := LiveZeebeWorkload(t.Context(), fake.NewClientBuilder().WithScheme(s).Build(), cluster)
	require.NoError(t, err)
	assert.Nil(t, workload)
	require.NotNil(t, failure)
	assert.Equal(t, v1.ReasonProgressing, failure.Reason)

	rendered := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{
		Namespace: "ns", Name: WorkloadName(cluster, ComponentZeebe),
	}}
	reader := fake.NewClientBuilder().WithScheme(s).WithObjects(rendered).Build()
	workload, failure, err = LiveZeebeWorkload(t.Context(), reader, cluster)
	require.NoError(t, err)
	assert.Nil(t, failure)
	assert.Equal(t, rendered.Name, workload.Name)
}

func TestRunningConfigHashWaitsForATemplateWithoutAHash(t *testing.T) {
	workload := &appsv1.StatefulSet{}

	_, failure := RunningConfigHash(workload)
	require.NotNil(t, failure)
	assert.Equal(t, v1.ReasonProgressing, failure.Reason)

	workload.Spec.Template.Annotations = map[string]string{ConfigHashAnnotation: "hash-1"}
	hash, failure := RunningConfigHash(workload)
	assert.Nil(t, failure)
	assert.Equal(t, "hash-1", hash)
}

func TestTemplateEnvValueReadsOnlyAPlainValue(t *testing.T) {
	template := &corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{
		{Name: "init", Env: []corev1.EnvVar{{
			Name:      "FROM_SECRET",
			ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: "k"}},
		}}},
		{Name: "main", Env: []corev1.EnvVar{{Name: "PLAIN", Value: "v"}}},
	}}}

	value, ok := TemplateEnvValue(template, "PLAIN")
	assert.True(t, ok, "a value on any container counts")
	assert.Equal(t, "v", value)

	_, ok = TemplateEnvValue(template, "FROM_SECRET")
	assert.False(t, ok, "a reference is not a value that the template runs")

	_, ok = TemplateEnvValue(template, "ABSENT")
	assert.False(t, ok)
}
