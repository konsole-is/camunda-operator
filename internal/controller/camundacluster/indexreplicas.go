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
	corev1 "k8s.io/api/core/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
)

// eventReasonIndexReplicasExceedNodes is the Warning event that the controller
// records when the index replica count of the cluster needs more nodes than
// the storage contract names. The cluster runs with the count, and the health
// of its indices stays yellow.
const eventReasonIndexReplicasExceedNodes = "IndexReplicasExceedNodes"

// recordUnplaceableReplicas records IndexReplicasExceedNodes when the resolved
// index replica count of an Elasticsearch secondary storage is more than its
// nodes can place.
func (r *CamundaClusterReconciler) recordUnplaceableReplicas(cluster *v1.CamundaCluster, in components.Input) {
	es := in.Storage.Elasticsearch
	if in.Storage.Type != v1.SecondaryStorageTypeElasticsearch || es == nil {
		return
	}

	replicas := es.IndexReplicas(in.Effective.IndexReplicas)
	if replicas == nil || !es.ReplicasExceedNodes(*replicas) {
		return
	}

	r.EventRecorder.Eventf(
		cluster,
		nil,
		corev1.EventTypeWarning,
		eventReasonIndexReplicasExceedNodes,
		eventActionReconcile,
		"indexReplicas %d needs %d Elasticsearch nodes, but the storage contract names %d. The indices stay yellow",
		*replicas,
		*replicas+1,
		*es.NodeCount,
	)
}
