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
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// updateGolden refreshes the golden manifests with the rendered output:
// go test ./pkg/components/logicalbackuprdbms/ -run Golden -update-golden
var updateGolden = flag.Bool("update-golden", false, "update golden files")

// testObjectKey is the dump key of the fixture backup: id 1748937221000
// under the fixture cluster, then the backup UID, then the file.
const testObjectKey = "clusters/my-cluster-ns/my-cluster/1748937221000/" +
	"2c8b0e6e-5f0a-4c25-9d5b-4d1d0f4b1a10/camunda.dump"

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
		ObjectKey:          testObjectKey,
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

// TestJobGoldenAzureWorkloadIdentity pins the pod label that the Azure
// webhook needs. Without azure.workload.identity/use on the pod, the webhook
// injects no token, whatever the ServiceAccount carries.
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

// TestLongBackupNamesRenderAValidJob pins the bounds. A backup name can be a
// full DNS subdomain (253 characters), but the Job name and every label
// value are DNS labels (63). The builder truncates both deterministically
// and keeps them unique with a hash of the full name.
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

// TestBuildJobEnvOfTheJobWins pins the precedence. An extra that names a
// connection variable does not replace the Job's own, and no name appears
// twice. A duplicate can therefore neither redirect the dump nor break the
// apply.
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

// TestBuildJobKeepsBackupEnvOutOfTheUploadContainer pins the boundary of a
// per-backup environment. Cloud SDKs read endpoint, proxy, and configuration
// variables from the environment. A backup author must not steer where the
// dump goes, so the backup's own block reaches the dump container only. The
// cluster's block reaches every container.
func TestBuildJobKeepsBackupEnvOutOfTheUploadContainer(t *testing.T) {
	t.Parallel()

	block := &v1.DumpPodSpec{
		ExtraEnv: []corev1.EnvVar{{Name: "AWS_ENDPOINT_URL", Value: "http://evil.example"}},
		ExtraEnvFrom: []corev1.EnvFromSource{{
			Prefix:       "X_",
			ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "extra"}},
		}},
	}

	in := input()
	in.Dump = block
	in.BackupOwnsDump = true
	job, err := BuildJob(in)
	require.NoError(t, err)
	dump := job.Spec.Template.Spec.InitContainers[0]
	upload := job.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "http://evil.example", envValue(dump, "AWS_ENDPOINT_URL"), "the dump container takes it")
	assert.Len(t, dump.EnvFrom, 1)
	assert.Equal(t, 0, countEnv(upload, "AWS_ENDPOINT_URL"), "the upload container never takes backup env")
	assert.Empty(t, upload.EnvFrom, "nor its sources")

	in.BackupOwnsDump = false
	job, err = BuildJob(in)
	require.NoError(t, err)
	upload = job.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "http://evil.example", envValue(upload, "AWS_ENDPOINT_URL"), "the cluster block reaches the upload")
	assert.Len(t, upload.EnvFrom, 1)
}

func TestBuildCleanupJobKeepsBackupEnvOutOfTheDeleteContainer(t *testing.T) {
	t.Parallel()

	block := &v1.DumpPodSpec{
		ExtraEnv: []corev1.EnvVar{{Name: "AWS_ENDPOINT_URL", Value: "http://evil.example"}},
	}
	in := CleanupJobInput{
		Backup: backup(), ClusterName: "my-cluster", Bucket: s3Bucket(s3Credentials()),
		Dump: block, BackupOwnsDump: true, ObjectKey: testObjectKey,
		CLIImage: "ghcr.io/konsole-is/camunda-operator-cli:0.1.0",
	}
	job, err := BuildCleanupJob(in)
	require.NoError(t, err)
	assert.Equal(t, 0, countEnv(job.Spec.Template.Spec.Containers[0], "AWS_ENDPOINT_URL"))

	in.BackupOwnsDump = false
	job, err = BuildCleanupJob(in)
	require.NoError(t, err)
	assert.Equal(t, "http://evil.example", envValue(job.Spec.Template.Spec.Containers[0], "AWS_ENDPOINT_URL"))
}

func TestReservedEnvNamesTheOffenders(t *testing.T) {
	t.Parallel()

	assert.Nil(t, ReservedEnv(nil))
	assert.Nil(t, ReservedEnv(&v1.DumpPodSpec{ExtraEnv: []corev1.EnvVar{{Name: "TZ"}, {Name: "XPG"}}}))
	// The rule is prefix-based. Names that the Job never sets are connection
	// policy too. PGHOSTADDR beats PGHOST inside libpq.
	assert.Equal(
		t,
		[]string{"PGSSLMODE", "PGHOSTADDR", "UPLOAD_KEY", "PGOPTIONS"},
		ReservedEnv(&v1.DumpPodSpec{ExtraEnv: []corev1.EnvVar{
			{Name: "PGSSLMODE"}, {Name: "PGHOSTADDR"}, {Name: "UPLOAD_KEY"},
			{Name: "PGOPTIONS"}, {Name: "PGSSLMODE"},
		}}),
	)
}

