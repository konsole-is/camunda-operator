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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// unheld removes the hold of the restore from the cluster of the world, in
// the store and in the copy that the step reads.
func (w *prepareWorld) unheld(t *testing.T) {
	t.Helper()

	current := w.live(t)
	delete(current.Annotations, holdKey(w.restore))
	require.NoError(t, w.client.Update(t.Context(), current))
	delete(w.cluster.Annotations, holdKey(w.restore))
}

// liveRestore returns the restore of the world as the store holds it.
func (w *prepareWorld) liveRestore(t *testing.T) *v1.PointInTimeRestore {
	t.Helper()

	var current v1.PointInTimeRestore
	require.NoError(t, w.client.Get(t.Context(), client.ObjectKeyFromObject(w.restore), &current))

	return &current
}

// The hold comes first, so a cluster that somebody unsuspends later stays down.
func TestPrepareHoldsTheClusterBeforeAnythingElse(t *testing.T) {
	t.Parallel()

	w := newPrepareWorld(t)
	w.unheld(t)
	w.unsuspended(t)

	w.look(t)

	require.Len(t, *w.applies, 1, "the suspension is recorded before it is written")
	hold := (*w.applies)[0]
	assert.Equal(t, string(holdManager(w.restore)), hold.manager)
	assert.Equal(t, clusterUID, hold.cluster.UID, "the apply carries no precondition")
	assert.Equal(
		t,
		map[string]string{holdKey(w.restore): "PointInTimeRestore ns/my-cluster-pitr restores into this cluster"},
		hold.cluster.Annotations,
	)
}

// The apply of one restore never removes the hold of another.
func TestHoldManagerIsPerRestore(t *testing.T) {
	t.Parallel()

	first, second := restoreOwner(1), restoreOwner(1)
	first.UID, second.UID = "first", "second"

	assert.NotEqual(t, holdManager(first), holdManager(second))
	assert.NotEqual(t, holdKey(first), holdKey(second))
	assert.Equal(t, v1.SuspensionHoldPrefix+"first", holdKey(first))
}

func TestResumeRemovesTheHoldOfACompletedRestore(t *testing.T) {
	t.Parallel()

	w := newPrepareWorld(t)

	require.NoError(t, Resume(
		t.Context(), w.client, w.client, w.restore, w.completed(), client.ObjectKeyFromObject(w.cluster),
	))

	holds := w.appliesBy(holdManager(w.restore))
	require.Len(t, holds, 1)
	assert.Empty(t, holds[0].cluster.Annotations)
	assert.Equal(t, clusterUID, holds[0].cluster.UID)
}

// Brokers must not start over the half-written volumes of a failed restore.
func TestResumeKeepsTheHoldOfAFailedRestore(t *testing.T) {
	t.Parallel()

	w := newPrepareWorld(t)
	Fail(w.progress(), v1.ReasonFailed, "a broker could not restore", metav1.Now())

	require.NoError(t, Resume(
		t.Context(), w.client, w.client, w.restore, w.progress(), client.ObjectKeyFromObject(w.cluster),
	))

	assert.Empty(t, *w.applies)
}

func TestAddHoldFinalizerAddsItOnce(t *testing.T) {
	t.Parallel()

	w := newPrepareWorld(t)
	owner := w.liveRestore(t)

	require.NoError(t, AddHoldFinalizer(t.Context(), w.client, owner))
	require.NoError(t, AddHoldFinalizer(t.Context(), w.client, owner))

	assert.Equal(t, []string{HoldFinalizer}, w.liveRestore(t).Finalizers)
}

// The suspension that the restore applied through spec.suspend stays.
func TestFinalizeHoldRemovesTheHoldAndThenTheFinalizer(t *testing.T) {
	t.Parallel()

	w := newPrepareWorld(t)
	owner := w.liveRestore(t)
	require.NoError(t, AddHoldFinalizer(t.Context(), w.client, owner))

	finalized, err := FinalizeHold(t.Context(), w.client, w.client, owner, podLabel(owner), w.cluster.Name)
	require.NoError(t, err)
	assert.True(t, finalized.Done)

	holds := w.appliesBy(holdManager(owner))
	require.Len(t, holds, 1)
	assert.Empty(t, holds[0].cluster.Annotations)
	assert.Empty(t, w.appliesBy(FieldManagerTargetSuspend))
	assert.False(t, controllerutil.ContainsFinalizer(w.liveRestore(t), HoldFinalizer))
}

