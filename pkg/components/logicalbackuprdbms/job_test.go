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

package logicalbackuprdbms

import (
	"flag"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/testing/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// updateGolden refreshes the golden manifests with the rendered output:
// go test ./pkg/components/logicalbackuprdbms/ -run Golden -update-golden
var updateGolden = flag.Bool("update-golden", false, "update golden files")

// jobPreview adapts the built Job to the golden previewer.
type jobPreview struct{ input JobInput }

func (p jobPreview) Preview() (client.Object, error) { return BuildJob(p.input) }

func goldenScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	return scheme
}

func backup() *v1.LogicalBackupRDBMS {
	return &v1.LogicalBackupRDBMS{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cluster-1748937221000", Namespace: "my-cluster-ns"},
		Spec: v1.LogicalBackupRDBMSSpec{
			ClusterRef: v1.ClusterRef{Name: "my-cluster"},
		},
	}
}

func s3Bucket(auth v1.S3StorageAuth) *v1.ObjectStorageConfig {
	return &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "my-backup-config"},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeS3,
			S3: &v1.S3Storage{
				BucketName: "camunda-backups",
				BasePath:   "clusters",
				Region:     "eu-west-1",
				Endpoint:   "http://minio.minio.svc:9000",
				Auth:       auth,
			},
		},
	}
}

func s3Credentials() v1.S3StorageAuth {
	return v1.S3StorageAuth{
		Type: v1.ObjectStorageAuthTypeCredentials,
		Credentials: &v1.S3Credentials{
			SecretRef: v1.S3CredentialsSecretRef{
				Name:               "minio-credentials",
				Namespace:          "camunda",
				AccessKeyIDKey:     "accessKeyId",
				SecretAccessKeyKey: "secretAccessKey",
			},
		},
	}
}

func input() JobInput {
	return JobInput{
		Backup:             backup(),
		ClusterName:        "my-cluster",
		ClusterNamespace:   "my-cluster-ns",
		Bucket:             s3Bucket(s3Credentials()),
		BucketSecretName:   "my-cluster-camunda-backup-credentials",
		DBSecretName:       "my-cluster-1748937221000-dump-credentials",
		DBUsernameKey:      "username",
		DBPasswordKey:      "password",
		ServiceAccountName: "my-cluster-camunda",
		ServerVersion:      "17",
		Host:               "postgres.databases.svc",
		Port:               5432,
		Database:           "camunda",
		ObjectKey:          "clusters/my-cluster-ns/my-cluster/1748937221000/camunda.dump",
		OperatorImage:      "ghcr.io/konsole-is/camunda-operator:0.1.0",
	}
}

func TestJobGoldenS3Credentials(t *testing.T) {
	t.Parallel()

	in := input()
	in.Dump = &v1.BackupDumpSpec{
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("512Mi")},
		},
		ExtraEnv:       []corev1.EnvVar{{Name: "PGSSLMODE", Value: "require"}},
		PodLabels:      map[string]string{"team": "data"},
		PodAnnotations: map[string]string{"sidecar.istio.io/inject": "false"},
		Scheduling: &v1.SchedulingSpec{
			Tolerations: []corev1.Toleration{{Key: "backups", Operator: corev1.TolerationOpExists}},
		},
		ScratchVolume: &v1.ScratchVolumeSpec{SizeLimit: ptrQuantity("50Gi")},
	}

	golden.AssertYAML(
		t, "testdata/golden/s3-credentials/job.yaml", jobPreview{in},
		golden.WithScheme(goldenScheme(t)), golden.Update(*updateGolden),
	)
}

func TestJobGoldenS3WorkloadIdentity(t *testing.T) {
	t.Parallel()

	in := input()
	in.Bucket = s3Bucket(v1.S3StorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity})
	in.BucketSecretName = ""

	golden.AssertYAML(
		t, "testdata/golden/s3-workload-identity/job.yaml", jobPreview{in},
		golden.WithScheme(goldenScheme(t)), golden.Update(*updateGolden),
	)
}

func TestJobGoldenGCSCredentials(t *testing.T) {
	t.Parallel()

	in := input()
	in.Bucket = &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "my-backup-config"},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeGCS,
			GCS: &v1.GCSStorage{
				BucketName: "camunda-backups",
				BasePath:   "clusters",
				Auth: v1.GCSStorageAuth{
					Type: v1.ObjectStorageAuthTypeCredentials,
					Credentials: &v1.GCSCredentials{
						SecretRef: v1.SecretKeyRef{
							Name:      "gcs-key",
							Namespace: "camunda",
							Key:       "key.json",
						},
					},
				},
			},
		},
	}

	golden.AssertYAML(
		t, "testdata/golden/gcs-credentials/job.yaml", jobPreview{in},
		golden.WithScheme(goldenScheme(t)), golden.Update(*updateGolden),
	)
}

func TestJobGoldenAzureCredentials(t *testing.T) {
	t.Parallel()

	in := input()
	in.Bucket = &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "my-backup-config"},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeAzureBlob,
			AzureBlob: &v1.AzureBlobStorage{
				AccountName: "camundabackups",
				Container:   "backups",
				BasePath:    "clusters",
				Auth: v1.AzureBlobStorageAuth{
					Type: v1.ObjectStorageAuthTypeCredentials,
					Credentials: &v1.AzureBlobCredentials{
						SecretRef: v1.SecretKeyRef{
							Name:      "azure-key",
							Namespace: "camunda",
							Key:       "accountKey",
						},
					},
				},
			},
		},
	}

	golden.AssertYAML(
		t, "testdata/golden/azure-credentials/job.yaml", jobPreview{in},
		golden.WithScheme(goldenScheme(t)), golden.Update(*updateGolden),
	)
}

func TestJobGoldenScratchPVC(t *testing.T) {
	t.Parallel()

	in := input()
	in.Dump = &v1.BackupDumpSpec{
		ScratchVolume: &v1.ScratchVolumeSpec{
			SizeLimit:        ptrQuantity("200Gi"),
			StorageClassName: new("fast"),
		},
	}

	golden.AssertYAML(
		t, "testdata/golden/scratch-pvc/job.yaml", jobPreview{in},
		golden.WithScheme(goldenScheme(t)), golden.Update(*updateGolden),
	)
}

func TestJobNameDerivesFromTheBackupAlone(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "my-cluster-1748937221000-dump", JobName(backup()))
}

func TestBuildJobRejectsAnEmptyOperatorImage(t *testing.T) {
	t.Parallel()

	in := input()
	in.OperatorImage = ""

	_, err := BuildJob(in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "operator image is empty")
}

func TestBuildJobRunsBothContainersUnderTheServiceAccount(t *testing.T) {
	t.Parallel()

	job, err := BuildJob(input())
	require.NoError(t, err)

	pod := job.Spec.Template.Spec
	assert.Equal(t, "my-cluster-camunda", pod.ServiceAccountName)
	require.Len(t, pod.InitContainers, 1)
	require.Len(t, pod.Containers, 1)
	assert.Equal(t, corev1.RestartPolicyNever, pod.RestartPolicy)
}

func ptrQuantity(s string) *resource.Quantity {
	q := resource.MustParse(s)

	return &q
}
