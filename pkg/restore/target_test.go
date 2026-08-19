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

package restore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	return scheme
}

func testCluster() *v1.CamundaCluster {
	return &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cluster", Namespace: "ns"},
	}
}

// brokerStatefulSet is the suspended broker StatefulSet of the fixture
// cluster, shaped like pkg/components/camundacluster renders it.
func brokerStatefulSet() *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster-zeebe",
			Namespace: "ns",
			Labels: map[string]string{
				"camunda.io/cluster":           "my-cluster",
				"camunda.io/component":         "zeebe",
				"app.kubernetes.io/managed-by": "camunda-operator",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: new(int32(0)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					"camunda.io/cluster": "my-cluster", "camunda.io/component": "zeebe",
				}},
				Spec: corev1.PodSpec{
					ServiceAccountName: "my-cluster-camunda",
					Containers: []corev1.Container{{
						Name:  "camunda",
						Image: "camunda/camunda:8.9.9",
						Env: []corev1.EnvVar{
							{Name: "CAMUNDA_CLUSTER_SIZE", Value: "3"},
							{Name: "CAMUNDA_CLUSTER_PARTITIONCOUNT", Value: "6"},
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "data", MountPath: "/usr/local/camunda/data"},
						},
					}},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "data",
					Labels: map[string]string{"camunda.io/cluster": "my-cluster", "camunda.io/component": "zeebe"},
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
					},
				},
			}},
		},
	}
}

func TestReadTargetReadsTheFactsOffTheLiveBroker(t *testing.T) {
	t.Parallel()

	sts := brokerStatefulSet()
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(sts).Build()

	target, err := ReadTarget(context.Background(), c, testCluster())
	require.NoError(t, err)
	assert.Equal(t, int32(3), target.Brokers)
	assert.Equal(t, int32(6), target.Partitions)
	assert.Equal(t, "8.9.9", target.Version)
	assert.Equal(t, "camunda", target.Broker.Name)
	assert.Equal(t, "data", target.ClaimTemplate.Name)
	assert.Equal(t, "my-cluster-zeebe", target.StatefulSet.Name)
	assert.Equal(t, "my-cluster", target.ClusterName)
}

// The broker count comes from the live container, never from spec.replicas.
// A suspended StatefulSet runs at zero replicas, and a restore still has to
// recreate every broker volume.
func TestReadTargetTakesTheBrokerCountFromTheContainerNotTheReplicas(t *testing.T) {
	t.Parallel()

	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(brokerStatefulSet()).Build()

	target, err := ReadTarget(context.Background(), c, testCluster())
	require.NoError(t, err)
	assert.Equal(t, int32(0), *target.StatefulSet.Spec.Replicas)
	assert.Equal(t, int32(3), target.Brokers)
}

func TestReadTargetReportsAMissingStatefulSetAsAnInvalidReference(t *testing.T) {
	t.Parallel()

	c := fake.NewClientBuilder().WithScheme(testScheme(t)).Build()

	_, err := ReadTarget(context.Background(), c, testCluster())
	var failure *conditions.PreCheckFailure
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, v1.ReasonInvalidReference, failure.Reason)
	assert.Contains(t, failure.Message, "my-cluster-zeebe")
}

// Every fact the restore needs must be readable off the StatefulSet. A
// StatefulSet that cannot answer one of them is an invalid reference with a
// message that names what is missing.
func TestReadTargetReportsEveryUnreadableFact(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		mutate func(*appsv1.StatefulSet)
		text   string
	}{
		"no broker container": {
			mutate: func(s *appsv1.StatefulSet) {
				s.Spec.Template.Spec.Containers[0].Name = "sidecar"
			},
			text: "camunda",
		},
		"no cluster size": {
			mutate: func(s *appsv1.StatefulSet) {
				s.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
					{Name: "CAMUNDA_CLUSTER_PARTITIONCOUNT", Value: "6"},
				}
			},
			text: "CAMUNDA_CLUSTER_SIZE",
		},
		"cluster size is not a number": {
			mutate: func(s *appsv1.StatefulSet) {
				s.Spec.Template.Spec.Containers[0].Env[0].Value = "three"
			},
			text: "CAMUNDA_CLUSTER_SIZE",
		},
		"cluster size comes from a reference": {
			mutate: func(s *appsv1.StatefulSet) {
				s.Spec.Template.Spec.Containers[0].Env[0] = corev1.EnvVar{
					Name: "CAMUNDA_CLUSTER_SIZE",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
					},
				}
			},
			text: "CAMUNDA_CLUSTER_SIZE",
		},
		"no partition count": {
			mutate: func(s *appsv1.StatefulSet) {
				s.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
					{Name: "CAMUNDA_CLUSTER_SIZE", Value: "3"},
				}
			},
			text: "CAMUNDA_CLUSTER_PARTITIONCOUNT",
		},
		"broker image carries no tag": {
			mutate: func(s *appsv1.StatefulSet) {
				s.Spec.Template.Spec.Containers[0].Image = "camunda/camunda"
			},
			text: "camunda/camunda",
		},
		"no data claim template": {
			mutate: func(s *appsv1.StatefulSet) { s.Spec.VolumeClaimTemplates = nil },
			text:   "data",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			sts := brokerStatefulSet()
			tc.mutate(sts)
			c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(sts).Build()

			_, err := ReadTarget(context.Background(), c, testCluster())
			var failure *conditions.PreCheckFailure
			require.ErrorAs(t, err, &failure)
			assert.Equal(t, v1.ReasonInvalidReference, failure.Reason)
			assert.Contains(t, failure.Message, tc.text)
			assert.Contains(t, failure.Message, "my-cluster-zeebe")
		})
	}
}

// A digest pins the image without a tag, and the restore has no version to
// compare against then.
func TestReadTargetTakesTheVersionFromTheImageTag(t *testing.T) {
	t.Parallel()

	sts := brokerStatefulSet()
	sts.Spec.Template.Spec.Containers[0].Image = "registry.example.com:5000/camunda/camunda:8.9.10"
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(sts).Build()

	target, err := ReadTarget(context.Background(), c, testCluster())
	require.NoError(t, err)
	assert.Equal(t, "8.9.10", target.Version)
}
