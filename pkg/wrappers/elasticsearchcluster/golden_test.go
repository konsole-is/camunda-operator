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

package elasticsearchcluster

import (
	"flag"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/testing/golden"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	apiv1 "github.com/konsole-is/camunda-operator/api/v1"
)

// updateGolden refreshes the golden manifests with the rendered output:
// go test ./pkg/wrappers/elasticsearchcluster/ -run Golden -update-golden
var updateGolden = flag.Bool("update-golden", false, "update golden files")

// goldenCluster returns the ElasticsearchCluster that a built-in adapter
// applies.
func goldenCluster() *apiv1.ElasticsearchCluster {
	return &apiv1.ElasticsearchCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "orders-es", Namespace: "team-a"},
		Spec: apiv1.ElasticsearchClusterSpec{
			PresetRef:              "small",
			Replicas:               new(int32(3)),
			StorageSize:            new(resource.MustParse("50Gi")),
			SecondaryStorageConfig: "orders-es",
		},
	}
}

// TestElasticsearchClusterGolden pins the rendered ElasticsearchCluster.
func TestElasticsearchClusterGolden(t *testing.T) {
	t.Parallel()

	res, err := NewBuilder(goldenCluster()).Build()
	require.NoError(t, err)

	golden.AssertYAML(
		t, "testdata/golden/elasticsearchcluster.yaml", res,
		golden.WithScheme(goldenScheme(t)), golden.Update(*updateGolden),
	)
}

// TestSuspendedElasticsearchClusterGolden pins the rendered ElasticsearchCluster of a
// suspended component. The diff against elasticsearchcluster.yaml is spec.suspend
// alone.
func TestSuspendedElasticsearchClusterGolden(t *testing.T) {
	t.Parallel()

	res, err := NewBuilder(goldenCluster()).
		WithMutation(Mutation{Name: "suspend", Mutate: DefaultSuspendMutationHandler}).
		Build()
	require.NoError(t, err)

	golden.AssertYAML(
		t, "testdata/golden/elasticsearchcluster-suspended.yaml", res,
		golden.WithScheme(goldenScheme(t)), golden.Update(*updateGolden),
	)
}

func goldenScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, apiv1.AddToScheme(scheme))

	return scheme
}
