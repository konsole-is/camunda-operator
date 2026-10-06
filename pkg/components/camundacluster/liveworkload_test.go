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
	"github.com/konsole-is/camunda-operator/pkg/camundaconfig"
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

func TestRunningVersionWaitsForAWorkloadWithoutAVersion(t *testing.T) {
	workload := &appsv1.StatefulSet{}

	_, failure := RunningVersion(workload)
	require.NotNil(t, failure)
	assert.Equal(t, v1.ReasonProgressing, failure.Reason)

	workload.Annotations = map[string]string{BrokerVersionAnnotation: "8.9.9"}
	version, failure := RunningVersion(workload)
	assert.Nil(t, failure)
	assert.Equal(t, "8.9.9", version)
}

func TestRunsPublishedVersionRequiresTheVersionOfTheBinding(t *testing.T) {
	workload := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{
		Annotations: map[string]string{BrokerVersionAnnotation: "8.9.9"},
	}}
	tests := []struct {
		name    string
		binding *v1.ManagementBinding
		wait    string
	}{
		{name: "the binding publishes the running version", binding: &v1.ManagementBinding{Version: "8.9.9"}},
		{
			name:    "the binding publishes another version",
			binding: &v1.ManagementBinding{Version: "8.9.10"},
			wait:    "8.9.10",
		},
		{name: "the cluster publishes no binding", wait: `""`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := &v1.CamundaCluster{Status: v1.CamundaClusterStatus{Management: tt.binding}}

			failure := RunsPublishedVersion(workload, cluster)
			if tt.wait == "" {
				assert.Nil(t, failure)
				return
			}
			require.NotNil(t, failure)
			assert.Equal(t, v1.ReasonProgressing, failure.Reason)
			assert.Contains(t, failure.Message, "8.9.9")
			assert.Contains(t, failure.Message, tt.wait)
		})
	}
}

func TestRolledOutRequiresEveryReplicaOnTheCurrentTemplate(t *testing.T) {
	rolledOut := func() *appsv1.StatefulSet {
		replicas := int32(3)
		return &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Generation: 2},
			Spec:       appsv1.StatefulSetSpec{Replicas: &replicas},
			Status: appsv1.StatefulSetStatus{
				ObservedGeneration: 2, UpdatedReplicas: 3, ReadyReplicas: 3,
				CurrentRevision: "rev-2", UpdateRevision: "rev-2",
			},
		}
	}
	tests := []struct {
		name   string
		mutate func(*appsv1.StatefulSet)
		rolled bool
	}{
		{name: "every replica runs the current template", mutate: func(*appsv1.StatefulSet) {}, rolled: true},
		{
			name:   "the template is not observed yet",
			mutate: func(w *appsv1.StatefulSet) { w.Status.ObservedGeneration = 1 },
		},
		{
			name:   "a replica runs the previous template",
			mutate: func(w *appsv1.StatefulSet) { w.Status.UpdatedReplicas = 2 },
		},
		{
			name:   "a replica on the current template is not ready",
			mutate: func(w *appsv1.StatefulSet) { w.Status.ReadyReplicas = 2 },
		},
		{
			name:   "the update revision is not current yet",
			mutate: func(w *appsv1.StatefulSet) { w.Status.CurrentRevision = "rev-1" },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workload := rolledOut()
			tt.mutate(workload)

			failure := RolledOut(workload)
			if tt.rolled {
				assert.Nil(t, failure)
				return
			}
			require.NotNil(t, failure)
			assert.Equal(t, v1.ReasonProgressing, failure.Reason)
		})
	}
}

func TestRunsBackupStoreComparesThePlainValuesOfTheStore(t *testing.T) {
	cluster := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cc"}}
	bucket := func(name, secret string) *v1.ObjectStorageConfig {
		return &v1.ObjectStorageConfig{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "bucket"},
			Spec: v1.ObjectStorageConfigSpec{Type: v1.ObjectStorageTypeS3, S3: &v1.S3Storage{
				BucketName: name, Region: "eu-west-1",
				Auth: v1.S3StorageAuth{
					Type: v1.ObjectStorageAuthTypeCredentials,
					Credentials: &v1.S3Credentials{SecretRef: v1.S3CredentialsSecretRef{
						Name: secret, AccessKeyIDKey: "id", SecretAccessKeyKey: "key",
					}},
				},
			}},
		}
	}
	template := &corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{
		Name: "zeebe", Env: BackupStoreEnv(cluster, bucket("backups", "creds")),
	}}}}

	assert.Nil(t, RunsBackupStore(template, cluster, bucket("backups", "creds")))
	assert.Nil(t, RunsBackupStore(template, cluster, bucket("backups", "rotated")), "credentials are not compared")

	failure := RunsBackupStore(template, cluster, bucket("moved", "creds"))
	require.NotNil(t, failure)
	assert.Equal(t, v1.ReasonProgressing, failure.Reason)
	assert.Contains(t, failure.Message, "moved")

	withEndpoint := bucket("backups", "creds")
	withEndpoint.Spec.S3.Endpoint = "https://minio.example:9000"
	template.Spec.Containers[0].Env = BackupStoreEnv(cluster, withEndpoint)
	failure = RunsBackupStore(template, cluster, bucket("backups", "creds"))
	require.NotNil(t, failure, "a store key that the declared bucket drops must be gone from the template")
	assert.Contains(t, failure.Message, camundaconfig.KeyPrimaryBackupS3Endpoint.Env())
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
