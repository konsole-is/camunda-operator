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
	"fmt"
	"testing"
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
)

// recoveredClusterName is the cluster a server runs after one recovery.
const recoveredClusterName = "my-cluster-db-r1"

// archiveServerUID is the UID of archiveServer. The bucket directory of a
// server is derived from it, so the archive cases need one that does not move.
const archiveServerUID = "3f2b1c4d-5e6a-4b7c-8d9e-0f1a2b3c4d5e"

// archiveServer is the server that the archive cases render for.
func archiveServer() *v1.DatabaseServer {
	return &v1.DatabaseServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster-db",
			Namespace: "my-cluster-ns",
			UID:       archiveServerUID,
		},
	}
}

func TestDestinationPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		archive *ArchiveStorage
		want    string
	}{
		{
			name: "s3 under a base path",
			archive: &ArchiveStorage{Config: &v1.ObjectStorageConfig{
				Spec: v1.ObjectStorageConfigSpec{
					Type: v1.ObjectStorageTypeS3,
					S3:   &v1.S3Storage{BucketName: "backups", BasePath: "clusters", Region: "eu-west-1"},
				},
			}},
			want: "s3://backups/clusters/databaseserver/my-cluster-ns/" + ArchiveSegment(archiveServer()),
		},
		{
			name: "s3 at the bucket root",
			archive: &ArchiveStorage{Config: &v1.ObjectStorageConfig{
				Spec: v1.ObjectStorageConfigSpec{
					Type: v1.ObjectStorageTypeS3,
					S3:   &v1.S3Storage{BucketName: "backups", Region: "eu-west-1"},
				},
			}},
			want: "s3://backups/databaseserver/my-cluster-ns/" + ArchiveSegment(archiveServer()),
		},
		{
			name: "gcs",
			archive: &ArchiveStorage{Config: &v1.ObjectStorageConfig{
				Spec: v1.ObjectStorageConfigSpec{
					Type: v1.ObjectStorageTypeGCS,
					GCS:  &v1.GCSStorage{BucketName: "backups", BasePath: "clusters"},
				},
			}},
			want: "gs://backups/clusters/databaseserver/my-cluster-ns/" + ArchiveSegment(archiveServer()),
		},
		{
			name: "azure through the service endpoint of the account",
			archive: &ArchiveStorage{Config: &v1.ObjectStorageConfig{
				Spec: v1.ObjectStorageConfigSpec{
					Type: v1.ObjectStorageTypeAzureBlob,
					AzureBlob: &v1.AzureBlobStorage{
						AccountName: "backups", Container: "archive", BasePath: "clusters",
					},
				},
			}},
			want: "https://backups.blob.core.windows.net/archive/clusters/databaseserver/my-cluster-ns/" +
				ArchiveSegment(archiveServer()),
		},
		{
			name:    "no bucket",
			archive: nil,
			want:    "",
		},
		{
			name: "a type without its block",
			archive: &ArchiveStorage{Config: &v1.ObjectStorageConfig{
				Spec: v1.ObjectStorageConfigSpec{Type: v1.ObjectStorageTypeS3},
			}},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, destinationPath(tt.archive.resolveOrNil(archiveServer())))
		})
	}
}

// The objects of a deleted server stay in the bucket, and the Barman Cloud
// plugin refuses a fresh cluster whose archive is not empty. A server created
// again under the same name therefore has to write somewhere else.
func TestArchiveSegmentSeparatesIncarnations(t *testing.T) {
	t.Parallel()

	bucket := &ArchiveStorage{Config: &v1.ObjectStorageConfig{
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeS3,
			S3:   &v1.S3Storage{BucketName: "backups", BasePath: "clusters", Region: "eu-west-1"},
		},
	}}

	first, second := archiveServer(), archiveServer()
	second.UID = "8c7d6e5f-4a3b-4291-8071-1a2b3c4d5e6f"

	assert.NotEqual(t, ArchiveSegment(first), ArchiveSegment(second))
	assert.NotEqual(
		t,
		destinationPath(bucket.resolveOrNil(first)),
		destinationPath(bucket.resolveOrNil(second)),
	)
	assert.NotEqual(t, bucket.ArchiveLocation(first), bucket.ArchiveLocation(second))
}

