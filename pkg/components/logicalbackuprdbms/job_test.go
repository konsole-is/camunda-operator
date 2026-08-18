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
	"strings"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/testing/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
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
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster-1748937221000",
			Namespace: "my-cluster-ns",
			UID:       "3f2a9c1e-7b4d-4e0a-9c6f-2d1b8a5e4c73",
		},
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
		Bucket:             s3Bucket(s3Credentials()),
		BucketSecretName:   "my-cluster-camunda-backup-credentials",
		DBSecretName:       "my-cluster-camunda-dump-credentials",
		DBUsernameKey:      "username",
		DBPasswordKey:      "password",
		ServiceAccountName: "my-cluster-camunda",
		ServerVersion:      "17",
		Host:               "postgres.databases.svc",
		Port:               5432,
		Database:           "camunda",
		ObjectKey:          "clusters/my-cluster-ns/my-cluster/1748937221000/camunda.dump",
		CLIImage:           "ghcr.io/konsole-is/camunda-operator-cli:0.1.0",
	}
}

func TestJobGoldenS3Credentials(t *testing.T) {
	t.Parallel()

	in := input()
	in.Dump = &v1.DumpPodSpec{
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("512Mi")},
		},
		ExtraEnv:       []corev1.EnvVar{{Name: "PGSSLMODE", Value: "require"}},
		PodLabels:      map[string]string{"team": "data"},
		PodAnnotations: map[string]string{"sidecar.istio.io/inject": "false"},
		Scheduling: &v1.SchedulingSpec{
			Tolerations: []corev1.Toleration{{Key: "backups", Operator: corev1.TolerationOpExists}},
		},
		ScratchVolume: &v1.ScratchVolumeSpec{SizeLimit: new(resource.MustParse("50Gi"))},
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

// TestJobGoldenAzureWorkloadIdentity pins the pod label the Azure webhook
// needs: without azure.workload.identity/use on the pod, no token is
// injected, whatever the ServiceAccount carries.
func TestJobGoldenAzureWorkloadIdentity(t *testing.T) {
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
					Type: v1.ObjectStorageAuthTypeWorkloadIdentity,
				},
			},
		},
	}
	in.BucketSecretName = ""

	job, err := BuildJob(in)
	require.NoError(t, err)
	assert.Equal(t, "true", job.Spec.Template.Labels["azure.workload.identity/use"])

	golden.AssertYAML(
		t, "testdata/golden/azure-workload-identity/job.yaml", jobPreview{in},
		golden.WithScheme(goldenScheme(t)), golden.Update(*updateGolden),
	)
}

