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

package pointintimerestore

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestDecide(t *testing.T) {
	want := time.Date(2026, 7, 30, 14, 30, 0, 0, time.UTC)
	at := func(partition int32, offset time.Duration) v1.PartitionPosition {
		return v1.PartitionPosition{
			PartitionID: partition, LastUpdated: metav1.NewTime(want.Add(offset)),
		}
	}

	tests := []struct {
		name       string
		positions  []v1.PartitionPosition
		partitions int32
		contains   []string
		excludes   []string
	}{
		{
			name:       "every partition is behind the requested point",
			positions:  []v1.PartitionPosition{at(1, -time.Hour), at(2, -time.Minute), at(3, -time.Second)},
			partitions: 3,
		},
		{
			name:       "a partition sits exactly on the requested point",
			positions:  []v1.PartitionPosition{at(1, -time.Hour), at(2, 0)},
			partitions: 2,
		},
		{
			name:       "a partition is ahead inside the slack",
			positions:  []v1.PartitionPosition{at(1, 30*time.Second), at(2, -time.Hour)},
			partitions: 2,
		},
		{
			name:       "a partition is ahead past the slack",
			positions:  []v1.PartitionPosition{at(1, -time.Hour), at(2, 90*time.Second)},
			partitions: 2,
			contains: []string{
				"partition 2",
				want.Add(90 * time.Second).Format(time.RFC3339),
				want.Format(time.RFC3339),
			},
		},
		{
			name:       "a partition reports no position",
			positions:  []v1.PartitionPosition{at(1, -time.Hour)},
			partitions: 3,
			contains:   []string{"partition 2", "partition 3"},
		},
		{
			// A missing row and an ahead row at once name the missing rows
			// first. The database has to be restored either way, and a
			// message that names one cause at a time sends the reader back
			// twice.
			name:       "a partition is missing while another is ahead",
			positions:  []v1.PartitionPosition{at(2, 90*time.Second)},
			partitions: 2,
			contains:   []string{"partition 1"},
			excludes:   []string{"is ahead of the requested point"},
		},
		{
			name:       "a partition sits exactly on the edge of the slack",
			positions:  []v1.PartitionPosition{at(1, time.Minute)},
			partitions: 1,
		},
		{
			name:       "a partition sits one second past the slack",
			positions:  []v1.PartitionPosition{at(1, time.Minute+time.Second)},
			partitions: 1,
			contains:   []string{"partition 1"},
		},
		{
			name:       "the table holds no position at all",
			partitions: 2,
			contains:   []string{"no exporter position"},
		},
		{
			name:       "a partition that the cluster does not have is not the operator's business",
			positions:  []v1.PartitionPosition{at(1, -time.Hour), at(2, time.Hour)},
			partitions: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failure := decide(tt.positions, tt.partitions, want, clockSlack)

			if len(tt.contains) == 0 {
				assert.Nil(t, failure, "the database is not ahead of the requested point")

				return
			}

			require.NotNil(t, failure)
			assert.Equal(t, v1.ReasonDatabaseNotRestored, failure.Reason)
			for _, want := range tt.contains {
				assert.Contains(t, failure.Message, want)
			}
			for _, unwanted := range tt.excludes {
				assert.NotContains(t, failure.Message, unwanted)
			}
		})
	}
}

// The slack is what an ordinary clock difference between the database and the
// source of spec.timestamp costs. A restore must not fail on it, and a
// database that was never rolled back must not pass on it.
func TestClockSlackIsOneMinute(t *testing.T) {
	assert.Equal(t, time.Minute, clockSlack)
}

func TestBrokerClockComparable(t *testing.T) {
	env := func(vars ...corev1.EnvVar) *corev1.Container {
		return &corev1.Container{Name: "camunda", Env: vars}
	}

	tests := []struct {
		name       string
		broker     *corev1.Container
		comparable bool
		contains   string
	}{
		{
			name:       "a broker that sets no zone runs in UTC",
			broker:     env(),
			comparable: true,
		},
		{
			name:       "a broker that sets TZ to UTC",
			broker:     env(corev1.EnvVar{Name: "TZ", Value: "UTC"}),
			comparable: true,
		},
		{
			name:       "a broker that sets TZ to Etc/UTC",
			broker:     env(corev1.EnvVar{Name: "TZ", Value: "Etc/UTC"}),
			comparable: true,
		},
		{
			name:     "a broker that runs in another zone",
			broker:   env(corev1.EnvVar{Name: "TZ", Value: "America/New_York"}),
			contains: "TZ",
		},
		{
			name: "a broker whose Java options set another zone",
			broker: env(corev1.EnvVar{
				Name: "JAVA_TOOL_OPTIONS", Value: "-Xmx2g -Duser.timezone=Europe/Berlin -Xms1g",
			}),
			contains: "user.timezone",
		},
		{
			name: "a broker whose Java options set UTC",
			broker: env(corev1.EnvVar{
				Name: "JAVA_TOOL_OPTIONS", Value: "-Xmx2g -Duser.timezone=UTC",
			}),
			comparable: true,
		},
		{
			name: "a broker whose zone comes from a reference",
			broker: env(corev1.EnvVar{
				Name:      "TZ",
				ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "x"}},
			}),
			contains: "TZ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failure := brokerClockComparable(tt.broker)

			if tt.comparable {
				assert.Nil(t, failure, "the broker writes the exporter position in UTC")

				return
			}

			require.NotNil(t, failure)
			assert.Equal(t, v1.ReasonPitrUnavailable, failure.Reason)
			assert.Contains(t, failure.Message, tt.contains)
		})
	}
}
