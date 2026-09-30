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
	"testing"

	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/elasticsearchcluster"
)

// The request annotation is read live. The cache can still hold the ECK CR
// from before the last apply, and an old request there reports the same
// shrink again.
func TestDataVolumesReadsTheRequestLive(t *testing.T) {
	t.Parallel()

	s := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(s))
	require.NoError(t, esv1.AddToScheme(s))

	cluster := &v1.ElasticsearchCluster{ObjectMeta: metav1.ObjectMeta{Name: "es", Namespace: "ns"}}
	stale := &esv1.Elasticsearch{ObjectMeta: metav1.ObjectMeta{Name: "es", Namespace: "ns"}}
	applied := stale.DeepCopy()
	applied.Annotations = map[string]string{components.RequestedStorageSizeAnnotation: "512Mi"}

	r := &ElasticsearchClusterReconciler{
		Client:    fake.NewClientBuilder().WithScheme(s).WithObjects(stale).Build(),
		APIReader: fake.NewClientBuilder().WithScheme(s).WithObjects(applied).Build(),
	}

	volumes, err := r.dataVolumes(t.Context(), cluster)
	require.NoError(t, err)
	assert.Equal(t, "512Mi", volumes.requested)
}
