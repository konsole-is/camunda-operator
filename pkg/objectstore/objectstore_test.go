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

package objectstore

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gocloud.dev/blob/fileblob"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// fileBucket returns a Bucket backed by a temporary directory.
func fileBucket(t *testing.T) *Bucket {
	t.Helper()

	raw, err := fileblob.OpenBucket(t.TempDir(), nil)
	require.NoError(t, err)

	bucket := newBucket(raw)
	t.Cleanup(bucket.Close)

	return bucket
}

func TestBucketRoundTrip(t *testing.T) {
	ctx := context.Background()
	bucket := fileBucket(t)

	require.NoError(t, bucket.Upload(ctx, "clusters/ns/name/1/camunda.dump", strings.NewReader("dump-bytes")))
	require.NoError(t, bucket.Upload(ctx, "clusters/ns/name/2/camunda.dump", strings.NewReader("other")))

	keys, err := bucket.List(ctx, "clusters/ns/name/1/")
	require.NoError(t, err)
	assert.Equal(t, []string{"clusters/ns/name/1/camunda.dump"}, keys)

	require.NoError(t, bucket.Delete(ctx, "clusters/ns/name/1/camunda.dump"))

	keys, err = bucket.List(ctx, "clusters/ns/name/1/")
	require.NoError(t, err)
	assert.Empty(t, keys)

	keys, err = bucket.List(ctx, "clusters/ns/name/")
	require.NoError(t, err)
	assert.Equal(t, []string{"clusters/ns/name/2/camunda.dump"}, keys)
}

func TestBucketDeleteIsIdempotent(t *testing.T) {
	ctx := context.Background()
	bucket := fileBucket(t)

	assert.NoError(t, bucket.Delete(ctx, "absent-key"))
}

// failingReader yields some bytes and then fails, the shape of a pg_dump pipe
// that breaks partway or a pod that is evicted mid-upload.
type failingReader struct {
	remaining int
}

func (r *failingReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, errors.New("dump stream broke")
	}

	n := min(len(p), r.remaining)
	r.remaining -= n
	for i := range n {
		p[i] = 'x'
	}

	return n, nil
}

// A failed upload must leave no object behind. Closing the writer commits
// whatever was written, so a truncated dump would stay in the bucket and a
// later restore would read it as a whole one.
func TestUploadLeavesNoObjectWhenTheReaderFails(t *testing.T) {
	ctx := context.Background()
	bucket := fileBucket(t)

	err := bucket.Upload(ctx, "clusters/ns/name/1/camunda.dump", &failingReader{remaining: 4096})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "dump stream broke")

	keys, listErr := bucket.List(ctx, "clusters/ns/name/1/")
	require.NoError(t, listErr)
	assert.Empty(t, keys, "a partial upload must not be committed")
}

func TestOpenRejectsIncompleteContracts(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		cfg   *v1.ObjectStorageConfig
		creds *Credentials
		want  string
	}{
		{
			name: "credentials auth without resolved credentials",
			cfg: &v1.ObjectStorageConfig{
				Spec: v1.ObjectStorageConfigSpec{
					Type: v1.ObjectStorageTypeS3,
					S3: &v1.S3Storage{
						BucketName: "bucket",
						Endpoint:   "http://minio:9000",
						Auth: v1.S3StorageAuth{
							Type:        v1.ObjectStorageAuthTypeCredentials,
							Credentials: &v1.S3Credentials{},
						},
					},
				},
			},
			want: "credentials",
		},
		{
			name: "no storage block",
			cfg: &v1.ObjectStorageConfig{
				Spec: v1.ObjectStorageConfigSpec{Type: v1.ObjectStorageTypeS3},
			},
			want: "block",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Open(ctx, tt.cfg, tt.creds)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}
