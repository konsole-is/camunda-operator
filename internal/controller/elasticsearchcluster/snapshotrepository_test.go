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

package elasticsearchcluster

import (
	"testing"

	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/elasticsearchcluster"
	"github.com/konsole-is/camunda-operator/pkg/esadmin/esadmintest"
)

// The cache can hand a reconcile a copy of the cluster from before the status
// write of the registration. That copy shows no SnapshotRepositoryReady
// condition, and a registration of it verifies the repository a second time.
func TestRegisterSnapshotRepositoryKeepsARegistrationThatTheReadDoesNotShowYet(t *testing.T) {
	t.Parallel()

	r, search := repositoryRig(t)
	cluster := repositoryCluster("cluster-uid")
	storage := repositoryStorage()

	first := r.registerSnapshotRepository(t.Context(), cluster.DeepCopy(), storage)
	require.Equal(t, metav1.ConditionTrue, first.Status, first.Message)

	stale := cluster.DeepCopy()
	second := r.registerSnapshotRepository(t.Context(), stale, storage)

	assert.Equal(t, metav1.ConditionTrue, second.Status, second.Message)
	assert.Equal(t, components.RepositoryName(cluster), stale.Status.SnapshotRepository)
	assert.Equal(t, 1, search.RepositoryPuts(components.RepositoryName(cluster)))
}

// A cluster that was deleted and created again under the same name can sit on
// a new Elasticsearch, which holds no repository.
func TestRegisterSnapshotRepositoryRegistersARecreatedCluster(t *testing.T) {
	t.Parallel()

	r, search := repositoryRig(t)
	storage := repositoryStorage()

	first := r.registerSnapshotRepository(t.Context(), repositoryCluster("first-uid"), storage)
	require.Equal(t, metav1.ConditionTrue, first.Status, first.Message)

	recreated := repositoryCluster("second-uid")
	second := r.registerSnapshotRepository(t.Context(), recreated, storage)

	assert.Equal(t, metav1.ConditionTrue, second.Status, second.Message)
	assert.Equal(t, 2, search.RepositoryPuts(components.RepositoryName(recreated)))
}

// A cluster whose pre-check failed is registered again once the pre-check
// passes, because nothing said what Elasticsearch held in between.
func TestRegisterSnapshotRepositoryRegistersAgainAfterTheRecordIsForgotten(t *testing.T) {
	t.Parallel()

	r, search := repositoryRig(t)
	cluster := repositoryCluster("cluster-uid")
	storage := repositoryStorage()

	first := r.registerSnapshotRepository(t.Context(), cluster.DeepCopy(), storage)
	require.Equal(t, metav1.ConditionTrue, first.Status, first.Message)

	r.forgetSnapshotRepository(cluster)
	second := r.registerSnapshotRepository(t.Context(), cluster.DeepCopy(), storage)

	assert.Equal(t, metav1.ConditionTrue, second.Status, second.Message)
	assert.Equal(t, 2, search.RepositoryPuts(components.RepositoryName(cluster)))
}

// repositoryRig returns a reconciler that reaches a fake Elasticsearch with
// the Secrets that ECK publishes for the cluster of repositoryCluster.
func repositoryRig(t *testing.T) (*ElasticsearchClusterReconciler, *esadmintest.Server) {
	t.Helper()

	search := esadmintest.NewTLS()
	t.Cleanup(search.Close)

	s := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(s))

	cluster := repositoryCluster("")
	password := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: esv1.ElasticUserSecret(cluster.Name), Namespace: cluster.Namespace},
		Data:       map[string][]byte{elasticPasswordKey: []byte("secret")},
	}
	ca := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: components.CACertSecretName(cluster), Namespace: cluster.Namespace},
		Data:       map[string][]byte{components.CACertKey: search.CertificatePEM()},
	}

	r := &ElasticsearchClusterReconciler{
		APIReader:   fake.NewClientBuilder().WithScheme(s).WithObjects(password, ca).Build(),
		EndpointFor: func(*v1.ElasticsearchCluster) string { return search.URL() },
	}

	return r, search
}

func repositoryCluster(uid types.UID) *v1.ElasticsearchCluster {
	return &v1.ElasticsearchCluster{ObjectMeta: metav1.ObjectMeta{Name: "es", Namespace: "ns", UID: uid}}
}

func repositoryStorage() *components.SnapshotStorage {
	return &components.SnapshotStorage{Config: &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "bucket", Namespace: "ns"},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeS3,
			S3: &v1.S3Storage{
				BucketName: "camunda-backups",
				Region:     "eu-west-1",
				Auth:       v1.S3StorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity},
			},
		},
	}}
}
