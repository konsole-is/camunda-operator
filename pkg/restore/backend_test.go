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

package restore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	clustercomponents "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

const backendClaimNamespace = "camunda-system"

// backendTarget is a target on the Elasticsearch contract "storage" and the
// relational contract "rdbms-storage" of namespace ns.
func backendTarget(storageRef string) *v1.CamundaCluster {
	return &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "target", Namespace: "ns", UID: "uid-target"},
		Spec:       v1.CamundaClusterSpec{StorageRef: storageRef},
	}
}

// backendObjects are the storage chains of backendTarget: one Elasticsearch
// contract, and one relational contract with its DatabaseConfig and
// DatabaseServerConfig.
func backendObjects() []client.Object {
	return []client.Object{
		&v1.SecondaryStorageConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "storage", Namespace: "ns"},
			Spec: v1.SecondaryStorageConfigSpec{
				Type:          v1.SecondaryStorageTypeElasticsearch,
				Elasticsearch: &v1.ElasticsearchStorage{Endpoint: "https://es:9200"},
			},
		},
		&v1.SecondaryStorageConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "rdbms-storage", Namespace: "ns"},
			Spec: v1.SecondaryStorageConfigSpec{
				Type:  v1.SecondaryStorageTypeRDBMS,
				RDBMS: &v1.RDBMSStorage{DatabaseConfigRef: "db"},
			},
		},
		&v1.DatabaseConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "ns"},
			Spec:       v1.DatabaseConfigSpec{ServerRef: "server", DatabaseName: "camunda"},
		},
		&v1.DatabaseServerConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "server", Namespace: "ns"},
			Spec:       v1.DatabaseServerConfigSpec{Host: "postgres", Port: 5432},
		},
	}
}

// backendClient is a fake client over objects. The fake client refuses the
// "!=" field selector of the pod list, which the API server serves, so the
// interceptor drops it.
func backendClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()

	return fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(objects...).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				if _, pods := list.(*metav1.PartialObjectMetadataList); !pods {
					return c.List(ctx, list, opts...)
				}
				kept := make([]client.ListOption, 0, len(opts))
				for _, opt := range opts {
					if _, phase := opt.(client.MatchingFieldsSelector); !phase {
						kept = append(kept, opt)
					}
				}

				return c.List(ctx, list, kept...)
			},
		}).
		Build()
}

// resolveBackend resolves the backend the way the restore controllers do.
func resolveBackend(t *testing.T, c client.Client, cluster *v1.CamundaCluster) (string, *conditions.PreCheckFailure) {
	t.Helper()

	storage, failure, err := ResolveStorage(context.Background(), c, cluster)
	require.NoError(t, err)
	if failure != nil {
		return "", failure
	}
	key, failure, err := BackendOf(context.Background(), c, storage)
	require.NoError(t, err)

	return key, failure
}

func TestBackendOfIsTheClaimKeyOfTheCluster(t *testing.T) {
	c := backendClient(t, backendObjects()...)

	key, failure := resolveBackend(t, c, backendTarget("storage"))
	require.Nil(t, failure)
	assert.Equal(
		t,
		"elasticsearch|https://es.ns.svc:9200",
		key,
		"a bare host resolves in the namespace of the contract",
	)

	// A bare host resolves in the namespace of the contract, the same way the
	// cluster computes its own key.
	key, failure = resolveBackend(t, c, backendTarget("rdbms-storage"))
	require.Nil(t, failure)
	want, err := clustercomponents.StorageClaimKey(clustercomponents.Storage{
		Type:      v1.SecondaryStorageTypeRDBMS,
		Namespace: "ns",
		RDBMS:     &clustercomponents.RDBMSStorage{Host: "postgres", Port: 5432, Database: "camunda"},
	})
	require.NoError(t, err)
	assert.Equal(t, want, key)
	assert.Contains(t, key, "postgres.ns.svc")
}

func TestBackendOfReportsABrokenChain(t *testing.T) {
	objects := backendObjects()[:2] // no DatabaseConfig, no server
	c := backendClient(t, objects...)

	_, failure := resolveBackend(t, c, backendTarget("rdbms-storage"))
	require.NotNil(t, failure)
	assert.Equal(t, v1.ReasonInvalidReference, failure.Reason)
	assert.Contains(t, failure.Message, "DatabaseConfig ns/db does not exist")
}

