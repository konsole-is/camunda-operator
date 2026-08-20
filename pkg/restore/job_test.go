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

// The fixture restore that every case renders from: its name, the name its
// Jobs derive, and the argument that the restore application takes.
const (
	restoreName = "my-cluster-pitr"
	restoreJob0 = "my-cluster-pitr-pitr-0"
	restoreArg  = "--to=2026-07-30T14:30:00Z"
)

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

	target.StatefulSet.Spec.Template.Annotations = map[string]string{
		"camunda.io/config-hash": "abc123",
	}
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
	broker.Env = append(broker.Env, corev1.EnvVar{
		Name: "CAMUNDA_DATA_SECONDARYSTORAGE_ELASTICSEARCH_PASSWORD",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "es-credentials"},
				Key:                  "password",
			},
		},
	})
	broker.EnvFrom = []corev1.EnvFromSource{{
		ConfigMapRef: &corev1.ConfigMapEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{Name: "broker-extra"},
		},
	}}
	broker.VolumeMounts = append(broker.VolumeMounts, corev1.VolumeMount{
		Name:             "es-ca",
		MountPath:        "/etc/camunda/es-ca",
		MountPropagation: new(corev1.MountPropagationNone),
	})
	broker.Resources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
		Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("4Gi")},
	}
	broker.SecurityContext = &corev1.SecurityContext{RunAsNonRoot: new(true)}
	target.Broker = broker

	return target
}

// restoreInput is the input that every case starts from: a rich target, the
// fixture restore as the owner, and the arguments of the restore application.
func restoreInput() JobInput {
	return JobInput{
		Target:     richTarget(),
		Owner:      pointInTimeRestore(),
		OwnerLabel: labels.PointInTimeRestore(restoreName),
		Ordinal:    0,
		Args:       []string{restoreArg},
	}
}

// esPasswordEnv is the fixture variable that reads from a Secret. A test
// mutates its reference to reach pointer depth.
const esPasswordEnv = "CAMUNDA_DATA_SECONDARYSTORAGE_ELASTICSEARCH_PASSWORD"

// envSecretRef returns the secret reference that the named variable reads
// from, so a test can read or mutate it at pointer depth.
func envSecretRef(t *testing.T, env []corev1.EnvVar, name string) *corev1.SecretKeySelector {
	t.Helper()

	for i := range env {
		if env[i].Name == name && env[i].ValueFrom != nil {
			return env[i].ValueFrom.SecretKeyRef
		}
	}

	require.Fail(t, "no secret reference named "+name)

	return nil
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

	in := restoreInput()
	in.Ordinal = 2
	target := in.Target

	job, err := BuildJob(in)
	require.NoError(t, err)

	container := job.Spec.Template.Spec.Containers[0]
	assert.Equal(t, target.Broker.Image, container.Image)
	assert.Equal(t, []string{RestoreEntrypoint}, container.Command)
	assert.Equal(t, []string{restoreArg}, container.Args)
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
	assert.Equal(t, restoreName, job.Labels["camunda.io/point-in-time-restore"])
	assert.Equal(t, "my-cluster", job.Labels["camunda.io/cluster"])
	assert.Equal(t, corev1.RestartPolicyNever, job.Spec.Template.Spec.RestartPolicy)
}

// The restore pods spread exactly as the brokers do, so the recreated volumes
// land in zones the brokers can schedule into afterwards. The selector has to
// point at the restore pods: they carry camunda.io/component: restore, so the
// broker's own selector would count nothing.
func TestBuildJobSpreadsTheRestorePodsLikeTheBrokers(t *testing.T) {
	t.Parallel()

	in := restoreInput()
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
			"camunda.io/point-in-time-restore": restoreName,
			"camunda.io/component":             "restore",
		},
		spread.LabelSelector.MatchLabels,
	)
	assert.Subset(t, job.Spec.Template.Labels, spread.LabelSelector.MatchLabels)
}

// A Job pod has a random host name, so the broker's node-id shell wrapper
// cannot run here. The ordinal arrives as a plain value instead.
func TestBuildJobReplacesTheBrokerCommand(t *testing.T) {
	t.Parallel()

	in := restoreInput()
	in.Target.Broker.Command = []string{
		"bash", "-c", "export CAMUNDA_CLUSTER_NODEID=${HOSTNAME##*-}; exec /usr/local/camunda/bin/camunda",
	}

	job, err := BuildJob(in)
	require.NoError(t, err)

	container := job.Spec.Template.Spec.Containers[0]
	assert.Equal(t, []string{"/usr/local/camunda/bin/restore"}, container.Command)
	assert.Equal(t, "0", envValue(container.Env, "CAMUNDA_CLUSTER_NODEID"))
}

