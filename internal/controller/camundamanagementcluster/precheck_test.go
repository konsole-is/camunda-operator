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

package camundamanagementcluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
)

// precheckNamespace is the management namespace of these tests.
const precheckNamespace = "camunda"

// Management Identity is the one component that signs in to Keycloak with the
// administrator, and the one that mounts the administrator password. Both
// reads therefore belong under its component. In the shared inputs a rotation
// of either Secret would roll Console and Web Modeler too, and neither reads
// them.
func TestKeycloakAdministratorRollsIdentityAlone(t *testing.T) {
	t.Parallel()

	mc := &v1.CamundaManagementCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "management", Namespace: precheckNamespace},
		Spec: v1.CamundaManagementClusterSpec{
			IdentityProvider: v1.IdentityProviderSpec{ExternalKeycloak: &v1.ExternalKeycloakSpec{
				URL: "https://keycloak.example.com/auth",
				AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
					Name:        "keycloak-admin",
					UsernameKey: "username",
					PasswordKey: "password",
				},
			}},
			Identity: v1.IdentitySpec{Admin: v1.IdentityAdminSpec{
				Username: "platform-admin",
				PasswordSecretRef: &v1.LocalSecretKeyRef{
					Name: "admin-password", Key: "password",
				},
			}},
		},
	}
	res := newResolver(
		t,
		mc,
		secretWithKeys("keycloak-admin", "username", "password"),
		secretWithKeys("admin-password", "password"),
	)

	require.NoError(t, res.resolveKeycloakAdmin(t.Context()))
	require.NoError(t, res.resolveGeneratedSecrets(t.Context(), &resolved{}))

	assert.Empty(t, res.inputs)
	identity := res.componentInputs[components.ComponentIdentity]
	require.Len(t, identity, 2)
	assert.Contains(t, identity[0], "Secret/"+precheckNamespace+"/keycloak-admin=")
	assert.Contains(t, identity[1], "Secret/"+precheckNamespace+"/admin-password=")
}

// newResolver returns a resolver of mc that reads the given objects.
func newResolver(t *testing.T, mc *v1.CamundaManagementCluster, objects ...*corev1.Secret) *resolver {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	builder := fake.NewClientBuilder().WithScheme(scheme)
	for _, object := range objects {
		builder = builder.WithObjects(object)
	}

	return &resolver{
		reader:          builder.Build(),
		scheme:          scheme,
		mc:              mc,
		mirrors:         map[components.MirrorPurpose]map[string][]byte{},
		componentInputs: map[string][]string{},
	}
}

// secretWithKeys returns a Secret of the management namespace that carries a
// value under each of keys.
func secretWithKeys(name string, keys ...string) *corev1.Secret {
	data := make(map[string][]byte, len(keys))
	for _, key := range keys {
		data[key] = []byte("s3cret")
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: precheckNamespace},
		Data:       data,
	}
}
