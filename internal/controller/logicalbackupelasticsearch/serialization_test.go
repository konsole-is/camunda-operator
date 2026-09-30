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

package logicalbackupelasticsearch

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin"
	"github.com/konsole-is/camunda-operator/pkg/clusterclaim"
)

func pending(name string, created time.Time) *v1.LogicalBackupElasticsearch {
	return &v1.LogicalBackupElasticsearch{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:         "ns",
			Name:              name,
			CreationTimestamp: metav1.NewTime(created),
		},
	}
}

func TestBlocksIsATotalOrder(t *testing.T) {
	at := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	a := pending("m-backup", at)

	// The started sibling blocks regardless of order.
	started := pending("zzz", at.Add(time.Hour))
	started.Status.BackupID = 42
	assert.True(t, blocks(started, a))

	// The older creation time wins before any tie-break.
	older := pending("zzz", at.Add(-time.Second))
	assert.True(t, blocks(older, a))
	assert.False(t, blocks(a, older))

	// Same time: the smaller name goes first, and only one of the two
	// yields. Names are unique in the one namespace both backups share.
	first := pending("a-backup", at)
	assert.True(t, blocks(first, a))
	assert.False(t, blocks(a, first))
}

func TestFailureReasonNamesTheFailingParts(t *testing.T) {
	tests := []struct {
		name   string
		status camundaadmin.BackupStatus
		want   string
	}{
		{
			name:   "state alone when nothing reported a reason",
			status: camundaadmin.BackupStatus{State: camundaadmin.StateIncomplete},
			want:   "INCOMPLETE",
		},
		{
			name: "aggregate reason alone",
			status: camundaadmin.BackupStatus{
				State:         camundaadmin.StateFailed,
				FailureReason: "repository unavailable",
			},
			want: "repository unavailable",
		},
		{
			name: "per-part reasons name their part",
			status: camundaadmin.BackupStatus{
				State: camundaadmin.StateFailed,
				Details: []camundaadmin.Detail{
					{Name: "camunda_webapps_1_8.9_part_1_of_2", State: "SUCCESS"},
					{Name: "camunda_webapps_1_8.9_part_2_of_2", State: "FAILED", Reason: "shard 3 unassigned"},
				},
			},
			want: "camunda_webapps_1_8.9_part_2_of_2: shard 3 unassigned",
		},
		{
			name:   "aggregate and parts together",
			status: statusWithAggregateAndParts(),
			want:   "partition 2 lost its snapshot; 2: leader changed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, failureReason(tt.status))
		})
	}
}

func statusWithAggregateAndParts() camundaadmin.BackupStatus {
	return camundaadmin.BackupStatus{
		State:         camundaadmin.StateFailed,
		FailureReason: "partition 2 lost its snapshot",
		Details:       []camundaadmin.Detail{{Name: "2", State: "FAILED", Reason: "leader changed"}},
	}
}

// A backup that waits names the holder by its kind, because a restore takes
// the same claim as a backup.
func TestClaimClusterNamesTheHolderByItsKind(t *testing.T) {
	tests := []struct {
		name   string
		holder client.Object
		kind   string
	}{
		{
			name: "a restore holds the cluster",
			holder: &v1.LogicalRestoreElasticsearch{
				ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "my-restore", UID: "uid-restore"},
			},
			kind: "LogicalRestoreElasticsearch",
		},
		{
			name: "another backup holds the cluster",
			holder: &v1.LogicalBackupElasticsearch{
				ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "weekly", UID: "uid-weekly"},
			},
			kind: "LogicalBackupElasticsearch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			s := runtime.NewScheme()
			require.NoError(t, scheme.AddToScheme(s))
			require.NoError(t, v1.AddToScheme(s))

			backup := &v1.LogicalBackupElasticsearch{
				ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "nightly", UID: "uid-nightly"},
				Spec:       v1.LogicalBackupElasticsearchSpec{ClusterRef: v1.ClusterRef{Name: "cc"}},
			}
			c := fake.NewClientBuilder().WithScheme(s).WithObjects(backup, tt.holder).Build()
			r := &Reconciler{Client: c, APIReader: c}

			holder := clusterclaim.Claimant{Kind: tt.kind, Name: tt.holder.GetName(), UID: tt.holder.GetUID()}
			blocking, err := clusterclaim.Claim(ctx, c, c, "ns", "cc", holder)
			require.NoError(t, err)
			require.Empty(t, blocking)

			message, err := r.claimCluster(ctx, backup)
			require.NoError(t, err)

			assert.Contains(t, message, holder.Display()+" holds CamundaCluster ns/cc")
			assert.NotContains(t, message, "backup "+tt.kind+"/")
			assert.Contains(t, message, "Only one backup or restore of a cluster runs at a time")
		})
	}
}
