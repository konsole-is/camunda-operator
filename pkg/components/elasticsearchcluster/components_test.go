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
	"flag"
	"fmt"
	"path/filepath"
	"testing"

	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/testing/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/credentials"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
)

// updateGolden refreshes the golden manifests with the rendered output:
// go test ./pkg/components/elasticsearchcluster/ -run Golden -update-golden
var updateGolden = flag.Bool("update-golden", false, "update golden files")

// goldenPassword replaces the generated password, so the golden manifests
// stay deterministic.
const goldenPassword = "golden-test-password"

// goldenScheme registers every type for which the golden serializer must
// resolve TypeMeta.
func goldenScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, esv1.AddToScheme(scheme))
	require.NoError(t, monitoringv1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	return scheme
}

// goldenMinimalElasticsearchCluster is the minimal example of the CRD doc
// with a deterministic name, resolved against the "standard" preset of the
// doc.
func goldenMinimalElasticsearchCluster() (*v1.ElasticsearchCluster, *v1.ElasticsearchClusterPresetSpec) {
	cluster := &v1.ElasticsearchCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cluster-es", Namespace: "my-cluster-ns"},
		Spec: v1.ElasticsearchClusterSpec{
			PresetRef:              "standard",
			SecondaryStorageConfig: "my-storage-config",
		},
	}
	preset := &v1.ElasticsearchClusterPresetSpec{
		Cluster: v1.ElasticsearchClusterSpec{
			Version:     "9.2.4",
			Replicas:    new(int32(3)),
			StorageSize: new(resource.MustParse("64Gi")),
		},
	}

	return cluster, preset
}

// goldenRealisticElasticsearchCluster is the realistic example of the CRD doc
// with a deterministic name. It adds every optional rendering input (service
// account, extra env, pod annotations), so the goldens pin all of them.
func goldenRealisticElasticsearchCluster() *v1.ElasticsearchCluster {
	return &v1.ElasticsearchCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cluster-es", Namespace: "my-cluster-ns"},
		Spec: v1.ElasticsearchClusterSpec{
			Version:  "9.2.4",
			Replicas: new(int32(3)),
			Resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
			},
			StorageSize:      new(resource.MustParse("128Gi")),
			StorageClassName: new("ssd"),
			ServiceAccount: &v1.ServiceAccountSpec{
				Annotations: map[string]string{
					"eks.amazonaws.com/role-arn": "arn:aws:iam::123456789012:role/my-es-snapshot-role",
				},
			},
			ExtraEnv:       []corev1.EnvVar{{Name: "ES_JAVA_OPTS", Value: "-Xms2g -Xmx2g"}},
			ExtraEnvFrom:   []corev1.EnvFromSource{{Prefix: "EXTRA_"}},
			PodLabels:      map[string]string{"team": "platform"},
			PodAnnotations: map[string]string{"prometheus.io/scrape": "true"},
			Scheduling: &v1.SchedulingSpec{
				Tolerations: []corev1.Toleration{{
					Key:      "dedicated",
					Operator: corev1.TolerationOpEqual,
					Value:    "elasticsearch",
					Effect:   corev1.TaintEffectNoSchedule,
				}},
			},
			SecondaryStorageConfig: "my-storage-config",
			Monitoring: &v1.MonitoringSpec{
				ServiceMonitor: &v1.ServiceMonitorSpec{
					Enabled: true,
					Labels:  map[string]string{"release": "prometheus"},
				},
			},
			PersistentVolumeClaimRetentionPolicy: &v1.PersistentVolumeClaimRetentionPolicy{
				WhenDeleted: v1.RetainPersistentVolumeClaimRetentionPolicyType,
			},
		},
	}
}

