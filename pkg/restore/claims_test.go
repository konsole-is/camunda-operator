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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

const testManager = FieldManagerPointInTimeRestore

// targetFixture is the target that ReadTarget builds from brokerStatefulSet.
func targetFixture() *Target {
	sts := brokerStatefulSet()

	return &Target{
		ClusterName:   "my-cluster",
		StatefulSet:   sts,
		Broker:        &sts.Spec.Template.Spec.Containers[0],
		Brokers:       3,
		Partitions:    6,
		Version:       "8.9.9",
		ClaimTemplate: &sts.Spec.VolumeClaimTemplates[0],
	}
}

func TestClaimNamesFollowTheStatefulSet(t *testing.T) {
	t.Parallel()

	assert.Equal(
		t,
		[]string{"data-my-cluster-zeebe-0", "data-my-cluster-zeebe-1", "data-my-cluster-zeebe-2"},
		targetFixture().ClaimNames(),
	)
}

// The backup records the effective restore size of a broker volume. It is
// the size the restored data needs, which can exceed the size the claim
// template asks for.
func TestClaimSizePrefersTheRecordedRestoreSize(t *testing.T) {
	t.Parallel()

	target := targetFixture()

	recorded := resource.MustParse("30Gi")
	assert.Equal(t, recorded, target.ClaimSize(&recorded))
	assert.Equal(t, resource.MustParse("10Gi"), target.ClaimSize(nil))
}

// The StatefulSet owns the broker claims. An owner reference to the restore
// would delete a live broker volume as soon as the restore is deleted.
func TestBuildClaimCarriesNoOwnerReference(t *testing.T) {
	t.Parallel()

	target := targetFixture()

	claim := target.BuildClaim(0, resource.MustParse("10Gi"))
	assert.Empty(t, claim.OwnerReferences)
	assert.Equal(t, "data-my-cluster-zeebe-0", claim.Name)
	assert.Equal(t, "ns", claim.Namespace)
	assert.Equal(t, "my-cluster", claim.Labels["camunda.io/cluster"])
	assert.Equal(t, "zeebe", claim.Labels["camunda.io/component"])
	assert.Equal(
		t,
		[]corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		claim.Spec.AccessModes,
	)
	assert.Equal(
		t,
		resource.MustParse("10Gi"),
		claim.Spec.Resources.Requests[corev1.ResourceStorage],
	)
}

func existingClaim(name string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
			},
		},
	}
}

func claimInput(size string) ClaimInput {
	return ClaimInput{
		Target:       targetFixture(),
		Size:         resource.MustParse(size),
		FieldManager: testManager,
	}
}

// The restore application refuses a non-empty data directory, so every broker
// volume is deleted and created again at the size the restore asks for. The
// delete and the create never share a call: the caller persists the record of
// the deletion in between.
func TestRecreateClaimsDeletesEveryClaimThenCreatesItEmpty(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(
			existingClaim("data-my-cluster-zeebe-0"),
			existingClaim("data-my-cluster-zeebe-1"),
			existingClaim("data-my-cluster-zeebe-2"),
		).
		Build()

	in := claimInput("30Gi")
	first, err := RecreateClaims(ctx, c, c, in)
	require.NoError(t, err)
	assert.False(t, first.Done)
	assert.Equal(
		t,
		[]string{"data-my-cluster-zeebe-0", "data-my-cluster-zeebe-1", "data-my-cluster-zeebe-2"},
		first.Recreated,
	)

	in.Recreated = first.Recreated
	second, err := RecreateClaims(ctx, c, c, in)
	require.NoError(t, err)
	assert.True(t, second.Done)
	assert.Equal(t, first.Recreated, second.Recreated)

	for _, name := range second.Recreated {
		var claim corev1.PersistentVolumeClaim
		require.NoError(t, c.Get(ctx, client.ObjectKey{Namespace: "ns", Name: name}, &claim))
		assert.Equal(
			t,
			resource.MustParse("30Gi"),
			claim.Spec.Resources.Requests[corev1.ResourceStorage],
		)
	}
}

// The controller records a name before it creates the empty volume. On
// re-entry the recorded name is never deleted again, so a restore that
// crashed after the apply cannot erase the volume it just created.
func TestRecreateClaimsNeverDeletesARecordedClaimTwice(t *testing.T) {
	t.Parallel()

	deleted := map[string]int{}
	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(
			existingClaim("data-my-cluster-zeebe-0"),
			existingClaim("data-my-cluster-zeebe-1"),
			existingClaim("data-my-cluster-zeebe-2"),
		).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(
				ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption,
			) error {
				deleted[obj.GetName()]++

				return c.Delete(ctx, obj, opts...)
			},
		}).
		Build()

	in := claimInput("10Gi")
	in.Recreated = []string{"data-my-cluster-zeebe-0"}

	progress, err := RecreateClaims(context.Background(), c, c, in)
	require.NoError(t, err)
	assert.Zero(t, deleted["data-my-cluster-zeebe-0"])
	assert.Equal(t, 1, deleted["data-my-cluster-zeebe-1"])
	assert.Equal(
		t,
		[]string{"data-my-cluster-zeebe-0", "data-my-cluster-zeebe-1", "data-my-cluster-zeebe-2"},
		progress.Recreated,
	)
}

