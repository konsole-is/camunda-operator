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

package deleteobject

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/logicalbackuprdbms"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
)

// fakeBucket records the deletes and stands in for the backup bucket. Like
// objectstore.Bucket, a key that does not exist is success.
type fakeBucket struct {
	deleted []string
	config  *v1.ObjectStorageConfig
	creds   *objectstore.Credentials
}

func (b *fakeBucket) Delete(_ context.Context, key string) error {
	b.deleted = append(b.deleted, key)

	return nil
}

func (b *fakeBucket) Close() {}

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

// wifSpec is a workload-identity bucket. There are no credentials to
// project, and the pod identity is the whole authentication.
const wifSpec = `{"type":"S3","s3":{"bucketName":"b","region":"r",` +
	`"auth":{"type":"workloadIdentity","workloadIdentity":{"roleArn":"arn:aws:iam::1:role/x"}}}}`

func TestRunDeletesExactlyTheKey(t *testing.T) {
	t.Parallel()

	bucket := &fakeBucket{}
	open := func(
		_ context.Context, cfg *v1.ObjectStorageConfig, creds *objectstore.Credentials,
	) (deleter, error) {
		bucket.config, bucket.creds = cfg, creds

		return bucket, nil
	}

	err := run(
		context.Background(), env(map[string]string{
			components.EnvUploadKey:         "clusters/ns/cluster/1/camunda.dump",
			components.EnvUploadStorageName: "my-backup-config",
			components.EnvUploadStorageSpec: wifSpec,
		}), open,
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"clusters/ns/cluster/1/camunda.dump"}, bucket.deleted)
	assert.Nil(t, bucket.creds, "workload identity projects no credentials")
	assert.Equal(t, "my-backup-config", bucket.config.Name)

	// A second run deletes the same key again. The bucket treats a missing
	// key as success, so the retry is idempotent end to end.
	require.NoError(t, run(
		context.Background(), env(map[string]string{
			components.EnvUploadKey:         "clusters/ns/cluster/1/camunda.dump",
			components.EnvUploadStorageName: "my-backup-config",
			components.EnvUploadStorageSpec: wifSpec,
		}), open,
	))
}

func TestRunRejectsAMissingKey(t *testing.T) {
	t.Parallel()

	err := run(context.Background(), env(nil), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), components.EnvUploadKey)
}
