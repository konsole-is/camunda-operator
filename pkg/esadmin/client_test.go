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

	cfg := esadmin.RepositoryConfig{
		Type:            esadmin.RepositoryTypeS3,
		Bucket:          "camunda-backups",
		BasePath:        "clusters/ns/name",
		Endpoint:        "http://minio.minio.svc:9000",
		Region:          "eu-west-1",
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
	assert.Equal(t, "eu-west-1", repo.Settings["region"])
	assert.Equal(t, true, repo.Settings["path_style_access"])
	assert.Equal(t, "default", repo.Settings["client"])
}

// A repository registered without a region carries no region setting, so the
// node keeps whatever its own client settings resolve. The caller decides
// what an endpoint without a region signs for; this client never invents a
// value of its own.
func TestEnsureSnapshotRepositoryOmitsAnEmptyRegion(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)

	require.NoError(t, client.EnsureSnapshotRepository(ctx, "my-cluster", esadmin.RepositoryConfig{
		Type:     esadmin.RepositoryTypeS3,
		Bucket:   "camunda-backups",
		BasePath: "clusters/ns/name",
	}))

	repo := server.Repository("my-cluster")
	require.NotNil(t, repo)
	assert.NotContains(t, repo.Settings, "region")
}

func TestCreateSnapshotIsIdempotent(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)
	require.NoError(
		t,
		client.EnsureSnapshotRepository(
			ctx,
			"repo",
			esadmin.RepositoryConfig{Type: esadmin.RepositoryTypeS3, Bucket: "b"},
		),
	)

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
	require.NoError(
		t,
		client.EnsureSnapshotRepository(
			ctx,
			"repo",
			esadmin.RepositoryConfig{Type: esadmin.RepositoryTypeS3, Bucket: "b"},
		),
	)
	// A snapshot that another actor created: no metadata.
	server.SetSnapshotState("repo", "records-42", "SUCCESS")

	err := client.CreateSnapshot(
		ctx, "repo", "records-42", []string{"idx"},
		map[string]string{"camunda-operator/backup-uid": "uid-42"},
	)
	require.ErrorIs(t, err, esadmin.ErrRejected)
	assert.Equal(t, 0, server.SnapshotCreates("repo", "records-42"))
}

// The ownership evidence of the duplicate rule is the metadata. A caller
// that carries none has no evidence, so an existing metadata-free snapshot
// under the name is not adopted: sameMetadata(nil, nil) is true, and the
// rule must not rest on it. The duplicate rejection reaches the caller.
func TestCreateSnapshotWithoutMetadataDoesNotResolveADuplicate(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)
	require.NoError(
		t,
		client.EnsureSnapshotRepository(
			ctx,
			"repo",
			esadmin.RepositoryConfig{Type: esadmin.RepositoryTypeS3, Bucket: "b"},
		),
	)
	// A snapshot that another actor created: no metadata.
	server.SetSnapshotState("repo", "records-42", "SUCCESS")

	err := client.CreateSnapshot(ctx, "repo", "records-42", []string{"idx"}, nil)
	require.ErrorIs(t, err, esadmin.ErrRejected)
	assert.Equal(t, 0, server.SnapshotCreates("repo", "records-42"))
}

// The metadata travels with the snapshot. The creation carries it, and the
// status returns it. A caller can therefore tell its own snapshot from a
// foreign one under the same name.
func TestSnapshotMetadataRoundTrips(t *testing.T) {
	ctx := context.Background()
	client, _ := newClient(t)
	require.NoError(
		t,
		client.EnsureSnapshotRepository(
			ctx,
			"repo",
			esadmin.RepositoryConfig{Type: esadmin.RepositoryTypeS3, Bucket: "b"},
		),
	)
	metadata := map[string]string{"camunda-operator/backup-uid": "uid-42"}

	require.NoError(t, client.CreateSnapshot(ctx, "repo", "records-42", []string{"idx"}, metadata))

	snapshot, err := client.SnapshotStatus(ctx, "repo", "records-42")
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"camunda-operator/backup-uid": "uid-42"}, snapshot.Metadata)
	uid, ok := snapshot.MetadataString("camunda-operator/backup-uid")
	assert.True(t, ok)
	assert.Equal(t, "uid-42", uid)

	require.NoError(t, client.CreateSnapshot(ctx, "repo", "plain", []string{"idx"}, nil))
	plain, err := client.SnapshotStatus(ctx, "repo", "plain")
	require.NoError(t, err)
	assert.Empty(t, plain.Metadata)
}