// A pod that still holds a claim keeps it terminating. That is a wait, not a
// failure: the caller polls until the volume is gone and created again.
func TestRecreateClaimsWaitsWhileAClaimTerminates(t *testing.T) {
	t.Parallel()

	terminating := existingClaim("data-my-cluster-zeebe-1")
	terminating.DeletionTimestamp = new(metav1.Now())
	terminating.Finalizers = []string{"kubernetes.io/pvc-protection"}

	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(
			existingClaim("data-my-cluster-zeebe-0"),
			terminating,
			existingClaim("data-my-cluster-zeebe-2"),
		).
		Build()

	in := claimInput("10Gi")
	in.Recreated = targetFixture().ClaimNames()

	progress, err := RecreateClaims(context.Background(), c, c, in)
	require.NoError(t, err)
	assert.False(t, progress.Done)
	assert.Contains(t, progress.Message, "data-my-cluster-zeebe-1")
	assert.Contains(t, progress.Message, "terminating")
}

// A claim that never existed needs no delete. A restore into a cluster whose
// volumes were already removed still creates them.
func TestRecreateClaimsCreatesAClaimThatIsGone(t *testing.T) {
	t.Parallel()

	c := fake.NewClientBuilder().WithScheme(testScheme(t)).Build()

	progress, err := RecreateClaims(context.Background(), c, c, claimInput("10Gi"))
	require.NoError(t, err)
	assert.True(t, progress.Done)

	var claim corev1.PersistentVolumeClaim
	require.NoError(t, c.Get(
		context.Background(),
		client.ObjectKey{Namespace: "ns", Name: "data-my-cluster-zeebe-2"},
		&claim,
	))
}

// RecreateClaims runs inside a reconcile, and it deletes volumes. A Target
// that is missing any of its parts reaches it only through misuse, and it
// must fail before it touches anything rather than panic the manager.
func TestRecreateClaimsRejectsAnIncompleteTarget(t *testing.T) {
	t.Parallel()

	for name, tc := range incompleteTargets() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := fake.NewClientBuilder().
				WithScheme(testScheme(t)).
				WithObjects(existingClaim("data-my-cluster-zeebe-0")).
				Build()

			in := claimInput("10Gi")
			in.Target = tc.target

			progress, err := RecreateClaims(context.Background(), c, c, in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.text)
			assert.Contains(t, err.Error(), "broker volumes")
			assert.Equal(t, Progress{}, progress)

			// Nothing was deleted on the way out.
			var claim corev1.PersistentVolumeClaim
			assert.NoError(t, c.Get(
				context.Background(),
				client.ObjectKey{Namespace: "ns", Name: "data-my-cluster-zeebe-0"},
				&claim,
			))
		})
	}
}

// A transport error must reach the caller. Reporting progress on a failed
// delete would let the restore run the Jobs on the old data.
func TestRecreateClaimsReportsADeleteErrorAsAnError(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(existingClaim("data-my-cluster-zeebe-0")).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(
				context.Context, client.WithWatch, client.Object, ...client.DeleteOption,
			) error {
				return boom
			},
		}).
		Build()

	_, err := RecreateClaims(context.Background(), c, c, claimInput("10Gi"))
	require.ErrorIs(t, err, boom)
}

// terminatingClaim is the broker volume of one ordinal, as the API server
// leaves it while a pod still holds it under the protection finalizer.
func terminatingClaim(name string) *corev1.PersistentVolumeClaim {
	claim := existingClaim(name)
	claim.DeletionTimestamp = new(metav1.Now())
	claim.Finalizers = []string{"kubernetes.io/pvc-protection"}

	return claim
}

// holdingPod is a pod that mounts the named claim, under the given
// controller.
func holdingPod(name, claim string, controller *metav1.OwnerReference) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{{
				Name: "data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claim},
				},
			}},
		},
	}
	if controller != nil {
		pod.OwnerReferences = []metav1.OwnerReference{*controller}
	}

	return pod
}

// controllerRef is the controller reference that a pod or a Job carries.
func controllerRef(apiVersion, kind, name string) *metav1.OwnerReference {
	return &metav1.OwnerReference{
		APIVersion: apiVersion, Kind: kind, Name: name, Controller: new(true),
	}
}

// terminatingHold runs one pass over a world whose second broker volume still
// terminates, and returns the message that the restore reports.
func terminatingHold(t *testing.T, objects ...client.Object) string {
	t.Helper()

	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(append(
			[]client.Object{
				existingClaim("data-my-cluster-zeebe-0"),
				terminatingClaim("data-my-cluster-zeebe-1"),
				existingClaim("data-my-cluster-zeebe-2"),
			},
			objects...,
		)...).
		Build()

	in := claimInput("10Gi")
	in.Recreated = targetFixture().ClaimNames()

	progress, err := RecreateClaims(context.Background(), c, c, in)
	require.NoError(t, err)
	require.False(t, progress.Done)

	return progress.Message
}

