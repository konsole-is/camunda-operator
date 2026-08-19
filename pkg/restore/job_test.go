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
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// updateGolden refreshes the golden manifests with the rendered output:
// go test ./pkg/restore/ -run Golden -update-golden
var updateGolden = flag.Bool("update-golden", false, "update golden files")

// jobPreview adapts the built Job to the golden previewer.
type jobPreview struct{ input JobInput }

func (p jobPreview) Preview() (client.Object, error) { return BuildJob(p.input) }

func logicalRestore() *v1.LogicalRestore {
	return &v1.LogicalRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster-restore",
			Namespace: "ns",
			UID:       "8f2a9c1e-7b4d-4e0a-9c6f-2d1b8a5e4c73",
		},
		Spec: v1.LogicalRestoreSpec{
			BackupRef: v1.LogicalBackupRef{
				Kind: v1.LogicalBackupKindElasticsearch,
				Name: "my-cluster-backup",
			},
			TargetClusterRef: v1.ClusterRef{Name: "my-cluster"},
		},
	}
}

func pointInTimeRestore() *v1.PointInTimeRestore {
	return &v1.PointInTimeRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster-pitr",
			Namespace: "ns",
			UID:       "1c7d4e6f-2a3b-4c5d-8e9f-0a1b2c3d4e5f",
		},
		Spec: v1.PointInTimeRestoreSpec{ClusterRef: v1.ClusterRef{Name: "my-cluster"}},
	}
}

// richTarget is the fixture target with everything a real broker pod carries:
// extra volumes and mounts, resources, a security context, scheduling, and an
// image pull secret. The Job must mirror all of it.
func richTarget() *Target {
	target := targetFixture()
	pod := &target.StatefulSet.Spec.Template.Spec

	pod.ImagePullSecrets = []corev1.LocalObjectReference{{Name: "registry"}}
	pod.SecurityContext = &corev1.PodSecurityContext{
		RunAsUser:    new(int64(1001)),
		RunAsGroup:   new(int64(1001)),
		FSGroup:      new(int64(1001)),
		RunAsNonRoot: new(true),
	}
	pod.NodeSelector = map[string]string{"workload": "camunda"}
	pod.Tolerations = []corev1.Toleration{{Key: "camunda", Operator: corev1.TolerationOpExists}}
	pod.TopologySpreadConstraints = []corev1.TopologySpreadConstraint{{
		MaxSkew:           1,
		TopologyKey:       "topology.kubernetes.io/zone",
		WhenUnsatisfiable: corev1.DoNotSchedule,
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"camunda.io/component": "zeebe"},
		},
	}}
	pod.Volumes = []corev1.Volume{{
		Name: "es-ca",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{SecretName: "es-ca"},
		},
	}}

	broker := &pod.Containers[0]
	broker.Env = append(
		broker.Env, corev1.EnvVar{
			Name: "CAMUNDA_DATA_SECONDARYSTORAGE_ELASTICSEARCH_URL", Value: "https://es:9200",
		}, corev1.EnvVar{
			Name: "SPRING_PROFILES_ACTIVE", Value: "broker,consolidated-auth",
		},
	)
	broker.VolumeMounts = append(broker.VolumeMounts, corev1.VolumeMount{
		Name: "es-ca", MountPath: "/etc/camunda/es-ca",
	})
	broker.Resources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
		Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("4Gi")},
	}
	broker.SecurityContext = &corev1.SecurityContext{RunAsNonRoot: new(true)}
	target.Broker = broker

	return target
}

func logicalInput() JobInput {
	return JobInput{
		Target:     richTarget(),
		Owner:      logicalRestore(),
		OwnerLabel: labels.LogicalRestore("my-cluster-restore"),
		Ordinal:    0,
		Args:       []string{"--backupId=1748937221"},
	}
}

func envValue(env []corev1.EnvVar, name string) string {
	for _, e := range env {
		if e.Name == name {
			return e.Value
		}
	}

	return ""
}

