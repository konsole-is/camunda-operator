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

package keycloak

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// The Keycloak CRD schema declares neither a creationTimestamp on the pod
// template of spec.unsupported nor a status the operator may write, and
// Server-Side Apply rejects an undeclared field, so the serialized patch must
// not carry the zero values that the typed structs always produce.
func TestSanitizeForApplyDropsFieldsTheKeycloakSchemaDoesNotDeclare(t *testing.T) {
	t.Parallel()

	kc := &Keycloak{
		TypeMeta:   metav1.TypeMeta{APIVersion: "k8s.keycloak.org/v2alpha1", Kind: "Keycloak"},
		ObjectMeta: metav1.ObjectMeta{Name: "kc", Namespace: "ns"},
		Spec: KeycloakSpec{
			Instances: new(int32(1)),
			Unsupported: &KeycloakUnsupportedSpec{PodTemplate: &corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"a": "b"}},
			}},
		},
		Status: KeycloakStatus{Instances: 1},
	}

	u, err := sanitizeForApply(kc)
	require.NoError(t, err)

	_, found, err := unstructured.NestedMap(u.Object, "status")
	require.NoError(t, err)
	assert.False(t, found, "status must be dropped")

	_, found, err = unstructured.NestedString(u.Object, "metadata", "creationTimestamp")
	require.NoError(t, err)
	assert.False(t, found, "the creationTimestamp of the Keycloak must be dropped")

	_, found, err = unstructured.NestedFieldNoCopy(
		u.Object, "spec", "unsupported", "podTemplate", "metadata", "creationTimestamp",
	)
	require.NoError(t, err)
	assert.False(t, found, "the creationTimestamp of the pod template must be dropped")

	podLabels, found, err := unstructured.NestedStringMap(
		u.Object, "spec", "unsupported", "podTemplate", "metadata", "labels",
	)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, map[string]string{"a": "b"}, podLabels)
}