// The directory of a server must never move: the archive it already wrote is
// there. The literal pins the rule against a later change of the hash.
func TestArchiveSegmentIsStableForOneUID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "my-cluster-db-a0d468fa", ArchiveSegment(archiveServer()))
}

// The pre-check answers with the same resolution the renderers use, so a
// bucket it accepts is one every renderer can render.
func TestValidateArchiveStorage(t *testing.T) {
	t.Parallel()

	valid := &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "backups"},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeS3,
			S3:   &v1.S3Storage{BucketName: "backups", Region: "eu-west-1"},
		},
	}
	require.NoError(t, ValidateArchiveStorage(valid))

	mismatched := &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "backups"},
		Spec:       v1.ObjectStorageConfigSpec{Type: v1.ObjectStorageTypeGCS},
	}
	assert.ErrorContains(t, ValidateArchiveStorage(mismatched), `declares type GCS without the matching block`)
}

// The plugin reads the region of an S3 bucket from a Secret key whatever
// authenticates the upload, so the Secret exists for workload identity too.
func TestArchiveSecretCarriesTheRegionUnderWorkloadIdentity(t *testing.T) {
	t.Parallel()

	archive := &ArchiveStorage{Config: &v1.ObjectStorageConfig{
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeS3,
			S3: &v1.S3Storage{
				BucketName: "backups",
				Region:     "eu-west-1",
				Auth:       v1.S3StorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity},
			},
		},
	}}

	resolved, err := archive.resolve(archiveServer())
	require.NoError(t, err)

	assert.Equal(t, map[string][]byte{regionKey: []byte("eu-west-1")}, resolved.secretData)
	require.NotNil(t, resolved.s3Credentials)
	assert.True(t, resolved.s3Credentials.InheritFromIAMRole)
	assert.Nil(t, resolved.s3Credentials.AccessKeyID)
	require.NotNil(t, resolved.s3Credentials.Region)
	assert.Equal(t, "my-cluster-db-archive", resolved.s3Credentials.Region.Name)
}

// A bucket addressed by endpoint has no region of its own, and every consumer
// signs for the same placeholder. Trailing slashes never reach the plugin: a
// doubled separator turns a valid endpoint into a signature failure.
func TestArchiveEndpointAndPlaceholderRegion(t *testing.T) {
	t.Parallel()

	archive := &ArchiveStorage{
		Config: &v1.ObjectStorageConfig{
			Spec: v1.ObjectStorageConfigSpec{
				Type: v1.ObjectStorageTypeS3,
				S3: &v1.S3Storage{
					BucketName: "backups",
					Endpoint:   "https://minio.example.com/",
					Auth:       v1.S3StorageAuth{Type: v1.ObjectStorageAuthTypeCredentials},
				},
			},
		},
		Credentials: &objectstore.Credentials{AccessKeyID: "root", SecretAccessKey: "secret"},
	}

	resolved, err := archive.resolve(archiveServer())
	require.NoError(t, err)

	assert.Equal(t, "https://minio.example.com", resolved.endpointURL)
	assert.Equal(t, []byte(v1.PlaceholderS3Region), resolved.secretData[regionKey])
	assert.Equal(t, []byte("root"), resolved.secretData[accessKeyIDKey])
	assert.Equal(t, []byte("secret"), resolved.secretData[secretAccessKeyKey])
}

// A base backup is what a recovery starts from, so an archive without one
// cannot be recovered to any point however well the uploads run.
func TestBaseBackupGuardHoldsTheArchiveUntilTheFirstBackup(t *testing.T) {
	t.Parallel()

	guard := baseBackupGuard(archiveServer(), nil, false)
	status, err := guard(cnpgv1.Cluster{})
	require.NoError(t, err)
	assert.Equal(t, concepts.GuardStatusBlocked, status.Status)
	assert.Contains(t, status.Reason, `the first base backup of "my-cluster-db" is not complete yet`)

	completed := metav1.NewTime(time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC))
	guard = baseBackupGuard(archiveServer(), &completed, false)
	status, err = guard(cnpgv1.Cluster{})
	require.NoError(t, err)
	assert.Equal(t, concepts.GuardStatusUnblocked, status.Status)

	// A suspended server has nothing to wait on: its schedule is suspended
	// with it, so no base backup can ever complete.
	guard = baseBackupGuard(archiveServer(), nil, true)
	status, err = guard(cnpgv1.Cluster{})
	require.NoError(t, err)
	assert.Equal(t, concepts.GuardStatusUnblocked, status.Status)
	assert.Contains(t, status.Reason, "suspended")
}

