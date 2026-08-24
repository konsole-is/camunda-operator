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

package camundamanagementcluster

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The settings that make one orchestration cluster report to Console. Camunda
// 8.9 reads them under camunda.console.ping. Camunda 8.10 renames the block to
// camunda.hub.ping and reads the old one only while the new one is off.
//
// Console ping configuration:
// https://docs.camunda.io/docs/self-managed/components/orchestration-cluster/zeebe/configuration/broker-config/
const (
	consolePingEnvEnabled     = "CAMUNDA_CONSOLE_PING_ENABLED"
	consolePingEnvEndpoint    = "CAMUNDA_CONSOLE_PING_ENDPOINT"
	consolePingEnvClusterName = "CAMUNDA_CONSOLE_PING_CLUSTERNAME"
	consolePingEnvPingPeriod  = "CAMUNDA_CONSOLE_PING_PINGPERIOD"
	hubPingEnvEnabled         = "CAMUNDA_HUB_PING_ENABLED"
	hubPingEnvEndpoint        = "CAMUNDA_HUB_PING_ENDPOINT"
	hubPingEnvClusterName     = "CAMUNDA_HUB_PING_CLUSTERNAME"
	hubPingEnvPingPeriod      = "CAMUNDA_HUB_PING_PINGPERIOD"
)

// The literal values of the ping.
const (
	// hubPingVersionFloor is the first Camunda version that reads
	// camunda.hub.ping.
	hubPingVersionFloor = "8.10.0"
	// pingPeriod is how often a cluster reports. It is the period of the
	// Camunda documentation, and the ping carries nothing that a shorter one
	// would keep fresher.
	pingPeriod = "1h"
	// pingEnabled turns the ping on.
	pingEnabled = "true"
)

// pingEnvNames is the name of each ping setting in one key set.
type pingEnvNames struct {
	enabled, endpoint, clusterName, pingPeriod string
}

// PingCollision is an entry of spec.extraEnv that holds a ping setting with
// valueFrom.
type PingCollision struct {
	// Index is where the entry sits in spec.extraEnv of the cluster given to
	// PingCollisions.
	Index int
	// Name is the ping setting that the entry holds.
	Name string
	// Manager is the field manager that set valueFrom, and is empty when no
	// entry of metadata.managedFields claims that field.
	Manager string
}

// PingEnv returns the environment entries that make the orchestration cluster
// clusterName report to the Console at consoleURL. The names follow
// clusterVersion: Camunda 8.10 renamed Console to Hub and the settings with
// it. A version that does not parse gets the 8.9 names, which every supported
// cluster reads.
//
// The entries carry no credentials. Camunda 8.10 also expects M2M credentials
// under the ping, which the management plane does not issue yet, so a cluster
// of that version logs a validation error and reports to nobody.
func PingEnv(consoleURL, clusterName, clusterVersion string) []corev1.EnvVar {
	names := pingEnvNamesFor(clusterVersion)

	return []corev1.EnvVar{
		{Name: names.enabled, Value: pingEnabled},
		{Name: names.endpoint, Value: consoleURL},
		{Name: names.clusterName, Value: clusterName},
		{Name: names.pingPeriod, Value: pingPeriod},
	}
}

// pingEnvNamesFor returns the key set that a cluster of this version reads.
func pingEnvNamesFor(clusterVersion string) pingEnvNames {
	if atLeast(clusterVersion, hubPingVersionFloor) {
		return pingEnvNames{
			enabled:     hubPingEnvEnabled,
			endpoint:    hubPingEnvEndpoint,
			clusterName: hubPingEnvClusterName,
			pingPeriod:  hubPingEnvPingPeriod,
		}
	}

	return pingEnvNames{
		enabled:     consolePingEnvEnabled,
		endpoint:    consolePingEnvEndpoint,
		clusterName: consolePingEnvClusterName,
		pingPeriod:  consolePingEnvPingPeriod,
	}
}

// PingsConsole reports whether env points an orchestration cluster at the
// Console at consoleURL, under either key set. The controller withdraws the
// ping from a cluster it no longer serves, and the endpoint is what tells the
// entries of one management plane from those of another.
func PingsConsole(env []corev1.EnvVar, consoleURL string) bool {
	for _, e := range env {
		if e.Name != consolePingEnvEndpoint && e.Name != hubPingEnvEndpoint {
			continue
		}
		if e.Value == consoleURL {
			return true
		}
	}

	return false
}

// PingCollisions returns the entries of spec.extraEnv that hold a ping setting
// of the key set that clusterVersion reads, with valueFrom, from the last of
// the list to the first. An entry under the other key set stays: the
// management plane never writes it for this cluster. Pass the version that
// PingEnv gets, so both read one key set. A caller that
// removes them in that order keeps the index of every entry it has not
// reached yet.
//
// The management plane writes a value under those names, and server-side
// apply merges per field inside one entry: its apply leaves the valueFrom of
// another manager where it is, and the API server refuses an entry that
// carries value and valueFrom together. Such an entry has to go before the
// apply.
func PingCollisions(cluster *v1.CamundaCluster, clusterVersion string) []PingCollision {
	names := pingEnvNamesFor(clusterVersion)
	var collisions []PingCollision
	for i := len(cluster.Spec.ExtraEnv) - 1; i >= 0; i-- {
		entry := cluster.Spec.ExtraEnv[i]
		if entry.ValueFrom == nil || !names.holds(entry.Name) {
			continue
		}
		collisions = append(collisions, PingCollision{
			Index:   i,
			Name:    entry.Name,
			Manager: valueFromManager(cluster.ManagedFields, entry.Name),
		})
	}

	return collisions
}

// holds reports whether name is one of the four settings of this key set.
func (n pingEnvNames) holds(name string) bool {
	return name == n.enabled || name == n.endpoint || name == n.clusterName || name == n.pingPeriod
}

// valueFromManager returns the field manager that owns valueFrom of the
// spec.extraEnv entry name.
func valueFromManager(fields []metav1.ManagedFieldsEntry, name string) string {
	encoded, err := json.Marshal(name)
	if err != nil {
		return ""
	}
	// The API server keys a merged list entry by its key fields, as JSON.
	key := `k:{"name":` + string(encoded) + `}`

	for _, entry := range fields {
		if entry.Subresource != "" {
			continue
		}
		var owned map[string]any
		if err := json.Unmarshal(entry.FieldsV1.GetRawBytes(), &owned); err != nil {
			continue
		}
		for _, step := range []string{"f:spec", "f:extraEnv", key} {
			owned, _ = owned[step].(map[string]any)
		}
		if _, ok := owned["f:valueFrom"]; ok {
			return entry.Manager
		}
	}

	return ""
}
