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

package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// fakeClient returns a fake client with the status subresource of every kind
// the simulator writes, holding objs.
func fakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()

	s := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(s))
	return fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(
			&appsv1.StatefulSet{}, &appsv1.Deployment{}, &appsv1.ReplicaSet{},
			&batchv1.Job{}, &corev1.Pod{}, &corev1.PersistentVolumeClaim{},
		).
		WithObjects(objs...).
		Build()
}

// brokers is a StatefulSet of 3 replicas with one claim template.
func brokers() *appsv1.StatefulSet {
	labels := map[string]string{"app": "zeebe"}
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "orders-zeebe", Generation: 1},
		Spec: appsv1.StatefulSetSpec{
			Replicas: ptr.To[int32](3),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{
					Name:         "zeebe",
					Image:        "camunda/camunda:8.9.23",
					VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/usr/local/zeebe/data"}},
				}}},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "data"},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
					},
				},
			}},
		},
	}
}

// webapp is a Deployment of 2 replicas.
func webapp() *appsv1.Deployment {
	labels := map[string]string{"app": "optimize"}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "optimize", Generation: 1},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](2),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{
					Name: "optimize", Image: "camunda/optimize:8.9.23",
				}}},
			},
		},
	}
}

// backupJob is a Job with one container.
func backupJob() *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "backup"},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers:    []corev1.Container{{Name: "backup", Image: "camunda-operator-cli:test"}},
				},
			},
		},
	}
}

// getPod returns the Pod of team-a with name.
func getPod(t *testing.T, c client.Client, name string) *corev1.Pod {
	t.Helper()

	var pod corev1.Pod
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "team-a", Name: name}, &pod))
	return &pod
}

func getClaim(t *testing.T, c client.Client, namespace, name string) *corev1.PersistentVolumeClaim {
	t.Helper()

	var claim corev1.PersistentVolumeClaim
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: name}, &claim))
	return &claim
}

func podReady(pod *corev1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

func TestSimulatorMakesAStatefulSetReady(t *testing.T) {
	c := fakeClient(t, brokers())
	sim := newSimulator(c)
	sim.watch("team-a")

	_, err := sim.pass(context.Background())
	require.NoError(t, err)

	var sts appsv1.StatefulSet
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "team-a", Name: "orders-zeebe"}, &sts))
	assert.Equal(t, int32(3), sts.Status.ReadyReplicas)
	assert.Equal(t, sts.Generation, sts.Status.ObservedGeneration)

	for ordinal := range 3 {
		pod := getPod(t, c, fmt.Sprintf("orders-zeebe-%d", ordinal))
		assert.True(t, podReady(pod))
		claim := getClaim(t, c, "team-a", fmt.Sprintf("data-orders-zeebe-%d", ordinal))
		assert.Equal(t, corev1.ClaimBound, claim.Status.Phase)
	}

	writes, err := sim.pass(context.Background())
	require.NoError(t, err)
	assert.Zero(t, writes, "a second pass over a ready StatefulSet writes nothing")
}

func TestSimulatorAppliesAReadyReplicasStep(t *testing.T) {
	c := fakeClient(t, brokers())
	sim := newSimulator(c)
	sim.watch("team-a")
	sim.add([]workloadStep{{
		Kind: kindStatefulSet, Namespace: "team-a", Name: "orders-zeebe", ReadyReplicas: ptr.To[int32](1),
	}})

	_, err := sim.pass(context.Background())
	require.NoError(t, err)

	var sts appsv1.StatefulSet
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "team-a", Name: "orders-zeebe"}, &sts))
	assert.Equal(t, int32(1), sts.Status.ReadyReplicas)
	assert.Equal(t, int32(3), sts.Status.Replicas)
	assert.True(t, podReady(getPod(t, c, "orders-zeebe-0")))
	assert.False(t, podReady(getPod(t, c, "orders-zeebe-1")))
	assert.False(t, podReady(getPod(t, c, "orders-zeebe-2")))
	assert.Equal(t, corev1.PodRunning, getPod(t, c, "orders-zeebe-2").Status.Phase)
}

func TestSimulatorMakesADeploymentReadyThroughAReplicaSet(t *testing.T) {
	c := fakeClient(t, webapp())
	sim := newSimulator(c)
	sim.watch("team-a")

	_, err := sim.pass(context.Background())
	require.NoError(t, err)

	var sets appsv1.ReplicaSetList
	require.NoError(t, c.List(context.Background(), &sets, client.InNamespace("team-a")))
	require.Len(t, sets.Items, 1)
	rs := sets.Items[0]
	assert.Equal(t, "optimize", metav1.GetControllerOf(&rs).Name)

	var pods corev1.PodList
	require.NoError(t, c.List(context.Background(), &pods, client.InNamespace("team-a")))
	require.Len(t, pods.Items, 2)
	for i := range pods.Items {
		assert.Equal(t, rs.Name, metav1.GetControllerOf(&pods.Items[i]).Name)
		assert.True(t, podReady(&pods.Items[i]))
	}

	var d appsv1.Deployment
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "team-a", Name: "optimize"}, &d))
	assert.Equal(t, int32(2), d.Status.ReadyReplicas)
	available := d.Status.Conditions[0]
	assert.Equal(t, appsv1.DeploymentAvailable, available.Type)
	assert.Equal(t, corev1.ConditionTrue, available.Status)

	writes, err := sim.pass(context.Background())
	require.NoError(t, err)
	assert.Zero(t, writes, "a second pass over a ready Deployment writes nothing")
}