// assertElasticsearchClusterGoldens renders the components for the merged
// spec and pins each against its golden manifest under dir. The metrics
// component is pinned only when the fixture enables monitoring.
func assertElasticsearchClusterGoldens(
	t *testing.T,
	dir string,
	cluster *v1.ElasticsearchCluster,
	merged v1.ElasticsearchClusterSpec,
	storage *SnapshotStorage,
) {
	registered := storage != nil
	t.Helper()

	scheme := goldenScheme(t)
	base := filepath.Join("testdata", "golden", dir)

	credentialsComp, err := CredentialsComponent(cluster, credentials.Password{Value: goldenPassword})
	require.NoError(t, err)
	golden.AssertComponentYAML(
		t, filepath.Join(base, "credentials.yaml"), credentialsComp,
		golden.WithScheme(scheme), golden.Update(*updateGolden),
	)

	keystore, err := KeystoreComponent(cluster, storage)
	require.NoError(t, err)
	golden.AssertComponentYAML(
		t, filepath.Join(base, "keystore.yaml"), keystore,
		golden.WithScheme(scheme), golden.Update(*updateGolden),
	)

	elasticsearch, err := ElasticsearchComponent(cluster, merged, storage)
	require.NoError(t, err)
	golden.AssertComponentYAML(
		t, filepath.Join(base, "elasticsearch.yaml"), elasticsearch,
		golden.WithScheme(scheme), golden.Update(*updateGolden),
	)

	storageContract, err := StorageContractComponent(cluster, merged, storage, registered)
	require.NoError(t, err)
	golden.AssertComponentYAML(
		t, filepath.Join(base, "storage-contract.yaml"), storageContract,
		golden.WithScheme(scheme), golden.Update(*updateGolden),
	)

	if MonitoringEnabled(merged) {
		metrics, err := MetricsComponent(cluster, merged, true)
		require.NoError(t, err)
		golden.AssertComponentYAML(
			t, filepath.Join(base, "metrics.yaml"), metrics,
			golden.WithScheme(scheme), golden.Update(*updateGolden),
		)
	}
}

func TestElasticsearchClusterGoldenMinimal(t *testing.T) {
	t.Parallel()

	cluster, preset := goldenMinimalElasticsearchCluster()

	assertElasticsearchClusterGoldens(t, "minimal", cluster, MergePreset(cluster.Spec, preset), nil)
}

// The 8.19 line is the other supported version family. Its rendering is
// pinned separately from the 9.2 fixtures.
func TestElasticsearchClusterGoldenMinimalES8(t *testing.T) {
	t.Parallel()

	cluster, preset := goldenMinimalElasticsearchCluster()
	preset.Cluster.Version = "8.19.0"

	assertElasticsearchClusterGoldens(t, "minimal-es8", cluster, MergePreset(cluster.Spec, preset), nil)
}

func TestElasticsearchClusterGoldenRealistic(t *testing.T) {
	t.Parallel()

	cluster := goldenRealisticElasticsearchCluster()

	assertElasticsearchClusterGoldens(t, "realistic", cluster, MergePreset(cluster.Spec, nil), nil)
}

// The suspended variant must render the same desired content as its
// non-suspended baseline. Suspension is a runtime mutation (the wrapper
// switches the applied CR to the retaining volume policy, then deletes it)
// and must never alter the declared state, in particular the data volume
// claims.
func TestElasticsearchClusterGoldenSuspended(t *testing.T) {
	t.Parallel()

	cluster, preset := goldenMinimalElasticsearchCluster()
	cluster.Spec.Suspend = true

	assertElasticsearchClusterGoldens(t, "suspended", cluster, MergePreset(cluster.Spec, preset), nil)
}

// goldenBucketName is the bucket contract that the golden cases reference.
const goldenBucketName = "my-backup-config"

// snapshotBucket returns a bucket contract with the given auth, for the
// golden cases that reference one.
func snapshotBucket(auth v1.S3StorageAuth) *v1.ObjectStorageConfig {
	return &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: goldenBucketName},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeS3,
			S3: &v1.S3Storage{
				BucketName: "camunda-backups",
				BasePath:   "clusters",
				Region:     "eu-west-1",
				Auth:       auth,
			},
		},
	}
}

// A cluster whose bucket uses workload identity gets the identity annotation
// on a ServiceAccount that the operator renders even though the spec asks for
// none, and no keystore Secret: the nodes authenticate as their
// ServiceAccount.
func TestElasticsearchClusterGoldenSnapshotWorkloadIdentity(t *testing.T) {
	t.Parallel()

	cluster, preset := goldenMinimalElasticsearchCluster()
	cluster.Spec.SnapshotStorageRef = goldenBucketName
	storage := &SnapshotStorage{
		Config: snapshotBucket(v1.S3StorageAuth{
			Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
			WorkloadIdentity: &v1.S3WorkloadIdentity{RoleARN: "arn:aws:iam::123456789012:role/camunda"},
		}),
	}

	assertElasticsearchClusterGoldens(
		t, "snapshot-workload-identity", cluster, MergePreset(cluster.Spec, preset), storage,
	)
}

