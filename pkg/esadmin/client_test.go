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
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

	client, err := esadmin.New(server.URL(), "camunda", "secret", nil)
	require.NoError(t, err)

	return client, server
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
	require.NoError(t, client.EnsureSnapshotRepository(ctx, "repo", esadmin.S3RepositoryConfig{Bucket: "b"}))

	indices := []string{"camunda_zeebe_records_backup_42"}
	metadata := map[string]string{"camunda-operator/backup-uid": "uid-42"}
	require.NoError(t, client.CreateSnapshot(ctx, "repo", "records-42", indices, metadata))

	// A second create hits the duplicate rejection, and the client resolves
	// it through the status: the same metadata, so it is its own snapshot.
	require.NoError(t, client.CreateSnapshot(ctx, "repo", "records-42", indices, metadata))
	assert.Equal(t, 1, server.SnapshotCreates("repo", "records-42"))
}

// The duplicate resolution is bounded by ownership. A snapshot under the
// same name with other metadata belongs to another actor. The duplicate
// rejection must reach the caller instead of reading as success.
func TestCreateSnapshotDoesNotResolveAForeignDuplicate(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)
	require.NoError(t, client.EnsureSnapshotRepository(ctx, "repo", esadmin.S3RepositoryConfig{Bucket: "b"}))
	// A snapshot that another actor created: no metadata.
	server.SetSnapshotState("repo", "records-42", "SUCCESS")

	err := client.CreateSnapshot(
		ctx, "repo", "records-42", []string{"idx"},
		map[string]string{"camunda-operator/backup-uid": "uid-42"},
	)
	require.ErrorIs(t, err, esadmin.ErrRejected)
	assert.Equal(t, 0, server.SnapshotCreates("repo", "records-42"))
}

// The metadata travels with the snapshot. The creation carries it, and the
// status returns it. A caller can therefore tell its own snapshot from a
// foreign one under the same name.
func TestSnapshotMetadataRoundTrips(t *testing.T) {
	ctx := context.Background()
	client, _ := newClient(t)
	require.NoError(t, client.EnsureSnapshotRepository(ctx, "repo", esadmin.S3RepositoryConfig{Bucket: "b"}))
	metadata := map[string]string{"camunda-operator/backup-uid": "uid-42"}

	require.NoError(t, client.CreateSnapshot(ctx, "repo", "records-42", []string{"idx"}, metadata))

	snapshot, err := client.SnapshotStatus(ctx, "repo", "records-42")
	require.NoError(t, err)
	assert.Equal(t, metadata, snapshot.Metadata)

	require.NoError(t, client.CreateSnapshot(ctx, "repo", "plain", []string{"idx"}, nil))
	plain, err := client.SnapshotStatus(ctx, "repo", "plain")
	require.NoError(t, err)
	assert.Empty(t, plain.Metadata)
}

func TestSnapshotStatus(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)
	require.NoError(t, client.EnsureSnapshotRepository(ctx, "repo", esadmin.S3RepositoryConfig{Bucket: "b"}))

	snapshot, err := client.SnapshotStatus(ctx, "repo", "absent")
	require.NoError(t, err)
	assert.Equal(t, esadmin.SnapshotMissing, snapshot.State)

	require.NoError(t, client.CreateSnapshot(ctx, "repo", "records-42", []string{"idx"}, nil))

	snapshot, err = client.SnapshotStatus(ctx, "repo", "records-42")
	require.NoError(t, err)
	assert.Equal(t, esadmin.SnapshotInProgress, snapshot.State)

	server.SetSnapshotState("repo", "records-42", "SUCCESS")

	snapshot, err = client.SnapshotStatus(ctx, "repo", "records-42")
	require.NoError(t, err)
	assert.Equal(t, esadmin.SnapshotSuccess, snapshot.State)
}

func TestDeleteSnapshotIsIdempotent(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)
	require.NoError(t, client.EnsureSnapshotRepository(ctx, "repo", esadmin.S3RepositoryConfig{Bucket: "b"}))

	require.NoError(t, client.CreateSnapshot(ctx, "repo", "records-42", []string{"idx"}, nil))
	require.NoError(t, client.DeleteSnapshot(ctx, "repo", "records-42"))
	assert.False(t, server.SnapshotExists("repo", "records-42"))

	// Deleting an absent snapshot is success for the re-entrant finalizer.
	assert.NoError(t, client.DeleteSnapshot(ctx, "repo", "records-42"))
}

