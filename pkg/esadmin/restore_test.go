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

package esadmin_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/konsole-is/camunda-operator/pkg/esadmin"
	"github.com/konsole-is/camunda-operator/pkg/esadmin/esadmintest"
)

// The resolution turns the patterns of the caller into the concrete names
// that the delete needs, sorted, and it reports nothing for a pattern that
// matches no index. An empty pattern list asks Elasticsearch nothing at all:
// the injected failure would answer any request that left the client.
func TestResolveIndicesReturnsTheSortedConcreteNames(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)
	server.SetIndices("camunda-record-2", "camunda-record-1", "operate-list", "tasklist-task")

	names, err := client.ResolveIndices(ctx, []string{"camunda-record*", "operate-list"})
	require.NoError(t, err)
	assert.Equal(t, []string{"camunda-record-1", "camunda-record-2", "operate-list"}, names)

	names, err = client.ResolveIndices(ctx, []string{"no-such-index*"})
	require.NoError(t, err)
	assert.Empty(t, names)

	server.FailNext("indexResolve", 1)
	names, err = client.ResolveIndices(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, names)
}

func TestResolveIndicesMapsBothErrorClasses(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)
	server.SetIndices("camunda-record-1")

	server.FailNext("indexResolve", 1)
	_, err := client.ResolveIndices(ctx, []string{"camunda-record*"})
	require.ErrorIs(t, err, esadmin.ErrRejected)
	assert.Contains(t, err.Error(), "injected index resolve failure")

	server.DropNext("indexResolve", 1)
	_, err = client.ResolveIndices(ctx, []string{"camunda-record*"})
	require.ErrorIs(t, err, esadmin.ErrUnreachable)

	_, err = client.ResolveIndices(ctx, []string{"camunda-record*"})
	assert.NoError(t, err, "one drop, then reachable again")
}

// The delete resolves the patterns first and then names the concrete indices,
// because Elasticsearch refuses a wildcard delete under its own default of
// action.destructive_requires_name. The resolved set goes in one request: a
// partial deletion leaves indices that the restore then collides with. An
// index that no pattern matches stays.
func TestDeleteIndicesResolvesThenDeletesInOneRequest(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)
	server.SetIndices("camunda-record-2", "camunda-record-1", "operate-list", "tasklist-task")

	require.NoError(t, client.DeleteIndices(ctx, []string{"camunda-record*", "operate-list"}))

	assert.Equal(t, []string{"camunda-record-1", "camunda-record-2", "operate-list"}, server.DeletedIndices())
	assert.Equal(t, 1, server.IndexDeleteCalls())
	assert.Equal(t, []string{"tasklist-task"}, server.Indices())
}

// A pattern that matches no index resolves to nothing, and an empty target
// would delete every index in the cluster. The client must send no delete at
// all. An empty pattern list asks nothing at all, resolution included.
func TestDeleteIndicesWithoutAMatchSendsNoDelete(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)
	server.SetIndices("tasklist-task")

	require.NoError(t, client.DeleteIndices(ctx, []string{"camunda-record*"}))
	assert.Equal(t, 0, server.IndexDeleteCalls())
	assert.Empty(t, server.DeletedIndices())

	server.FailNext("indexResolve", 1)
	require.NoError(t, client.DeleteIndices(ctx, nil))
	require.NoError(t, client.DeleteIndices(ctx, []string{}))

	assert.Equal(t, []string{"tasklist-task"}, server.Indices())
}

// Both calls of the flow map both error classes: the resolution that reads the
// names, and the delete that removes them.
func TestDeleteIndicesMapsBothErrorClasses(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)
	server.SetIndices("camunda-record-1")
	patterns := []string{"camunda-record*"}

	server.FailNext("indexResolve", 1)
	err := client.DeleteIndices(ctx, patterns)
	require.ErrorIs(t, err, esadmin.ErrRejected)
	assert.Contains(t, err.Error(), "injected index resolve failure")

	server.DropNext("indexResolve", 1)
	require.ErrorIs(t, client.DeleteIndices(ctx, patterns), esadmin.ErrUnreachable)

	server.FailNext("indexDelete", 1)
	err = client.DeleteIndices(ctx, patterns)
	require.ErrorIs(t, err, esadmin.ErrRejected)
	assert.Contains(t, err.Error(), "injected index delete failure")

	server.DropNext("indexDelete", 1)
	require.ErrorIs(t, client.DeleteIndices(ctx, patterns), esadmin.ErrUnreachable)

	require.NoError(t, client.DeleteIndices(ctx, patterns), "one drop, then reachable again")
	assert.Empty(t, server.Indices())
}