// A cluster whose bucket holds static keys gets them in a keystore Secret,
// referenced from spec.secureSettings ahead of the sources of the spec.
// Elasticsearch reads repository credentials from the keystore alone.
func TestElasticsearchClusterGoldenSnapshotCredentials(t *testing.T) {
	t.Parallel()

	cluster, preset := goldenMinimalElasticsearchCluster()
	cluster.Spec.SnapshotStorageRef = goldenBucketName
	cluster.Spec.SecureSettings = []v1.SecureSettingsSource{{
		SecretName: "extra-keystore-entries",
		Entries:    []v1.SecureSettingEntry{{Key: "someKey", Path: "some.secure.setting"}},
	}}
	cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{Name: "es-prod"}
	storage := &SnapshotStorage{
		Config: snapshotBucket(v1.S3StorageAuth{
			Type: v1.ObjectStorageAuthTypeCredentials,
			Credentials: &v1.S3Credentials{
				SecretRef: v1.S3CredentialsSecretRef{
					Name:               "minio-credentials",
					Namespace:          "camunda",
					AccessKeyIDKey:     "accessKeyId",
					SecretAccessKeyKey: "secretAccessKey",
				},
			},
		}),
		Credentials: &objectstore.Credentials{AccessKeyID: "minio-root", SecretAccessKey: "minio-secret"},
	}

	assertElasticsearchClusterGoldens(
		t, "snapshot-credentials", cluster, MergePreset(cluster.Spec, preset), storage,
	)
}

// A ServiceAccount the operator does not create is never rendered, but the
// pods still run under its name.
func TestForeignServiceAccountIsNamedButNotRendered(t *testing.T) {
	t.Parallel()

	cluster, preset := goldenMinimalElasticsearchCluster()
	no := false
	cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{Name: "platform-es", Create: &no}
	merged := MergePreset(cluster.Spec, preset)

	comp, err := ElasticsearchComponent(cluster, merged, nil)
	require.NoError(t, err)

	objects, err := comp.Preview()
	require.NoError(t, err)

	var es *esv1.Elasticsearch
	for _, obj := range objects {
		_, isServiceAccount := obj.(*corev1.ServiceAccount)
		assert.False(t, isServiceAccount, "a ServiceAccount with create false must not be rendered")
		if typed, ok := obj.(*esv1.Elasticsearch); ok {
			es = typed
		}
	}
	require.NotNil(t, es)
	assert.Equal(t, "platform-es", es.Spec.NodeSets[0].PodTemplate.Spec.ServiceAccountName)
}

// The contract names a repository only once its registration converged: a
// name published earlier sends a consumer's first snapshot into a repository
// that does not exist. Suspension keeps the last converged name, because the
// registration persists in the cluster state that the data volumes retain.
func TestPublishedRepositoryName(t *testing.T) {
	t.Parallel()

	cluster := &v1.ElasticsearchCluster{ObjectMeta: metav1.ObjectMeta{Name: "my-es"}}
	storage := &SnapshotStorage{Config: snapshotBucket(v1.S3StorageAuth{
		Type: v1.ObjectStorageAuthTypeWorkloadIdentity,
	})}

	assert.Empty(t, publishedRepositoryName(cluster, storage, false, false), "not yet registered")
	assert.Equal(t, "my-es", publishedRepositoryName(cluster, storage, true, false))
	assert.Equal(
		t, "my-es", publishedRepositoryName(cluster, nil, true, true),
		"suspension resolves no bucket but keeps the registered name",
	)
	assert.Empty(
		t, publishedRepositoryName(cluster, nil, true, false),
		"a dropped bucket reference clears the name",
	)
}

// An annotation-less workload identity (Pod Identity, Workload Identity
// Federation) still binds the ServiceAccount by name, so the account is
// rendered and referenced with nothing on it.
func TestPodIdentityRendersTheServiceAccount(t *testing.T) {
	t.Parallel()

	cluster, preset := goldenMinimalElasticsearchCluster()
	cluster.Spec.SnapshotStorageRef = goldenBucketName
	merged := MergePreset(cluster.Spec, preset)
	storage := &SnapshotStorage{Config: snapshotBucket(v1.S3StorageAuth{
		Type: v1.ObjectStorageAuthTypeWorkloadIdentity,
	})}

	comp, err := ElasticsearchComponent(cluster, merged, storage)
	require.NoError(t, err)

	objects, err := comp.Preview()
	require.NoError(t, err)

	var account *corev1.ServiceAccount
	var es *esv1.Elasticsearch
	for _, obj := range objects {
		switch typed := obj.(type) {
		case *corev1.ServiceAccount:
			account = typed
		case *esv1.Elasticsearch:
			es = typed
		}
	}
	require.NotNil(t, account, "the documented principal must exist")
	assert.Empty(t, account.Annotations)
	require.NotNil(t, es)
	assert.Equal(t, account.Name, es.Spec.NodeSets[0].PodTemplate.Spec.ServiceAccountName)
}

