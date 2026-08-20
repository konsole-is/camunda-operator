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

package download

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
	backup "github.com/konsole-is/camunda-operator/pkg/components/logicalbackuprdbms"
	components "github.com/konsole-is/camunda-operator/pkg/components/logicalrestore"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
)

// dirBucket is a fileblob-backed bucket, so the round trip runs through the
// same gocloud reader path that the real stores use.
type dirBucket struct {
	dir    string
	creds  *objectstore.Credentials
	config *v1.ObjectStorageConfig
}

func (b *dirBucket) Download(ctx context.Context, key string, w io.Writer) error {
	bucket, err := fileblob.OpenBucket(b.dir, &fileblob.Options{CreateDir: true})
	if err != nil {
		return err
	}
	defer func() { _ = bucket.Close() }()

	r, err := bucket.NewReader(ctx, key, nil)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()

	_, err = io.Copy(w, r)

	return err
}

func (b *dirBucket) Close() {}

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

const spec = `{"type":"S3","s3":{"bucketName":"b","region":"r",` +
	`"auth":{"type":"credentials","credentials":{"secretRef":{"name":"s","namespace":"n",` +
	`"accessKeyIdKey":"id","secretAccessKeyKey":"key"}}}}}`

// writeObject puts the object into the fileblob directory that the fake
// bucket reads from.
func writeObject(t *testing.T, dir, key string, content []byte) {
	t.Helper()

	path := filepath.Join(dir, filepath.FromSlash(key))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, content, 0o600))
}

func TestRunRoundTripsTheObject(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	key := "clusters/ns/cluster/1/camunda.dump"
	writeObject(t, dir, key, []byte("dump-bytes"))

	// The Job mounts the scratch volume, so the directory of the file always
	// exists when the subcommand runs.
	target := filepath.Join(t.TempDir(), "camunda.dump")
	bucket := &dirBucket{dir: dir}
	open := func(
		_ context.Context, cfg *v1.ObjectStorageConfig, creds *objectstore.Credentials,
	) (downloader, error) {
		bucket.config, bucket.creds = cfg, creds

		return bucket, nil
	}

	err := run(
		context.Background(), env(map[string]string{
			components.EnvDownloadFile:             target,
			components.EnvDownloadKey:              key,
			backup.EnvUploadStorageName:            "my-backup-config",
			backup.EnvUploadStorageSpec:            spec,
			backup.EnvUploadCredentialKeys:         "id,key",
			backup.EnvUploadCredentialPrefix + "0": "id-value",
			backup.EnvUploadCredentialPrefix + "1": "key-value",
		}), open,
	)
	require.NoError(t, err)

	downloaded, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, []byte("dump-bytes"), downloaded)

	require.NotNil(t, bucket.creds)
	assert.Equal(t, "id-value", bucket.creds.AccessKeyID)
	assert.Equal(t, "key-value", bucket.creds.SecretAccessKey)
	assert.Equal(t, "my-backup-config", bucket.config.Name)
	require.NotNil(t, bucket.config.Spec.S3)
	assert.Equal(t, "b", bucket.config.Spec.S3.BucketName)
}

func TestRunRejectsAMissingFileOrKey(t *testing.T) {
	t.Parallel()

	err := run(context.Background(), env(nil), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), components.EnvDownloadFile)
	assert.Contains(t, err.Error(), components.EnvDownloadKey)
}

func TestRunRejectsAProjectedKeyWithoutAValue(t *testing.T) {
	t.Parallel()

	err := run(
		context.Background(), env(map[string]string{
			components.EnvDownloadFile:             filepath.Join(t.TempDir(), "camunda.dump"),
			components.EnvDownloadKey:              "k",
			backup.EnvUploadStorageSpec:            spec,
			backup.EnvUploadCredentialKeys:         "id,key",
			backup.EnvUploadCredentialPrefix + "0": "id-value",
			// UPLOAD_CREDENTIAL_1 is deliberately absent.
		}), nil,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "without a value")
}

// TestRunLeavesNoFileWhenTheDownloadFails proves that a broken transfer does
// not leave a partial archive behind. pg_restore of a truncated file fails
// late, so the Job must fail early instead.
func TestRunLeavesNoFileWhenTheDownloadFails(t *testing.T) {
	t.Parallel()

	target := filepath.Join(t.TempDir(), "camunda.dump")
	open := func(
		context.Context, *v1.ObjectStorageConfig, *objectstore.Credentials,
	) (downloader, error) {
		return failingBucket{}, nil
	}

	err := run(
		context.Background(), env(map[string]string{
			components.EnvDownloadFile:  target,
			components.EnvDownloadKey:   "k",
			backup.EnvUploadStorageSpec: spec,
		}), open,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "downloading")

	_, statErr := os.Stat(target)
	assert.True(t, os.IsNotExist(statErr), "a failed download leaves no file")
}

type failingBucket struct{}

func (failingBucket) Download(context.Context, string, io.Writer) error {
	return errors.New("connection reset")
}

func (failingBucket) Close() {}
