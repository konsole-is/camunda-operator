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

package camundaoptimize

import (
	corev1 "k8s.io/api/core/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

const (
	// eventReasonIndexReplicasExceedNodes is the Warning event for an index
	// replica count that the nodes of the storage contract cannot place.
	eventReasonIndexReplicasExceedNodes = "IndexReplicasExceedNodes"
	eventActionReconcile                = "Reconcile"
)

func (r *Reconciler) recordUnplaceableReplicas(optimize *v1.CamundaOptimize, res resolved) {
	storage := res.Input.Storage
	replicas := storage.IndexReplicas(optimize.Spec.IndexReplicas)
	if replicas == nil || !storage.ReplicasExceedNodes(*replicas) {
		return
	}

	r.EventRecorder.Eventf(
		optimize,
		nil,
		corev1.EventTypeWarning,
		eventReasonIndexReplicasExceedNodes,
		eventActionReconcile,
		"indexReplicas %d needs %d Elasticsearch nodes, but the storage contract names %d. The indices stay yellow",
		*replicas,
		*replicas+1,
		*storage.NodeCount,
	)
}
