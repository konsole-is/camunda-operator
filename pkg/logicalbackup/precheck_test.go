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

package logicalbackup_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
)

// clusterNamespace is where the fixtures put the cluster and its secondary
// storage.
const clusterNamespace = "my-ns"

// cluster builds a CamundaCluster that passes every pre-check, so each test
// can break exactly one thing.
func cluster() *v1.CamundaCluster {
	return &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cluster", Namespace: clusterNamespace},
		Spec: v1.CamundaClusterSpec{
			StorageRef:       "my-storage",
			BackupStorageRef: "my-bucket",
		},
	}
}

func storage(storageType v1.SecondaryStorageType) *v1.SecondaryStorageConfig {
	return &v1.SecondaryStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "my-storage", Namespace: clusterNamespace},
		Spec:       v1.SecondaryStorageConfigSpec{Type: storageType},
	}
}

func bucket() *v1.ObjectStorageConfig {
	return &v1.ObjectStorageConfig{ObjectMeta: metav1.ObjectMeta{Name: "my-bucket"}}
}

func newReader(t *testing.T, objects ...client.Object) client.Reader {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

// request builds a pre-check request for an Elasticsearch-backed backup in
// the cluster namespace, with no other backup running.
func request(reader client.Reader) logicalbackup.PreCheckRequest {
	return logicalbackup.PreCheckRequest{
		Reader:      reader,
		Ref:         v1.ClusterRef{Name: "my-cluster"},
		Namespace:   clusterNamespace,
		StorageType: v1.SecondaryStorageTypeElasticsearch,
		InProgress: func(context.Context) (string, error) {
			return "", nil
		},
	}
}

func TestPreCheckPasses(t *testing.T) {
	reader := newReader(t, cluster(), storage(v1.SecondaryStorageTypeElasticsearch), bucket())

	result, err := logicalbackup.PreCheck(context.Background(), request(reader))

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "my-cluster", result.Cluster.Name)
	assert.Equal(t, "my-storage", result.Storage.Name)
	assert.Equal(t, "my-bucket", result.Bucket.Name)
}

// An empty ref namespace means the namespace of the backup itself, so the
// common case needs no namespace in the manifest.
func TestPreCheckDefaultsTheNamespaceToTheBackup(t *testing.T) {
	reader := newReader(t, cluster(), storage(v1.SecondaryStorageTypeElasticsearch), bucket())

	req := request(reader)
	req.Namespace = clusterNamespace
	req.Ref.Namespace = ""
	result, err := logicalbackup.PreCheck(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, clusterNamespace, result.Cluster.Namespace)
}

// A backup can name a cluster in another namespace. The cluster and its
// secondary storage are then read from the namespace of the reference, not
// from the namespace of the backup.
func TestPreCheckResolvesAClusterInAnotherNamespace(t *testing.T) {
	reader := newReader(t, cluster(), storage(v1.SecondaryStorageTypeElasticsearch), bucket())

	req := request(reader)
	req.Namespace = "backups-ns"
	req.Ref.Namespace = clusterNamespace

	result, err := logicalbackup.PreCheck(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, clusterNamespace, result.Cluster.Namespace)
	assert.Equal(t, clusterNamespace, result.Storage.Namespace)
}

// A reference that names another namespace never falls back to the namespace
// of the backup, which would silently back up the wrong cluster.
func TestPreCheckDoesNotFallBackToTheBackupNamespace(t *testing.T) {
	reader := newReader(t, cluster(), storage(v1.SecondaryStorageTypeElasticsearch), bucket())

	req := request(reader)
	req.Namespace = clusterNamespace
	req.Ref.Namespace = "elsewhere"

	_, err := logicalbackup.PreCheck(context.Background(), req)

	var failure *conditions.PreCheckFailure
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, v1.ReasonInvalidReference, failure.Reason)
	assert.Contains(t, failure.Message, "elsewhere/my-cluster")
}

func TestPreCheckFailures(t *testing.T) {
	tests := []struct {
		name    string
		objects []client.Object
		mutate  func(*logicalbackup.PreCheckRequest)
		reason  string
		waiting bool
		message string
	}{
		{
			name:    "the cluster does not exist",
			objects: []client.Object{storage(v1.SecondaryStorageTypeElasticsearch), bucket()},
			reason:  v1.ReasonInvalidReference,
			message: "my-ns/my-cluster",
		},
		{
			name: "the cluster has no backup storage",
			objects: func() []client.Object {
				c := cluster()
				c.Spec.BackupStorageRef = ""
				return []client.Object{c, storage(v1.SecondaryStorageTypeElasticsearch), bucket()}
			}(),
			reason:  v1.ReasonInvalidReference,
			message: "backupStorageRef",
		},
		{
			name: "the cluster is suspended",
			objects: func() []client.Object {
				c := cluster()
				c.Spec.Suspend = true
				return []client.Object{c, storage(v1.SecondaryStorageTypeElasticsearch), bucket()}
			}(),
			reason:  v1.ReasonClusterSuspended,
			waiting: true,
			message: "suspended",
		},
		{
			name:    "the secondary storage does not exist",
			objects: []client.Object{cluster(), bucket()},
			reason:  v1.ReasonInvalidReference,
			message: "my-ns/my-storage",
		},
		{
			name:    "the storage type does not match the backup kind",
			objects: []client.Object{cluster(), storage(v1.SecondaryStorageTypeRDBMS), bucket()},
			reason:  v1.ReasonStorageTypeMismatch,
			message: "rdbms",
		},
		{
			name:    "the bucket does not exist",
			objects: []client.Object{cluster(), storage(v1.SecondaryStorageTypeElasticsearch)},
			reason:  v1.ReasonInvalidReference,
			message: "my-bucket",
		},
		{
			name:    "another backup of the cluster runs",
			objects: []client.Object{cluster(), storage(v1.SecondaryStorageTypeElasticsearch), bucket()},
			mutate: func(req *logicalbackup.PreCheckRequest) {
				req.InProgress = func(context.Context) (string, error) {
					return "my-cluster-1748937221", nil
				}
			},
			reason:  v1.ReasonBackupInProgress,
			waiting: true,
			message: "my-cluster-1748937221",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := request(newReader(t, tt.objects...))
			if tt.mutate != nil {
				tt.mutate(&req)
			}

			result, err := logicalbackup.PreCheck(context.Background(), req)

			assert.Nil(t, result)
			var failure *conditions.PreCheckFailure
			require.ErrorAs(t, err, &failure)
			assert.Equal(t, tt.reason, failure.Reason)
			assert.Contains(t, failure.Message, tt.message)
			assert.Equal(t, tt.waiting, logicalbackup.Waiting(err))
		})
	}
}

// A failure to read the API server is transient, not a state of the backup:
// it must not surface as a condition reason.
func TestPreCheckPropagatesTransientErrors(t *testing.T) {
	req := request(newReader(t, cluster(), storage(v1.SecondaryStorageTypeElasticsearch), bucket()))
	req.InProgress = func(context.Context) (string, error) {
		return "", errors.New("listing backups: connection refused")
	}

	result, err := logicalbackup.PreCheck(context.Background(), req)

	assert.Nil(t, result)
	require.Error(t, err)
	var failure *conditions.PreCheckFailure
	assert.NotErrorAs(t, err, &failure)
	assert.False(t, logicalbackup.Waiting(err))
}
