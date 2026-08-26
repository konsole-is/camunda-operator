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
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/testing/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// archiveAt returns an archive record of serverName that opens at from and
// closes at to. A nil to leaves it open. The record names no bucket, as a
// record written before the field existed does.
func archiveAt(serverName, from string, to *string) v1.ArchiveRecord {
	record := v1.ArchiveRecord{ServerName: serverName, From: atTime(from)}
	if to != nil {
		end := atTime(*to)
		record.To = &end
	}

	return record
}

// fixtureServerName is the cluster that the archive records of these fixtures
// belong to.
const fixtureServerName = "camunda"

// archiveInBucket returns archiveAt with the bucket that holds the record, and
// the location that bucket resolves to.
func archiveInBucket(bucket, from string, to *string) v1.ArchiveRecord {
	record := archiveAt(fixtureServerName, from, to)
	record.ObjectStorageRef = bucket
	record.Location = locationOf(bucket)

	return record
}

// locationOf is the destination that a bucket of these fixtures resolves to,
// in the shape ArchiveStorage renders.
func locationOf(bucket string) string {
	return "s3://" + bucket + "/clusters/databaseserver/my-cluster-ns/" + fixtureServerName
}

// atTime parses an RFC 3339 timestamp of a fixture.
func atTime(value string) metav1.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}

	return metav1.NewTime(parsed)
}

// recoveryHistory is a server that archived under its own name, was recovered
// once, and archives under the recovered name now.
func recoveryHistory() []v1.ArchiveRecord {
	closed := "2026-08-10T00:00:00Z"

	return []v1.ArchiveRecord{
		archiveAt("camunda", "2026-08-01T00:00:00Z", &closed),
		archiveAt("camunda-r1", closed, nil),
	}
}

func TestSelectArchive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		history []v1.ArchiveRecord
		target  string
		bucket  string
		want    string
		wantErr string
	}{
		{
			name:    "a point inside a closed archive comes from that archive",
			history: recoveryHistory(),
			target:  "2026-08-05T12:00:00Z",
			want:    "camunda",
		},
		{
			name:    "a point inside the archive the server writes now comes from it",
			history: recoveryHistory(),
			target:  "2026-08-20T12:00:00Z",
			want:    "camunda-r1",
		},
		{
			name:    "a point on the start of an archive comes from that archive",
			history: recoveryHistory(),
			target:  "2026-08-01T00:00:00Z",
			want:    "camunda",
		},
		{
			name:    "a point on a boundary belongs to the archive that starts there",
			history: recoveryHistory(),
			target:  "2026-08-10T00:00:00Z",
			want:    "camunda-r1",
		},
		{
			name:    "a point before every archive is refused",
			history: recoveryHistory(),
			target:  "2026-07-31T23:59:59Z",
			wantErr: "2026-07-31T23:59:59Z lies in none of those windows",
		},
		{
			name:    "a point after an archive that closed is refused",
			history: []v1.ArchiveRecord{archiveAt("camunda", "2026-08-01T00:00:00Z", new("2026-08-10T00:00:00Z"))},
			target:  "2026-08-11T00:00:00Z",
			wantErr: "camunda from 2026-08-01T00:00:00Z to 2026-08-10T00:00:00Z",
		},
		{
			name:    "a server that archived nothing is refused",
			history: nil,
			target:  "2026-08-11T00:00:00Z",
			wantErr: "It archived nothing",
		},
		{
			name: "a point in the bucket the server archives to now is answered",
			history: []v1.ArchiveRecord{
				archiveInBucket("bucket-b", "2026-08-01T00:00:00Z", nil),
			},
			bucket: "bucket-b",
			target: "2026-08-05T12:00:00Z",
			want:   "camunda",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bucket := tt.bucket
			if bucket == "" {
				bucket = "bucket-a"
			}

			got, err := SelectArchive(tt.history, atTime(tt.target).Time, locationOf(bucket), bucket)
			if tt.wantErr != "" {
				require.ErrorIs(t, err, ErrNoArchiveHolds)
				assert.Contains(t, err.Error(), tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got.ServerName)
			assert.Equal(t, bucket, got.ObjectStorageRef)
		})
	}
}