func TestJobGoldenScratchPVC(t *testing.T) {
	t.Parallel()

	in := input()
	in.Dump = &v1.DumpPodSpec{
		ScratchVolume: &v1.ScratchVolumeSpec{
			SizeLimit:        new(resource.MustParse("200Gi")),
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

// TestLongBackupNamesRenderAValidJob pins the bounds: a backup name may be a
// full DNS subdomain (253 characters), while the Job name and every label
// value are DNS labels (63). Both are truncated deterministically and kept
// unique by a hash of the full name.
func TestLongBackupNamesRenderAValidJob(t *testing.T) {
	t.Parallel()

	for _, length := range []int{63, 100, 253} {
		in := input()
		in.Backup.Name = strings.Repeat("b", length)
		job, err := BuildJob(in)
		require.NoError(t, err, length)

		assert.Empty(t, validation.IsDNS1123Label(job.Name), "%d: %s", length, job.Name)
		assert.True(t, strings.HasSuffix(job.Name, "-dump"), job.Name)
		for key, value := range job.Labels {
			assert.Empty(t, validation.IsValidLabelValue(value), "%d: %s=%s", length, key, value)
		}
		for key, value := range job.Spec.Template.Labels {
			assert.Empty(t, validation.IsValidLabelValue(value), "%d: %s=%s", length, key, value)
		}

		// Two long names that agree on the truncated prefix stay apart.
		sibling := input()
		sibling.Backup.Name = strings.Repeat("b", length-1) + "c"
		siblingJob, err := BuildJob(sibling)
		require.NoError(t, err)
		if length > 63-len("-dump") {
			assert.NotEqual(t, job.Name, siblingJob.Name, length)
		}
		assert.Equal(t, JobName(in.Backup), JobName(in.Backup), "the name is deterministic")
	}
}

func TestJobBelongsToChecksTheUIDLabel(t *testing.T) {
	t.Parallel()

	in := input()
	in.Backup.UID = "uid-1"
	job, err := BuildJob(in)
	require.NoError(t, err)
	assert.True(t, JobBelongsTo(job, in.Backup))

	stranger := in.Backup.DeepCopy()
	stranger.UID = "uid-2"
	assert.False(t, JobBelongsTo(job, stranger))
	assert.False(t, JobBelongsTo(&batchv1.Job{}, in.Backup), "no label means not ours")
}

// TestBuildJobEnvOfTheJobWins pins the precedence: an extra that names a
// connection variable does not replace the Job's own, and no name appears
// twice, so a duplicate can neither redirect the dump nor break the apply.
func TestBuildJobEnvOfTheJobWins(t *testing.T) {
	t.Parallel()

	in := input()
	in.Dump = &v1.DumpPodSpec{ExtraEnv: []corev1.EnvVar{
		{Name: "PGHOST", Value: "evil.example"},
		{Name: "PGSSLMODE", Value: "require"},
		{Name: EnvUploadKey, Value: "somewhere/else"},
	}}
	job, err := BuildJob(in)
	require.NoError(t, err)

	dump := job.Spec.Template.Spec.InitContainers[0]
	assert.Equal(t, "postgres.databases.svc", envValue(dump, "PGHOST"))
	assert.Equal(t, "require", envValue(dump, "PGSSLMODE"))
	assert.Equal(t, 1, countEnv(dump, "PGHOST"))

	upload := job.Spec.Template.Spec.Containers[0]
	assert.Equal(t, in.ObjectKey, envValue(upload, EnvUploadKey))
	assert.Equal(t, 1, countEnv(upload, EnvUploadKey))
}

func TestReservedEnvNamesTheOffenders(t *testing.T) {
	t.Parallel()

	assert.Nil(t, ReservedEnv(nil))
	assert.Nil(t, ReservedEnv(&v1.DumpPodSpec{ExtraEnv: []corev1.EnvVar{{Name: "PGSSLMODE"}}}))
	assert.Equal(t, []string{"PGPASSWORD", "UPLOAD_KEY"}, ReservedEnv(&v1.DumpPodSpec{
		ExtraEnv: []corev1.EnvVar{
			{Name: "PGSSLMODE"}, {Name: "PGPASSWORD"}, {Name: "UPLOAD_KEY"}, {Name: "PGPASSWORD"},
		},
	}))
}

func envValue(container corev1.Container, name string) string {
	for _, env := range container.Env {
		if env.Name == name {
			return env.Value
		}
	}

	return ""
}

func countEnv(container corev1.Container, name string) int {
	n := 0
	for _, env := range container.Env {
		if env.Name == name {
			n++
		}
	}

	return n
}

func TestBuildJobRejectsAnEmptyCLIImage(t *testing.T) {
	t.Parallel()

	in := input()
	in.CLIImage = ""

	_, err := BuildJob(in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "camunda-operator-cli image is empty")
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

	// A PVC-backed scratch volume is commonly root-owned; the fsGroup makes
	// the kubelet hand it to the postgres group, so pg_dump can write it.
	require.NotNil(t, pod.SecurityContext.FSGroup)
	assert.Equal(t, int64(999), *pod.SecurityContext.FSGroup)

	// No override means the production default, never "forever".
	require.NotNil(t, job.Spec.ActiveDeadlineSeconds)
	assert.Equal(t, int64(24*60*60), *job.Spec.ActiveDeadlineSeconds)
}

// TestBuildJobTakesTheImageFromTheClusterOnly pins the policy boundary: the
// pod settings may come from the backup, the image never does — a backup CR
// cannot even express one (DumpPodSpec has no image field), and the Job
// renders the cluster's image while the backup overrides every pod knob.
func TestBuildJobTakesTheImageFromTheClusterOnly(t *testing.T) {
	t.Parallel()

	in := input()
	in.PostgresImage = "mirror.example/postgres:17.4"
	in.Dump = &v1.DumpPodSpec{ // the backup's own pod settings
		PodLabels:             map[string]string{"from": "backup"},
		ActiveDeadlineSeconds: new(int64(600)),
	}
	job, err := BuildJob(in)
	require.NoError(t, err)

	assert.Equal(t, "mirror.example/postgres:17.4", job.Spec.Template.Spec.InitContainers[0].Image)
	assert.Equal(t, "backup", job.Spec.Template.Labels["from"])
	assert.Equal(t, int64(600), *job.Spec.ActiveDeadlineSeconds)

	in.PostgresImage = ""
	job, err = BuildJob(in)
	require.NoError(t, err)
	assert.Equal(
		t,
		"postgres:17",
		job.Spec.Template.Spec.InitContainers[0].Image,
		"the default follows the server major",
	)
}

func TestBuildJobHonorsAnExplicitDeadline(t *testing.T) {
	t.Parallel()

	in := input()
	in.Dump = &v1.DumpPodSpec{ActiveDeadlineSeconds: new(int64(7200))}
	job, err := BuildJob(in)
	require.NoError(t, err)
	assert.Equal(t, int64(7200), *job.Spec.ActiveDeadlineSeconds)
}

// TestCleanupJobGolden pins the cleanup Job: the delete subcommand of the CLI
// image, under the cluster ServiceAccount with the bucket's workload-identity
// pod labels — the same identity surface as the dump Job — bounded by the
// dump block's deadline.
func TestCleanupJobGolden(t *testing.T) {
	t.Parallel()

	in := CleanupJobInput{
		Backup:      backup(),
		ClusterName: "my-cluster",
		Dump: &v1.DumpPodSpec{
			PodAnnotations:        map[string]string{"sidecar.istio.io/inject": "false"},
			ActiveDeadlineSeconds: new(int64(3600)),
		},
		Bucket: &v1.ObjectStorageConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "my-backup-config"},
			Spec: v1.ObjectStorageConfigSpec{
				Type: v1.ObjectStorageTypeAzureBlob,
				AzureBlob: &v1.AzureBlobStorage{
					AccountName: "camundabackups",
					Container:   "backups",
					BasePath:    "clusters",
					Auth: v1.AzureBlobStorageAuth{
						Type: v1.ObjectStorageAuthTypeWorkloadIdentity,
					},
				},
			},
		},
		ServiceAccountName: "my-cluster-camunda",
		ObjectKey:          "clusters/my-cluster-ns/my-cluster/1748937221000/camunda.dump",
		CLIImage:           "ghcr.io/konsole-is/camunda-operator-cli:0.1.0",
	}

	golden.AssertYAML(
		t, "testdata/golden/cleanup-job/job.yaml", cleanupPreview{in},
		golden.WithScheme(goldenScheme(t)), golden.Update(*updateGolden),
	)
}

// cleanupPreview adapts the built cleanup Job to the golden previewer.
type cleanupPreview struct{ input CleanupJobInput }

func (p cleanupPreview) Preview() (client.Object, error) { return BuildCleanupJob(p.input) }

func TestBuildCleanupJobRejectsAnEmptyCLIImage(t *testing.T) {
	t.Parallel()

	_, err := BuildCleanupJob(CleanupJobInput{Backup: backup(), Bucket: s3Bucket(s3Credentials())})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "camunda-operator-cli image is empty")
}
