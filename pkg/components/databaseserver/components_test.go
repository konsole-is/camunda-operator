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

package databaseserver

import (
	"flag"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"github.com/sourcehawk/operator-component-framework/pkg/testing/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/barmanobjectstore"
)

// updateGolden refreshes the golden manifests with the rendered output:
// go test ./pkg/components/databaseserver/ -run Golden -update-golden
var updateGolden = flag.Bool("update-golden", false, "update golden files")

// goldenScheme registers every type for which the golden serializer must
// resolve TypeMeta.
func goldenScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, cnpgv1.AddToScheme(scheme))
	require.NoError(t, barmanobjectstore.AddToScheme(scheme))
	require.NoError(t, monitoringv1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	return scheme
}

// goldenServerUID is the UID of every golden fixture. The bucket directory of
// a server is derived from it, so the goldens need one that does not move.
const goldenServerUID = "b7e4d2a1-3c58-4f60-9a1d-2e5f8c0b4d71"

// goldenMinimalDatabaseServer is the minimal example of the CRD doc with a
// deterministic name, resolved against the "standard" preset of the doc.
func goldenMinimalDatabaseServer() (*v1.DatabaseServer, *v1.DatabaseServerPresetSpec) {
	server := &v1.DatabaseServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster-db",
			Namespace: "my-cluster-ns",
			UID:       goldenServerUID,
		},
		Spec: v1.DatabaseServerSpec{
			PresetRef:            "standard",
			DatabaseServerConfig: "my-database-server",
		},
	}
	preset := &v1.DatabaseServerPresetSpec{
		Server: v1.DatabaseServerSpec{
			Version:     "17",
			Instances:   new(int32(2)),
			StorageSize: new(resource.MustParse("64Gi")),
		},
	}

	return server, preset
}

// goldenRealisticDatabaseServer is the realistic example of the CRD doc with a
// deterministic name. It adds every optional rendering input, so the goldens
// pin all of them.
func goldenRealisticDatabaseServer() *v1.DatabaseServer {
	return &v1.DatabaseServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster-db",
			Namespace: "my-cluster-ns",
			UID:       goldenServerUID,
		},
		Spec: v1.DatabaseServerSpec{
			Version:   "17",
			Instances: new(int32(3)),
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
			WALStorageSize:   new(resource.MustParse("16Gi")),
			ServiceAccount: &v1.DatabaseServerServiceAccountSpec{
				Annotations: map[string]string{
					"eks.amazonaws.com/role-arn": "arn:aws:iam::123456789012:role/my-archive-role",
				},
			},
			Scheduling: &v1.SchedulingSpec{
				Tolerations: []corev1.Toleration{{
					Key:      "dedicated",
					Operator: corev1.TolerationOpEqual,
					Value:    "postgres",
					Effect:   corev1.TaintEffectNoSchedule,
				}},
			},
			PodLabels:      map[string]string{"team": "platform"},
			PodAnnotations: map[string]string{"prometheus.io/scrape": "true"},
			Monitoring: &v1.DatabaseServerMonitoringSpec{
				PodMonitor: &v1.PodMonitorSpec{
					Enabled:  true,
					Labels:   map[string]string{"release": "prometheus"},
					Interval: "30s",
				},
			},
			DatabaseServerConfig: "my-database-server",
		},
	}
}

// assertDatabaseServerGoldens renders the components for the merged spec and
// pins each against its golden manifest under dir. The monitoring component is
// pinned only when the fixture enables scraping.
func assertDatabaseServerGoldens(
	t *testing.T,
	dir string,
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
	archive *ArchiveStorage,
) {
	t.Helper()

	scheme := goldenScheme(t)
	base := filepath.Join("testdata", "golden", dir)

	cluster, _, err := ClusterComponent(server, merged, archive, nil, "")
	require.NoError(t, err)
	golden.AssertComponentYAML(
		t, filepath.Join(base, "cluster.yaml"), cluster,
		golden.WithScheme(scheme), golden.Update(*updateGolden),
	)

	// Preview renders the desired state of a resource whatever its gate says,
	// so a server without an archive would pin an ObjectStore that the
	// component only ever deletes.
	if merged.Archive != nil {
		archiveComp, err := ArchiveComponent(server, merged, archive, nil, "")
		require.NoError(t, err)
		golden.AssertComponentYAML(
			t, filepath.Join(base, "archive.yaml"), archiveComp,
			golden.WithScheme(scheme), golden.Update(*updateGolden),
		)
	}

	contract, err := ContractComponent(server, merged, "")
	require.NoError(t, err)
	golden.AssertComponentYAML(
		t, filepath.Join(base, "contract.yaml"), contract,
		golden.WithScheme(scheme), golden.Update(*updateGolden),
	)

	if MonitoringEnabled(merged) {
		monitoring, err := MonitoringComponent(server, merged, true, "")
		require.NoError(t, err)
		golden.AssertComponentYAML(
			t, filepath.Join(base, "monitoring.yaml"), monitoring,
			golden.WithScheme(scheme), golden.Update(*updateGolden),
		)
	}
}