// Elasticsearch accepts any JSON value as snapshot metadata. A snapshot that
// another actor created can carry numbers, lists, or objects under the
// deterministic name; the status must still decode, so a caller can see the
// snapshot is not its own and leave it alone instead of failing on it forever.
func TestSnapshotStatusDecodesForeignMetadataOfAnyType(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)
	require.NoError(
		t,
		client.EnsureSnapshotRepository(
			ctx,
			"repo",
			esadmin.RepositoryConfig{Type: esadmin.RepositoryTypeS3, Bucket: "b"},
		),
	)
	server.SetSnapshotMetadata("repo", "records-42", map[string]any{
		"retention-days":              float64(30),
		"created-by":                  map[string]any{"tool": "curator", "tags": []any{"nightly"}},
		"camunda-operator/backup-uid": float64(42),
	})

	snapshot, err := client.SnapshotStatus(ctx, "repo", "records-42")
	require.NoError(t, err)
	assert.Equal(t, esadmin.SnapshotSuccess, snapshot.State)
	assert.Equal(t, float64(30), snapshot.Metadata["retention-days"])
	_, ok := snapshot.MetadataString("camunda-operator/backup-uid")
	assert.False(t, ok, "a non-string value under the ownership key is not an owner")
	_, ok = snapshot.MetadataString("absent")
	assert.False(t, ok)

	// The duplicate resolution does not read a same-length metadata map of
	// other types as its own either.
	err = client.CreateSnapshot(
		ctx, "repo", "records-42", []string{"idx"},
		map[string]string{"a": "1", "b": "2", "camunda-operator/backup-uid": "42"},
	)
	require.ErrorIs(t, err, esadmin.ErrRejected)
	assert.Equal(t, 0, server.SnapshotCreates("repo", "records-42"))
}

func TestSnapshotStatus(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)
	require.NoError(
		t,
		client.EnsureSnapshotRepository(
			ctx,
			"repo",
			esadmin.RepositoryConfig{Type: esadmin.RepositoryTypeS3, Bucket: "b"},
		),
	)

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
	require.NoError(
		t,
		client.EnsureSnapshotRepository(
			ctx,
			"repo",
			esadmin.RepositoryConfig{Type: esadmin.RepositoryTypeS3, Bucket: "b"},
		),
	)

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

	require.NoError(
		t,
		client.EnsureSnapshotRepository(
			ctx,
			"repo",
			esadmin.RepositoryConfig{Type: esadmin.RepositoryTypeS3, Bucket: "b"},
		),
	)
	assert.NotNil(t, server.Repository("repo"))
}

// A repository name from a hand-written contract is user input; a slash in it
// must not retarget the request to another API path.
func TestPathSegmentsAreEscaped(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)

	err := client.EnsureSnapshotRepository(
		ctx,
		"prod/main",
		esadmin.RepositoryConfig{Type: esadmin.RepositoryTypeS3, Bucket: "b"},
	)
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
	err := client.EnsureSnapshotRepository(
		ctx,
		"repo",
		esadmin.RepositoryConfig{Type: esadmin.RepositoryTypeS3, Bucket: "b"},
	)
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

// DropNext covers every operation FailNext names, not only the snapshot
// create: a fake that is unreachable for one operation and reachable for
// another must be able to say so for any of them.
func TestDropNextReachesEveryOperation(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)
	require.NoError(
		t,
		client.EnsureSnapshotRepository(
			ctx,
			"repo",
			esadmin.RepositoryConfig{Type: esadmin.RepositoryTypeS3, Bucket: "b"},
		),
	)

	server.DropNext("stats", 1)
	_, _, err := client.MaxNodeFSTotalAndUsedBytes(ctx)
	require.ErrorIs(t, err, esadmin.ErrUnreachable)
	_, _, err = client.MaxNodeFSTotalAndUsedBytes(ctx)
	require.NoError(t, err, "one drop, then reachable again")

	server.DropNext("snapshotStatus", 1)
	_, err = client.SnapshotStatus(ctx, "repo", "absent")
	require.ErrorIs(t, err, esadmin.ErrUnreachable)

	server.DropNext("snapshotDelete", 1)
	require.ErrorIs(t, client.DeleteSnapshot(ctx, "repo", "absent"), esadmin.ErrUnreachable)

	server.DropNext("reload", 1)
	require.ErrorIs(t, client.ReloadSecureSettings(ctx), esadmin.ErrUnreachable)

	server.DropNext("repository", 1)
	require.ErrorIs(
		t,
		client.EnsureSnapshotRepository(
			ctx,
			"repo",
			esadmin.RepositoryConfig{Type: esadmin.RepositoryTypeS3, Bucket: "b"},
		),
		esadmin.ErrUnreachable,
	)
}

