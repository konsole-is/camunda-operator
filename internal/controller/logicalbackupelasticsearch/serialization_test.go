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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin"
)

func pending(namespace, name string, created time.Time) *v1.LogicalBackupElasticsearch {
	return &v1.LogicalBackupElasticsearch{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:         namespace,
			Name:              name,
			CreationTimestamp: metav1.NewTime(created),
		},
	}
}

func TestBlocksIsATotalOrderAcrossNamespaces(t *testing.T) {
	at := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

	// Two backups in different namespaces, same name, same second: exactly
	// one of them must yield, or both start against one cluster.
	a := pending("alpha", "backup", at)
	b := pending("beta", "backup", at)
	assert.True(t, blocks(a, b), "alpha sorts before beta")
	assert.False(t, blocks(b, a))

	// The started sibling blocks regardless of order.
	started := pending("zulu", "zzz", at.Add(time.Hour))
	started.Status.BackupID = 42
	assert.True(t, blocks(started, a))

	// The older creation time wins before any tie-break.
	older := pending("zulu", "zzz", at.Add(-time.Second))
	assert.True(t, blocks(older, a))
	assert.False(t, blocks(a, older))

	// Same namespace and time: the smaller name goes first.
	first := pending("alpha", "a-backup", at)
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
