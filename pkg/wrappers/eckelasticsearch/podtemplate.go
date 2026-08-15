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
	"fmt"

	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/mutation/editors"
	corev1 "k8s.io/api/core/v1"
)

// ContainerName is the fixed name that ECK gives the Elasticsearch container.
// A pod template entry with this name merges over the container that ECK
// renders.
const ContainerName = "elasticsearch"

// EditContainer records an edit of the Elasticsearch container in the pod
// template of the named node set. When the template carries no such
// container, the edit adds one first: ECK merges the entry over its own
// container by name, so an entry that only sets resources or env is complete.
// The edit runs during Apply, in registration order with the other object
// edits. A missing node set is an error at Apply time.
func (m *Mutator) EditContainer(nodeSet string, edit func(*editors.ContainerEditor) error) {
	m.Edit(func(es *esv1.Elasticsearch) error {
		for i := range es.Spec.NodeSets {
			if es.Spec.NodeSets[i].Name != nodeSet {
				continue
			}
			pod := &es.Spec.NodeSets[i].PodTemplate.Spec
			return edit(editors.NewContainerEditor(ensureContainer(pod)))
		}
		return fmt.Errorf("node set %q not found on Elasticsearch %q", nodeSet, es.Name)
	})
}

// ensureContainer returns the Elasticsearch container of pod, and adds it
// when absent. The pointer aliases the slice element, so edits land on pod.
func ensureContainer(pod *corev1.PodSpec) *corev1.Container {
	for i := range pod.Containers {
		if pod.Containers[i].Name == ContainerName {
			return &pod.Containers[i]
		}
	}

	pod.Containers = append(pod.Containers, corev1.Container{Name: ContainerName})
	return &pod.Containers[len(pod.Containers)-1]
}
