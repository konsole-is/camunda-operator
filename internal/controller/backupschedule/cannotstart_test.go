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

package backupschedule

import (
	"context"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestCannotStart(t *testing.T) {
	binding := &v1.ManagementBinding{
		Endpoint:         "http://cc-zeebe.ns.svc:9600",
		Version:          "8.9.9",
		BackupRepository: "camunda",
	}
	cluster := func(binding *v1.ManagementBinding, status metav1.ConditionStatus, reason string) *v1.CamundaCluster {
		return &v1.CamundaCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "cc", Namespace: "ns"},
			Status: v1.CamundaClusterStatus{
				Management: binding,
				Conditions: []metav1.Condition{{Type: v1.ConditionReady, Status: status, Reason: reason}},
			},
		}
	}

	tests := []struct {
		name        string
		cluster     *v1.CamundaCluster
		storageType v1.SecondaryStorageType
		want        string
	}{
		{
			name:        "a ready RDBMS cluster starts the backup",
			cluster:     cluster(binding, metav1.ConditionTrue, v1.ReasonHealthy),
			storageType: v1.SecondaryStorageTypeRDBMS,
			want:        "",
		},
		{
			name:        "an RDBMS cluster whose Ready is False names its Ready reason",
			cluster:     cluster(binding, metav1.ConditionFalse, v1.ReasonInvalidReference),
			storageType: v1.SecondaryStorageTypeRDBMS,
			want:        `CamundaCluster "cc" is not ready: Ready is False with reason InvalidReference`,
		},
		{
			name:        "an RDBMS cluster whose Ready is Unknown does not start the backup",
			cluster:     cluster(binding, metav1.ConditionUnknown, v1.ReasonProgressing),
			storageType: v1.SecondaryStorageTypeRDBMS,
			want:        `CamundaCluster "cc" is not ready: Ready is Unknown with reason Progressing`,
		},
		{
			name: "an RDBMS cluster without a Ready condition does not start the backup",
			cluster: &v1.CamundaCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "cc", Namespace: "ns"},
				Status:     v1.CamundaClusterStatus{Management: binding},
			},
			storageType: v1.SecondaryStorageTypeRDBMS,
			want:        `CamundaCluster "cc" is not ready: it reports no Ready condition yet`,
		},
		{
			name:        "a degraded Elasticsearch cluster that publishes its binding starts the backup",
			cluster:     cluster(binding, metav1.ConditionFalse, string(component.Degraded)),
			storageType: v1.SecondaryStorageTypeElasticsearch,
			want:        "",
		},
		{
			name: "an Elasticsearch binding without a backup repository names the Ready reason",
			cluster: cluster(
				&v1.ManagementBinding{Endpoint: binding.Endpoint, Version: binding.Version},
				metav1.ConditionFalse,
				v1.ReasonInvalidReference,
			),
			storageType: v1.SecondaryStorageTypeElasticsearch,
			want: `CamundaCluster "cc" publishes no backup repository, ` +
				"and Ready is False with reason InvalidReference",
		},
		{
			name:        "a cluster without a binding does not start the backup of either kind",
			cluster:     cluster(nil, metav1.ConditionFalse, v1.ReasonInvalidReference),
			storageType: v1.SecondaryStorageTypeElasticsearch,
			want: "CamundaCluster ns/cc has not published its management binding yet, " +
				"and Ready is False with reason InvalidReference",
		},
		{
			name:        "a binding without an endpoint counts as not published",
			cluster:     cluster(&v1.ManagementBinding{}, metav1.ConditionTrue, v1.ReasonHealthy),
			storageType: v1.SecondaryStorageTypeRDBMS,
			want: "CamundaCluster ns/cc has not published its management binding yet, " +
				"and Ready is True with reason Healthy",
		},
		{
			name: "a cluster that the operator did not reconcile yet does not start the backup",
			cluster: &v1.CamundaCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "cc", Namespace: "ns"},
			},
			storageType: v1.SecondaryStorageTypeRDBMS,
			want: "CamundaCluster ns/cc has not published its management binding yet, " +
				"and it reports no Ready condition yet",
		},
		{
			name: "a binding that the backup controllers refuse does not start the backup",
			cluster: cluster(
				&v1.ManagementBinding{Endpoint: binding.Endpoint, Version: "8.10.0", BackupRepository: "camunda"},
				metav1.ConditionTrue,
				v1.ReasonHealthy,
			),
			storageType: v1.SecondaryStorageTypeElasticsearch,
			want: "the management binding of CamundaCluster ns/cc is unusable: " +
				`unsupported Camunda version "8.10.0": this client knows 8.9 only, ` +
				"and Ready is True with reason Healthy",
		},
	}

	r := &BackupScheduleReconciler{APIReader: fake.NewClientBuilder().Build()}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := r.cannotStart(context.Background(), tt.cluster, tt.storageType)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