// A volume that a finished restore holds names a volume, and a user cannot
// act on that. The message names the pod, the restore that runs it, and the
// one action that frees the volume.
func TestRecreateClaimsNamesTheRestoreThatHoldsATerminatingClaim(t *testing.T) {
	t.Parallel()

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "camunda-es-restore-lres-1",
			Namespace: "ns",
			OwnerReferences: []metav1.OwnerReference{*controllerRef(
				v1.GroupVersion.String(), "LogicalRestoreElasticsearch", "camunda-es-restore",
			)},
		},
	}
	pod := holdingPod(
		"camunda-es-restore-lres-1-x8k2p",
		"data-my-cluster-zeebe-1",
		controllerRef("batch/v1", "Job", job.Name),
	)

	message := terminatingHold(t, job, pod)

	assert.Contains(t, message, "data-my-cluster-zeebe-1 is still terminating")
	assert.Contains(t, message, "camunda-es-restore-lres-1-x8k2p")
	assert.Contains(t, message, "LogicalRestoreElasticsearch camunda-es-restore")
	assert.Contains(t, message, "Delete that LogicalRestoreElasticsearch")
}

// The other holder of a broker volume is a cluster that nobody suspended. Its
// broker pod belongs to the StatefulSet, and the remedy is the suspend flag.
func TestRecreateClaimsNamesTheWorkloadThatHoldsATerminatingClaim(t *testing.T) {
	t.Parallel()

	pod := holdingPod(
		"my-cluster-zeebe-1",
		"data-my-cluster-zeebe-1",
		controllerRef("apps/v1", "StatefulSet", "my-cluster-zeebe"),
	)

	message := terminatingHold(t, pod)

	assert.Contains(t, message, "my-cluster-zeebe-1")
	assert.Contains(t, message, "StatefulSet my-cluster-zeebe")
	assert.Contains(t, message, "Suspend the cluster")
}

// A lookup that finds no pod cannot tell the two causes apart, so the message
// names both of them with the remedy for each.
func TestRecreateClaimsNamesBothCausesWhenNoPodHoldsTheClaim(t *testing.T) {
	t.Parallel()

	message := terminatingHold(t)

	assert.Contains(t, message, "data-my-cluster-zeebe-1 is still terminating")
	assert.Contains(t, message, "Suspend the cluster")
	assert.Contains(t, message, "delete the restore that ran before this one")
}

// The message of a wait is never worth a failed restore. A pod lookup that
// fails reports the two causes and the restore keeps waiting.
func TestRecreateClaimsKeepsWaitingWhenThePodLookupFails(t *testing.T) {
	t.Parallel()

	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(
			existingClaim("data-my-cluster-zeebe-0"),
			terminatingClaim("data-my-cluster-zeebe-1"),
			existingClaim("data-my-cluster-zeebe-2"),
		).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(
				_ context.Context, _ client.WithWatch, list client.ObjectList, _ ...client.ListOption,
			) error {
				if _, pods := list.(*corev1.PodList); pods {
					return errors.New("boom")
				}

				return nil
			},
		}).
		Build()

	in := claimInput("10Gi")
	in.Recreated = targetFixture().ClaimNames()

	progress, err := RecreateClaims(context.Background(), c, c, in)
	require.NoError(t, err)
	assert.False(t, progress.Done)
	assert.Contains(t, progress.Message, "delete the restore that ran before this one")
}

// A workload that is not the broker StatefulSet of the target belongs to
// somebody else. The message names it and asks for the pod, because
// suspending the cluster does not remove it.
func TestRecreateClaimsAsksForAPodOfAForeignWorkload(t *testing.T) {
	t.Parallel()

	pod := holdingPod(
		"debug-shell-7f9c",
		"data-my-cluster-zeebe-1",
		controllerRef("apps/v1", "Deployment", "debug-shell"),
	)

	message := terminatingHold(t, pod)

	assert.Contains(t, message, "Deployment debug-shell")
	assert.Contains(t, message, "Remove that pod, or the workload that runs it")
	assert.NotContains(t, message, "Suspend the cluster")
}

// Naming the holder lists the pods of the namespace, and only the first hold
// reaches the caller. A restore whose volumes all terminate must not pay for
// that list once per volume.
func TestRecreateClaimsListsThePodsOnceWhileEveryClaimTerminates(t *testing.T) {
	t.Parallel()

	lists := 0
	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(
			terminatingClaim("data-my-cluster-zeebe-0"),
			terminatingClaim("data-my-cluster-zeebe-1"),
			terminatingClaim("data-my-cluster-zeebe-2"),
		).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(
				ctx context.Context,
				c client.WithWatch,
				list client.ObjectList,
				opts ...client.ListOption,
			) error {
				if _, pods := list.(*corev1.PodList); pods {
					lists++
				}

				return c.List(ctx, list, opts...)
			},
		}).
		Build()

	in := claimInput("10Gi")
	in.Recreated = targetFixture().ClaimNames()

	progress, err := RecreateClaims(context.Background(), c, c, in)
	require.NoError(t, err)
	assert.False(t, progress.Done)
	assert.Equal(t, 1, lists)
}