// The gcs repository names its bucket and its client; the credentials reach
// the nodes through the keystore alone, so no setting carries them. The
// region is a setting of the s3 type alone, and a gcs repository drops one
// that the config carries.
func TestEnsureSnapshotRepositoryRegistersGCS(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)

	cfg := esadmin.RepositoryConfig{
		Type:     esadmin.RepositoryTypeGCS,
		Bucket:   "camunda-backups",
		BasePath: "clusters/ns/name",
		Region:   "eu-west-1",
	}

	require.NoError(t, client.EnsureSnapshotRepository(ctx, "my-cluster", cfg))

	repo := server.Repository("my-cluster")
	require.NotNil(t, repo)
	assert.Equal(t, "gcs", repo.Type)
	assert.Equal(t, "camunda-backups", repo.Settings["bucket"])
	assert.Equal(t, "clusters/ns/name", repo.Settings["base_path"])
	assert.Equal(t, "default", repo.Settings["client"])
	assert.NotContains(t, repo.Settings, "region")
}

// The azure repository names its blob container under the container setting,
// not bucket, and takes neither of the s3 endpoint settings: the service
// endpoint of an azure client is node configuration, not repository
// settings. It takes no region either.
func TestEnsureSnapshotRepositoryRegistersAzure(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)

	cfg := esadmin.RepositoryConfig{
		Type:     esadmin.RepositoryTypeAzure,
		Bucket:   "camunda-backups",
		BasePath: "clusters/ns/name",
		Region:   "eu-west-1",
	}

	require.NoError(t, client.EnsureSnapshotRepository(ctx, "my-cluster", cfg))

	repo := server.Repository("my-cluster")
	require.NotNil(t, repo)
	assert.Equal(t, "azure", repo.Type)
	assert.Equal(t, "camunda-backups", repo.Settings["container"])
	assert.Equal(t, "clusters/ns/name", repo.Settings["base_path"])
	assert.Equal(t, "default", repo.Settings["client"])
	assert.NotContains(t, repo.Settings, "bucket")
	assert.NotContains(t, repo.Settings, "region")
}

// The type and the bucket are what every repository needs, and the caller
// supplies both. A config that carries neither reaches Elasticsearch as a
// registration of type "", which it rejects with a message about the request
// body rather than about the caller. Rejecting it here names the real fault.
func TestEnsureSnapshotRepositoryRejectsAnIncompleteConfig(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)

	for name, cfg := range map[string]esadmin.RepositoryConfig{
		"no type":   {Bucket: "camunda-backups"},
		"no bucket": {Type: esadmin.RepositoryTypeS3},
	} {
		t.Run(name, func(t *testing.T) {
			require.Error(t, client.EnsureSnapshotRepository(ctx, "my-cluster", cfg))
			assert.Nil(t, server.Repository("my-cluster"), "nothing reaches Elasticsearch")
		})
	}
}

// The message names the field as the repository type names it. An azure
// repository has a container, and telling its operator that a "bucket" is
// empty sends them looking for a field that its settings do not have.
func TestEnsureSnapshotRepositoryNamesTheEmptyFieldPerType(t *testing.T) {
	ctx := context.Background()
	client, _ := newClient(t)

	err := client.EnsureSnapshotRepository(ctx, "my-cluster", esadmin.RepositoryConfig{
		Type: esadmin.RepositoryTypeAzure,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "container")

	err = client.EnsureSnapshotRepository(ctx, "my-cluster", esadmin.RepositoryConfig{
		Type: esadmin.RepositoryTypeS3,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bucket")
}
