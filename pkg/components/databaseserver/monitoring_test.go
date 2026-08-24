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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestMonitoringEnabled(t *testing.T) {
	t.Parallel()

	assert.False(t, MonitoringEnabled(v1.DatabaseServerSpec{}))
	assert.False(t, MonitoringEnabled(v1.DatabaseServerSpec{
		Monitoring: &v1.DatabaseServerMonitoringSpec{},
	}))
	assert.False(t, MonitoringEnabled(v1.DatabaseServerSpec{
		Monitoring: &v1.DatabaseServerMonitoringSpec{PodMonitor: &v1.PodMonitorSpec{}},
	}))
	assert.True(t, MonitoringEnabled(v1.DatabaseServerSpec{
		Monitoring: &v1.DatabaseServerMonitoringSpec{PodMonitor: &v1.PodMonitorSpec{Enabled: true}},
	}))
}

// The PodMonitor selects the instance pods by the label CloudNativePG puts on
// them, and follows the cluster across a recovery. Extra labels a user asks
// for never displace the labels the operator owns.
func TestPodMonitorSelectsTheInstancePods(t *testing.T) {
	t.Parallel()

	server := archiveServer()
	server.Status.Cluster = recoveredClusterName
	merged := v1.DatabaseServerSpec{
		Monitoring: &v1.DatabaseServerMonitoringSpec{
			PodMonitor: &v1.PodMonitorSpec{
				Enabled:  true,
				Labels:   map[string]string{"release": "prometheus", "camunda.io/component": "not-postgres"},
				Interval: "30s",
			},
		},
	}

	monitor := podMonitor(server, merged)

	assert.Equal(t, "my-cluster-db-metrics", monitor.Name)
	assert.Equal(
		t, map[string]string{CNPGClusterNameLabel: recoveredClusterName},
		monitor.Spec.Selector.MatchLabels,
	)
	require.Len(t, monitor.Spec.PodMetricsEndpoints, 1)
	require.NotNil(t, monitor.Spec.PodMetricsEndpoints[0].Port)
	assert.Equal(t, MetricsPortName, *monitor.Spec.PodMetricsEndpoints[0].Port)
	assert.EqualValues(t, "30s", monitor.Spec.PodMetricsEndpoints[0].Interval)

	assert.Equal(t, "prometheus", monitor.Labels["release"])
	assert.Equal(t, componentLabel, monitor.Labels["camunda.io/component"])
}
