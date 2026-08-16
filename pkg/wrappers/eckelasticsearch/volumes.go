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

package eckelasticsearch

import (
	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
)

// SetVolumeClaimDeletePolicy records an edit that sets the volume claim
// delete policy of the Elasticsearch CR. ECK deletes the volume of every node
// that it scales away under either policy. The policies differ on cluster
// deletion only: DeleteOnScaledownAndClusterDeletion, the ECK default,
// removes the volumes with the cluster, and DeleteOnScaledownOnly keeps them.
// The edit runs during Apply, in registration order with the other edits.
func (m *Mutator) SetVolumeClaimDeletePolicy(policy esv1.VolumeClaimDeletePolicy) {
	m.Edit(func(es *esv1.Elasticsearch) error {
		es.Spec.VolumeClaimDeletePolicy = policy
		return nil
	})
}

// RetainVolumesOnDeletion records an edit that keeps the data volumes when
// the Elasticsearch CR is deleted: it sets DeleteOnScaledownOnly. Suspension
// and a Retain retention policy both use it.
func (m *Mutator) RetainVolumesOnDeletion() {
	m.SetVolumeClaimDeletePolicy(esv1.DeleteOnScaledownOnlyPolicy)
}