func dataClaimName(job *batchv1.Job) string {
	for _, volume := range job.Spec.Template.Spec.Volumes {
		if volume.Name == "data" && volume.PersistentVolumeClaim != nil {
			return volume.PersistentVolumeClaim.ClaimName
		}
	}

	return ""
}

// The restore application must run with the configuration the brokers run
// with, on the volume of its own broker.
func TestBuildJobMirrorsTheBrokerAndPinsTheNodeID(t *testing.T) {
	t.Parallel()

	in := logicalInput()
	in.Ordinal = 2
	target := in.Target

	job, err := BuildJob(in)
	require.NoError(t, err)

	container := job.Spec.Template.Spec.Containers[0]
	assert.Equal(t, target.Broker.Image, container.Image)
	assert.Equal(t, []string{RestoreEntrypoint}, container.Command)
	assert.Equal(t, []string{"--backupId=1748937221"}, container.Args)
	assert.Equal(t, target.Broker.Resources, container.Resources)
	assert.Equal(t, target.Broker.SecurityContext, container.SecurityContext)
	assert.Equal(t, "2", envValue(container.Env, "CAMUNDA_CLUSTER_NODEID"))
	assert.Equal(t, "3", envValue(container.Env, "CAMUNDA_CLUSTER_SIZE"))
	assert.Equal(t, "6", envValue(container.Env, "CAMUNDA_CLUSTER_PARTITIONCOUNT"))
	assert.Equal(
		t,
		"https://es:9200",
		envValue(container.Env, "CAMUNDA_DATA_SECONDARYSTORAGE_ELASTICSEARCH_URL"),
	)
	assert.Equal(t, target.Broker.VolumeMounts, container.VolumeMounts)

	assert.Equal(t, "data-my-cluster-zeebe-2", dataClaimName(job))
	assert.Equal(
		t,
		target.StatefulSet.Spec.Template.Spec.ServiceAccountName,
		job.Spec.Template.Spec.ServiceAccountName,
	)
	assert.Equal(t, "restore", job.Labels["camunda.io/component"])
	assert.Equal(t, "my-cluster-restore", job.Labels["camunda.io/logical-restore"])
	assert.Equal(t, "my-cluster", job.Labels["camunda.io/cluster"])
	assert.Equal(t, corev1.RestartPolicyNever, job.Spec.Template.Spec.RestartPolicy)
}

// The restore pods spread exactly as the brokers do, so the recreated volumes
// land in zones the brokers can schedule into afterwards. The selector has to
// point at the restore pods: they carry camunda.io/component: restore, so the
// broker's own selector would count nothing.
func TestBuildJobSpreadsTheRestorePodsLikeTheBrokers(t *testing.T) {
	t.Parallel()

	in := logicalInput()
	broker := in.Target.StatefulSet.Spec.Template.Spec.TopologySpreadConstraints[0]
	in.Target.StatefulSet.Spec.Template.Spec.TopologySpreadConstraints[0].MatchLabelKeys =
		[]string{"pod-template-hash"}

	job, err := BuildJob(in)
	require.NoError(t, err)

	require.Len(t, job.Spec.Template.Spec.TopologySpreadConstraints, 1)
	spread := job.Spec.Template.Spec.TopologySpreadConstraints[0]
	assert.Equal(t, broker.TopologyKey, spread.TopologyKey)
	assert.Equal(t, broker.MaxSkew, spread.MaxSkew)
	assert.Equal(t, broker.WhenUnsatisfiable, spread.WhenUnsatisfiable)
	assert.Nil(t, spread.MatchLabelKeys)
	assert.Equal(
		t,
		map[string]string{
			"camunda.io/logical-restore": "my-cluster-restore",
			"camunda.io/component":       "restore",
		},
		spread.LabelSelector.MatchLabels,
	)
	assert.Subset(t, job.Spec.Template.Labels, spread.LabelSelector.MatchLabels)
}