// A cluster without the ServiceMonitor CRD must not render one, even when
// monitoring is enabled on the spec. The controller omits the resource
// instead of failing every reconcile against a missing kind.
func TestMetricsComponentOmitsUnsupportedServiceMonitor(t *testing.T) {
	t.Parallel()

	cluster := goldenRealisticElasticsearchCluster()
	comp, err := MetricsComponent(cluster, cluster.Spec, false)
	require.NoError(t, err)

	// Typed objects carry no TypeMeta until serialized, so compare Go types.
	objects, err := comp.Preview()
	require.NoError(t, err)
	types := make([]string, 0, len(objects))
	for _, obj := range objects {
		types = append(types, fmt.Sprintf("%T", obj))
	}
	assert.ElementsMatch(t, []string{"*v1.Deployment", "*v1.Service"}, types)
}

// The generated password must land in the published Secret verbatim, under
// the documented basic-auth file-realm keys.
func TestCredentialsComponentCarriesThePassword(t *testing.T) {
	t.Parallel()

	cluster, _ := goldenMinimalElasticsearchCluster()

	comp, err := CredentialsComponent(cluster, credentials.Password{Value: "s3cret"})
	require.NoError(t, err)

	objects, err := comp.Preview()
	require.NoError(t, err)

	secrets := map[string]*corev1.Secret{}
	for _, obj := range objects {
		secret, ok := obj.(*corev1.Secret)
		require.True(t, ok)
		secrets[secret.Name] = secret
	}

	user := secrets["my-cluster-es-es-user"]
	require.NotNil(t, user)
	assert.Equal(t, []byte("camunda"), user.Data["username"])
	assert.Equal(t, []byte("s3cret"), user.Data["password"])
	assert.Equal(t, []byte("camunda"), user.Data["roles"])

	// The role the user holds must be defined, or Elasticsearch grants it
	// nothing. superuser is deliberately gone.
	roles := secrets["my-cluster-es-es-roles"]
	require.NotNil(t, roles)
	definition := string(roles.Data["roles.yml"])
	assert.Contains(t, definition, "camunda:")
	assert.Contains(t, definition, "create_snapshot")
	assert.Contains(t, definition, "monitor")
	assert.NotContains(t, definition, "superuser")
}

// A password that came from an existing Secret must bind its apply to that
// Secret, or a delete between the read and the apply recreates the Secret with
// the old password and the delete rotates nothing. The role Secret holds no
// credential and takes no precondition.
func TestCredentialsComponentCarriesTheApplyPrecondition(t *testing.T) {
	t.Parallel()

	cluster, _ := goldenMinimalElasticsearchCluster()

	reused := credentials.Password{Value: "s3cret", SourceUID: "uid-1"}
	comp, err := CredentialsComponent(cluster, reused)
	require.NoError(t, err)

	objects, err := comp.Preview()
	require.NoError(t, err)

	annotations := map[string]map[string]string{}
	for _, obj := range objects {
		annotations[obj.GetName()] = obj.GetAnnotations()
	}
	assert.Equal(
		t,
		map[string]string{credentials.PreconditionAnnotation: "uid-1"},
		annotations["my-cluster-es-es-user"],
	)
	assert.Empty(t, annotations["my-cluster-es-es-roles"])

	// A new password has no source object, so the apply must be free to
	// create the Secret.
	fresh, err := CredentialsComponent(cluster, credentials.Password{Value: "s3cret"})
	require.NoError(t, err)
	objects, err = fresh.Preview()
	require.NoError(t, err)
	for _, obj := range objects {
		assert.Empty(t, obj.GetAnnotations(), obj.GetName())
	}
}

// User pod labels must not override the discovery labels that extensions
// find the pods by.
func TestPodLabelsDoNotOverrideDiscoveryLabels(t *testing.T) {
	t.Parallel()

	cluster := goldenRealisticElasticsearchCluster()
	cluster.Spec.PodLabels = map[string]string{
		"camunda.io/elasticsearch-cluster": "someone-else",
		"camunda.io/component":             "not-elasticsearch",
		"team":                             "platform",
	}
	comp, err := ElasticsearchComponent(cluster, cluster.Spec, nil)
	require.NoError(t, err)

	objects, err := comp.Preview()
	require.NoError(t, err)
	var es *esv1.Elasticsearch
	for _, obj := range objects {
		if typed, ok := obj.(*esv1.Elasticsearch); ok {
			es = typed
		}
	}
	require.NotNil(t, es)

	labels := es.Spec.NodeSets[0].PodTemplate.Labels
	assert.Equal(t, cluster.Name, labels["camunda.io/elasticsearch-cluster"])
	assert.Equal(t, "elasticsearch", labels["camunda.io/component"])
	assert.Equal(t, "platform", labels["team"])
}