// TestSelectArchiveInAnotherLocation covers the point that a recorded interval
// holds somewhere the spec no longer archives to. The recovered cluster is
// given one ObjectStore, so the point is out of reach until the operator can
// add a second one for the source.
func TestSelectArchiveInAnotherLocation(t *testing.T) {
	t.Parallel()

	closed := "2026-08-10T00:00:00Z"
	history := []v1.ArchiveRecord{
		archiveInBucket("bucket-a", "2026-08-01T00:00:00Z", &closed),
		archiveInBucket("bucket-b", closed, nil),
	}

	_, err := SelectArchive(
		history, atTime("2026-08-05T12:00:00Z").Time, locationOf("bucket-b"), "bucket-b",
	)

	require.ErrorIs(t, err, ErrArchiveInAnotherLocation)
	assert.NotErrorIs(t, err, ErrNoArchiveHolds)
	assert.Contains(t, err.Error(), `ObjectStorageConfig "bucket-a"`)
	assert.Contains(t, err.Error(), locationOf("bucket-a"))
	assert.Contains(t, err.Error(), `ObjectStorageConfig "bucket-b"`)
	assert.Contains(t, err.Error(), locationOf("bucket-b"))
	assert.Contains(t, err.Error(), "not supported yet")
}

// An ObjectStorageConfig is mutable, and a delete and create keeps its name.
// The archive of a record therefore stays out of reach when the name is the
// one the spec still names but the bucket behind it has moved.
func TestSelectArchiveInAnotherLocationUnderOneName(t *testing.T) {
	t.Parallel()

	moved := archiveInBucket("bucket-a", "2026-08-01T00:00:00Z", nil)
	history := []v1.ArchiveRecord{moved}

	_, err := SelectArchive(
		history, atTime("2026-08-05T12:00:00Z").Time,
		"s3://moved-bucket/clusters/databaseserver/my-cluster-ns/camunda", "bucket-a",
	)

	require.ErrorIs(t, err, ErrArchiveInAnotherLocation)
	assert.Contains(t, err.Error(), moved.Location)
	assert.Contains(t, err.Error(), "s3://moved-bucket/")
}

// A record written before the location was recorded carries only the bucket
// contract. It is the archive the server writes now when that contract is the
// one it writes through. When the contract is another one, the record moved
// since and nothing says where its objects went, so it cannot answer either.
func TestSelectArchiveAdoptsALocationOnlyUnderItsOwnContract(t *testing.T) {
	t.Parallel()

	legacy := archiveAt(fixtureServerName, "2026-08-01T00:00:00Z", nil)
	legacy.ObjectStorageRef = "bucket-a"
	target := atTime("2026-08-05T12:00:00Z").Time

	got, err := SelectArchive(
		[]v1.ArchiveRecord{legacy}, target, locationOf("bucket-a"), "bucket-a",
	)
	require.NoError(t, err)
	assert.Equal(t, locationOf("bucket-a"), got.Location)

	_, err = SelectArchive(
		[]v1.ArchiveRecord{legacy}, target, locationOf("bucket-b"), "bucket-b",
	)
	require.ErrorIs(t, err, ErrArchiveInAnotherLocation)
	assert.Contains(t, err.Error(), `ObjectStorageConfig "bucket-a"`)
	assert.Contains(t, err.Error(), "was not recorded")
	assert.NotContains(t, err.Error(), locationOf("bucket-b"))
}

