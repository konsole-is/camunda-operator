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
	"os"
	"path/filepath"
	"testing"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/konsole-is/camunda-operator/pkg/wrappers/barmanobjectstore"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/cnpgcluster"
	"github.com/konsole-is/camunda-operator/test/utils"
)

// fieldManager names this test as the owner of the applied fields, the way
// the framework names the component that owns a resource.
const fieldManager = "cnpgcluster-apply-test"

// TestWrappersApplyAgainstTheCRDs applies the rendered objects of the
// CloudNativePG and Barman Cloud wrappers with server-side apply, against the
// schemas the two operators serve. The API server types a server-side apply
// patch against the schema of the target before it merges anything, so a Go
// type that carries a field the CRD does not declare fails the whole apply.
// The test is the cheap place to catch that: without it the failure first
// appears on a real cluster.
func TestWrappersApplyAgainstTheCRDs(t *testing.T) {
	ctx := t.Context()
	apiClient := startControlPlane(t)

	cluster, err := cnpgcluster.NewBuilder(&cnpgv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "apply-server", Namespace: "default"},
		Spec: cnpgv1.ClusterSpec{
			Instances: 1,
			ImageName: "ghcr.io/cloudnative-pg/postgresql:17",
			StorageConfiguration: cnpgv1.StorageConfiguration{
				Size: "1Gi",
			},
			Plugins: []cnpgv1.PluginConfiguration{{
				Name:          "barman-cloud.cloudnative-pg.io",
				IsWALArchiver: new(true),
				Parameters: map[string]string{
					"barmanObjectName": "apply-archive",
					"serverName":       "apply-server",
				},
			}},
		},
	}).Build()
	require.NoError(t, err)

	store, err := barmanobjectstore.NewBuilder(&barmanobjectstore.ObjectStore{
		ObjectMeta: metav1.ObjectMeta{Name: "apply-archive", Namespace: "default"},
		Spec: barmanobjectstore.ObjectStoreSpec{
			Configuration: barmanobjectstore.BarmanObjectStoreConfiguration{
				DestinationPath: "s3://apply-bucket/databaseserver/default/apply-server/",
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

	for _, res := range []interface {
		Identity() string
		Preview() (client.Object, error)
	}{cluster, store} {
		t.Run(res.Identity(), func(t *testing.T) {
			desired, err := res.Preview()
			require.NoError(t, err)

			// The framework stamps the kind on the object before it patches,
			// because server-side apply reads it from the body.
			gvks, _, err := scheme.Scheme.ObjectKinds(desired)
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
}

// startControlPlane boots an envtest control plane that serves the vendored
// CloudNativePG and Barman Cloud CRDs, and returns a client against it.
func startControlPlane(t *testing.T) client.Client {
	t.Helper()

	require.NoError(t, cnpgv1.AddToScheme(scheme.Scheme))
	require.NoError(t, barmanobjectstore.AddToScheme(scheme.Scheme))

	cnpgCRDPath, err := utils.CNPGCRDPath()
	require.NoError(t, err)
	barmanCRDPath, err := utils.BarmanCRDPath()
	require.NoError(t, err)

	control := &envtest.Environment{
		CRDDirectoryPaths:     []string{cnpgCRDPath, barmanCRDPath},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: envtestBinaryDir(),
	}

	cfg, err := control.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, control.Stop())
	})

	apiClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	require.NoError(t, err)

	return apiClient
}

// envtestBinaryDir returns the first versioned binary directory under bin/k8s
// of this module, so the test runs from an IDE without KUBEBUILDER_ASSETS
// set. An empty string leaves envtest with KUBEBUILDER_ASSETS, which
// 'make setup-envtest' writes.
func envtestBinaryDir() string {
	base := filepath.Join("..", "..", "..", "bin", "k8s")

	entries, err := os.ReadDir(base)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(base, entry.Name())
		}
	}

	return ""
}
