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

package builtin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
)

func TestReadBackingReturnsTheLiveObjectsAndLeavesOutAMissingOne(t *testing.T) {
	request := requestFor(pluginstartup.RegistrationExistingObjectStorage, `{"name":"bucket"}`)
	h := &recordingHandler{refs: []backingRef{
		clusterRef("orders-es", request.Namespace),
		clusterRef("gone-es", request.Namespace),
	}}
	r := &Reconciler{APIReader: fakeReader(t, storedCluster("orders-es", request.Namespace, "50Gi"))}

	live, err := r.readBacking(t.Context(), request, h)

	require.NoError(t, err)
	got, ok := liveAs[*v1.ElasticsearchCluster](live, "ElasticsearchCluster", "orders-es")
	require.True(t, ok)
	assert.Equal(t, "50Gi", got.Spec.StorageSize.String())
	_, ok = liveAs[*v1.ElasticsearchCluster](live, "ElasticsearchCluster", "gone-es")
	assert.False(t, ok)
}

func TestReadBackingReadsAClusterScopedObject(t *testing.T) {
	request := requestFor(pluginstartup.RegistrationExistingObjectStorage, `{"name":"bucket"}`)
	h := &recordingHandler{refs: []backingRef{{
		kind:   "ElasticsearchClusterPreset",
		object: &v1.ElasticsearchClusterPreset{ObjectMeta: metav1.ObjectMeta{Name: "small"}},
	}}}
	stored := &v1.ElasticsearchClusterPreset{
		ObjectMeta: metav1.ObjectMeta{Name: "small"},
		Spec:       v1.ElasticsearchClusterPresetSpec{Cluster: v1.ElasticsearchClusterSpec{Replicas: new(int32(3))}},
	}
	r := &Reconciler{APIReader: fakeReader(t, stored)}

	live, err := r.readBacking(t.Context(), request, h)

	require.NoError(t, err)
	got, ok := liveAs[*v1.ElasticsearchClusterPreset](live, "ElasticsearchClusterPreset", "small")
	require.True(t, ok)
	assert.Equal(t, int32(3), *got.Spec.Cluster.Replicas)
}

func TestBuildGivesTheHandlerTheLiveObjects(t *testing.T) {
	request := requestFor(pluginstartup.RegistrationExistingObjectStorage, `{"name":"bucket"}`)
	schema, err := newParameterSchema(schemaOf(t, pluginstartup.RegistrationExistingObjectStorage))
	require.NoError(t, err)
	h := &recordingHandler{refs: []backingRef{clusterRef("orders-es", request.Namespace)}}
	r := &Reconciler{
		APIReader: fakeReader(t, storedCluster("orders-es", request.Namespace, "50Gi")),
		schemas:   map[string]*parameterSchema{pluginstartup.RegistrationExistingObjectStorage: schema},
	}

	_, err = r.build(t.Context(), request, h)

	require.NoError(t, err)
	got, ok := liveAs[*v1.ElasticsearchCluster](h.builtWith, "ElasticsearchCluster", "orders-es")
	require.True(t, ok)
	assert.Equal(t, "50Gi", got.Spec.StorageSize.String())
}

func TestLiveAsReportsAnObjectOfAnotherType(t *testing.T) {
	live := liveObjects{
		{kind: "ElasticsearchCluster", name: "orders-es"}: &v1.SecondaryStorageConfig{},
	}

	_, ok := liveAs[*v1.ElasticsearchCluster](live, "ElasticsearchCluster", "orders-es")

	assert.False(t, ok)
}

// recordingHandler names refs as its backing objects and keeps the live
// objects that build gets.
type recordingHandler struct {
	existing

	refs      []backingRef
	builtWith liveObjects
}

func (h *recordingHandler) build(_ *pluginsv1alpha1.PluginRequest, live liveObjects) (served, error) {
	h.builtWith = live
	return served{}, nil
}

func (h *recordingHandler) backing(*pluginsv1alpha1.PluginRequest) []backingRef {
	return h.refs
}

func clusterRef(name, namespace string) backingRef {
	return backingRef{
		kind:   "ElasticsearchCluster",
		object: &v1.ElasticsearchCluster{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}},
	}
}

func storedCluster(name, namespace, size string) *v1.ElasticsearchCluster {
	return &v1.ElasticsearchCluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       v1.ElasticsearchClusterSpec{StorageSize: new(resource.MustParse(size))},
	}
}