func TestFinalizeHoldLetsGoWhenTheClusterIsGone(t *testing.T) {
	t.Parallel()

	w := newPrepareWorld(t)
	owner := w.liveRestore(t)
	require.NoError(t, AddHoldFinalizer(t.Context(), w.client, owner))
	require.NoError(t, w.client.Delete(t.Context(), w.cluster))

	finalized, err := FinalizeHold(t.Context(), w.client, w.client, owner, podLabel(owner), w.cluster.Name)
	require.NoError(t, err)
	assert.True(t, finalized.Done)

	assert.Empty(t, *w.applies)
	assert.False(t, controllerutil.ContainsFinalizer(w.liveRestore(t), HoldFinalizer))
}

// A Job pod of the restore still writes the storage of the target until the
// Job is gone.
func TestFinalizeHoldKeepsTheHoldUntilTheJobsOfTheRestoreAreGone(t *testing.T) {
	t.Parallel()

	w := newPrepareWorld(t)
	owner := w.liveRestore(t)
	require.NoError(t, AddHoldFinalizer(t.Context(), w.client, owner))
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name:       "r-pg-restore",
		Namespace:  owner.Namespace,
		Finalizers: []string{"test/pods-remain"},
	}}
	require.NoError(t, controllerutil.SetControllerReference(owner, job, w.client.Scheme()))
	require.NoError(t, w.client.Create(t.Context(), job))

	finalized, err := FinalizeHold(t.Context(), w.client, w.client, owner, podLabel(owner), w.cluster.Name)
	require.NoError(t, err)
	assert.False(t, finalized.Done)
	assert.Positive(t, finalized.Wait)
	assert.Empty(t, w.appliesBy(holdManager(owner)))
	assert.True(t, controllerutil.ContainsFinalizer(w.liveRestore(t), HoldFinalizer))

	var deleting batchv1.Job
	require.NoError(t, w.client.Get(t.Context(), client.ObjectKeyFromObject(job), &deleting))
	assert.NotNil(t, deleting.DeletionTimestamp)

	deleting.Finalizers = nil
	require.NoError(t, w.client.Update(t.Context(), &deleting))

	finalized, err = FinalizeHold(t.Context(), w.client, w.client, w.liveRestore(t), podLabel(owner), w.cluster.Name)
	require.NoError(t, err)
	assert.True(t, finalized.Done)
	assert.Len(t, w.appliesBy(holdManager(owner)), 1)
	assert.False(t, controllerutil.ContainsFinalizer(w.liveRestore(t), HoldFinalizer))
}

// A Job deleted with background propagation is gone before its pods are.
func TestFinalizeHoldKeepsTheHoldUntilTheJobPodsOfTheRestoreAreGone(t *testing.T) {
	t.Parallel()

	w := newPrepareWorld(t)
	owner := w.liveRestore(t)
	require.NoError(t, AddHoldFinalizer(t.Context(), w.client, owner))
	label := podLabel(owner)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:      "r-pitr-0-abcde",
		Namespace: owner.Namespace,
		Labels:    map[string]string{label.Key: label.Name},
	}}
	require.NoError(t, w.client.Create(t.Context(), pod))

	finalized, err := FinalizeHold(t.Context(), w.client, w.client, owner, label, w.cluster.Name)
	require.NoError(t, err)
	assert.False(t, finalized.Done)
	assert.Empty(t, w.appliesBy(holdManager(owner)))

	require.NoError(t, w.client.Delete(t.Context(), pod))

	finalized, err = FinalizeHold(t.Context(), w.client, w.client, w.liveRestore(t), label, w.cluster.Name)
	require.NoError(t, err)
	assert.True(t, finalized.Done)
	assert.Len(t, w.appliesBy(holdManager(owner)), 1)
}

// A failed read is an error, not a wait.
func TestFinalizeHoldReturnsAFailedReadWithoutAWait(t *testing.T) {
	t.Parallel()

	w := newPrepareWorld(t)
	owner := w.liveRestore(t)
	require.NoError(t, AddHoldFinalizer(t.Context(), w.client, owner))

	finalized, err := FinalizeHold(
		t.Context(),
		w.client,
		failingLister{w.client},
		owner,
		podLabel(owner),
		w.cluster.Name,
	)
	require.Error(t, err)
	assert.Zero(t, finalized)
	assert.True(t, controllerutil.ContainsFinalizer(w.liveRestore(t), HoldFinalizer))
}

type failingLister struct{ client.Reader }

func (failingLister) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return errors.New("list refused")
}

func podLabel(owner *v1.PointInTimeRestore) labels.Owner {
	return labels.PointInTimeRestore(owner.Name)
}