func TestSimulatorReportsARecreateDeploymentWithAnUnreadyPodAsUnavailable(t *testing.T) {
	d := webapp()
	d.Spec.Replicas = new(int32(4))
	d.Spec.Strategy.Type = appsv1.RecreateDeploymentStrategyType
	c := fakeClient(t, d)
	sim := newSimulator(c)
	sim.watch("team-a")
	sim.add([]workloadStep{{
		Kind: kindDeployment, Namespace: "team-a", Name: "optimize", ReadyReplicas: new(int32(3)),
	}})

	_, err := sim.pass(context.Background())
	require.NoError(t, err)

	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(d), d))
	available := d.Status.Conditions[0]
	assert.Equal(t, appsv1.DeploymentAvailable, available.Type)
	assert.Equal(t, corev1.ConditionFalse, available.Status, "a Recreate Deployment allows no unavailable Pod")
}

func TestSimulatorReplacesThePodsOfAnOldTemplate(t *testing.T) {
	c := fakeClient(t, brokers())
	sim := newSimulator(c)
	sim.watch("team-a")
	_, err := sim.pass(context.Background())
	require.NoError(t, err)
	before := getPod(t, c, "orders-zeebe-0").Labels[appsv1.ControllerRevisionHashLabelKey]

	var sts appsv1.StatefulSet
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "team-a", Name: "orders-zeebe"}, &sts))
	sts.Spec.Template.Spec.Containers[0].Image = "camunda/camunda:8.9.24"
	require.NoError(t, c.Update(context.Background(), &sts))

	for range 2 {
		_, err = sim.pass(context.Background())
		require.NoError(t, err)
	}

	for ordinal := range 3 {
		pod := getPod(t, c, fmt.Sprintf("orders-zeebe-%d", ordinal))
		assert.NotEqual(t, before, pod.Labels[appsv1.ControllerRevisionHashLabelKey])
		assert.Equal(t, "camunda/camunda:8.9.24", pod.Spec.Containers[0].Image)
	}
}

func TestSimulatorCompletesAJob(t *testing.T) {
	c := fakeClient(t, backupJob())
	sim := newSimulator(c)
	sim.watch("team-a")

	_, err := sim.pass(context.Background())
	require.NoError(t, err)

	var job batchv1.Job
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "team-a", Name: "backup"}, &job))
	assert.Equal(t, int32(1), job.Status.Succeeded)
	assert.True(t, hasJobCondition(&job, batchv1.JobComplete))

	var pods corev1.PodList
	require.NoError(t, c.List(context.Background(), &pods, client.InNamespace("team-a")))
	require.Len(t, pods.Items, 1)
	assert.Equal(t, corev1.PodSucceeded, pods.Items[0].Status.Phase)

	writes, err := sim.pass(context.Background())
	require.NoError(t, err)
	assert.Zero(t, writes, "a finished Job is not run again")
}

func TestSimulatorFailsAJobOnAFailedStep(t *testing.T) {
	c := fakeClient(t, backupJob())
	sim := newSimulator(c)
	sim.watch("team-a")
	sim.add([]workloadStep{{Kind: kindJob, Namespace: "team-a", Name: "backup", JobOutcome: jobFailed}})

	_, err := sim.pass(context.Background())
	require.NoError(t, err)

	var job batchv1.Job
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "team-a", Name: "backup"}, &job))
	assert.Equal(t, int32(7), job.Status.Failed, "the default backoff limit of 6 allows 7 tries")
	assert.True(t, hasJobCondition(&job, batchv1.JobFailed))
	assert.False(t, hasJobCondition(&job, batchv1.JobComplete))

	var pods corev1.PodList
	require.NoError(t, c.List(context.Background(), &pods, client.InNamespace("team-a")))
	assert.Len(t, pods.Items, 7)
	for i := range pods.Items {
		assert.Equal(t, corev1.PodFailed, pods.Items[i].Status.Phase)
	}
}

func TestSimulatorTriesAFailingJobAsOftenAsItsBackoffLimitAllows(t *testing.T) {
	job := backupJob()
	job.Spec.BackoffLimit = new(int32(3))
	c := fakeClient(t, job)
	sim := newSimulator(c)
	sim.watch("team-a")
	sim.add([]workloadStep{{Kind: kindJob, Namespace: "team-a", Name: "backup", JobOutcome: jobFailed}})

	_, err := sim.pass(context.Background())
	require.NoError(t, err)

	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(job), job))
	assert.Equal(t, int32(4), job.Status.Failed)
}