// A missing repository is not a missing snapshot. Both answer 404, but a
// dropped repository read as "snapshot gone" would let a finalizer release
// with the artifacts still in the bucket.
func TestMissingRepositoryIsAnErrorNotAMissingSnapshot(t *testing.T) {
	ctx := context.Background()
	client, _ := newClient(t)

	_, err := client.SnapshotStatus(ctx, "no-such-repo", "records-42")
	require.ErrorIs(t, err, esadmin.ErrRejected)
	assert.Contains(t, err.Error(), "repository_missing_exception")

	err = client.DeleteSnapshot(ctx, "no-such-repo", "records-42")
	require.ErrorIs(t, err, esadmin.ErrRejected)
	assert.Contains(t, err.Error(), "repository_missing_exception")
}

// A CA that is present but empty means the caller read the wrong Secret key;
// falling back to the system pool would hide that as a TLS failure on every
// later call.
func TestNewRejectsAPresentButEmptyCA(t *testing.T) {
	t.Parallel()

	_, err := esadmin.New("https://es:9200", "u", "p", []byte{})
	require.ErrorContains(t, err, "empty")

	_, err = esadmin.New("https://es:9200", "u", "p", []byte("not a certificate"))
	require.ErrorContains(t, err, "no PEM certificate")
}

// The TLS fake serves the self-signed certificate that CertificatePEM
// returns, so the CA path of New is exercised the way production runs it.
func TestClientVerifiesTheFakesTLSCertificate(t *testing.T) {
	ctx := context.Background()
	server := esadmintest.NewTLS()
	t.Cleanup(server.Close)

	client, err := esadmin.New(server.URL(), "camunda", "secret", server.CertificatePEM())
	require.NoError(t, err)

	require.NoError(t, client.EnsureSnapshotRepository(ctx, "repo", esadmin.S3RepositoryConfig{Bucket: "b"}))
	assert.NotNil(t, server.Repository("repo"))
}

// A repository name from a hand-written contract is user input; a slash in it
// must not retarget the request to another API path.
func TestPathSegmentsAreEscaped(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)

	err := client.EnsureSnapshotRepository(ctx, "prod/main", esadmin.S3RepositoryConfig{Bucket: "b"})
	require.NoError(t, err)
	assert.NotNil(t, server.Repository("prod/main"), "the name must arrive as one segment")
	assert.Nil(t, server.Repository("main"))
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

	unreachable, err := esadmin.New("http://127.0.0.1:1", "", "", nil)
	require.NoError(t, err)
	err = unreachable.ReloadSecureSettings(ctx)
	require.ErrorIs(t, err, esadmin.ErrUnreachable)
}

// An unusable CA must fail where the mistake is, not as an opaque TLS error
// on every later call: an empty pool trusts nothing.
func TestNewRejectsAnUnusableCABundle(t *testing.T) {
	_, err := esadmin.New("https://elasticsearch:9200", "camunda", "secret", []byte("not a certificate"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "PEM certificate")
}

// An empty index list is an empty pattern, which selects nothing. A snapshot
// that holds no data must never report success.
func TestCreateSnapshotRejectsAnEmptyIndexList(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)

	err := client.CreateSnapshot(ctx, "repo", "records-42", nil, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "names no index")
	assert.False(t, server.SnapshotExists("repo", "records-42"))
}

// A rejection carries the response body so the operator can read why. A body
// that is far larger than a condition allows must not travel whole into the
// error, or every status flush that carries it is refused.
func TestRejectedErrorBoundsTheBody(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(bytes.Repeat([]byte("x"), 100_000))
	}))
	t.Cleanup(server.Close)
	client, err := esadmin.New(server.URL, "camunda", "secret", nil)
	require.NoError(t, err)

	err = client.ReloadSecureSettings(ctx)
	require.ErrorIs(t, err, esadmin.ErrRejected)
	assert.Less(t, len(err.Error()), 2_000)
	assert.Contains(t, err.Error(), "(truncated, 100000 bytes)")
}

func TestRejectedErrorKeepsASmallBodyWhole(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("  repository is missing  "))
	}))
	t.Cleanup(server.Close)
	client, err := esadmin.New(server.URL, "camunda", "secret", nil)
	require.NoError(t, err)

	err = client.ReloadSecureSettings(ctx)
	require.ErrorIs(t, err, esadmin.ErrRejected)
	assert.True(t, strings.HasSuffix(err.Error(), "returned 500: repository is missing"), err.Error())
	assert.NotContains(t, err.Error(), "truncated")
}
