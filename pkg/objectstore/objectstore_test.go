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
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gocloud.dev/blob/fileblob"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// keysUnder collects the keys under prefix. Tests bound their own listings;
// the package streams so a caller never has to hold a whole bucket.
func keysUnder(t *testing.T, bucket *Bucket, prefix string) []string {
	t.Helper()

	var keys []string
	require.NoError(t, bucket.Walk(context.Background(), prefix, func(key string) error {
		keys = append(keys, key)
		return nil
	}))

	return keys
}

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

	assert.Equal(t, []string{"clusters/ns/name/1/camunda.dump"}, keysUnder(t, bucket, "clusters/ns/name/1/"))

	require.NoError(t, bucket.Delete(ctx, "clusters/ns/name/1/camunda.dump"))

	assert.Empty(t, keysUnder(t, bucket, "clusters/ns/name/1/"))

	assert.Equal(t, []string{"clusters/ns/name/2/camunda.dump"}, keysUnder(t, bucket, "clusters/ns/name/"))
}

// A walk stops at the first error the callback returns and hands it back
// unchanged, so a caller can abandon a listing it no longer needs.
func TestWalkStopsOnCallbackError(t *testing.T) {
	ctx := context.Background()
	bucket := fileBucket(t)

	for _, key := range []string{"p/a", "p/b", "p/c"} {
		require.NoError(t, bucket.Upload(ctx, key, strings.NewReader("x")))
	}

	stop := errors.New("enough")
	seen := 0
	err := bucket.Walk(ctx, "p/", func(string) error {
		seen++
		return stop
	})

	require.ErrorIs(t, err, stop)
	assert.Equal(t, 1, seen)
}

// TestBucketDownloadReadsBackWhatWasUploaded pins the read side of the round
// trip that a restore runs. A key that does not exist is an error, unlike a
// delete: a restore that cannot find its archive must fail.
func TestBucketDownloadReadsBackWhatWasUploaded(t *testing.T) {
	ctx := context.Background()
	bucket := fileBucket(t)

	require.NoError(t, bucket.Upload(ctx, "clusters/ns/name/1/camunda.dump", strings.NewReader("dump-bytes")))

	var out strings.Builder
	require.NoError(t, bucket.Download(ctx, "clusters/ns/name/1/camunda.dump", &out))
	assert.Equal(t, "dump-bytes", out.String())

	assert.Error(t, bucket.Download(ctx, "absent-key", &out))
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

	assert.Empty(t, keysUnder(t, bucket, "clusters/ns/name/1/"), "a partial upload must not be committed")
}

// A trailing slash on the endpoint must not double the URL separator: the
// Azure request signature is computed over the canonical resource, and the
// 403 that follows reads as bad credentials.
func TestAzureContainerURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec v1.AzureBlobStorage
		want string
	}{
		{
			name: "default endpoint from the account",
			spec: v1.AzureBlobStorage{AccountName: "camundabackups", Container: "backups"},
			want: "https://camundabackups.blob.core.windows.net/backups",
		},
		{
			name: "explicit endpoint",
			spec: v1.AzureBlobStorage{
				AccountName: "devstoreaccount1",
				Container:   "backups",
				Endpoint:    "http://127.0.0.1:10000/devstoreaccount1",
			},
			want: "http://127.0.0.1:10000/devstoreaccount1/backups",
		},
		{
			name: "trailing slash trimmed",
			spec: v1.AzureBlobStorage{
				AccountName: "devstoreaccount1",
				Container:   "backups",
				Endpoint:    "http://127.0.0.1:10000/devstoreaccount1/",
			},
			want: "http://127.0.0.1:10000/devstoreaccount1/backups",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, azureContainerURL(&test.spec))
		})
	}
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
				ObjectMeta: metav1.ObjectMeta{Name: "my-bucket"},
				Spec:       v1.ObjectStorageConfigSpec{Type: v1.ObjectStorageTypeS3},
			},
			want: "spec.s3",
		},
		{
			// The declared type decides. A gcs block under type S3 is a
			// configuration error, not an instruction to open GCS.
			name: "the block does not match the declared type",
			cfg: &v1.ObjectStorageConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "my-bucket"},
				Spec: v1.ObjectStorageConfigSpec{
					Type: v1.ObjectStorageTypeS3,
					GCS:  &v1.GCSStorage{BucketName: "bucket"},
				},
			},
			want: "spec.s3",
		},
		{
			name: "unknown type",
			cfg: &v1.ObjectStorageConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "my-bucket"},
				Spec:       v1.ObjectStorageConfigSpec{Type: "Swift"},
			},
			want: "unknown spec.type",
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