func TestSimulatorCompletesEachCompletionOfAJob(t *testing.T) {
	job := backupJob()
	job.Spec.Completions = new(int32(3))
	c := fakeClient(t, job)
	sim := newSimulator(c)
	sim.watch("team-a")

	_, err := sim.pass(context.Background())
	require.NoError(t, err)

	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(job), job))
	assert.Equal(t, int32(3), job.Status.Succeeded)
	var pods corev1.PodList
	require.NoError(t, c.List(context.Background(), &pods, client.InNamespace("team-a")))
	assert.Len(t, pods.Items, 3)
}

func TestSimulatorRefusesAStepThatChangesTheOutcomeOfAFinishedJob(t *testing.T) {
	c := fakeClient(t, backupJob())
	sim := newSimulator(c)
	sim.watch("team-a")
	_, err := sim.pass(context.Background())
	require.NoError(t, err)

	sim.add([]workloadStep{{Kind: kindJob, Namespace: "team-a", Name: "backup", JobOutcome: jobFailed}})
	_, err = sim.pass(context.Background())
	assert.EqualError(
		t, err,
		"workload step gives Job team-a/backup the outcome Failed, but the Job finished Complete",
	)
}

func hasJobCondition(job *batchv1.Job, t batchv1.JobConditionType) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == t && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func TestSimulatorBindsAPendingClaim(t *testing.T) {
	c := fakeClient(t, &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "data"},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("5Gi")},
			},
		},
	})
	sim := newSimulator(c)
	sim.watch("team-a")

	_, err := sim.pass(context.Background())
	require.NoError(t, err)

	claim := getClaim(t, c, "team-a", "data")
	assert.Equal(t, corev1.ClaimBound, claim.Status.Phase)
	assert.True(t, resource.MustParse("5Gi").Equal(claim.Status.Capacity[corev1.ResourceStorage]))
	assert.NotEmpty(t, claim.Spec.VolumeName)
}

func TestSimulatorReleasesADeletedClaimOnceNoPodUsesIt(t *testing.T) {
	claim := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  "team-a",
			Name:       "data",
			Finalizers: []string{pvcProtectionFinalizer},
		},
	}
	user := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "user"},
		Spec: corev1.PodSpec{Volumes: []corev1.Volume{{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data"},
			},
		}}},
	}
	c := fakeClient(t, claim, user)
	sim := newSimulator(c)
	sim.watch("team-a")
	require.NoError(t, c.Delete(context.Background(), claim))

	_, err := sim.pass(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, getClaim(t, c, "team-a", "data").DeletionTimestamp, "a Pod still uses the claim")

	require.NoError(t, c.Delete(context.Background(), user))
	_, err = sim.pass(context.Background())
	require.NoError(t, err)
	err = c.Get(context.Background(), client.ObjectKeyFromObject(claim), &corev1.PersistentVolumeClaim{})
	assert.True(t, apierrors.IsNotFound(err), "the claim is gone once no Pod uses it")
}

func TestSimulatorNamesAStepWhoseObjectIsMissing(t *testing.T) {
	c := fakeClient(t, brokers())
	sim := newSimulator(c)
	sim.watch("team-a")
	sim.add([]workloadStep{{
		Kind: kindStatefulSet, Namespace: "team-a", Name: "orders-zeeb", ReadyReplicas: ptr.To[int32](1),
	}})

	_, err := sim.pass(context.Background())
	require.NoError(t, err)

	err = sim.missingSteps(context.Background())
	assert.EqualError(t, err, "workload step names StatefulSet team-a/orders-zeeb, which does not exist")
}

func TestSimulatorAllowsTheDeletionOfAStepObjectThatExisted(t *testing.T) {
	sts := brokers()
	c := fakeClient(t, sts)
	sim := newSimulator(c)
	sim.add([]workloadStep{{
		Kind: kindStatefulSet, Namespace: "team-a", Name: "orders-zeebe", ReadyReplicas: new(int32(1)),
	}})
	require.NoError(t, sim.missingSteps(context.Background()))

	require.NoError(t, c.Delete(context.Background(), sts))
	assert.NoError(t, sim.missingSteps(context.Background()), "a later state deleted the StatefulSet")

	sim.add([]workloadStep{{
		Kind: kindStatefulSet, Namespace: "team-a", Name: "orders-zeebe", ReadyReplicas: new(int32(2)),
	}})
	assert.Error(t, sim.missingSteps(context.Background()), "a new step needs its object again")
}

func TestSimulatorLeavesOtherNamespacesAlone(t *testing.T) {
	c := fakeClient(t, brokers())
	sim := newSimulator(c)
	sim.watch("team-b")

	writes, err := sim.pass(context.Background())
	require.NoError(t, err)
	assert.Zero(t, writes)

	var pods corev1.PodList
	require.NoError(t, c.List(context.Background(), &pods, client.InNamespace("team-a")))
	assert.Empty(t, pods.Items)
}