func TestCheckBackend(t *testing.T) {
	target := backendTarget("storage")
	const key = "elasticsearch|https://es.ns.svc:9200"
	claim := clustercomponents.StorageClaimSchema().LeaseName(key)
	other := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "apps", UID: "uid-other"}}
	lease := func(holder *v1.CamundaCluster) client.Object {
		return clustercomponents.StorageClaimSchema().NewLease(backendClaimNamespace, key, holder)
	}
	pod := func(name string, podLabels map[string]string) client.Object {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "apps", Labels: podLabels}}
	}
	ownJob := map[string]string{labels.StorageClaimKey: labels.OwnerName(claim), "restore-uid": "uid-restore"}

	storage := backendObjects()[0].(*v1.SecondaryStorageConfig)
	moved := storage.DeepCopy()
	moved.Spec.Elasticsearch = &v1.ElasticsearchStorage{Endpoint: "https://old:9200"}

	cases := map[string]struct {
		pinned  string
		storage *v1.SecondaryStorageConfig
		objects []client.Object
		ownPod  func(map[string]string) bool
		reason  string
		message string
	}{
		"the target holds the backend alone": {pinned: key, objects: []client.Object{lease(target)}},
		// The restore writes through the contract it resolved. A live contract
		// that names the pinned backend again does not make that copy safe.
		"the contract the restore writes through names another backend": {
			pinned:  key,
			storage: moved,
			objects: []client.Object{lease(target)},
			reason:  v1.ReasonInvalidReference,
			message: `now resolves to the backend "elasticsearch|https://old.ns.svc:9200"`,
		},
		"the target was pointed at another backend": {
			pinned:  "elasticsearch|https://old:9200",
			objects: []client.Object{lease(target)},
			reason:  v1.ReasonInvalidReference,
			message: `now resolves to the backend "elasticsearch|https://es.ns.svc:9200"`,
		},
		// A suspended target takes a free backend on its own pass.
		"nobody holds the backend yet": {
			pinned:  key,
			reason:  v1.ReasonWaitingForHandover,
			message: "does not hold the backend",
		},
		// The target reports this Lease as InvalidReference and never takes
		// the backend, so the restore does not wait for it.
		"a Lease that names no cluster claims the backend": {
			pinned: key,
			objects: []client.Object{&coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{
				Name:      claim,
				Namespace: backendClaimNamespace,
			}}},
			reason:  v1.ReasonInvalidReference,
			message: "names no CamundaCluster",
		},
		"another cluster holds the backend": {
			pinned:  key,
			objects: []client.Object{lease(other)},
			reason:  v1.ReasonStorageAlreadyAttached,
			message: "CamundaCluster apps/other holds the backend",
		},
		"pods of another cluster still write the backend": {
			pinned: key,
			objects: []client.Object{
				lease(target),
				pod("other-zeebe-0", clustercomponents.StoragePodLabels("other", other.UID, claim)),
			},
			reason:  v1.ReasonWaitingForHandover,
			message: "apps/other-zeebe-0",
		},
		// The target is suspended for the whole restore, so a pod of it that
		// still runs, such as its Optimize importer, writes beside the restore.
		"a pod of the target that still runs": {
			pinned: key,
			objects: []client.Object{
				lease(target),
				pod("target-optimize-importer-0", clustercomponents.StoragePodLabels("target", target.UID, claim)),
			},
			reason:  v1.ReasonWaitingForHandover,
			message: "apps/target-optimize-importer-0",
		},
		"the pod of the restore itself": {
			pinned:  key,
			objects: []client.Object{lease(target), pod("restore-pg-restore-x", ownJob)},
			ownPod: func(podLabels map[string]string) bool {
				return podLabels["restore-uid"] == "uid-restore"
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := backendClient(t, append(backendObjects(), tc.objects...)...)
			through := storage
			if tc.storage != nil {
				through = tc.storage
			}

			failure, err := CheckBackend(context.Background(), c, c, BackendCheck{
				ClaimNamespace: backendClaimNamespace,
				Cluster:        target,
				Storage:        through,
				Pinned:         tc.pinned,
				OwnPod:         tc.ownPod,
			})
			require.NoError(t, err)

			if tc.reason == "" {
				assert.Nil(t, failure)

				return
			}
			require.NotNil(t, failure)
			assert.Equal(t, tc.reason, failure.Reason)
			assert.Contains(t, failure.Message, tc.message)
		})
	}
}

// A pg_restore Job writes the database of the objects it was built from, so
// their key must be the one the check covered.
func TestDatabaseBackendIsTheKeyThatBackendOfPins(t *testing.T) {
	objects := backendObjects()
	c := backendClient(t, objects...)
	storage := objects[1].(*v1.SecondaryStorageConfig)
	config := objects[2].(*v1.DatabaseConfig)
	server := objects[3].(*v1.DatabaseServerConfig)

	pinned, failure := resolveBackend(t, c, backendTarget("rdbms-storage"))
	require.Nil(t, failure)

	key, failure := DatabaseBackend(storage, config, server)
	require.Nil(t, failure)
	assert.Equal(t, pinned, key)
	assert.Nil(t, MovedBackend(backendTarget("rdbms-storage"), key, pinned))

	moved := server.DeepCopy()
	moved.Spec.Host = "other-postgres"
	key, failure = DatabaseBackend(storage, config, moved)
	require.Nil(t, failure)
	failure = MovedBackend(backendTarget("rdbms-storage"), key, pinned)
	require.NotNil(t, failure)
	assert.Equal(t, v1.ReasonInvalidReference, failure.Reason)
}
