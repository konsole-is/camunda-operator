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
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/logicalbackuprdbms"
)

// TestReleaseJobNeverDeletesAStranger pins the identity guard of the release.
// A same-named Job of another backup survives the release, which only clears
// the recorded name. The deterministic name makes such a Job possible after
// a delete-and-recreate.
func TestReleaseJobNeverDeletesAStranger(t *testing.T) {
	t.Parallel()

	scheme := dumpScheme(t)
	backup := trackedBackup()
	stranger := ownJob(backup)
	stranger.Labels[components.BackupUIDLabel] = foreignUID

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(stranger).Build()
	r := &LogicalBackupRDBMSReconciler{Client: c, APIReader: c}

	require.NoError(t, r.releaseJob(context.Background(), backup))
	assert.Empty(t, backup.Status.JobName, "the name clears; there is nothing of ours to release")

	var survived batchv1.Job
	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(stranger), &survived))
	assert.Equal(t, foreignUID, survived.Labels[components.BackupUIDLabel])
}

// TestReleaseJobDeletesOwnJobWithItsUID proves the happy path deletes exactly
// the observed Job, with its UID as the delete precondition.
func TestReleaseJobDeletesOwnJobWithItsUID(t *testing.T) {
	t.Parallel()

	scheme := dumpScheme(t)
	backup := trackedBackup()
	own := ownJob(backup)
	own.UID = "job-uid-1"

	var preconditionUID string
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(own).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(
				ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption,
			) error {
				options := &client.DeleteOptions{}
				for _, opt := range opts {
					opt.ApplyToDelete(options)
				}
				if options.Preconditions != nil && options.Preconditions.UID != nil {
					preconditionUID = string(*options.Preconditions.UID)
				}

				return cl.Delete(ctx, obj, opts...)
			},
		}).Build()
	r := &LogicalBackupRDBMSReconciler{Client: c, APIReader: c}

	require.NoError(t, r.releaseJob(context.Background(), backup))
	assert.Empty(t, backup.Status.JobName)
	assert.Equal(t, "job-uid-1", preconditionUID, "the delete carries the observed UID")
	err := c.Get(context.Background(), client.ObjectKeyFromObject(own), &batchv1.Job{})
	assert.True(t, apierrors.IsNotFound(err), "our own Job is gone")
}

// TestFinalizerDeleteJobRetriesOnAPreconditionConflict pins the
// read-then-delete atomicity of the finalizer. A Job replaced between the two
// answers Conflict. The replacement survives, and the deletion retries with a
// fresh read instead of a failure or a fall-through.
func TestFinalizerDeleteJobRetriesOnAPreconditionConflict(t *testing.T) {
	t.Parallel()

	scheme := dumpScheme(t)
	backup := trackedBackup()
	backup.Status.JobName = ""
	own := ownJob(backup)
	own.Name = components.JobName(backup)
	own.UID = "job-uid-1"

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(own).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(
				ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption,
			) error {
				// The stranger landed between the read and the delete. The
				// UID precondition no longer matches.
				return apierrors.NewConflict(
					batchv1.Resource("jobs"), obj.GetName(),
					assert.AnError,
				)
			},
		}).Build()
	r := &LogicalBackupRDBMSReconciler{Client: c, APIReader: c}

	gone, err := r.deleteJob(context.Background(), backup)
	require.NoError(t, err, "a conflict is a changed object, not a failure")
	assert.False(t, gone, "the deletion retries with a fresh read")

	var survived batchv1.Job
	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(own), &survived))
}

// TestClaimJobNameNeverMutatesTheWinner pins the atomicity of the initial
// Job creation. It is a create-only identity claim. So a same-named foreign
// Job that lands between the absence check and the create wins the name
// untouched, with its labels and owner references intact. The backup takes
// the bounded foreign-Job failure instead of a forced apply over it.
func TestClaimJobNameNeverMutatesTheWinner(t *testing.T) {
	t.Parallel()

	scheme := dumpScheme(t)
	backup := trackedBackup()
	backup.Status.JobName = ""

	// The foreign winner is already in the store when the create runs. That
	// is exactly the interleaving where it was created after the NotFound
	// read.
	winner := ownJob(backup)
	winner.Name = components.JobName(backup)
	winner.UID = "winner-uid"
	winner.Labels[components.BackupUIDLabel] = foreignUID
	winner.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: "v1", Kind: "ConfigMap", Name: "someone", UID: "someone-uid",
	}}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(winner).Build()
	r := &LogicalBackupRDBMSReconciler{
		Client:        c,
		APIReader:     c,
		EventRecorder: events.NewFakeRecorder(4),
		opts:          Options{RetryInterval: time.Second},
	}

	ours := ownJob(backup)
	ours.Name = winner.Name
	wait, err := r.claimJobName(context.Background(), backup, ours)
	require.NoError(t, err)
	assert.Equal(t, settle, wait)
	assert.Equal(t, v1.LogicalBackupFailed, backup.Status.Phase)
	assert.Contains(t, backup.Status.FailureMessage, "belongs to another backup")

	var survived batchv1.Job
	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(winner), &survived))
	assert.Equal(
		t,
		foreignUID,
		survived.Labels[components.BackupUIDLabel],
		"the winner's identity is untouched",
	)
	require.Len(t, survived.OwnerReferences, 1)
	assert.Equal(t, "someone", survived.OwnerReferences[0].Name, "the winner's owner is untouched")
}
