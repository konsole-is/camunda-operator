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

package logicalrestore

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
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// updateGolden refreshes the golden manifests with the rendered output:
// go test ./pkg/components/logicalrestore/ -run Golden -update-golden
var updateGolden = flag.Bool("update-golden", false, "update golden files")

// testObjectKey is the dump key that the fixture backup recorded.
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

func restore() *v1.LogicalRestore {
	return &v1.LogicalRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster-restore",
			Namespace: "my-cluster-ns",
			UID:       "9d4c1f77-2b3e-4a58-8c0d-7e5f6a1b2c34",
		},
		Spec: v1.LogicalRestoreSpec{
			BackupRef: v1.LogicalBackupRef{
				Kind: v1.LogicalBackupKindRDBMS,
				Name: "my-cluster-1748937221000",
			},
			TargetClusterRef: v1.ClusterRef{Name: "my-cluster"},
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
		Restore:            restore(),
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
	in.Pod = &v1.DumpPodSpec{
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

func TestJobGoldenScratchPVC(t *testing.T) {
	t.Parallel()

	in := input()
	in.Pod = &v1.DumpPodSpec{
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

// TestBuildJobConnectsWithTheTargetCoordinatesAndBackupCredentials pins the
// connection half of the restore container. The Job restores into the target
// database over the backup user of the target, never over a credential of the
// cluster the backup came from.
func TestBuildJobConnectsWithTheTargetCoordinatesAndBackupCredentials(t *testing.T) {
	t.Parallel()

	job, err := BuildJob(input())
	require.NoError(t, err)

	pod := job.Spec.Template.Spec
	require.Len(t, pod.InitContainers, 1)
	require.Len(t, pod.Containers, 1)

	restoreContainer := pod.Containers[0]
	assert.Equal(t, "postgres.databases.svc", envValue(restoreContainer, "PGHOST"))
	assert.Equal(t, "5432", envValue(restoreContainer, "PGPORT"))
	assert.Equal(t, "camunda", envValue(restoreContainer, "PGDATABASE"))
	assert.Equal(t, "postgres:17", restoreContainer.Image)
	assert.Equal(
		t,
		[]string{
			"pg_restore", "--clean", "--if-exists", "--no-owner",
			"--dbname=camunda", "/scratch/camunda.dump",
		},
		restoreContainer.Command,
	)

	user := secretRef(t, restoreContainer, "PGUSER")
	assert.Equal(t, "my-cluster-camunda-dump-credentials", user.Name)
	assert.Equal(t, "username", user.Key)

	password := secretRef(t, restoreContainer, "PGPASSWORD")
	assert.Equal(t, "my-cluster-camunda-dump-credentials", password.Name)
	assert.Equal(t, "password", password.Key)

	// The init container reads the object that the backup recorded.
	download := pod.InitContainers[0]
	assert.Equal(t, []string{"download"}, download.Args)
	assert.Equal(t, "/scratch/camunda.dump", envValue(download, EnvDownloadFile))
	assert.Equal(t, testObjectKey, envValue(download, EnvDownloadKey))
}

func TestBuildJobCarriesTheOwnerLabelsAndTheUID(t *testing.T) {
	t.Parallel()

	job, err := BuildJob(input())
	require.NoError(t, err)

	want := map[string]string{
		labels.LogicalRestoreKey: "my-cluster-restore",
		labels.ComponentKey:      ComponentName,
		labels.ManagedByKey:      labels.ManagedBy,
		labels.ClusterKey:        "my-cluster",
		RestoreUIDLabel:          "9d4c1f77-2b3e-4a58-8c0d-7e5f6a1b2c34",
	}
	assert.Equal(t, want, job.Labels)
	for key, value := range want {
		assert.Equal(t, value, job.Spec.Template.Labels[key], key)
	}

	assert.Equal(t, "my-cluster-ns", job.Namespace)
	assert.Equal(t, "my-cluster-restore-pg-restore", job.Name)
	assert.Empty(t, job.OwnerReferences, "the caller sets the controller reference")
}

func TestBuildJobRunsUnderTheClusterServiceAccount(t *testing.T) {
	t.Parallel()

	job, err := BuildJob(input())
	require.NoError(t, err)

	pod := job.Spec.Template.Spec
	assert.Equal(t, "my-cluster-camunda", pod.ServiceAccountName)
	assert.Equal(t, corev1.RestartPolicyNever, pod.RestartPolicy)

	// A PVC-backed scratch volume is commonly root-owned. The fsGroup makes
	// the kubelet hand it to the postgres group, so both containers can write
	// it.
	require.NotNil(t, pod.SecurityContext.FSGroup)
	assert.Equal(t, int64(999), *pod.SecurityContext.FSGroup)

	require.NotNil(t, job.Spec.ActiveDeadlineSeconds)
	assert.Equal(t, DefaultActiveDeadlineSeconds, *job.Spec.ActiveDeadlineSeconds)
	require.NotNil(t, job.Spec.BackoffLimit)
	assert.Equal(t, int32(3), *job.Spec.BackoffLimit)
}

func TestBuildJobRejectsAnEmptyCLIImage(t *testing.T) {
	t.Parallel()

	in := input()
	in.CLIImage = ""

	_, err := BuildJob(in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "camunda-operator-cli image is empty")
}

func TestBuildJobRejectsAStorageClassWithoutASizeLimit(t *testing.T) {
	t.Parallel()

	in := input()
	in.Pod = &v1.DumpPodSpec{
		ScratchVolume: &v1.ScratchVolumeSpec{StorageClassName: new("fast")},
	}

	_, err := BuildJob(in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "storage class needs a sizeLimit")
}

func TestJobNameDerivesFromTheRestoreAlone(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "my-cluster-restore-pg-restore", JobName(restore()))
}

// TestLongRestoreNamesRenderAValidJob pins the bounds. A restore name can be a
// full DNS subdomain (253 characters), but the Job name and every label value
// are DNS labels (63). The builder truncates both deterministically and keeps
// them unique with a hash of the full name.
func TestLongRestoreNamesRenderAValidJob(t *testing.T) {
	t.Parallel()

	for _, length := range []int{63, 100, 253} {
		in := input()
		in.Restore.Name = strings.Repeat("r", length)
		job, err := BuildJob(in)
		require.NoError(t, err, length)

		assert.Empty(t, validation.IsDNS1123Label(job.Name), "%d: %s", length, job.Name)
		assert.True(t, strings.HasSuffix(job.Name, jobNameSuffix), job.Name)
		for key, value := range job.Labels {
			assert.Empty(t, validation.IsValidLabelValue(value), "%d: %s=%s", length, key, value)
		}
		for key, value := range job.Spec.Template.Labels {
			assert.Empty(t, validation.IsValidLabelValue(value), "%d: %s=%s", length, key, value)
		}

		// Two long names that agree on the truncated prefix stay apart.
		sibling := input()
		sibling.Restore.Name = strings.Repeat("r", length-1) + "s"
		siblingJob, err := BuildJob(sibling)
		require.NoError(t, err)
		if length > validation.DNS1123LabelMaxLength-len(jobNameSuffix) {
			assert.NotEqual(t, job.Name, siblingJob.Name, length)
		}
		// The builder and the name function answer the same name. A controller
		// finds the Job it created by asking JobName, and a builder that bounded
		// the name its own way would hide the Job from it.
		assert.Equal(t, JobName(in.Restore), job.Name, length)
	}
}

func TestJobBelongsToChecksTheUIDLabel(t *testing.T) {
	t.Parallel()

	in := input()
	in.Restore.UID = "uid-1"
	job, err := BuildJob(in)
	require.NoError(t, err)
	assert.True(t, JobBelongsTo(job, in.Restore))

	stranger := in.Restore.DeepCopy()
	stranger.UID = "uid-2"
	assert.False(t, JobBelongsTo(job, stranger))
	assert.False(t, JobBelongsTo(&batchv1.Job{}, in.Restore), "no label means not ours")
}

// TestBuildJobEnvOfTheJobWins pins the precedence. The pod settings come from
// the target cluster, so their extras reach both containers. An extra that
// names a connection variable or a contract variable does not replace the
// Job's own, and no name appears twice.
func TestBuildJobEnvOfTheJobWins(t *testing.T) {
	t.Parallel()

	in := input()
	in.Pod = &v1.DumpPodSpec{
		ExtraEnv: []corev1.EnvVar{
			{Name: "PGHOST", Value: "evil.example"},
			{Name: "PGSSLMODE", Value: "require"},
			{Name: EnvDownloadKey, Value: "somewhere/else"},
		},
		ExtraEnvFrom: []corev1.EnvFromSource{{
			Prefix: "X_",
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: "extras"},
			},
		}},
	}
	job, err := BuildJob(in)
	require.NoError(t, err)

	download := job.Spec.Template.Spec.InitContainers[0]
	assert.Equal(t, in.ObjectKey, envValue(download, EnvDownloadKey))
	assert.Equal(t, 1, countEnv(download, EnvDownloadKey))
	assert.Equal(t, "require", envValue(download, "PGSSLMODE"))

	restoreContainer := job.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "postgres.databases.svc", envValue(restoreContainer, "PGHOST"))
	assert.Equal(t, 1, countEnv(restoreContainer, "PGHOST"))
	assert.Equal(t, "require", envValue(restoreContainer, "PGSSLMODE"))

	for _, container := range []corev1.Container{download, restoreContainer} {
		require.Len(t, container.EnvFrom, 1, container.Name)
		assert.Equal(t, "X_", container.EnvFrom[0].Prefix, container.Name)
	}
}

func TestBuildJobHonorsAnExplicitDeadlineAndImage(t *testing.T) {
	t.Parallel()

	in := input()
	in.PostgresImage = "mirror.example/postgres:17.4"
	in.Pod = &v1.DumpPodSpec{ActiveDeadlineSeconds: new(int64(7200))}

	job, err := BuildJob(in)
	require.NoError(t, err)
	assert.Equal(t, int64(7200), *job.Spec.ActiveDeadlineSeconds)
	assert.Equal(t, "mirror.example/postgres:17.4", job.Spec.Template.Spec.Containers[0].Image)
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

func secretRef(t *testing.T, container corev1.Container, name string) *corev1.SecretKeySelector {
	t.Helper()

	for _, env := range container.Env {
		if env.Name == name {
			require.NotNil(t, env.ValueFrom)
			require.NotNil(t, env.ValueFrom.SecretKeyRef)

			return env.ValueFrom.SecretKeyRef
		}
	}
	require.Fail(t, "the container sets no "+name)

	return nil
}