func TestRecoveryClusterName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status v1.DatabaseServerStatus
		want   string
	}{
		{
			name: "the first recovery of a server that wrote one archive is r1",
			status: v1.DatabaseServerStatus{
				Cluster: "camunda",
				Archive: &v1.DatabaseServerArchiveStatus{
					History: []v1.ArchiveRecord{archiveAt("camunda", "2026-08-01T00:00:00Z", nil)},
				},
			},
			want: "camunda-r1",
		},
		{
			name: "a server that wrote two archives recovers into r2",
			status: v1.DatabaseServerStatus{
				Cluster: "camunda-r1",
				Archive: &v1.DatabaseServerArchiveStatus{History: recoveryHistory()},
			},
			want: "camunda-r2",
		},
		{
			name: "a count that lands on the cluster that runs now takes the next number",
			status: v1.DatabaseServerStatus{
				Cluster: "camunda-r2",
				Archive: &v1.DatabaseServerArchiveStatus{History: recoveryHistory()},
			},
			want: "camunda-r3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := &v1.DatabaseServer{
				ObjectMeta: metav1.ObjectMeta{Name: "camunda", Namespace: "camunda-ns"},
				Status:     tt.status,
			}

			assert.Equal(t, tt.want, RecoveryClusterName(server))
		})
	}
}

// The Services that CloudNativePG derives from a cluster name are DNS labels,
// so the name of a recovery has to leave room for the longest suffix it adds.
func TestRecoveryClusterNameFitsAService(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("a", 60)
	server := &v1.DatabaseServer{
		ObjectMeta: metav1.ObjectMeta{Name: long, Namespace: "my-cluster-ns"},
		Status: v1.DatabaseServerStatus{
			Cluster: long,
			Archive: &v1.DatabaseServerArchiveStatus{
				History: []v1.ArchiveRecord{archiveAt(long, "2026-08-01T00:00:00Z", nil)},
			},
		},
	}

	name := RecoveryClusterName(server)
	assert.LessOrEqual(t, len(name+"-rw"), validation.DNS1035LabelMaxLength)
	assert.Empty(t, validation.IsDNS1035Label(name))
	assert.True(t, strings.HasSuffix(name, "-r1"), name)

	// The bound is on the name of the server, so the number of the recovery
	// survives it and two recoveries of one server never share a name.
	server.Status.Cluster = name
	assert.NotEqual(t, name, RecoveryClusterName(server))
}

// Admission holds the name of a DatabaseServer to 46 characters, which is the
// 50 that CloudNativePG accepts for a cluster name less the four of "-r99". A
// name at that bound therefore reaches a recovery cluster whole while the
// index stays below 100, and CloudNativePG accepts the shortened name above
// that. The index is the number of archive records, so a rollback is one of
// several things that advance it.
func TestRecoveryNameFitsCloudNativePG(t *testing.T) {
	t.Parallel()

	base := strings.Repeat("a", 46)

	tests := []struct {
		name  string
		n     int
		whole bool
	}{
		{name: "index 1", n: 1, whole: true},
		{name: "index 10", n: 10, whole: true},
		{name: "index 99", n: 99, whole: true},
		{name: "index 100", n: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			suffix := recoveryNameSeparator + strconv.Itoa(tt.n)
			name := recoveryName(base, tt.n)

			assert.LessOrEqual(t, len(name), cnpgClusterNameMaxLength)
			assert.Empty(t, validation.IsDNS1035Label(name))
			assert.True(t, strings.HasSuffix(name, suffix), name)

			if tt.whole {
				assert.Equal(t, base+suffix, name)
				return
			}

			assert.NotEqual(t, base+suffix, name)
		})
	}
}

