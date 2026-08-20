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
		})
	}
}

// The slack is what an ordinary clock difference between the database and the
// source of spec.timestamp costs. A restore must not fail on it, and a
// database that was never rolled back must not pass on it.
func TestClockSlackIsOneMinute(t *testing.T) {
	assert.Equal(t, time.Minute, clockSlack)
}
