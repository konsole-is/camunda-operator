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

package logicalbackup_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
)

const gi = 1 << 30

func TestZeebeSize(t *testing.T) {
	tests := []struct {
		name     string
		volumes  []v1.VolumeStatus
		expected string
	}{
		{
			name:     "no volumes leaves the size unset",
			volumes:  nil,
			expected: "",
		},
		{
			name: "the largest bound claim wins",
			volumes: []v1.VolumeStatus{
				{Name: "data-zeebe-0", Capacity: resource.MustParse("10Gi")},
				{Name: "data-zeebe-1", Capacity: resource.MustParse("30Gi")},
				{Name: "data-zeebe-2", Capacity: resource.MustParse("20Gi")},
			},
			expected: "30Gi",
		},
		{
			name:     "a size between steps rounds up to the next 10Gi",
			volumes:  []v1.VolumeStatus{{Name: "data-zeebe-0", Capacity: resource.MustParse("11Gi")}},
			expected: "20Gi",
		},
		{
			name:     "a size on a step is kept",
			volumes:  []v1.VolumeStatus{{Name: "data-zeebe-0", Capacity: resource.MustParse("20Gi")}},
			expected: "20Gi",
		},
		{
			name:     "a size below one step rounds up to the first step",
			volumes:  []v1.VolumeStatus{{Name: "data-zeebe-0", Capacity: resource.MustParse("1Gi")}},
			expected: "10Gi",
		},
		{
			name:     "a zero capacity leaves the size unset",
			volumes:  []v1.VolumeStatus{{Name: "data-zeebe-0", Capacity: resource.MustParse("0")}},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := logicalbackup.ZeebeSize(tt.volumes)

			if tt.expected == "" {
				assert.Nil(t, size)
				return
			}
			require.NotNil(t, size)
			assert.Equal(t, tt.expected, size.String())
		})
	}
}

func TestElasticsearchSize(t *testing.T) {
	tests := []struct {
		name     string
		total    int64
		used     int64
		expected string
	}{
		{
			name:     "usage with headroom below the disk caps the recorded size",
			total:    200 * gi,
			used:     20 * gi,
			expected: "30Gi", // 20Gi * 1.5 = 30Gi, already on a step
		},
		{
			name:     "the disk size is the ceiling: headroom never grows it",
			total:    100 * gi,
			used:     90 * gi, // 90Gi * 1.5 = 135Gi, above the disk
			expected: "100Gi",
		},
		{
			name:     "a capped size rounds up to the next 10Gi",
			total:    200 * gi,
			used:     21 * gi, // 21Gi * 1.5 = 31.5Gi
			expected: "40Gi",
		},
		{
			name:     "unknown usage records the disk size alone",
			total:    64 * gi,
			used:     0,
			expected: "70Gi",
		},
		{
			name:     "an unknown disk size leaves the size unset",
			total:    0,
			used:     10 * gi,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := logicalbackup.ElasticsearchSize(tt.total, tt.used)

			if tt.expected == "" {
				assert.Nil(t, size)
				return
			}
			require.NotNil(t, size)
			assert.Equal(t, tt.expected, size.String())
		})
	}
}

// Recording is best effort and never overwrites: the sizes are captured once,
// when the backup starts, and a later reconcile must not move them.
func TestRecordStorageSizesKeepsWhatIsSet(t *testing.T) {
	existing := resource.MustParse("50Gi")
	sizes := v1.LogicalBackupStorageSizes{Zeebe: &existing}

	computed := resource.MustParse("10Gi")
	logicalbackup.RecordStorageSizes(&sizes, v1.LogicalBackupStorageSizes{
		Zeebe:         &computed,
		Elasticsearch: &computed,
	})

	require.NotNil(t, sizes.Zeebe)
	assert.Equal(t, "50Gi", sizes.Zeebe.String(), "an already recorded size is kept")
	require.NotNil(t, sizes.Elasticsearch)
	assert.Equal(t, "10Gi", sizes.Elasticsearch.String(), "an unset size is filled in")
}

func TestRecordStorageSizesLeavesUncomputedUnset(t *testing.T) {
	var sizes v1.LogicalBackupStorageSizes

	logicalbackup.RecordStorageSizes(&sizes, v1.LogicalBackupStorageSizes{})

	assert.Nil(t, sizes.Zeebe)
	assert.Nil(t, sizes.Elasticsearch)
}

// The recorded sizes are the ones captured when the backup started. A
// quantity the caller reuses and mutates must not rewrite them.
func TestRecordStorageSizesCopiesTheQuantities(t *testing.T) {
	shared := resource.MustParse("10Gi")

	var sizes v1.LogicalBackupStorageSizes
	logicalbackup.RecordStorageSizes(&sizes, v1.LogicalBackupStorageSizes{
		Zeebe:         &shared,
		Elasticsearch: &shared,
	})

	shared.Set(999 << 30)

	require.NotNil(t, sizes.Zeebe)
	assert.Equal(t, "10Gi", sizes.Zeebe.String())
	require.NotNil(t, sizes.Elasticsearch)
	assert.Equal(t, "10Gi", sizes.Elasticsearch.String())
}
