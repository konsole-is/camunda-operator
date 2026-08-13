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

package eckelasticsearch_test

import (
	"testing"

	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/databaseconfig"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/eckelasticsearch"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/secondarystorageconfig"
)

// TestWrappersAssembleIntoComponent proves the three generated wrappers
// register with an ocf component and render through Preview, the way the
// storage backend controllers consume them.
func TestWrappersAssembleIntoComponent(t *testing.T) {
	t.Parallel()

	elasticsearch, err := eckelasticsearch.NewBuilder(&esv1.Elasticsearch{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cluster-es", Namespace: "my-cluster-ns"},
		Spec:       esv1.ElasticsearchSpec{Version: "9.2.4"},
	}).Build()
	require.NoError(t, err)

	storageContract, err := secondarystorageconfig.NewBuilder(&v1.SecondaryStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "my-storage-config", Namespace: "my-cluster-ns"},
		Spec:       v1.SecondaryStorageConfigSpec{Type: v1.SecondaryStorageTypeElasticsearch},
	}).Build()
	require.NoError(t, err)

	dbContract, err := databaseconfig.NewBuilder(&v1.DatabaseConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "my-camunda-db", Namespace: "my-cluster-ns"},
		Spec:       v1.DatabaseConfigSpec{ServerRef: "my-db-server", DatabaseName: "camunda"},
	}).Build()
	require.NoError(t, err)

	comp, err := component.NewComponentBuilder().
		WithName("storage").
		WithConditionType("StorageReady").
		WithResource(elasticsearch).
		WithResource(storageContract).
		WithResource(dbContract).
		Build()
	require.NoError(t, err)

	objects, err := comp.Preview()
	require.NoError(t, err)
	require.Len(t, objects, 3)

	assert.Equal(t, "my-cluster-es", objects[0].GetName())
	assert.Equal(t, "my-storage-config", objects[1].GetName())
	assert.Equal(t, "my-camunda-db", objects[2].GetName())
}