// The retention the operator enforces on the bucket is the number the contract
// publishes, so the declared window and the enforced window are one.
func TestRetentionPolicyMatchesTheDeclaredWindow(t *testing.T) {
	t.Parallel()

	assert.Empty(t, retentionPolicy(v1.DatabaseServerSpec{}))
	assert.Equal(t, "30d", retentionPolicy(v1.DatabaseServerSpec{
		Archive: &v1.DatabaseServerArchiveSpec{RetentionPeriodDays: 30},
	}))
}

// A recovered cluster must archive under its own serverName, or it overwrites
// the archive it recovered from.
func TestArchivePluginNamesTheClusterAsTheArchiveServer(t *testing.T) {
	t.Parallel()

	server := archiveServer()
	server.Status.Cluster = recoveredClusterName

	plugin := archivePlugin(server)

	assert.Equal(t, BarmanPluginName, plugin.Name)
	require.NotNil(t, plugin.IsWALArchiver)
	assert.True(t, *plugin.IsWALArchiver)
	assert.Equal(t, "my-cluster-db", plugin.Parameters["barmanObjectName"])
	assert.Equal(t, recoveredClusterName, plugin.Parameters["serverName"])
}

// The plugin entry names the ObjectStore by name, so a cluster that keeps it
// while another owner holds that name writes its write-ahead log into the
// bucket, and under the credentials, of that owner. A taken name therefore
// takes the archive off the cluster and the base backup schedule with it.
func TestATakenObjectStoreTakesTheArchiveOffTheCluster(t *testing.T) {
	t.Parallel()

	server := archiveServer()
	merged := v1.DatabaseServerSpec{
		Version:     "17",
		StorageSize: new(resource.MustParse("1Gi")),
		Archive: &v1.DatabaseServerArchiveSpec{
			ObjectStorageRef: "backups", RetentionPeriodDays: 30,
		},
	}
	archive := &ArchiveStorage{Config: &v1.ObjectStorageConfig{
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeS3,
			S3:   &v1.S3Storage{BucketName: "backups", Region: "eu-west-1"},
		},
	}}
	taken := ArchiveTakenMessage(ObjectStoreName(server), metav1.OwnerReference{
		Kind: "DatabaseServer", Name: "other",
	})

	free, _, err := ClusterComponent(server, merged, archive, "", nil, "")
	require.NoError(t, err)
	plugins := previewCluster(t, free).Spec.Plugins
	require.Len(t, plugins, 1)
	assert.Equal(t, ObjectStoreName(server), plugins[0].Parameters["barmanObjectName"])

	held, _, err := ClusterComponent(server, merged, archive, taken, nil, "")
	require.NoError(t, err)
	assert.Empty(t, previewCluster(t, held).Spec.Plugins)

	// The schedule takes its base backups through that same plugin entry, so
	// a cluster without one has nothing to write a base backup with. The
	// schedule is registered for deletion instead, and Preview renders no
	// resource registered that way.
	//
	// The ObjectStore and the Secret stay registered. They describe the
	// bucket, and the archive that the server wrote before is in it.
	assert.Equal(
		t, []string{"*v1.Secret", "*barmanobjectstore.ObjectStore", "*v1.ScheduledBackup"},
		previewKinds(t, server, merged, archive, ""),
	)
	assert.Equal(
		t, []string{"*v1.Secret", "*barmanobjectstore.ObjectStore"},
		previewKinds(t, server, merged, archive, taken),
	)
}

// previewKinds renders the archive component of a server and returns the Go
// type of every object it applies, in registration order. A render carries no
// TypeMeta, so the Go type is what names the object here.
func previewKinds(
	t *testing.T,
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
	archive *ArchiveStorage,
	archiveTaken string,
) []string {
	t.Helper()

	comp, err := ArchiveComponent(server, merged, archive, nil, "", archiveTaken)
	require.NoError(t, err)

	objects, err := comp.Preview()
	require.NoError(t, err)

	kinds := make([]string, 0, len(objects))
	for _, object := range objects {
		kinds = append(kinds, fmt.Sprintf("%T", object))
	}

	return kinds
}