func TestCredentialsFrom(t *testing.T) {
	s3 := &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "my-bucket"},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeS3,
			S3: &v1.S3Storage{
				BucketName: "bucket",
				Endpoint:   "http://minio:9000",
				Auth: v1.S3StorageAuth{
					Type: v1.ObjectStorageAuthTypeCredentials,
					Credentials: &v1.S3Credentials{
						SecretRef: v1.S3CredentialsSecretRef{
							Name:               "minio",
							AccessKeyIDKey:     "accessKeyId",
							SecretAccessKeyKey: "secretAccessKey",
						},
					},
				},
			},
		},
	}

	creds, err := CredentialsFrom(s3, map[string][]byte{
		"accessKeyId":     []byte("AKIA"),
		"secretAccessKey": []byte("s3cret"),
	})

	require.NoError(t, err)
	require.NotNil(t, creds)
	assert.Equal(t, "AKIA", creds.AccessKeyID)
	assert.Equal(t, "s3cret", creds.SecretAccessKey)
}

// A key named by the contract but absent from the Secret must be reported by
// name: the whole point of the mapping is that the user chose the key names.
func TestCredentialsFromReportsTheMissingKey(t *testing.T) {
	cfg := &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "my-bucket", Namespace: "camunda"},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeGCS,
			GCS: &v1.GCSStorage{
				BucketName: "bucket",
				Auth: v1.GCSStorageAuth{
					Type: v1.ObjectStorageAuthTypeCredentials,
					Credentials: &v1.GCSCredentials{
						SecretRef: v1.LocalSecretKeyRef{Name: "gcs", Key: "key.json"},
					},
				},
			},
		},
	}

	_, err := CredentialsFrom(cfg, map[string][]byte{"other": []byte("x")})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "key.json")
	assert.Contains(t, err.Error(), "camunda/gcs")
}

// Workload identity resolves to no credentials at all, which is what tells
// Open to fall back to the provider default chain.
func TestCredentialsFromWorkloadIdentityIsNil(t *testing.T) {
	cfg := &v1.ObjectStorageConfig{
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeS3,
			S3: &v1.S3Storage{
				BucketName: "bucket",
				Region:     "eu-west-1",
				Auth:       v1.S3StorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity},
			},
		},
	}

	creds, err := CredentialsFrom(cfg, nil)

	require.NoError(t, err)
	assert.Nil(t, creds)
}

func TestBasePath(t *testing.T) {
	tests := []struct {
		name string
		spec v1.ObjectStorageConfigSpec
		want string
	}{
		{
			name: "s3",
			spec: v1.ObjectStorageConfigSpec{
				Type: v1.ObjectStorageTypeS3,
				S3:   &v1.S3Storage{BasePath: "backups"},
			},
			want: "backups",
		},
		{
			name: "gcs",
			spec: v1.ObjectStorageConfigSpec{
				Type: v1.ObjectStorageTypeGCS,
				GCS:  &v1.GCSStorage{BasePath: "documents"},
			},
			want: "documents",
		},
		{
			name: "azureBlob",
			spec: v1.ObjectStorageConfigSpec{
				Type:      v1.ObjectStorageTypeAzureBlob,
				AzureBlob: &v1.AzureBlobStorage{BasePath: "clusters"},
			},
			want: "clusters",
		},
		{
			name: "unset means the bucket root",
			spec: v1.ObjectStorageConfigSpec{
				Type: v1.ObjectStorageTypeS3,
				S3:   &v1.S3Storage{},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &v1.ObjectStorageConfig{Spec: tt.spec}
			assert.Equal(t, tt.want, cfg.BasePath())
		})
	}
}

// closingReader is a reader whose Close reports what its Read never did. A
// blob driver behaves this way when it finds the transfer incomplete only
// when it closes it, and gocloud hands that error to the caller of
// blob.Reader.Close.
type closingReader struct {
	body     string
	readErr  error
	closeErr error
	closed   bool
}

func (r *closingReader) Read(p []byte) (int, error) {
	if r.readErr != nil {
		return 0, r.readErr
	}
	if r.body == "" {
		return 0, io.EOF
	}
	n := copy(p, r.body)
	r.body = r.body[n:]

	return n, nil
}

func (r *closingReader) Close() error {
	r.closed = true

	return r.closeErr
}

// TestDrainReportsTheCloseOfTheTransfer pins the rule that a download is only
// whole when the close of the transfer agrees. A driver that reports the
// truncation on Close would otherwise hand a partial archive to pg_restore as
// a complete one.
func TestDrainReportsTheCloseOfTheTransfer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		reader   *closingReader
		wantErr  string
		wantBody string
	}{
		{
			name:     "a whole transfer",
			reader:   &closingReader{body: "dump-bytes"},
			wantBody: "dump-bytes",
		},
		{
			name:    "a close that reports the truncation",
			reader:  &closingReader{body: "dump-", closeErr: errors.New("unexpected end of stream")},
			wantErr: "finishing the download",
		},
		{
			name:    "a read that fails",
			reader:  &closingReader{readErr: errors.New("connection reset")},
			wantErr: "downloading",
		},
		{
			name: "a read and a close that both fail: the read is the cause",
			reader: &closingReader{
				readErr: errors.New("connection reset"), closeErr: errors.New("late close"),
			},
			wantErr: "downloading",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out strings.Builder
			err := drain(&out, tt.reader, "clusters/ns/name/1/camunda.dump")

			assert.True(t, tt.reader.closed, "the transfer is closed whatever happens")
			if tt.wantErr == "" {
				require.NoError(t, err)
				assert.Equal(t, tt.wantBody, out.String())

				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