// The pod runs the restore application and nothing else. A broker init
// container prepares a broker, and the platform injects its own again when
// the Job pod is created, so carrying the broker's copy over would run it
// twice.
func TestBuildJobDropsTheBrokerInitContainers(t *testing.T) {
	t.Parallel()

	in := restoreInput()
	in.Target.StatefulSet.Spec.Template.Spec.InitContainers = []corev1.Container{{
		Name: "wait-for-gateway", Image: "busybox",
	}}

	job, err := BuildJob(in)
	require.NoError(t, err)

	assert.Empty(t, job.Spec.Template.Spec.InitContainers)
	require.Len(t, job.Spec.Template.Spec.Containers, 1)
	assert.Equal(t, ComponentRestore, job.Spec.Template.Spec.Containers[0].Name)
}

// One Target renders one Job per broker. Nothing the builder returns may
// share memory with the Target or with another Job, or a caller that edits
// one Job silently edits the live StatefulSet copy and every sibling.
func TestBuildJobSharesNothingWithTheTargetOrASibling(t *testing.T) {
	t.Parallel()

	in := restoreInput()
	target := in.Target

	first, err := BuildJob(in)
	require.NoError(t, err)

	second := in
	second.Ordinal = 1
	other, err := BuildJob(second)
	require.NoError(t, err)

	edited := first.Spec.Template.Spec.Containers[0]
	edited.SecurityContext.RunAsNonRoot = nil
	edited.VolumeMounts[0].MountPath = "/mutated"
	edited.EnvFrom = append(edited.EnvFrom, corev1.EnvFromSource{})
	edited.Resources.Requests[corev1.ResourceCPU] = resource.MustParse("99")
	first.Spec.Template.Annotations["mutated"] = "yes"

	// A shallow clone of a slice copies the elements but not what they point
	// at, so these three reach through the copy into the target.
	*edited.VolumeMounts[1].MountPropagation = corev1.MountPropagationBidirectional
	edited.EnvFrom[0].ConfigMapRef.Name = "mutated"
	envSecretRef(t, edited.Env, esPasswordEnv).Name = "mutated"

	untouched := other.Spec.Template.Spec.Containers[0]
	assert.Equal(t, new(true), untouched.SecurityContext.RunAsNonRoot)
	assert.Equal(t, "/usr/local/camunda/data", untouched.VolumeMounts[0].MountPath)
	assert.Equal(t, resource.MustParse("1"), untouched.Resources.Requests[corev1.ResourceCPU])
	assert.NotContains(t, other.Spec.Template.Annotations, "mutated")
	assert.Equal(t, corev1.MountPropagationNone, *untouched.VolumeMounts[1].MountPropagation)
	assert.Equal(t, "broker-extra", untouched.EnvFrom[0].ConfigMapRef.Name)
	assert.Equal(t, "es-credentials", envSecretRef(t, untouched.Env, esPasswordEnv).Name)

	assert.Equal(t, new(true), target.Broker.SecurityContext.RunAsNonRoot)
	assert.Equal(t, "/usr/local/camunda/data", target.Broker.VolumeMounts[0].MountPath)
	assert.Equal(t, resource.MustParse("1"), target.Broker.Resources.Requests[corev1.ResourceCPU])
	assert.NotContains(t, target.StatefulSet.Spec.Template.Annotations, "mutated")
	assert.Equal(t, corev1.MountPropagationNone, *target.Broker.VolumeMounts[1].MountPropagation)
	assert.Equal(t, "broker-extra", target.Broker.EnvFrom[0].ConfigMapRef.Name)
	assert.Equal(t, "es-credentials", envSecretRef(t, target.Broker.Env, esPasswordEnv).Name)
}

// The restore application runs under its own Spring profile. The broker
// profile of the copied environment would start a broker instead.
func TestBuildJobRunsUnderTheRestoreProfile(t *testing.T) {
	t.Parallel()

	job, err := BuildJob(restoreInput())
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

	job, err := BuildJob(restoreInput())
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

	in := restoreInput()
	in.Target.StatefulSet.Spec.Template.Labels["camunda.io/component"] = "zeebe"

	job, err := BuildJob(in)
	require.NoError(t, err)

	pod := job.Spec.Template.Labels
	assert.Equal(t, "restore", pod["camunda.io/component"])
	assert.Equal(t, "my-cluster", pod["camunda.io/cluster"])
	assert.Equal(t, restoreName, pod["camunda.io/point-in-time-restore"])
	assert.Equal(t, "camunda-operator", pod["app.kubernetes.io/managed-by"])
}

