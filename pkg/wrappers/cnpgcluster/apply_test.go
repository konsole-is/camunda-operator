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

package cnpgcluster_test

import (
	"testing"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/konsole-is/camunda-operator/pkg/wrappers/barmanobjectstore"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/cnpgcluster"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/cnpgscheduledbackup"
	"github.com/konsole-is/camunda-operator/test/utils"
)

// fieldManager names this test as the owner of the applied fields, the way
// the framework names the component that owns a resource.
const fieldManager = "cnpgcluster-apply-test"

// TestWrappersApplyAgainstTheCRDs applies the rendered objects of the three
// CloudNativePG and Barman Cloud wrappers with server-side apply, against the
// schemas the two operators serve. The API server types a server-side apply
// patch against the schema of the target before it merges anything, so a Go
// type that carries a field the CRD does not declare fails the whole apply.
// The test is the cheap place to catch that: without it the failure first
// appears on a real cluster.
func TestWrappersApplyAgainstTheCRDs(t *testing.T) {
	ctx := t.Context()
	apiClient, testScheme := startControlPlane(t)

	cluster, err := cnpgcluster.NewBuilder(clusterFixture("apply-server")).Build()
	require.NoError(t, err)

	// The suspended Cluster goes through the same apply. CloudNativePG puts a
	// minimum of 1 on spec.instances, so a suspension that scaled to zero
	// would be rejected here and nowhere else.
	suspended, err := cnpgcluster.NewBuilder(clusterFixture("apply-server-suspended")).
		WithMutation(cnpgcluster.Mutation{
			Name:   "suspend",
			Mutate: cnpgcluster.DefaultSuspendMutationHandler,
		}).
		Build()
	require.NoError(t, err)

	store, err := barmanobjectstore.NewBuilder(&barmanobjectstore.ObjectStore{
		ObjectMeta: metav1.ObjectMeta{Name: "apply-archive", Namespace: "default"},
		Spec: barmanobjectstore.ObjectStoreSpec{
			Configuration: barmanobjectstore.BarmanObjectStoreConfiguration{
				DestinationPath: "s3://apply-bucket/databaseserver/default/apply-server-6c2f81ba/",
				EndpointURL:     "http://minio.default.svc:9000",
				S3Credentials: &barmanobjectstore.S3Credentials{
					AccessKeyID: &barmanobjectstore.SecretKeySelector{Name: "apply-archive", Key: "accessKeyId"},
					SecretAccessKey: &barmanobjectstore.SecretKeySelector{
						Name: "apply-archive",
						Key:  "secretAccessKey",
					},
				},
				Wal:  &barmanobjectstore.WalBackupConfiguration{Compression: barmanobjectstore.CompressionTypeGzip},
				Data: &barmanobjectstore.DataBackupConfiguration{Compression: barmanobjectstore.CompressionTypeGzip},
			},
			RetentionPolicy: "30d",
		},
	}).Build()
	require.NoError(t, err)

	backup, err := cnpgscheduledbackup.NewBuilder(&cnpgv1.ScheduledBackup{
		ObjectMeta: metav1.ObjectMeta{Name: "apply-base-backup", Namespace: "default"},
		Spec: cnpgv1.ScheduledBackupSpec{
			Schedule:            "0 0 2 * * *",
			Immediate:           new(true),
			Method:              cnpgv1.BackupMethodPlugin,
			PluginConfiguration: &cnpgv1.BackupPluginConfiguration{Name: "barman-cloud.cloudnative-pg.io"},
			Cluster:             cnpgv1.LocalObjectReference{Name: "apply-server"},
		},
	}).Build()
	require.NoError(t, err)

	for _, res := range []interface {
		Identity() string
		Preview() (client.Object, error)
	}{cluster, suspended, store, backup} {
		t.Run(res.Identity(), func(t *testing.T) {
			desired, err := res.Preview()
			require.NoError(t, err)

			// The framework stamps the kind on the object before it patches,
			// because server-side apply reads it from the body.
			gvks, _, err := testScheme.ObjectKinds(desired)
			require.NoError(t, err)
			require.NotEmpty(t, gvks)
			desired.GetObjectKind().SetGroupVersionKind(gvks[0])

			require.NoError(t, apiClient.Patch(
				ctx,
				desired,
				client.Apply, //nolint:staticcheck // ocf applies through the deprecated client.Apply patch
				client.FieldOwner(fieldManager),
				client.ForceOwnership,
			))
			assert.NotEmpty(t, desired.GetUID(), "the API server accepted the apply and returned the object")
		})
	}

	var suspendedStored cnpgv1.Cluster
	require.NoError(t, apiClient.Get(
		ctx, client.ObjectKey{Namespace: "default", Name: "apply-server-suspended"}, &suspendedStored,
	))
	assert.Equal(
		t, cnpgcluster.HibernationOn, suspendedStored.Annotations[cnpgcluster.HibernationAnnotation],
	)
	assert.Equal(t, 1, suspendedStored.Spec.Instances, "suspension leaves the instance count alone")

	// The running Cluster declares the annotation too, which is what lets a
	// later apply take a hand-set hibernation back.
	var runningStored cnpgv1.Cluster
	require.NoError(t, apiClient.Get(
		ctx, client.ObjectKey{Namespace: "default", Name: "apply-server"}, &runningStored,
	))
	assert.Equal(
		t, cnpgcluster.HibernationOff, runningStored.Annotations[cnpgcluster.HibernationAnnotation],
	)
}

// clusterFixture returns a Cluster that archives through the Barman Cloud
// plugin, the shape a database server applies.
func clusterFixture(name string) *cnpgv1.Cluster {
	return &cnpgv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: cnpgv1.ClusterSpec{
			Instances:            1,
			ImageName:            "ghcr.io/cloudnative-pg/postgresql:17",
			StorageConfiguration: cnpgv1.StorageConfiguration{Size: "1Gi"},
			Plugins: []cnpgv1.PluginConfiguration{{
				Name:          "barman-cloud.cloudnative-pg.io",
				IsWALArchiver: new(true),
				Parameters: map[string]string{
					"barmanObjectName": "apply-archive",
					"serverName":       name,
				},
			}},
		},
	}
}

// startControlPlane boots an envtest control plane that serves the vendored
// CloudNativePG and Barman Cloud CRDs, and returns a client against it and
// the scheme that client reads through.
func startControlPlane(t *testing.T) (client.Client, *runtime.Scheme) {
	t.Helper()

	control := &envtest.Environment{
		CRDDirectoryPaths:     []string{crdPath(t, utils.CNPGCRDPath), crdPath(t, utils.BarmanCRDPath)},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: utils.EnvtestBinaryDir(),
	}

	cfg, err := control.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, control.Stop())
	})

	// The scheme is private to this package. Registering into the global
	// scheme.Scheme would leak the kinds into every other test binary that
	// shares it.
	testScheme := goldenScheme(t)

	apiClient, err := client.New(cfg, client.Options{Scheme: testScheme})
	require.NoError(t, err)

	return apiClient, testScheme
}

// crdPath resolves one vendored CRD directory.
func crdPath(t *testing.T, resolve func() (string, error)) string {
	t.Helper()

	path, err := resolve()
	require.NoError(t, err)

	return path
}