func TestSafeEnvFromPrefix(t *testing.T) {
	t.Parallel()

	for _, safe := range []string{"X_", "MY_", "EXTRA_", "GP", "PX"} {
		assert.True(t, SafeEnvFromPrefix(safe), safe)
	}
	// An empty prefix, a reserved head, or a head OF a reserved prefix is
	// unsafe. "P" plus a key "GHOST" spells PGHOST, and "UPLOAD" plus "_KEY"
	// spells the upload contract.
	for _, unsafe := range []string{"", "P", "PG", "PGX", "U", "UPLOAD", "UPLOAD_", "UPLOAD_X"} {
		assert.False(t, SafeEnvFromPrefix(unsafe), unsafe)
	}
}

func TestUnsafeEnvFromNamesTheSources(t *testing.T) {
	t.Parallel()

	assert.Nil(t, UnsafeEnvFrom(nil))
	assert.Nil(t, UnsafeEnvFrom(&v1.DumpPodSpec{ExtraEnvFrom: []corev1.EnvFromSource{{Prefix: "X_"}}}))
	unsafe := UnsafeEnvFrom(&v1.DumpPodSpec{ExtraEnvFrom: []corev1.EnvFromSource{
		{Prefix: "X_"}, {}, {Prefix: "P"},
	}})
	require.Len(t, unsafe, 2)
	assert.Contains(t, unsafe[0], "source 1")
	assert.Contains(t, unsafe[1], "source 2")
}

// TestBuildJobKeepsTheEnvFromPrefix proves that the safe prefix survives to
// both containers. The prefix is what neutralizes the keys of the source at
// runtime.
func TestBuildJobKeepsTheEnvFromPrefix(t *testing.T) {
	t.Parallel()

	in := input()
	in.Dump = &v1.DumpPodSpec{ExtraEnvFrom: []corev1.EnvFromSource{{
		Prefix: "X_",
		ConfigMapRef: &corev1.ConfigMapEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{Name: "extras"},
		},
	}}}
	job, err := BuildJob(in)
	require.NoError(t, err)

	for _, container := range []corev1.Container{
		job.Spec.Template.Spec.InitContainers[0], job.Spec.Template.Spec.Containers[0],
	} {
		require.Len(t, container.EnvFrom, 1, container.Name)
		assert.Equal(t, "X_", container.EnvFrom[0].Prefix, container.Name)
	}
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

// A cluster that renders no ServiceAccount gives the Job none. The pod then
// carries no serviceAccountName and runs under the default account of its
// namespace. A name that no account matches would make the API server refuse
// the pod, which is how a backup of a cluster with static credentials came to
// hang with no pod at all.
func TestBuildJobRunsUnderTheDefaultAccountWithoutAName(t *testing.T) {
	t.Parallel()

	in := input()
	in.ServiceAccountName = ""

	job, err := BuildJob(in)
	require.NoError(t, err)

	assert.Empty(t, job.Spec.Template.Spec.ServiceAccountName)
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

	// A PVC-backed scratch volume is commonly root-owned. The fsGroup makes
	// the kubelet hand it to the postgres group, so pg_dump can write it.
	require.NotNil(t, pod.SecurityContext.FSGroup)
	assert.Equal(t, int64(999), *pod.SecurityContext.FSGroup)

	// No override means the production default, which is finite.
	require.NotNil(t, job.Spec.ActiveDeadlineSeconds)
	assert.Equal(t, int64(24*60*60), *job.Spec.ActiveDeadlineSeconds)
}

// TestBuildJobTakesTheImageFromTheClusterOnly pins the policy boundary. The
// pod settings can come from the backup, but the image never does. A backup
// CR cannot express one, because DumpPodSpec has no image field. The Job
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

// TestCleanupJobGolden pins the cleanup Job. It runs the delete subcommand
// of the CLI image under the cluster ServiceAccount, with the bucket's
// workload-identity pod labels. That is the same identity surface as the
// dump Job. The deadline of the dump block bounds it.
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
		ObjectKey:          testObjectKey,
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

func TestDumpObjectKeyCarriesTheIDAndTheUID(t *testing.T) {
	t.Parallel()

	key := DumpObjectKey("/clusters/", "ns", "my-cluster", 1748937221000, "2c8b0e6e-5f0a-4c25-9d5b-4d1d0f4b1a10")

	assert.Equal(t, "clusters/ns/my-cluster/1748937221000/2c8b0e6e-5f0a-4c25-9d5b-4d1d0f4b1a10/camunda.dump", key)
	assert.NotEqual(
		t, key, DumpObjectKey("/clusters/", "ns", "my-cluster", 1748937221000, "another-uid"),
		"the same id from another backup names another object",
	)
}

// The manager caches pods through an informer that labels.ManagedSelector
// scopes. A pod that the selector does not match is a pod that podstate.Stuck
// cannot see, and a container that cannot start would then read as progress.
// The operator labels go over the labels of a user, so a user cannot take the
// key away.
func TestBuildJobPodsMatchTheManagedSelector(t *testing.T) {
	t.Parallel()

	in := input()
	in.Dump = &v1.DumpPodSpec{PodLabels: map[string]string{labels.ManagedByKey: "helm"}}

	job, err := BuildJob(in)
	require.NoError(t, err)

	assert.True(t, labels.ManagedSelector().Matches(k8slabels.Set(job.Spec.Template.Labels)))
}