// BuildJob renders without a scheme, so it sets no owner reference. The
// controller adds the controller reference before it applies the Job. This
// pins that deliberate gap, so nobody assumes garbage collection is wired.
func TestBuildJobSetsNoOwnerReference(t *testing.T) {
	t.Parallel()

	job, err := BuildJob(restoreInput())
	require.NoError(t, err)

	assert.Empty(t, job.OwnerReferences)
	assert.Empty(t, job.Spec.Template.OwnerReferences)
	assert.Equal(t, "ns", job.Namespace)
	assert.Equal(t, restoreJob0, job.Name)
}

// Two restores of different kinds and the same name can live in one
// namespace. Without the kind in the Job name they would fight over one Job,
// each adopting the other's.
func TestJobNameCarriesTheRestoreKind(t *testing.T) {
	t.Parallel()

	pitr := labels.PointInTimeRestore("my-cluster-recovery")

	assert.Equal(t, "my-cluster-recovery-pitr-0", JobName(pitr, 0))
	for key, infix := range jobKindInfixes {
		assert.Contains(t, JobName(labels.Owner{Key: key, Name: "r"}, 0), "-"+infix+"-")
	}
}

// An owner that names no restore kind yields no name. BuildJob turns that
// into an error rather than applying a Job under a name nothing can find.
func TestJobNameRefusesAnythingButARestoreOwner(t *testing.T) {
	t.Parallel()

	assert.Empty(t, JobName(labels.Cluster("my-cluster"), 0))
	assert.Empty(t, JobName(labels.Owner{}, 0))
	assert.Empty(t, JobName(labels.PointInTimeRestore(""), 0))
	assert.Empty(t, JobName(labels.PointInTimeRestore("r"), -1))
}

// The Job takes its name from OwnerLabel and its namespace from Owner. If the
// two name different resources, the Job lands under one restore's name in the
// other's namespace, and neither controller finds it.
func TestBuildJobRejectsAnOwnerThatDisagreesWithItsLabel(t *testing.T) {
	t.Parallel()

	in := restoreInput()
	in.OwnerLabel = labels.PointInTimeRestore("another-restore")

	job, err := BuildJob(in)
	require.Error(t, err)
	assert.Nil(t, job)
	assert.Contains(t, err.Error(), "another-restore")
	assert.Contains(t, err.Error(), in.Owner.GetName())
}

// The infix of a Job name is the short name of its CRD, so a user who reads
// a Job name can reach the restore with the kubectl alias they already know.
// The marker is the source of truth:
//
//	api/v1/pointintimerestore_types.go: +kubebuilder:resource:path=pointintimerestores,shortName=pitr
//
// Changing a marker without changing the infix breaks that promise, and this
// test is what says so. Every restore kind needs an entry of its own here.
func TestJobNameInfixesMatchTheCRDShortNames(t *testing.T) {
	t.Parallel()

	assert.Equal(t, map[string]string{labels.PointInTimeRestoreKey: "pitr"}, jobKindInfixes)
}

// A consumer that builds a selector by hand misses every Job of a restore
// whose name passes a label value, because the owner label carries the
// bounded name. The exported helpers are the only correct source.
func TestJobSelectorRoundTripsALongRestoreName(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("a", 200)
	owner := labels.PointInTimeRestore(long)

	in := restoreInput()
	in.Owner.SetName(long)
	in.OwnerLabel = owner

	job, err := BuildJob(in)
	require.NoError(t, err)

	selector := JobSelector(owner)
	require.Len(t, selector, 2)
	for key, value := range selector {
		assert.LessOrEqual(t, len(value), validation.LabelValueMaxLength, key)
		assert.Equal(t, value, job.Labels[key], key)
		assert.Equal(t, value, job.Spec.Template.Labels[key], key)
	}

	// The naive selector that a consumer would write by hand does not match.
	naive := map[string]string{
		labels.PointInTimeRestoreKey: long,
		labels.ComponentKey:          ComponentRestore,
	}
	assert.NotEqual(
		t, naive[labels.PointInTimeRestoreKey], job.Labels[labels.PointInTimeRestoreKey],
	)

	full := JobLabels(owner, "my-cluster")
	assert.Equal(t, full, job.Labels)
	assert.Equal(t, "my-cluster", full[labels.ClusterKey])
	assert.Subset(t, full, selector)
}

