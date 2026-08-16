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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/konsole-is/camunda-operator/pkg/esadmin"
	"github.com/konsole-is/camunda-operator/pkg/esadmin/esadmintest"
)

// newClient builds a client against a fresh fake.
func newClient(t *testing.T) (*esadmin.Client, *esadmintest.Server) {
	t.Helper()

	server := esadmintest.New()
	t.Cleanup(server.Close)

	return esadmin.New(server.URL(), "camunda", "secret", nil), server
}

func TestEnsureSnapshotRepositoryConverges(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)

	cfg := esadmin.S3RepositoryConfig{
		Bucket:          "camunda-backups",
		BasePath:        "clusters/ns/name",
		Endpoint:        "http://minio.minio.svc:9000",
		PathStyleAccess: true,
	}

	require.NoError(t, client.EnsureSnapshotRepository(ctx, "my-cluster", cfg))
	// The PUT is idempotent: a second registration converges in place.
	require.NoError(t, client.EnsureSnapshotRepository(ctx, "my-cluster", cfg))

	repo := server.Repository("my-cluster")
	require.NotNil(t, repo)
	assert.Equal(t, "s3", repo.Type)
	assert.Equal(t, "camunda-backups", repo.Settings["bucket"])
	assert.Equal(t, "clusters/ns/name", repo.Settings["base_path"])
	assert.Equal(t, "http://minio.minio.svc:9000", repo.Settings["endpoint"])
	assert.Equal(t, true, repo.Settings["path_style_access"])
	assert.Equal(t, "default", repo.Settings["client"])
}

func TestCreateSnapshotIsIdempotent(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)

	indices := []string{"camunda_zeebe_records_backup_42"}
	require.NoError(t, client.CreateSnapshot(ctx, "repo", "records-42", indices))

	// A second create hits the duplicate rejection, and the client resolves
	// it through the status: success.
	require.NoError(t, client.CreateSnapshot(ctx, "repo", "records-42", indices))
	assert.Equal(t, 1, server.SnapshotCreates("repo", "records-42"))
}

func TestSnapshotStatus(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)

	state, err := client.SnapshotStatus(ctx, "repo", "absent")
	require.NoError(t, err)
	assert.Equal(t, esadmin.SnapshotMissing, state)

	require.NoError(t, client.CreateSnapshot(ctx, "repo", "records-42", []string{"idx"}))

	state, err = client.SnapshotStatus(ctx, "repo", "records-42")
	require.NoError(t, err)
	assert.Equal(t, esadmin.SnapshotInProgress, state)

	server.SetSnapshotState("repo", "records-42", "SUCCESS")

	state, err = client.SnapshotStatus(ctx, "repo", "records-42")
	require.NoError(t, err)
	assert.Equal(t, esadmin.SnapshotSuccess, state)
}

func TestDeleteSnapshotIsIdempotent(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)

	require.NoError(t, client.CreateSnapshot(ctx, "repo", "records-42", []string{"idx"}))
	require.NoError(t, client.DeleteSnapshot(ctx, "repo", "records-42"))
	assert.False(t, server.SnapshotExists("repo", "records-42"))

	// Deleting an absent snapshot is success for the re-entrant finalizer.
	assert.NoError(t, client.DeleteSnapshot(ctx, "repo", "records-42"))
}

func TestReloadSecureSettings(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)

	require.NoError(t, client.ReloadSecureSettings(ctx))
	assert.Equal(t, 1, server.ReloadCalls())
}

func TestMaxNodeFSTotalAndUsedBytes(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)

	server.SetNodeFS("node-0", 200<<30, 60<<30)

	total, used, err := client.MaxNodeFSTotalAndUsedBytes(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(200<<30), total)
	assert.Equal(t, int64(60<<30), used)
}

// The two values are independent worst cases: a cluster with uneven nodes
// reports the largest total and the largest used even when they come from
// different nodes.
func TestMaxNodeFSTotalAndUsedBytesAcrossNodes(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)

	server.SetNodeFS("node-0", 200<<30, 10<<30)
	server.SetNodeFS("node-1", 100<<30, 80<<30)

	total, used, err := client.MaxNodeFSTotalAndUsedBytes(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(200<<30), total)
	assert.Equal(t, int64(80<<30), used)
}

func TestErrorClasses(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)

	server.FailNext("repository", 1)
	err := client.EnsureSnapshotRepository(ctx, "repo", esadmin.S3RepositoryConfig{Bucket: "b"})
	require.ErrorIs(t, err, esadmin.ErrRejected)
	assert.Contains(t, err.Error(), "injected repository failure")

	unreachable := esadmin.New("http://127.0.0.1:1", "", "", nil)
	err = unreachable.ReloadSecureSettings(ctx)
	require.ErrorIs(t, err, esadmin.ErrUnreachable)
}
