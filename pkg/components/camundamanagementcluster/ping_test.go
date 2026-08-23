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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

// A cluster before 8.10 reads the ping under camunda.console.ping.
func TestPingEnvOfACamunda89Cluster(t *testing.T) {
	t.Parallel()

	env := PingEnv("http://my-management-console.camunda.svc:80", "production", "8.9.9")

	assert.Equal(
		t, []corev1.EnvVar{
			{Name: "CAMUNDA_CONSOLE_PING_ENABLED", Value: "true"},
			{Name: "CAMUNDA_CONSOLE_PING_ENDPOINT", Value: "http://my-management-console.camunda.svc:80"},
			{Name: "CAMUNDA_CONSOLE_PING_CLUSTERNAME", Value: "production"},
			{Name: "CAMUNDA_CONSOLE_PING_PINGPERIOD", Value: "1h"},
		}, env,
	)
}

// Camunda 8.10 renamed Console to Hub, and a cluster of that version reads the
// ping under camunda.hub.ping.
func TestPingEnvOfACamunda810Cluster(t *testing.T) {
	t.Parallel()

	for _, version := range []string{"8.10.0", "8.10.3", "9.0.0"} {
		t.Run(version, func(t *testing.T) {
			t.Parallel()

			env := PingEnv("http://console.camunda.svc:80", "production", version)

			assert.Equal(
				t, []corev1.EnvVar{
					{Name: "CAMUNDA_HUB_PING_ENABLED", Value: "true"},
					{Name: "CAMUNDA_HUB_PING_ENDPOINT", Value: "http://console.camunda.svc:80"},
					{Name: "CAMUNDA_HUB_PING_CLUSTERNAME", Value: "production"},
					{Name: "CAMUNDA_HUB_PING_PINGPERIOD", Value: "1h"},
				}, env,
			)
		})
	}
}

// A cluster that publishes no version yet reads the names that every supported
// cluster understands.
func TestPingEnvOfAClusterWithoutAVersion(t *testing.T) {
	t.Parallel()

	env := PingEnv("http://console.camunda.svc:80", "production", "")

	assert.Equal(t, "CAMUNDA_CONSOLE_PING_ENABLED", env[0].Name)
}

// The endpoint tells the entries of one management plane from those of
// another, under either key set.
func TestPingsConsoleReadsTheEndpoint(t *testing.T) {
	t.Parallel()

	ours := "http://my-management-console.camunda.svc:80"

	assert.True(t, PingsConsole(PingEnv(ours, "production", "8.9.9"), ours))
	assert.True(t, PingsConsole(PingEnv(ours, "production", "8.10.0"), ours))
	assert.False(t, PingsConsole(PingEnv("http://other-console.other.svc:80", "production", "8.9.9"), ours))
	assert.False(t, PingsConsole(nil, ours))
	assert.False(t, PingsConsole([]corev1.EnvVar{{Name: "ZEEBE_BROKER_CLUSTER_SIZE", Value: "3"}}, ours))
}
