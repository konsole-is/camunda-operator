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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// listAll returns the ReplicaSets and Pods of team-a as unstructured
// objects, as the scope of a scenario lists them.
func listAll(t *testing.T, c client.Client) []unstructured.Unstructured {
	t.Helper()

	objs := make([]unstructured.Unstructured, 0, 2)
	for _, gvk := range []schema.GroupVersionKind{
		appsv1.SchemeGroupVersion.WithKind("ReplicaSetList"),
		corev1.SchemeGroupVersion.WithKind("PodList"),
	} {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(gvk)
		require.NoError(t, c.List(context.Background(), list, client.InNamespace("team-a")))
		objs = append(objs, list.Items...)
	}
	return objs
}

// ownedPod returns a Pod of team-a that rs controls.
func ownedPod(name string, rs *appsv1.ReplicaSet) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "team-a",
			Name:      name,
			UID:       k8stypes.UID("uid-" + name),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(rs, appsv1.SchemeGroupVersion.WithKind("ReplicaSet")),
			},
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "app"}}},
	}
}

// replicaSet returns a ReplicaSet of team-a with uid.
func replicaSet(name, uid string) *appsv1.ReplicaSet {
	return &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: name, UID: k8stypes.UID(uid)}}
}

func TestCollectGarbageDeletesAnObjectWhoseOwnerIsGone(t *testing.T) {
	gone := replicaSet("old", "uid-old")
	c := fakeClient(t, ownedPod("old-pod", gone))
	sim := newSimulator(c)

	writes, err := sim.collectGarbage(context.Background(), listAll(t, c))
	require.NoError(t, err)

	assert.Equal(t, 1, writes)
	err = c.Get(context.Background(), client.ObjectKey{Namespace: "team-a", Name: "old-pod"}, &corev1.Pod{})
	assert.True(t, apierrors.IsNotFound(err))
}

func TestCollectGarbageKeepsAnObjectWhoseOwnerExists(t *testing.T) {
	rs := replicaSet("current", "uid-current")
	c := fakeClient(t, rs, ownedPod("current-pod", rs))
	sim := newSimulator(c)

	writes, err := sim.collectGarbage(context.Background(), listAll(t, c))
	require.NoError(t, err)

	assert.Zero(t, writes)
	assert.NotNil(t, getPod(t, c, "current-pod"))
}

func TestCollectGarbageFinishesAForegroundDeletion(t *testing.T) {
	rs := replicaSet("current", "uid-current")
	rs.Finalizers = []string{metav1.FinalizerDeleteDependents}
	c := fakeClient(t, rs, ownedPod("current-pod", rs))
	sim := newSimulator(c)
	require.NoError(t, c.Delete(context.Background(), rs))

	_, err := sim.collectGarbage(context.Background(), listAll(t, c))
	require.NoError(t, err)
	err = c.Get(context.Background(), client.ObjectKey{Namespace: "team-a", Name: "current-pod"}, &corev1.Pod{})
	assert.True(t, apierrors.IsNotFound(err), "the dependent goes first")

	_, err = sim.collectGarbage(context.Background(), listAll(t, c))
	require.NoError(t, err)
	err = c.Get(context.Background(), client.ObjectKeyFromObject(rs), &appsv1.ReplicaSet{})
	assert.True(t, apierrors.IsNotFound(err), "then the owner")
}

func TestCollectGarbageOrphansTheDependentsOfAnOrphanDeletion(t *testing.T) {
	rs := replicaSet("current", "uid-current")
	rs.Finalizers = []string{metav1.FinalizerOrphanDependents}
	c := fakeClient(t, rs, ownedPod("current-pod", rs))
	sim := newSimulator(c)
	require.NoError(t, c.Delete(context.Background(), rs))

	_, err := sim.collectGarbage(context.Background(), listAll(t, c))
	require.NoError(t, err)

	pod := getPod(t, c, "current-pod")
	assert.Empty(t, pod.OwnerReferences)
	err = c.Get(context.Background(), client.ObjectKeyFromObject(rs), &appsv1.ReplicaSet{})
	assert.True(t, apierrors.IsNotFound(err))
}
