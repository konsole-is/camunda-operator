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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// enabledNames returns the components of the enabled processes, in order.
func enabledNames(processes []Process) []string {
	var names []string
	for _, p := range processes {
		if p.Enabled {
			names = append(names, p.Component)
		}
	}
	return names
}

// Resolve always returns the six processes in the same order, so a
// component exists for every process that an earlier topology enabled.
func TestResolveReturnsEveryProcess(t *testing.T) {
	t.Parallel()

	got := Resolve(NewEffective(v1.CamundaClusterSpec{}))
	names := make([]string, 0, len(got))
	for _, p := range got {
		names = append(names, p.Component)
	}
	assert.Equal(
		t,
		[]string{
			ComponentZeebe,
			ComponentGateway,
			ComponentOperate,
			ComponentTasklist,
			ComponentAdmin,
			ComponentConnectors,
		},
		names,
	)
	assert.Equal(t, v1.ConditionOperateReady, got[2].ConditionType)
	assert.Equal(t, ProcessDeployment, got[2].Kind)
	assert.False(t, got[2].Enabled)
	assert.False(t, got[5].Enabled)
}

// The 8.9 default: a standalone gateway hosts every web application.
func TestResolveDefaultTopology(t *testing.T) {
	t.Parallel()

	got := Resolve(NewEffective(v1.CamundaClusterSpec{}))
	assert.Equal(t, []string{ComponentZeebe, ComponentGateway}, enabledNames(got))

	zeebe, gateway := got[0], got[1]
	assert.Equal(t, ComponentZeebe, zeebe.Component)
	assert.Equal(t, ProcessStatefulSet, zeebe.Kind)
	assert.Equal(t, int32(1), zeebe.Replicas)
	assert.Equal(t, []string{"broker", "consolidated-auth"}, zeebe.Profiles)
	assert.False(t, zeebe.EmbeddedGateway)
	assert.False(t, zeebe.ServesGRPC)
	assert.False(t, zeebe.ServesHTTP)
	assert.Equal(t, v1.ConditionZeebeReady, zeebe.ConditionType)

	assert.Equal(t, ComponentGateway, gateway.Component)
	assert.Equal(t, ProcessDeployment, gateway.Kind)
	assert.Equal(t, []string{"admin", "consolidated-auth", "gateway", "operate", "tasklist"}, gateway.Profiles)
	assert.True(t, gateway.ServesGRPC)
	assert.True(t, gateway.ServesHTTP)
	assert.Equal(t, v1.ConditionGatewayReady, gateway.ConditionType)
}

// An embedded gateway folds everything into the brokers.
func TestResolveAllInOne(t *testing.T) {
	t.Parallel()

	got := Resolve(NewEffective(v1.CamundaClusterSpec{
		Gateway: &v1.GatewaySpec{Mode: v1.ComponentModeEmbedded},
	}))
	assert.Equal(t, []string{ComponentZeebe}, enabledNames(got))

	zeebe := got[0]
	assert.Equal(t, []string{"admin", "broker", "consolidated-auth", "operate", "tasklist"}, zeebe.Profiles)
	assert.True(t, zeebe.EmbeddedGateway)
	assert.True(t, zeebe.ServesGRPC)
	assert.True(t, zeebe.ServesHTTP)
}

// A standalone web application is a gateway process that serves one
// application; the host drops that application's profile.
func TestResolveStandaloneWebApp(t *testing.T) {
	t.Parallel()

	got := Resolve(NewEffective(v1.CamundaClusterSpec{
		Operate: &v1.WebAppSpec{
			Mode:         v1.ComponentModeStandalone,
			WorkloadSpec: v1.WorkloadSpec{Replicas: new(int32(2))},
		},
	}))
	assert.Equal(t, []string{ComponentZeebe, ComponentGateway, ComponentOperate}, enabledNames(got))

	gateway, operate := got[1], got[2]
	assert.Equal(t, []string{"admin", "consolidated-auth", "gateway", "tasklist"}, gateway.Profiles)

	assert.Equal(t, ComponentOperate, operate.Component)
	assert.Equal(t, ProcessDeployment, operate.Kind)
	assert.Equal(t, int32(2), operate.Replicas)
	assert.Equal(t, []string{"consolidated-auth", "gateway", "operate"}, operate.Profiles)
	assert.False(t, operate.ServesGRPC)
	assert.True(t, operate.ServesHTTP)
	assert.Equal(t, v1.ConditionOperateReady, operate.ConditionType)
}

