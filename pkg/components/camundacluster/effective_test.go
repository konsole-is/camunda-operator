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

package camundacluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestEffectiveDefaultsWhenNothingIsSet(t *testing.T) {
	t.Parallel()

	e := NewEffective(v1.CamundaClusterSpec{})

	assert.Equal(t, int32(1), e.ZeebeReplicas())
	assert.Equal(t, int32(1), e.Partitions())
	assert.Equal(t, int32(1), e.ReplicationFactor())
	assert.Equal(t, resource.MustParse("10Gi"), e.StorageSize())
	assert.Equal(t, v1.ComponentModeStandalone, e.GatewayMode())
	assert.Equal(t, v1.ComponentModeEmbedded, e.OperateMode())
	assert.Equal(t, v1.ComponentModeEmbedded, e.TasklistMode())
	assert.Equal(t, v1.ComponentModeEmbedded, e.AdminMode())
	assert.False(t, e.ConnectorsEnabled())
	assert.Equal(t, v1.DeletePersistentVolumeClaimRetentionPolicyType, e.VolumeRetention())
	for _, component := range []string{"zeebe", "gateway", "operate", "tasklist", "admin", "connectors"} {
		assert.Equal(t, int32(1), e.Replicas(component), component)
		assert.Equal(t, v1.WorkloadSpec{}, e.Workload(component), component)
	}
}

func TestEffectiveReadsTheSpec(t *testing.T) {
	t.Parallel()

	e := NewEffective(v1.CamundaClusterSpec{
		Zeebe: &v1.ZeebeSpec{
			WorkloadSpec:      v1.WorkloadSpec{Replicas: new(int32(3)), PodLabels: map[string]string{"tier": "broker"}},
			Partitions:        new(int32(6)),
			ReplicationFactor: new(int32(2)),
			StorageSize:       new(resource.MustParse("32Gi")),
			PersistentVolumeClaimRetentionPolicy: &v1.PersistentVolumeClaimRetentionPolicy{
				WhenDeleted: v1.RetainPersistentVolumeClaimRetentionPolicyType,
			},
		},
		Gateway: &v1.GatewaySpec{
			Mode:         v1.ComponentModeEmbedded,
			WorkloadSpec: v1.WorkloadSpec{Replicas: new(int32(2))},
		},
		Operate: &v1.WebAppSpec{
			Mode:         v1.ComponentModeStandalone,
			WorkloadSpec: v1.WorkloadSpec{Replicas: new(int32(4))},
		},
		Tasklist:   &v1.WebAppSpec{Mode: v1.ComponentModeStandalone},
		Admin:      &v1.WebAppSpec{WorkloadSpec: v1.WorkloadSpec{Replicas: new(int32(0))}},
		Connectors: &v1.ConnectorsSpec{Enabled: new(true), WorkloadSpec: v1.WorkloadSpec{Replicas: new(int32(5))}},
	})

	assert.Equal(t, int32(3), e.ZeebeReplicas())
	assert.Equal(t, int32(3), e.Replicas("zeebe"))
	assert.Equal(t, int32(6), e.Partitions())
	assert.Equal(t, int32(2), e.ReplicationFactor())
	assert.Equal(t, resource.MustParse("32Gi"), e.StorageSize())
	assert.Equal(t, v1.RetainPersistentVolumeClaimRetentionPolicyType, e.VolumeRetention())
	assert.Equal(t, v1.ComponentModeEmbedded, e.GatewayMode())
	assert.Equal(t, v1.ComponentModeStandalone, e.OperateMode())
	assert.Equal(t, v1.ComponentModeStandalone, e.TasklistMode())
	assert.Equal(t, v1.ComponentModeEmbedded, e.AdminMode())
	assert.True(t, e.ConnectorsEnabled())
	assert.Equal(t, int32(2), e.Replicas("gateway"))
	assert.Equal(t, int32(4), e.Replicas("operate"))
	assert.Equal(t, int32(1), e.Replicas("tasklist"))
	assert.Equal(t, int32(0), e.Replicas("admin"), "an explicit zero is not a default")
	assert.Equal(t, int32(5), e.Replicas("connectors"))
	assert.Equal(t, map[string]string{"tier": "broker"}, e.Workload("zeebe").PodLabels)
	assert.Equal(t, int32(5), *e.Workload("connectors").Replicas)
}

func TestEffectiveUnknownComponentIsEmpty(t *testing.T) {
	t.Parallel()

	e := NewEffective(v1.CamundaClusterSpec{})

	assert.Equal(t, v1.WorkloadSpec{}, e.Workload("optimize"))
	assert.Equal(t, int32(0), e.Replicas("optimize"))
}