// A Job pod has a random host name, so the broker's node-id shell wrapper
// cannot run here. The ordinal arrives as a plain value instead.
func TestBuildJobReplacesTheBrokerCommand(t *testing.T) {
	t.Parallel()

	in := logicalInput()
	in.Target.Broker.Command = []string{
		"bash", "-c", "export CAMUNDA_CLUSTER_NODEID=${HOSTNAME##*-}; exec /usr/local/camunda/bin/camunda",
	}

	job, err := BuildJob(in)
	require.NoError(t, err)

	container := job.Spec.Template.Spec.Containers[0]
	assert.Equal(t, []string{"/usr/local/camunda/bin/restore"}, container.Command)
	assert.Equal(t, "0", envValue(container.Env, "CAMUNDA_CLUSTER_NODEID"))
}

// The restore application runs under its own Spring profile. The broker
// profile of the copied environment would start a broker instead.
func TestBuildJobRunsUnderTheRestoreProfile(t *testing.T) {
	t.Parallel()

	job, err := BuildJob(logicalInput())
	require.NoError(t, err)

	env := job.Spec.Template.Spec.Containers[0].Env
	assert.Equal(t, "restore", envValue(env, "SPRING_PROFILES_ACTIVE"))

	count := 0
	for _, e := range env {
		if e.Name == "SPRING_PROFILES_ACTIVE" {
			count++
		}
	}
	assert.Equal(t, 1, count, "the copied broker value must be replaced, not repeated")
}

// The restore application refuses a non-empty data directory. A second pod
// would find the directory the first one wrote and fail for the wrong reason.
// A failed restore is retried with a new restore resource, which recreates
// the volume first.
func TestBuildJobNeverRetriesAPod(t *testing.T) {
	t.Parallel()

	job, err := BuildJob(logicalInput())
	require.NoError(t, err)

	require.NotNil(t, job.Spec.BackoffLimit)
	assert.Equal(t, int32(0), *job.Spec.BackoffLimit)
	assert.Equal(t, corev1.RestartPolicyNever, job.Spec.Template.Spec.RestartPolicy)

	container := job.Spec.Template.Spec.Containers[0]
	assert.Nil(t, container.ReadinessProbe)
	assert.Nil(t, container.LivenessProbe)
	assert.Nil(t, container.StartupProbe)
}

// The Job pods look like broker pods to a topology spread constraint, but the
// operator labels always win over the copied ones.
func TestBuildJobLabelsThePodsLikeBrokersUnderTheRestoreOwner(t *testing.T) {
	t.Parallel()

	in := logicalInput()
	in.Target.StatefulSet.Spec.Template.Labels["camunda.io/component"] = "zeebe"

	job, err := BuildJob(in)
	require.NoError(t, err)

	pod := job.Spec.Template.Labels
	assert.Equal(t, "restore", pod["camunda.io/component"])
	assert.Equal(t, "my-cluster", pod["camunda.io/cluster"])
	assert.Equal(t, "my-cluster-restore", pod["camunda.io/logical-restore"])
	assert.Equal(t, "camunda-operator", pod["app.kubernetes.io/managed-by"])
}

// The Job belongs to its restore resource, so deleting the resource removes
// the Jobs. The restore needs no finalizer: it writes no artifact to an
// external store.
func TestBuildJobIsOwnedByTheRestore(t *testing.T) {
	t.Parallel()

	job, err := BuildJob(logicalInput())
	require.NoError(t, err)

	assert.Equal(t, "ns", job.Namespace)
	assert.Equal(t, "my-cluster-restore-restore-0", job.Name)
}