// A Target that is missing any of its parts reaches BuildJob only through
// misuse, because ReadTarget fills all of them. It still must not panic.
func TestBuildJobRejectsAnIncompleteTarget(t *testing.T) {
	t.Parallel()

	for name, tc := range incompleteTargets() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			in := restoreInput()
			in.Target = tc.target

			job, err := BuildJob(in)
			require.Error(t, err)
			assert.Nil(t, job)
			assert.Contains(t, err.Error(), tc.text)
			assert.Contains(t, err.Error(), "restore Job")
		})
	}
}

// BuildJob runs inside a reconcile. A panic there takes the whole manager
// down with every other controller, so every input it cannot render is an
// error.
func TestBuildJobRejectsAnInputItCannotRender(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		mutate func(*JobInput)
		text   string
	}{
		"no owner": {
			mutate: func(in *JobInput) { in.Owner = nil },
			text:   "owner",
		},
		"a typed nil owner": {
			mutate: func(in *JobInput) { in.Owner = (*v1.PointInTimeRestore)(nil) },
			text:   "owner",
		},
		"no owner label": {
			mutate: func(in *JobInput) { in.OwnerLabel = labels.Owner{} },
			text:   "owner",
		},
		"an owner label without a name": {
			mutate: func(in *JobInput) {
				in.OwnerLabel = labels.Owner{Key: labels.PointInTimeRestoreKey}
			},
			text: "owner",
		},
		"an owner label of another kind": {
			mutate: func(in *JobInput) { in.OwnerLabel = labels.Cluster("my-cluster") },
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
			in := restoreInput()
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

	owner := labels.PointInTimeRestore(restoreName)
	assert.Equal(t, restoreJob0, JobName(owner, 0))
	assert.Equal(t, "my-cluster-pitr-pitr-12", JobName(owner, 12))

	name := JobName(labels.PointInTimeRestore(strings.Repeat("a", 200)), 0)
	assert.LessOrEqual(t, len(name), validation.DNS1123LabelMaxLength)
	assert.True(t, strings.HasSuffix(name, "-pitr-0"))
	assert.Empty(t, validation.IsDNS1123Label(name))

	other := JobName(labels.PointInTimeRestore(strings.Repeat("a", 199)+"b"), 0)
	assert.NotEqual(t, name, other)
	assert.LessOrEqual(t, len(other), validation.DNS1123LabelMaxLength)

	// A two-digit ordinal is the tighter bound on the name.
	twelve := JobName(labels.PointInTimeRestore(strings.Repeat("a", 200)), 12)
	assert.LessOrEqual(t, len(twelve), validation.DNS1123LabelMaxLength)
	assert.True(t, strings.HasSuffix(twelve, "-pitr-12"))
	assert.Empty(t, validation.IsDNS1123Label(twelve))
}

// The whole rendered Job of the first broker: the broker container mirrored
// into the restore container, the recreated data volume, and the labels.
func TestJobGoldenBroker0(t *testing.T) {
	t.Parallel()

	golden.AssertYAML(
		t, "testdata/golden/pitr-broker-0.yaml", jobPreview{restoreInput()},
		golden.WithScheme(testScheme(t)), golden.Update(*updateGolden),
	)
}

// The node id, the Job name, and the claim all follow the ordinal.
func TestJobGoldenBroker2(t *testing.T) {
	t.Parallel()

	in := restoreInput()
	in.Ordinal = 2

	golden.AssertYAML(
		t, "testdata/golden/pitr-broker-2.yaml", jobPreview{in},
		golden.WithScheme(testScheme(t)), golden.Update(*updateGolden),
	)
}

// A restore that passes no argument renders no args key at all. The relational
// logical restore takes that shape: its restore application reads the exporter
// position from the restored database itself.
func TestJobGoldenNoArgs(t *testing.T) {
	t.Parallel()

	in := restoreInput()
	in.Args = nil

	golden.AssertYAML(
		t, "testdata/golden/pitr-no-args.yaml", jobPreview{in},
		golden.WithScheme(testScheme(t)), golden.Update(*updateGolden),
	)
}
