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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func sanitizedFixture(t *testing.T) *unstructured.Unstructured {
	t.Helper()

	es := &esv1.Elasticsearch{
		TypeMeta: metav1.TypeMeta{APIVersion: "elasticsearch.k8s.elastic.co/v1", Kind: "Elasticsearch"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "es", Namespace: "ns", Labels: map[string]string{"a": "b"},
		},
		Spec: esv1.ElasticsearchSpec{
			Version: "9.2.4",
			NodeSets: []esv1.NodeSet{{
				Name:  "default",
				Count: 1,
				VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
					ObjectMeta: metav1.ObjectMeta{Name: "elasticsearch-data", Labels: map[string]string{"c": "d"}},
					Spec: corev1.PersistentVolumeClaimSpec{
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
						},
					},
				}},
			}},
		},
	}

	u, err := sanitizeForApply(es)
	require.NoError(t, err)

	return u
}

// The ECK CRD schema declares neither status nor creationTimestamp inside a
// volumeClaimTemplate, and Server-Side Apply rejects undeclared fields, so
// the serialized patch must not carry the typed struct's zero values for
// them.
func TestSanitizeForApplyDropsFieldsTheECKSchemaDoesNotDeclare(t *testing.T) {
	t.Parallel()

	u := sanitizedFixture(t)

	_, found, err := unstructured.NestedMap(u.Object, "status")
	require.NoError(t, err)
	assert.False(t, found, "top-level status must be dropped")

	nodeSets, _, err := unstructured.NestedSlice(u.Object, "spec", "nodeSets")
	require.NoError(t, err)
	require.Len(t, nodeSets, 1)

	claims := nodeSets[0].(map[string]any)["volumeClaimTemplates"].([]any)
	require.Len(t, claims, 1)
	claim := claims[0].(map[string]any)
	assert.NotContains(t, claim, "status")

	metadata := claim["metadata"].(map[string]any)
	assert.NotContains(t, metadata, "creationTimestamp")
}

func TestSanitizeForApplyKeepsTheDeclaredContent(t *testing.T) {
	t.Parallel()

	u := sanitizedFixture(t)

	assert.Equal(t, "elasticsearch.k8s.elastic.co/v1", u.GetAPIVersion())
	assert.Equal(t, "Elasticsearch", u.GetKind())
	assert.Equal(t, "es", u.GetName())

	version, _, err := unstructured.NestedString(u.Object, "spec", "version")
	require.NoError(t, err)
	assert.Equal(t, "9.2.4", version)

	nodeSets, _, err := unstructured.NestedSlice(u.Object, "spec", "nodeSets")
	require.NoError(t, err)
	claim := nodeSets[0].(map[string]any)["volumeClaimTemplates"].([]any)[0].(map[string]any)
	metadata := claim["metadata"].(map[string]any)
	assert.Equal(t, "elasticsearch-data", metadata["name"])
	assert.Equal(t, map[string]any{"c": "d"}, metadata["labels"])
}