// BuildJob runs inside a reconcile. A panic there takes the whole manager
// down with every other controller, so an input that names no owner is an
// error like an input that names no target.
func TestBuildJobRejectsAnInputWithoutAnOwner(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		mutate func(*JobInput)
		text   string
	}{
		"no target": {
			mutate: func(in *JobInput) { in.Target = nil },
			text:   "target",
		},
		"no broker container": {
			mutate: func(in *JobInput) { in.Target.Broker = nil },
			text:   "target",
		},
		"no owner": {
			mutate: func(in *JobInput) { in.Owner = nil },
			text:   "owner",
		},
		"a typed nil owner": {
			mutate: func(in *JobInput) { in.Owner = (*v1.LogicalRestore)(nil) },
			text:   "owner",
		},
		"no owner label": {
			mutate: func(in *JobInput) { in.OwnerLabel = labels.Owner{} },
			text:   "owner",
		},
		"an owner label without a name": {
			mutate: func(in *JobInput) { in.OwnerLabel = labels.Owner{Key: labels.LogicalRestoreKey} },
			text:   "owner",
		},
		"an ordinal beyond the broker count": {
			mutate: func(in *JobInput) { in.Ordinal = 3 },
			text:   "3 brokers",
		},
		"a negative ordinal": {
			mutate: func(in *JobInput) { in.Ordinal = -1 },
			text:   "3 brokers",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			in := logicalInput()
			tc.mutate(&in)

			job, err := BuildJob(in)
			require.Error(t, err)
			assert.Nil(t, job)
			assert.Contains(t, err.Error(), tc.text)
		})
	}
}

// A restore name can be a full DNS subdomain, but a Job name is a DNS label.
// A long name truncates deterministically and stays unique through a hash of
// the whole name.
func TestJobNameStaysADNSLabel(t *testing.T) {
	t.Parallel()

	restore := logicalRestore()
	assert.Equal(t, "my-cluster-restore-restore-0", JobName(restore, 0))
	assert.Equal(t, "my-cluster-restore-restore-12", JobName(restore, 12))

	long := logicalRestore()
	long.Name = strings.Repeat("a", 200)
	name := JobName(long, 0)
	assert.LessOrEqual(t, len(name), validation.DNS1123LabelMaxLength)
	assert.True(t, strings.HasSuffix(name, "-restore-0"))
	assert.Empty(t, validation.IsDNS1123Label(name))

	other := logicalRestore()
	other.Name = strings.Repeat("a", 199) + "b"
	assert.NotEqual(t, name, JobName(other, 0))
}

func TestJobGoldenElasticsearchBroker0(t *testing.T) {
	t.Parallel()

	golden.AssertYAML(
		t, "testdata/golden/elasticsearch-broker-0.yaml", jobPreview{logicalInput()},
		golden.WithScheme(testScheme(t)), golden.Update(*updateGolden),
	)
}

// The node id, the Job name, and the claim all follow the ordinal.
func TestJobGoldenElasticsearchBroker2(t *testing.T) {
	t.Parallel()

	in := logicalInput()
	in.Ordinal = 2

	golden.AssertYAML(
		t, "testdata/golden/elasticsearch-broker-2.yaml", jobPreview{in},
		golden.WithScheme(testScheme(t)), golden.Update(*updateGolden),
	)
}

// The relational path takes no argument. The restore application reads the
// exporter position from the restored database itself.
func TestJobGoldenRDBMSNoArgs(t *testing.T) {
	t.Parallel()

	in := logicalInput()
	in.Args = nil

	golden.AssertYAML(
		t, "testdata/golden/rdbms-no-args.yaml", jobPreview{in},
		golden.WithScheme(testScheme(t)), golden.Update(*updateGolden),
	)
}

func TestJobGoldenPointInTimeRestore(t *testing.T) {
	t.Parallel()

	in := JobInput{
		Target:     richTarget(),
		Owner:      pointInTimeRestore(),
		OwnerLabel: labels.PointInTimeRestore("my-cluster-pitr"),
		Ordinal:    1,
		Args:       []string{"--to=2026-07-30T14:30:00Z"},
	}

	golden.AssertYAML(
		t, "testdata/golden/pitr-to-timestamp.yaml", jobPreview{in},
		golden.WithScheme(testScheme(t)), golden.Update(*updateGolden),
	)
}
