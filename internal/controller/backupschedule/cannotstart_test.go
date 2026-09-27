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
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestCannotStart(t *testing.T) {
	binding := &v1.ManagementBinding{
		Endpoint:         "http://cc-zeebe.ns.svc:9600",
		Version:          "8.9.9",
		BackupRepository: "camunda",
	}
	cluster := func(binding *v1.ManagementBinding, status metav1.ConditionStatus, reason string) *v1.CamundaCluster {
		return &v1.CamundaCluster{Status: v1.CamundaClusterStatus{
			Management: binding,
			Conditions: []metav1.Condition{{Type: v1.ConditionReady, Status: status, Reason: reason}},
		}}
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
			want:        "Ready is False with reason InvalidReference",
		},
		{
			name:        "an RDBMS cluster whose Ready is Unknown does not start the backup",
			cluster:     cluster(binding, metav1.ConditionUnknown, v1.ReasonProgressing),
			storageType: v1.SecondaryStorageTypeRDBMS,
			want:        "Ready is Unknown with reason Progressing",
		},
		{
			name: "an RDBMS cluster without a Ready condition does not start the backup",
			cluster: &v1.CamundaCluster{Status: v1.CamundaClusterStatus{
				Management: binding,
			}},
			storageType: v1.SecondaryStorageTypeRDBMS,
			want:        "it reports no Ready condition yet",
		},
		{
			name:        "a degraded Elasticsearch cluster that publishes its binding starts the backup",
			cluster:     cluster(binding, metav1.ConditionFalse, string(component.Degraded)),
			storageType: v1.SecondaryStorageTypeElasticsearch,
			want:        "",
		},
		{
			name:        "an Elasticsearch cluster without a binding names its Ready reason",
			cluster:     cluster(nil, metav1.ConditionFalse, v1.ReasonInvalidReference),
			storageType: v1.SecondaryStorageTypeElasticsearch,
			want: "it has not published its management binding, " +
				"and Ready is False with reason InvalidReference",
		},
		{
			name: "an Elasticsearch cluster without a backup repository names its Ready reason",
			cluster: cluster(
				&v1.ManagementBinding{Endpoint: binding.Endpoint},
				metav1.ConditionFalse,
				v1.ReasonInvalidReference,
			),
			storageType: v1.SecondaryStorageTypeElasticsearch,
			want:        "it publishes no backup repository, and Ready is False with reason InvalidReference",
		},
		{
			name:        "a binding without an endpoint counts as not published",
			cluster:     cluster(&v1.ManagementBinding{}, metav1.ConditionTrue, v1.ReasonHealthy),
			storageType: v1.SecondaryStorageTypeRDBMS,
			want:        "it has not published its management binding, and Ready is True with reason Healthy",
		},
		{
			name:        "a cluster that the operator did not reconcile yet does not start the backup",
			cluster:     &v1.CamundaCluster{},
			storageType: v1.SecondaryStorageTypeElasticsearch,
			want:        "it has not published its management binding, and it reports no Ready condition yet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, cannotStart(tt.cluster, tt.storageType))
		})
	}
}