// With the gateway embedded, a standalone web application still gets its own
// process, and zeebe drops that profile.
func TestResolveStandaloneWebAppWithEmbeddedGateway(t *testing.T) {
	t.Parallel()

	got := Resolve(NewEffective(v1.CamundaClusterSpec{
		Gateway: &v1.GatewaySpec{Mode: v1.ComponentModeEmbedded},
		Admin:   &v1.WebAppSpec{Mode: v1.ComponentModeStandalone},
	}))
	assert.Equal(t, []string{ComponentZeebe, ComponentAdmin}, enabledNames(got))

	assert.Equal(t, []string{"broker", "consolidated-auth", "operate", "tasklist"}, got[0].Profiles)
	assert.Equal(t, ComponentAdmin, got[4].Component)
	assert.Equal(t, []string{"admin", "consolidated-auth", "gateway"}, got[4].Profiles)
	assert.Equal(t, v1.ConditionAdminReady, got[4].ConditionType)
}

// Every web application standalone: five enabled processes in a stable
// order.
func TestResolveSeparated(t *testing.T) {
	t.Parallel()

	got := Resolve(NewEffective(v1.CamundaClusterSpec{
		Operate:  &v1.WebAppSpec{Mode: v1.ComponentModeStandalone},
		Tasklist: &v1.WebAppSpec{Mode: v1.ComponentModeStandalone},
		Admin:    &v1.WebAppSpec{Mode: v1.ComponentModeStandalone},
	}))
	assert.Equal(
		t,
		[]string{ComponentZeebe, ComponentGateway, ComponentOperate, ComponentTasklist, ComponentAdmin},
		enabledNames(got),
	)
	assert.Equal(t, []string{"consolidated-auth", "gateway"}, got[1].Profiles)
	assert.Equal(t, []string{"consolidated-auth", "gateway", "tasklist"}, got[3].Profiles)
	assert.Equal(t, v1.ConditionTasklistReady, got[3].ConditionType)
}

// Connectors are the last process: a Deployment without profiles that serves
// HTTP.
func TestResolveConnectors(t *testing.T) {
	t.Parallel()

	got := Resolve(NewEffective(v1.CamundaClusterSpec{
		Connectors: &v1.ConnectorsSpec{
			Enabled:      new(true),
			Version:      "8.9.7",
			WorkloadSpec: v1.WorkloadSpec{Replicas: new(int32(2))},
		},
	}))
	assert.Equal(t, []string{ComponentZeebe, ComponentGateway, ComponentConnectors}, enabledNames(got))

	connectors := got[5]
	assert.True(t, connectors.Enabled)
	assert.Equal(t, ComponentConnectors, connectors.Component)
	assert.Equal(t, ProcessDeployment, connectors.Kind)
	assert.Equal(t, int32(2), connectors.Replicas)
	assert.Empty(t, connectors.Profiles)
	assert.False(t, connectors.EmbeddedGateway)
	assert.False(t, connectors.ServesGRPC)
	assert.True(t, connectors.ServesHTTP)
	assert.Equal(t, v1.ConditionConnectorsReady, connectors.ConditionType)
}

func TestGatewayHost(t *testing.T) {
	t.Parallel()

	cluster := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Name: "my-cluster", Namespace: "my-cluster-ns"}}

	assert.Equal(t, "my-cluster-gateway", GatewayHost(cluster, NewEffective(v1.CamundaClusterSpec{})))
	assert.Equal(t, "my-cluster-zeebe", GatewayHost(cluster, NewEffective(v1.CamundaClusterSpec{
		Gateway: &v1.GatewaySpec{Mode: v1.ComponentModeEmbedded},
	})))
}

func TestNames(t *testing.T) {
	t.Parallel()

	cluster := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Name: "my-cluster", Namespace: "my-cluster-ns"}}

	assert.Equal(t, "my-cluster-zeebe", WorkloadName(cluster, ComponentZeebe))
	assert.Equal(t, "my-cluster-connectors", WorkloadName(cluster, ComponentConnectors))
	assert.Equal(t, "my-cluster-camunda-admin", AdminSecretName(cluster))
	assert.Equal(t, "my-cluster-camunda", ServiceAccountName(cluster, NewEffective(cluster.Spec)))
	named := NewEffective(v1.CamundaClusterSpec{ServiceAccount: &v1.ServiceAccountSpec{Name: "camunda-prod"}})
	assert.Equal(t, "camunda-prod", ServiceAccountName(cluster, named))
}
