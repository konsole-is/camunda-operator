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
	"testing"

	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/mutation/editors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestEditContainerAddsTheContainerWhenAbsent(t *testing.T) {
	t.Parallel()

	es := testObject()
	es.Spec.NodeSets = []esv1.NodeSet{{Name: "default"}}

	mutator := NewMutator(es)
	mutator.EditContainer("default", func(c *editors.ContainerEditor) error {
		c.SetResourceRequest(corev1.ResourceMemory, resource.MustParse("4Gi"))
		return nil
	})
	require.NoError(t, mutator.Apply())

	containers := es.Spec.NodeSets[0].PodTemplate.Spec.Containers
	require.Len(t, containers, 1)
	assert.Equal(t, ContainerName, containers[0].Name)
	assert.Equal(t, resource.MustParse("4Gi"), containers[0].Resources.Requests[corev1.ResourceMemory])
}

func TestEditContainerEditsTheExistingContainer(t *testing.T) {
	t.Parallel()

	es := testObject()
	es.Spec.NodeSets = []esv1.NodeSet{{Name: "default"}}
	es.Spec.NodeSets[0].PodTemplate.Spec.Containers = []corev1.Container{
		{Name: "sidecar"},
		{Name: ContainerName, Env: []corev1.EnvVar{{Name: "A", Value: "1"}}},
	}

	mutator := NewMutator(es)
	mutator.EditContainer("default", func(c *editors.ContainerEditor) error {
		c.EnsureEnvVar(corev1.EnvVar{Name: "B", Value: "2"})
		return nil
	})
	require.NoError(t, mutator.Apply())

	containers := es.Spec.NodeSets[0].PodTemplate.Spec.Containers
	require.Len(t, containers, 2, "no container is added when one exists")
	assert.Equal(t, []corev1.EnvVar{{Name: "A", Value: "1"}, {Name: "B", Value: "2"}}, containers[1].Env)
}

func TestEditContainerRejectsAnUnknownNodeSet(t *testing.T) {
	t.Parallel()

	es := testObject()
	es.Spec.NodeSets = []esv1.NodeSet{{Name: "default"}}

	mutator := NewMutator(es)
	mutator.EditContainer("warm", func(*editors.ContainerEditor) error { return nil })

	assert.ErrorContains(t, mutator.Apply(), `node set "warm" not found`)
}