// recoveryServer is the server of the recovery cases: it archives to a bucket
// with static keys, it runs from its original cluster, and it records the
// recovery it is about to build.
func recoveryServer() (*v1.DatabaseServer, *v1.DatabaseServerPresetSpec, *ArchiveStorage) {
	server, preset := goldenMinimalDatabaseServer()
	server.Spec.Archive = archiveSpec()
	server.Status = v1.DatabaseServerStatus{
		Cluster: server.Name,
		Archive: &v1.DatabaseServerArchiveStatus{
			History: []v1.ArchiveRecord{archiveAt(server.Name, "2026-08-01T00:00:00Z", nil)},
		},
	}
	server.Status.Recovery = &v1.DatabaseServerRecoveryStatus{
		RequestID:   "3f2b1c4d-5e6a-4b7c-8d9e-0f1a2b3c4d5e",
		Contract:    "my-database-server",
		RequestedBy: "my-cluster-ns/pitr-1",
		TargetTime:  "2026-08-20T14:30:00Z",
		Cluster:     RecoveryClusterName(server),
	}

	archive := &ArchiveStorage{
		Config: archiveBucket(v1.S3StorageAuth{
			Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
			WorkloadIdentity: &v1.S3WorkloadIdentity{RoleARN: "arn:aws:iam::123456789012:role/camunda"},
		}),
	}

	return server, preset, archive
}

// The recovered cluster carries the whole shape of the running one, plus the
// bootstrap that reads the archive of the cluster it replaces. It archives
// under its own name, so it never writes over the archive it read.
func TestRecoveryClusterGolden(t *testing.T) {
	t.Parallel()

	server, preset, archive := recoveryServer()
	source := server.Status.Archive.History[0]

	recovered, err := RecoveryCluster(
		server, MergePreset(server.Spec, preset), archive, "", nil, source, "2026-08-20T14:30:00Z",
	)
	require.NoError(t, err)

	golden.AssertComponentYAML(
		t,
		filepath.Join("testdata", "golden", "recovery", "cluster.yaml"),
		recoveryPreview{recovered},
		golden.WithScheme(goldenScheme(t)), golden.Update(*updateGolden),
	)

	assert.Equal(t, "my-cluster-db-r1", recovered.Name)
	assert.Equal(t, "my-cluster-db", recovered.Spec.Bootstrap.Recovery.Source)
	assert.Equal(t, "2026-08-20T14:30:00Z", recovered.Spec.Bootstrap.Recovery.RecoveryTarget.TargetTime)

	require.Len(t, recovered.Spec.ExternalClusters, 1)
	assert.Equal(
		t, map[string]string{
			"barmanObjectName": ObjectStoreName(server),
			"serverName":       "my-cluster-db",
		}, recovered.Spec.ExternalClusters[0].PluginConfiguration.Parameters,
	)

	// The bucket location never moves. Only the serverName separates the
	// archive of the recovered cluster from the one it recovered from.
	require.Len(t, recovered.Spec.Plugins, 1)
	assert.Equal(
		t, map[string]string{
			"barmanObjectName": ObjectStoreName(server),
			"serverName":       "my-cluster-db-r1",
		}, recovered.Spec.Plugins[0].Parameters,
	)
}

// recoveryPreview presents the rendered cluster as a one-object component
// preview, so the golden of a recovery reads like every other golden here.
type recoveryPreview struct {
	obj client.Object
}

func (r recoveryPreview) Preview() ([]client.Object, error) {
	return []client.Object{r.obj}, nil
}

// The renderer refuses a server that records no recovery. The name of the
// cluster comes from that record, and building a cluster under a name nobody
// wrote down would leave a cluster nothing ever cleans up.
func TestRecoveryClusterNeedsTheRecordFirst(t *testing.T) {
	t.Parallel()

	server, preset, archive := recoveryServer()
	source := server.Status.Archive.History[0]
	server.Status.Recovery = nil

	_, err := RecoveryCluster(
		server, MergePreset(server.Spec, preset), archive, "", nil, source, "2026-08-20T14:30:00Z",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "records no recovery cluster")
}