// The restore starts the snapshot and returns. A call that waited would hold
// the reconcile worker for the whole restore, so the state machine polls the
// recovery instead.
func TestRestoreSnapshotDoesNotWaitForCompletion(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)
	seedSnapshot(t, client, server, "repo", "records-42")

	indices := []string{"camunda-record-8.8.0_", "operate-list"}
	require.NoError(t, client.RestoreSnapshot(ctx, "repo", "records-42", indices))

	restores := server.RestoreRequests()
	require.Len(t, restores, 1)
	assert.Equal(t, "repo", restores[0].Repo)
	assert.Equal(t, "records-42", restores[0].Name)
	assert.Equal(t, indices, restores[0].Indices)
	assert.False(t, restores[0].WaitForCompletion)
}

// An empty index list is an empty pattern, which selects nothing. A restore
// that brings back no data must never report success.
func TestRestoreSnapshotRejectsAnEmptyIndexList(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)
	seedSnapshot(t, client, server, "repo", "records-42")

	err := client.RestoreSnapshot(ctx, "repo", "records-42", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "names no index")
	assert.Empty(t, server.RestoreRequests())
}

func TestRestoreSnapshotMapsBothErrorClasses(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)
	seedSnapshot(t, client, server, "repo", "records-42")

	server.FailNext("snapshotRestore", 1)
	err := client.RestoreSnapshot(ctx, "repo", "records-42", []string{"idx"})
	require.ErrorIs(t, err, esadmin.ErrRejected)
	assert.Contains(t, err.Error(), "injected snapshot restore failure")

	server.DropNext("snapshotRestore", 1)
	err = client.RestoreSnapshot(ctx, "repo", "records-42", []string{"idx"})
	require.ErrorIs(t, err, esadmin.ErrUnreachable)

	assert.NoError(
		t,
		client.RestoreSnapshot(ctx, "repo", "records-42", []string{"idx"}),
		"one drop, then reachable again",
	)
}

// The restore is complete when no shard of the restore set recovers any more.
// An empty pattern list asks about no index, and must not ask Elasticsearch
// about every index in the cluster: the injected failure proves that no
// request leaves the client.
func TestRestoreProgressFollowsTheRecovery(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)
	patterns := []string{"camunda-record*", "operate-list"}

	server.SetRecoveryActive(true)
	state, err := client.RestoreProgress(ctx, patterns)
	require.NoError(t, err)
	assert.Equal(t, esadmin.RestoreInProgress, state)

	server.SetRecoveryActive(false)
	state, err = client.RestoreProgress(ctx, patterns)
	require.NoError(t, err)
	assert.Equal(t, esadmin.RestoreDone, state)

	server.FailNext("recovery", 1)
	state, err = client.RestoreProgress(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, esadmin.RestoreDone, state)
}

func TestRestoreProgressMapsBothErrorClasses(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)
	patterns := []string{"camunda-record*"}

	server.FailNext("recovery", 1)
	_, err := client.RestoreProgress(ctx, patterns)
	require.ErrorIs(t, err, esadmin.ErrRejected)
	assert.Contains(t, err.Error(), "injected recovery failure")

	server.DropNext("recovery", 1)
	_, err = client.RestoreProgress(ctx, patterns)
	require.ErrorIs(t, err, esadmin.ErrUnreachable)

	_, err = client.RestoreProgress(ctx, patterns)
	assert.NoError(t, err, "one drop, then reachable again")
}

// seedSnapshot registers repo and puts a restorable snapshot in it, the state
// a restore starts from.
func seedSnapshot(t *testing.T, client *esadmin.Client, server *esadmintest.Server, repo, name string) {
	t.Helper()

	require.NoError(
		t,
		client.EnsureSnapshotRepository(
			context.Background(),
			repo,
			esadmin.RepositoryConfig{Type: esadmin.RepositoryTypeS3, Bucket: "b"},
		),
	)
	server.SetSnapshotState(repo, name, "SUCCESS")
}

// TestConcurrentCallsAndAccessors runs the restore calls of two clients
// against one fake while a test reads its accessors. The fake serves one
// request at a time under the lock of adminhttptest.Fake, and its accessors
// take the same lock, so neither side sees half-written state. Run the package
// with -race to see it fail if that ever changes.
func TestConcurrentCallsAndAccessors(t *testing.T) {
	t.Parallel()

	server := esadmintest.New()
	defer server.Close()

	server.SetIndices("camunda-1", "operate-1")
	server.SetSnapshotState("repo", "snap", "SUCCESS")

	client, err := esadmin.New(server.URL(), "user", "pass", nil)
	require.NoError(t, err)
	require.NoError(t, client.EnsureSnapshotRepository(t.Context(), "repo", esadmin.RepositoryConfig{
		Type: esadmin.RepositoryTypeS3, Bucket: "bucket",
	}))

	var group sync.WaitGroup
	for range 8 {
		group.Go(func() {
			_ = client.RestoreSnapshot(t.Context(), "repo", "snap", []string{"camunda-*"})
			_, _ = client.RestoreProgress(t.Context(), []string{"camunda-*"})
			_ = client.DeleteIndices(t.Context(), []string{"operate-*"})
			server.RestoreRequests()
			server.Indices()
			server.DeletedIndices()
		})
	}
	group.Wait()

	assert.NotEmpty(t, server.RestoreRequests())
}