func TestDatabaseServerGoldenMinimal(t *testing.T) {
	t.Parallel()

	server, preset := goldenMinimalDatabaseServer()

	assertDatabaseServerGoldens(t, "minimal", server, MergePreset(server.Spec, preset), nil)
}

func TestDatabaseServerGoldenRealistic(t *testing.T) {
	t.Parallel()

	server := goldenRealisticDatabaseServer()

	assertDatabaseServerGoldens(t, "realistic", server, MergePreset(server.Spec, nil), nil)
}

// goldenBucketName is the bucket contract that the archive cases reference.
const goldenBucketName = "my-backup-config"

// archiveBucket returns an S3 bucket contract with the given auth, for the
// golden cases that archive to one.
func archiveBucket(auth v1.S3StorageAuth) *v1.ObjectStorageConfig {
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

// archiveSpec is the archive block that every archive golden case uses.
func archiveSpec() *v1.DatabaseServerArchiveSpec {
	return &v1.DatabaseServerArchiveSpec{
		ObjectStorageRef:    goldenBucketName,
		RetentionPeriodDays: 30,
		BaseBackupSchedule:  DefaultBaseBackupSchedule,
	}
}

// A server whose bucket uses workload identity gets the identity annotation on
// the ServiceAccount that CloudNativePG creates, and an archive Secret that
// carries the region alone: the plugin reads the region of an S3 bucket from a
// Secret key whatever authenticates it.
func TestDatabaseServerGoldenArchiveWorkloadIdentity(t *testing.T) {
	t.Parallel()

	server, preset := goldenMinimalDatabaseServer()
	server.Spec.Archive = archiveSpec()
	archive := &ArchiveStorage{
		Config: archiveBucket(v1.S3StorageAuth{
			Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
			WorkloadIdentity: &v1.S3WorkloadIdentity{RoleARN: "arn:aws:iam::123456789012:role/camunda"},
		}),
	}

	assertDatabaseServerGoldens(
		t, "archive-workload-identity", server, MergePreset(server.Spec, preset), archive,
	)
}

// A server whose bucket holds static keys gets them in the archive Secret, and
// the ObjectStore points every credential at that Secret.
func TestDatabaseServerGoldenArchiveCredentials(t *testing.T) {
	t.Parallel()

	server, preset := goldenMinimalDatabaseServer()
	server.Spec.Archive = archiveSpec()
	archive := &ArchiveStorage{
		Config: archiveBucket(v1.S3StorageAuth{
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

	assertDatabaseServerGoldens(
		t, "archive-credentials", server, MergePreset(server.Spec, preset), archive,
	)
}

// A GCS bucket with a static key carries the whole service-account JSON into
// one Secret key, which barman-cloud reads as a file.
func TestDatabaseServerGoldenArchiveGCSCredentials(t *testing.T) {
	t.Parallel()

	server, preset := goldenMinimalDatabaseServer()
	server.Spec.Archive = archiveSpec()
	archive := &ArchiveStorage{
		Config: &v1.ObjectStorageConfig{
			ObjectMeta: metav1.ObjectMeta{Name: goldenBucketName},
			Spec: v1.ObjectStorageConfigSpec{
				Type: v1.ObjectStorageTypeGCS,
				GCS: &v1.GCSStorage{
					BucketName: "camunda-backups",
					BasePath:   "clusters",
					Auth: v1.GCSStorageAuth{
						Type: v1.ObjectStorageAuthTypeCredentials,
						Credentials: &v1.GCSCredentials{
							SecretRef: v1.SecretKeyRef{Name: "gcs-key", Namespace: "camunda", Key: "key.json"},
						},
					},
				},
			},
		},
		Credentials: &objectstore.Credentials{
			ServiceAccountJSON: []byte(`{"type":"service_account","project_id":"camunda"}`),
		},
	}

	assertDatabaseServerGoldens(
		t, "archive-gcs-credentials", server, MergePreset(server.Spec, preset), archive,
	)
}

// Azure workload identity is the widest rendering of an archive bucket: the
// plugin takes the identity of the pod, and the pods carry the label without
// which the Azure webhook injects nothing.
func TestDatabaseServerGoldenArchiveAzureWorkloadIdentity(t *testing.T) {
	t.Parallel()

	server, preset := goldenMinimalDatabaseServer()
	server.Spec.Archive = archiveSpec()
	archive := &ArchiveStorage{
		Config: &v1.ObjectStorageConfig{
			ObjectMeta: metav1.ObjectMeta{Name: goldenBucketName},
			Spec: v1.ObjectStorageConfigSpec{
				Type: v1.ObjectStorageTypeAzureBlob,
				AzureBlob: &v1.AzureBlobStorage{
					AccountName: "camundabackups",
					Container:   "archive",
					BasePath:    "clusters",
					Auth: v1.AzureBlobStorageAuth{
						Type: v1.ObjectStorageAuthTypeWorkloadIdentity,
						WorkloadIdentity: &v1.AzureBlobWorkloadIdentity{
							ClientID: "00000000-0000-0000-0000-000000000000",
						},
					},
				},
			},
		},
	}

	assertDatabaseServerGoldens(
		t, "archive-azure-workload-identity", server, MergePreset(server.Spec, preset), archive,
	)
}

// The base backup schedule is the one part of the declared state that a
// suspension moves. The hibernation of the cluster is a runtime mutation the
// wrapper writes when the framework suspends the component, so the annotation
// this golden holds is the off value that every applied Cluster carries.
//
// TestSuspensionKeepsTheDeclaredState guards the rest of the rendering against
// a future field that moves with the suspension. This case is what a reader
// opens to see what a suspended server asks for.
func TestDatabaseServerGoldenSuspended(t *testing.T) {
	t.Parallel()

	server := goldenRealisticDatabaseServer()
	server.Spec.Archive = archiveSpec()
	server.Spec.Suspend = true
	archive := &ArchiveStorage{
		Config: archiveBucket(v1.S3StorageAuth{
			Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
			WorkloadIdentity: &v1.S3WorkloadIdentity{RoleARN: "arn:aws:iam::123456789012:role/camunda"},
		}),
	}

	assertDatabaseServerGoldens(t, "suspended", server, MergePreset(server.Spec, nil), archive)
}

// renderComponent returns the YAML of comp, the same bytes a golden holds.
func renderComponent(t *testing.T, comp *component.Component) string {
	t.Helper()

	objects, err := comp.Preview()
	require.NoError(t, err)

	out, err := golden.SerializeComponent(objects, goldenScheme(t))
	require.NoError(t, err)

	return string(out)
}

// Suspension is a runtime mutation: the wrapper writes the hibernation
// annotation on the cluster, and the base backup schedule is suspended with
// it. Nothing else about the declared state may move, so the two renders are
// compared rather than pinned twice. The data volume of an instance and the
// archive the server writes are what this protects.
func TestSuspensionKeepsTheDeclaredState(t *testing.T) {
	t.Parallel()

	archive := &ArchiveStorage{
		Config: archiveBucket(v1.S3StorageAuth{
			Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
			WorkloadIdentity: &v1.S3WorkloadIdentity{RoleARN: "arn:aws:iam::123456789012:role/camunda"},
		}),
	}

	render := func(suspend bool) (cluster, contract, archived string) {
		server, preset := goldenMinimalDatabaseServer()
		server.Spec.Archive = archiveSpec()
		server.Spec.Suspend = suspend
		merged := MergePreset(server.Spec, preset)

		clusterComp, _, err := ClusterComponent(server, merged, archive, nil, "")
		require.NoError(t, err)
		contractComp, err := ContractComponent(server, merged, "")
		require.NoError(t, err)
		archiveComp, err := ArchiveComponent(server, merged, archive, nil, "")
		require.NoError(t, err)

		return renderComponent(t, clusterComp),
			renderComponent(t, contractComp),
			renderComponent(t, archiveComp)
	}

	runningCluster, runningContract, runningArchive := render(false)
	suspendedCluster, suspendedContract, suspendedArchive := render(true)

	assert.Equal(t, runningCluster, suspendedCluster, "the declared cluster must not move")
	assert.Equal(t, runningContract, suspendedContract, "the published contract must not move")

	// The one field suspension declares is the suspension of the schedule. A
	// hibernated server has no instances, so every slot the schedule reached
	// would start a backup that cannot run.
	assert.Equal(
		t,
		strings.Replace(runningArchive, "suspend: false", "suspend: true", 1),
		suspendedArchive,
		"suspension must reach the base backup schedule and nothing else",
	)
}

// A server without the PodMonitor CRD must not render one, even when scraping
// is enabled on the spec. The controller leaves the resource out instead of
// failing every reconcile against a missing kind.
func TestMonitoringComponentOmitsUnsupportedPodMonitor(t *testing.T) {
	t.Parallel()

	server := goldenRealisticDatabaseServer()

	comp, err := MonitoringComponent(server, server.Spec, false, "")
	require.NoError(t, err)

	objects, err := comp.Preview()
	require.NoError(t, err)
	assert.Empty(t, objects)
}

// User pod labels must not override the discovery labels that extensions find
// the instance pods by.
func TestPodLabelsDoNotOverrideDiscoveryLabels(t *testing.T) {
	t.Parallel()

	server := goldenRealisticDatabaseServer()
	server.Spec.PodLabels = map[string]string{
		"camunda.io/database-server": "someone-else",
		"camunda.io/component":       "not-postgres",
		"team":                       "platform",
	}

	comp, _, err := ClusterComponent(server, server.Spec, nil, nil, "")
	require.NoError(t, err)

	cluster := previewCluster(t, comp)
	inherited := cluster.Spec.InheritedMetadata.Labels
	assert.Equal(t, server.Name, inherited["camunda.io/database-server"])
	assert.Equal(t, componentLabel, inherited["camunda.io/component"])
	assert.Equal(t, "platform", inherited["team"])
}

// The image of a server is resolved through pkg/images, so the platform config
// renames it the way it renames every other image.
func TestClusterImageComesFromThePlatformConfig(t *testing.T) {
	t.Parallel()

	server, preset := goldenMinimalDatabaseServer()
	merged := MergePreset(server.Spec, preset)

	comp, _, err := ClusterComponent(server, merged, nil, nil, "")
	require.NoError(t, err)
	assert.Equal(t, "ghcr.io/cloudnative-pg/postgresql:17", previewCluster(t, comp).Spec.ImageName)

	platform := &v1.CamundaPlatformConfigSpec{
		Images: &v1.ImagesSpec{Postgres: "mirror.example.com/postgresql"},
	}
	comp, _, err = ClusterComponent(server, merged, nil, platform, "")
	require.NoError(t, err)
	assert.Equal(t, "mirror.example.com/postgresql:17", previewCluster(t, comp).Spec.ImageName)
}

// A CloudNativePG cluster of the name a server derives holds a database when
// somebody else built it. ocf blocks the apply while another owner controls
// it, and this guard covers the case it does not: a cluster that nothing
// controls at all.
func TestClusterGuardBlocksAHeldName(t *testing.T) {
	t.Parallel()

	free, err := takenGuard("")(cnpgv1.Cluster{})
	require.NoError(t, err)
	assert.Equal(t, concepts.GuardStatusUnblocked, free.Status)

	held := ClusterTakenMessage("camunda", &metav1.OwnerReference{
		Kind: "DatabaseServer", Name: "other",
	})
	taken, err := takenGuard(held)(cnpgv1.Cluster{})
	require.NoError(t, err)
	assert.Equal(t, concepts.GuardStatusBlocked, taken.Status)
	assert.Contains(t, taken.Reason, `CloudNativePG cluster "camunda"`)
	assert.Contains(t, taken.Reason, `DatabaseServer "other"`)

	// A cluster that nothing controls is refused too. It holds a database
	// that no operator is minding, and the apply would make it a child of
	// this server.
	assert.Contains(t, ClusterTakenMessage("camunda", nil), "no owner controls it")
}

// previewCluster renders comp and returns the single CloudNativePG cluster in
// it.
func previewCluster(t *testing.T, comp *component.Component) *cnpgv1.Cluster {
	t.Helper()

	objects, err := comp.Preview()
	require.NoError(t, err)

	for _, obj := range objects {
		if cluster, ok := obj.(*cnpgv1.Cluster); ok {
			return cluster
		}
	}

	require.Fail(t, fmt.Sprintf("no CloudNativePG cluster in %d rendered objects", len(objects)))

	return nil
}