// The outcome patch states one field, and the identity of the object it was
// read from. Everything else on the contract belongs to the component that
// publishes it, or to the consumer that asks. The uid is what keeps the answer
// off a contract that was deleted and created again under one name: the API
// server refuses an apply that names another object.
func TestRecoveryOutcomePatchStatesOneField(t *testing.T) {
	t.Parallel()

	outcome := RecoveryOutcomeFor(
		v1.RecoveryRequest{
			RequestID:   "3f2b1c4d-5e6a-4b7c-8d9e-0f1a2b3c4d5e",
			RequestedBy: "camunda/pitr-1",
			TargetTime:  "2026-08-20T14:30:00Z",
		},
		v1.RecoveryResultCompleted,
		"",
		atTime("2026-08-20T15:02:11Z"),
	)

	patch, err := RecoveryOutcomePatch(&v1.DatabaseServerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "camunda", Name: "my-database-server", UID: "contract-uid",
		},
	}, outcome)
	require.NoError(t, err)

	assert.Equal(t, "DatabaseServerConfig", patch.GetKind())
	assert.Equal(t, v1.GroupVersion.String(), patch.GetAPIVersion())
	assert.Equal(t, "camunda", patch.GetNamespace())
	assert.Equal(t, "my-database-server", patch.GetName())
	assert.EqualValues(t, "contract-uid", patch.GetUID())

	pitr, found, err := unstructured.NestedMap(patch.Object, "spec", "pitr")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(
		t, map[string]any{
			"lastRecovery": map[string]any{
				"requestID":   "3f2b1c4d-5e6a-4b7c-8d9e-0f1a2b3c4d5e",
				"requestedBy": "camunda/pitr-1",
				"targetTime":  "2026-08-20T14:30:00Z",
				"completedAt": "2026-08-20T15:02:11Z",
				"result":      "Completed",
			},
		}, pitr,
	)
}

// A rollback records the identity of the bucket it reads, and the server holds
// that record while it holds the archive. An ObjectStorageConfig edited in
// place keeps its name, so the record is the only thing that keeps the two
// clusters on the identity of the archive: the running cluster writes its
// archive with it, and the recovering cluster reads the archive of the cluster
// it replaces with it.
func TestHeldIdentityStaysOnBothClusters(t *testing.T) {
	t.Parallel()

	const held = "arn:aws:iam::123456789012:role/held"
	const moved = "arn:aws:iam::123456789012:role/moved"

	server, preset, _ := recoveryServer()
	merged := MergePreset(server.Spec, preset)
	source := server.Status.Archive.History[0]

	archive := &ArchiveStorage{
		Config: archiveBucket(v1.S3StorageAuth{
			Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
			WorkloadIdentity: &v1.S3WorkloadIdentity{RoleARN: moved},
		}),
		// The pod label of an Azure identity is here on an S3 contract on
		// purpose: no S3 contract renders one, so a label that survives can
		// only have come from the record.
		HeldIdentity: &v1.RecoveryArchiveIdentity{
			Annotations: map[string]string{v1.IRSARoleARNAnnotation: held},
			PodLabels:   map[string]string{v1.AzureWorkloadIdentityUseLabel: "true"},
		},
	}

	clusterComp, _, err := ClusterComponent(server, merged, archive, "", nil, "")
	require.NoError(t, err)
	objects, err := clusterComp.Preview()
	require.NoError(t, err)
	require.Len(t, objects, 1)
	running, ok := objects[0].(*cnpgv1.Cluster)
	require.True(t, ok)

	recovered, err := RecoveryCluster(
		server, merged, archive, "", nil, source, "2026-08-20T14:30:00Z",
	)
	require.NoError(t, err)

	for name, rendered := range map[string]*cnpgv1.Cluster{
		"running": running, "recovering": recovered,
	} {
		t.Run(name, func(t *testing.T) {
			require.NotNil(t, rendered.Spec.ServiceAccountTemplate)
			assert.Equal(
				t,
				held,
				rendered.Spec.ServiceAccountTemplate.Metadata.Annotations[v1.IRSARoleARNAnnotation],
			)
			assert.Equal(
				t, "true", rendered.Spec.InheritedMetadata.Labels[v1.AzureWorkloadIdentityUseLabel],
			)
		})
	}
}
