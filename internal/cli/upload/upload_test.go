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

package upload

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gocloud.dev/blob/fileblob"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/logicalbackuprdbms"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
)

// dirBucket is a fileblob-backed bucket, so the round trip runs through the
// same gocloud writer path that the real stores use.
type dirBucket struct {
	dir    string
	creds  *objectstore.Credentials
	config *v1.ObjectStorageConfig
}

func (b *dirBucket) Upload(ctx context.Context, key string, r io.Reader) error {
	bucket, err := fileblob.OpenBucket(b.dir, &fileblob.Options{CreateDir: true})
	if err != nil {
		return err
	}
	defer func() { _ = bucket.Close() }()

	w, err := bucket.NewWriter(ctx, key, nil)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, r); err != nil {
		_ = w.Close()

		return err
	}

	return w.Close()
}

func (b *dirBucket) Close() {}

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

const spec = `{"type":"S3","s3":{"bucketName":"b","region":"r",` +
	`"auth":{"type":"credentials","credentials":{"secretRef":{"name":"s","namespace":"n",` +
	`"accessKeyIdKey":"id","secretAccessKeyKey":"key"}}}}}`

func TestRunRoundTripsTheFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	source := filepath.Join(dir, "camunda.dump")
	require.NoError(t, os.WriteFile(source, []byte("dump-bytes"), 0o600))

	bucket := &dirBucket{dir: dir}
	open := func(
		_ context.Context, cfg *v1.ObjectStorageConfig, creds *objectstore.Credentials,
	) (uploader, error) {
		bucket.config, bucket.creds = cfg, creds

		return bucket, nil
	}

	err := run(
		context.Background(), env(map[string]string{
			components.EnvUploadFile:                   source,
			components.EnvUploadKey:                    "clusters/ns/cluster/1/camunda.dump",
			components.EnvUploadStorageName:            "my-backup-config",
			components.EnvUploadStorageSpec:            spec,
			components.EnvUploadCredentialKeys:         "id,key",
			components.EnvUploadCredentialPrefix + "0": "id-value",
			components.EnvUploadCredentialPrefix + "1": "key-value",
		}), open,
	)
	require.NoError(t, err)

	uploaded, err := os.ReadFile(
		filepath.Join(dir, filepath.FromSlash("clusters/ns/cluster/1/camunda.dump")),
	)
	require.NoError(t, err)
	assert.Equal(t, []byte("dump-bytes"), uploaded)

	require.NotNil(t, bucket.creds)
	assert.Equal(t, "id-value", bucket.creds.AccessKeyID)
	assert.Equal(t, "key-value", bucket.creds.SecretAccessKey)
	assert.Equal(t, "my-backup-config", bucket.config.Name)
	require.NotNil(t, bucket.config.Spec.S3)
	assert.Equal(t, "b", bucket.config.Spec.S3.BucketName)
}

const gcsSpec = `{"type":"GCS","gcs":{"bucketName":"b",` +
	`"auth":{"type":"credentials","credentials":{"secretRef":{"name":"s","namespace":"n",` +
	`"key":"key.json"}}}}}`

// TestRunResolvesGCSStaticKey proves that the projection carries a whole
// service account key file, not only flat string pairs. GCS is the storage
// type that the old per-type mapping did not serve.
func TestRunResolvesGCSStaticKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	source := filepath.Join(dir, "camunda.dump")
	require.NoError(t, os.WriteFile(source, []byte("dump-bytes"), 0o600))

	keyFile := `{"type":"service_account","project_id":"p"}`
	bucket := &dirBucket{dir: dir}
	open := func(
		_ context.Context, cfg *v1.ObjectStorageConfig, creds *objectstore.Credentials,
	) (uploader, error) {
		bucket.config, bucket.creds = cfg, creds

		return bucket, nil
	}

	err := run(
		context.Background(), env(map[string]string{
			components.EnvUploadFile:                   source,
			components.EnvUploadKey:                    "k",
			components.EnvUploadStorageName:            "my-backup-config",
			components.EnvUploadStorageSpec:            gcsSpec,
			components.EnvUploadCredentialKeys:         "key.json",
			components.EnvUploadCredentialPrefix + "0": keyFile,
		}), open,
	)
	require.NoError(t, err)

	require.NotNil(t, bucket.creds)
	assert.Equal(t, keyFile, string(bucket.creds.ServiceAccountJSON))
}

func TestRunRejectsAProjectedKeyWithoutAValue(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	source := filepath.Join(dir, "camunda.dump")
	require.NoError(t, os.WriteFile(source, []byte("dump-bytes"), 0o600))

	err := run(
		context.Background(), env(map[string]string{
			components.EnvUploadFile:                   source,
			components.EnvUploadKey:                    "k",
			components.EnvUploadStorageSpec:            spec,
			components.EnvUploadCredentialKeys:         "id,key",
			components.EnvUploadCredentialPrefix + "0": "id-value",
			// UPLOAD_CREDENTIAL_1 is deliberately absent.
		}), nil,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "without a value")
}

func TestRunRejectsAMissingFileOrKey(t *testing.T) {
	t.Parallel()

	err := run(context.Background(), env(nil), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), components.EnvUploadFile)
}

func TestRunSurfacesAnUploadFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	source := filepath.Join(dir, "camunda.dump")
	require.NoError(t, os.WriteFile(source, []byte("dump-bytes"), 0o600))

	open := func(
		context.Context, *v1.ObjectStorageConfig, *objectstore.Credentials,
	) (uploader, error) {
		return failingBucket{}, nil
	}

	err := run(
		context.Background(), env(map[string]string{
			components.EnvUploadFile:        source,
			components.EnvUploadKey:         "k",
			components.EnvUploadStorageSpec: spec,
		}), open,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uploading the dump")
}

type failingBucket struct{}

func (failingBucket) Upload(context.Context, string, io.Reader) error {
	return errors.New("connection reset")
}

func (failingBucket) Close() {}