// The location is what an archive interval is compared by, so it has to change
// whenever a key written through the contract lands somewhere else. The
// endpoint and the region select the service that answers, and neither reaches
// the URL the plugin is given.
func TestArchiveLocationSeparatesTheService(t *testing.T) {
	t.Parallel()

	s3 := func(endpoint, region string) *ArchiveStorage {
		return &ArchiveStorage{Config: &v1.ObjectStorageConfig{
			Spec: v1.ObjectStorageConfigSpec{
				Type: v1.ObjectStorageTypeS3,
				S3: &v1.S3Storage{
					BucketName: "backups", BasePath: "clusters",
					Region: region, Endpoint: endpoint,
				},
			},
		}}
	}

	server := archiveServer()
	prefix := "s3://backups/clusters/databaseserver/my-cluster-ns/" + ArchiveSegment(server)

	first := s3("http://minio.a.svc:9000", "eu-west-1")
	second := s3("http://minio.b.svc:9000", "eu-west-1")
	region := s3("", "eu-west-1")
	other := s3("", "us-east-1")

	assert.Contains(t, first.ArchiveLocation(server), prefix)
	assert.NotEqual(t, first.ArchiveLocation(server), second.ArchiveLocation(server))
	assert.NotEqual(t, region.ArchiveLocation(server), other.ArchiveLocation(server))
	assert.Equal(t, first.ArchiveLocation(server), s3("http://minio.a.svc:9000", "eu-west-1").ArchiveLocation(server))

	// The URL the plugin is given carries neither, so it cannot stand in.
	assert.Equal(t, destinationPath(first.resolveOrNil(server)), destinationPath(second.resolveOrNil(server)))

	assert.Empty(t, (*ArchiveStorage)(nil).ArchiveLocation(server))
}

// A rollback records what Identity returns. A bucket with static credentials
// records nothing: the ObjectStore carries its own Secret, so holding that
// object holds the credentials with it.
func TestArchiveIdentity(t *testing.T) {
	t.Parallel()

	const role = "arn:aws:iam::123456789012:role/camunda"
	bucket := func(auth v1.S3StorageAuth) *v1.ObjectStorageConfig {
		return &v1.ObjectStorageConfig{
			Spec: v1.ObjectStorageConfigSpec{
				Type: v1.ObjectStorageTypeS3,
				S3:   &v1.S3Storage{BucketName: "backups", Region: "eu-west-1", Auth: auth},
			},
		}
	}
	held := &v1.RecoveryArchiveIdentity{
		Annotations: map[string]string{v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/held"},
	}

	tests := []struct {
		name    string
		archive *ArchiveStorage
		want    *v1.RecoveryArchiveIdentity
	}{
		{name: "no bucket", archive: nil, want: nil},
		{
			name: "static credentials",
			archive: &ArchiveStorage{Config: bucket(v1.S3StorageAuth{
				Type: v1.ObjectStorageAuthTypeCredentials,
				Credentials: &v1.S3Credentials{SecretRef: v1.S3CredentialsSecretRef{
					Name:               "keys",
					AccessKeyIDKey:     "accessKeyId",
					SecretAccessKeyKey: "secretAccessKey",
				}},
			})},
			want: nil,
		},
		{
			name: "workload identity",
			archive: &ArchiveStorage{Config: bucket(v1.S3StorageAuth{
				Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
				WorkloadIdentity: &v1.S3WorkloadIdentity{RoleARN: role},
			})},
			want: &v1.RecoveryArchiveIdentity{
				Annotations: map[string]string{v1.IRSARoleARNAnnotation: role},
			},
		},
		{
			name: "a held identity wins over the contract",
			archive: &ArchiveStorage{
				Config: bucket(v1.S3StorageAuth{
					Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
					WorkloadIdentity: &v1.S3WorkloadIdentity{RoleARN: role},
				}),
				HeldIdentity: held,
			},
			want: held,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, test.archive.Identity())
		})
	}
}
