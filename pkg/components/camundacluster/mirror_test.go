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
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestMirroredSecretName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "my-cluster-camunda-license", MirroredSecretName(fixtureMinimal(t).Cluster, MirrorPurposeLicense))
}

// One component carries the Secret of every purpose, in MirrorPurposes
// order, each with only the copied keys, in the cluster namespace, under the
// managed labels. The Secret of an absent purpose is gated off, so Preview
// leaves it out (a gated-off resource is a delete marker).
func TestMirroredSecretComponentCarriesEveryMirror(t *testing.T) {
	t.Parallel()

	in := fixtureMinimal(t)
	comp, err := MirroredSecretComponent(in.Cluster, map[string]map[string][]byte{
		MirrorPurposeOIDCClient: {"client-secret": []byte("oidc")},
		MirrorPurposeLicense:    {"key": []byte("license")},
	})
	require.NoError(t, err)
	assert.Equal(
		t,
		component.ConditionType(v1.ConditionMirroredSecretsReady),
		comp.GetCondition(in.Cluster).ConditionType(),
	)

	objects := previewObjects(t, comp)
	require.Len(t, objects, 2)

	license, ok := objects[0].(*corev1.Secret)
	require.True(t, ok)
	assert.Equal(t, "my-cluster-camunda-license", license.Name)
	assert.Equal(t, "my-cluster-ns", license.Namespace)
	assert.Equal(t, map[string][]byte{"key": []byte("license")}, license.Data)
	assert.Equal(t, "my-cluster", license.Labels["camunda.io/cluster"])

	oidc, ok := objects[1].(*corev1.Secret)
	require.True(t, ok)
	assert.Equal(t, "my-cluster-camunda-oidc-client", oidc.Name)
	assert.Equal(t, map[string][]byte{"client-secret": []byte("oidc")}, oidc.Data)
}
