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

package logicalbackup

import (
	"k8s.io/apimachinery/pkg/api/resource"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// sizeStepBytes is the granularity of a recorded restore size. Sizes round up
// to it, so a restore asks for a volume size an administrator recognizes
// instead of a byte count that happens to be what one node reported.
const sizeStepBytes int64 = 10 << 30

// usageHeadroomNumerator and usageHeadroomDenominator are the 1.5 factor that
// turns Elasticsearch disk usage into the smallest viable restore size. It
// covers three needs at once: Elasticsearch stops allocating shards at 85%
// disk usage and blocks writes at 95%, so the settled data alone needs more
// room than it occupies; segment merges and the translog need transient space
// after a restore; and a restored cluster should land below the threshold at
// which an auto-resizer would immediately grow the disk.
const (
	usageHeadroomNumerator   int64 = 3
	usageHeadroomDenominator int64 = 2
)

// ZeebeSize returns the effective restore size of one broker data volume: the
// largest bound claim capacity, rounded up to the next 10Gi. It returns nil
// when no capacity is known, which keeps the recorded size unset rather than
// recording a wrong one.
func ZeebeSize(volumes []v1.VolumeStatus) *resource.Quantity {
	var largest int64
	for _, volume := range volumes {
		largest = max(largest, volume.Capacity.Value())
	}

	return quantityOf(roundUpToStep(largest))
}

// ElasticsearchSize returns the effective restore size of one Elasticsearch
// data volume: the largest node disk, capped by the largest node usage plus
// headroom when that is smaller, rounded up to the next 10Gi. The cap can
// only shrink the size, never grow it, so a mostly empty disk restores into a
// volume that fits its data instead of its former size. Unknown usage records
// the disk size alone; an unknown disk size returns nil.
func ElasticsearchSize(totalBytes, usedBytes int64) *resource.Quantity {
	effective := totalBytes
	if usedBytes > 0 {
		if capped := usedBytes * usageHeadroomNumerator / usageHeadroomDenominator; capped < effective {
			effective = capped
		}
	}

	return quantityOf(roundUpToStep(effective))
}

// RecordStorageSizes fills every unset field of sizes from computed. A field
// that already holds a value is kept, so the sizes captured when the backup
// started never move, and a field that computed leaves nil stays unset.
func RecordStorageSizes(sizes *v1.LogicalBackupStorageSizes, computed v1.LogicalBackupStorageSizes) {
	if sizes.Elasticsearch == nil {
		sizes.Elasticsearch = copyQuantity(computed.Elasticsearch)
	}
	if sizes.Zeebe == nil {
		sizes.Zeebe = copyQuantity(computed.Zeebe)
	}
}

// copyQuantity returns a copy of q, or nil for nil. A recorded size must not
// change because the caller went on to mutate the quantity it passed in.
func copyQuantity(q *resource.Quantity) *resource.Quantity {
	if q == nil {
		return nil
	}

	copied := q.DeepCopy()

	return &copied
}

// roundUpToStep rounds bytes up to the next whole size step. A non-positive
// value stays zero, the signal for "not known".
func roundUpToStep(bytes int64) int64 {
	if bytes <= 0 {
		return 0
	}

	return (bytes + sizeStepBytes - 1) / sizeStepBytes * sizeStepBytes
}

// quantityOf returns bytes as a binary quantity, or nil for zero.
func quantityOf(bytes int64) *resource.Quantity {
	if bytes == 0 {
		return nil
	}

	return resource.NewQuantity(bytes, resource.BinarySI)
}
