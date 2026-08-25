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
	"strings"
	"testing"
	"time"

	"github.com/sourcehawk/operator-component-framework/pkg/testing/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
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

// archiveInBucket returns archiveAt with the bucket that holds the record.
func archiveInBucket(serverName, bucket, from string, to *string) v1.ArchiveRecord {
	record := archiveAt(serverName, from, to)
	record.ObjectStorageRef = bucket

	return record
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
				archiveInBucket("camunda", "bucket-b", "2026-08-01T00:00:00Z", nil),
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

			got, err := SelectArchive(tt.history, atTime(tt.target).Time, bucket)
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

// TestSelectArchiveInAnotherBucket covers the point that a recorded interval
// holds in a bucket the spec no longer names. The recovered cluster is given
// one ObjectStore, so the point is out of reach until the operator can add a
// second one for the source.
func TestSelectArchiveInAnotherBucket(t *testing.T) {
	t.Parallel()

	closed := "2026-08-10T00:00:00Z"
	history := []v1.ArchiveRecord{
		archiveInBucket("camunda", "bucket-a", "2026-08-01T00:00:00Z", &closed),
		archiveInBucket("camunda", "bucket-b", closed, nil),
	}

	_, err := SelectArchive(history, atTime("2026-08-05T12:00:00Z").Time, "bucket-b")

	require.ErrorIs(t, err, ErrArchiveInAnotherBucket)
	assert.NotErrorIs(t, err, ErrNoArchiveHolds)
	assert.Contains(t, err.Error(), `ObjectStorageConfig "bucket-a"`)
	assert.Contains(t, err.Error(), `archives to "bucket-b" now`)
	assert.Contains(t, err.Error(), "not supported yet")
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

// Admission holds the name of a DatabaseServer to 47 characters, which is the
// 50 that CloudNativePG accepts for a cluster name less the three of "-r" and
// a single-digit counter. A name at that bound therefore reaches the recovered
// cluster whole.
func TestRecoveryClusterNameFitsCloudNativePG(t *testing.T) {
	t.Parallel()

	base := strings.Repeat("a", 47)
	server := &v1.DatabaseServer{
		ObjectMeta: metav1.ObjectMeta{Name: base, Namespace: "my-cluster-ns"},
		Status: v1.DatabaseServerStatus{
			Cluster: base,
			Archive: &v1.DatabaseServerArchiveStatus{
				History: []v1.ArchiveRecord{archiveAt(base, "2026-08-01T00:00:00Z", nil)},
			},
		},
	}

	name := RecoveryClusterName(server)
	assert.LessOrEqual(t, len(name), 50)
	assert.Empty(t, validation.IsDNS1035Label(name))
	assert.Equal(t, base+"-r1", name)
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
		server, MergePreset(server.Spec, preset), archive, nil, source, "2026-08-20T14:30:00Z",
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
		server, MergePreset(server.Spec, preset), archive, nil, source, "2026-08-20T14:30:00Z",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "records no recovery cluster")
}

// The outcome patch states one field. Everything else on the contract belongs
// to the component that publishes it, or to the consumer that asks.
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

	patch, err := RecoveryOutcomePatch(
		types.NamespacedName{Namespace: "camunda", Name: "my-database-server"}, outcome,
	)
	require.NoError(t, err)

	assert.Equal(t, "DatabaseServerConfig", patch.GetKind())
	assert.Equal(t, v1.GroupVersion.String(), patch.GetAPIVersion())
	assert.Equal(t, "camunda", patch.GetNamespace())
	assert.Equal(t, "my-database-server", patch.GetName())

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
