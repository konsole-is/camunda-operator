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

package secondarystorageconfig_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/secondarystorageconfig"
)

func esScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	return scheme
}

func esContract(ca bool) *v1.SecondaryStorageConfig {
	es := &v1.ElasticsearchStorage{
		Endpoint: "https://es.my-ns.svc:9200",
		CredentialsSecretRef: v1.LocalCredentialsSecretRef{
			Name:        "es-user",
			UsernameKey: "username",
			PasswordKey: "password",
		},
	}
	if ca {
		es.CASecretRef = &v1.LocalSecretKeyRef{Name: "es-ca", Key: "ca.crt"}
	}

	return &v1.SecondaryStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "my-storage", Namespace: "my-ns"},
		Spec:       v1.SecondaryStorageConfigSpec{Type: v1.SecondaryStorageTypeElasticsearch, Elasticsearch: es},
	}
}

func userSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "es-user", Namespace: "my-ns"},
		Data:       map[string][]byte{"username": []byte("camunda"), "password": []byte("secret")},
	}
}

func TestElasticsearchAdminRejectsAContractWithoutAnElasticsearchBlock(t *testing.T) {
	reader := fake.NewClientBuilder().WithScheme(esScheme(t)).Build()
	storage := &v1.SecondaryStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "my-storage", Namespace: "my-ns"},
		Spec:       v1.SecondaryStorageConfigSpec{Type: v1.SecondaryStorageTypeRDBMS},
	}

	admin, failure, err := secondarystorageconfig.ElasticsearchAdmin(t.Context(), reader, storage)

	require.NoError(t, err)
	assert.Nil(t, admin)
	require.NotNil(t, failure)
	assert.Equal(t, v1.ReasonInvalidReference, failure.Reason)
}

func TestElasticsearchAdminReportsAMissingCredentialsSecret(t *testing.T) {
	reader := fake.NewClientBuilder().WithScheme(esScheme(t)).Build()

	admin, failure, err := secondarystorageconfig.ElasticsearchAdmin(t.Context(), reader, esContract(false))

	require.NoError(t, err)
	assert.Nil(t, admin)
	require.NotNil(t, failure)
	assert.Equal(t, v1.ReasonMissingSecret, failure.Reason)
	assert.Contains(t, failure.Message, "es-user")
}

func TestElasticsearchAdminReportsAMissingCASecret(t *testing.T) {
	reader := fake.NewClientBuilder().WithScheme(esScheme(t)).WithObjects(userSecret()).Build()

	admin, failure, err := secondarystorageconfig.ElasticsearchAdmin(t.Context(), reader, esContract(true))

	require.NoError(t, err)
	assert.Nil(t, admin)
	require.NotNil(t, failure)
	assert.Equal(t, v1.ReasonMissingSecret, failure.Reason)
	assert.Contains(t, failure.Message, "es-ca")
}

func TestElasticsearchAdminBuildsWithoutACA(t *testing.T) {
	reader := fake.NewClientBuilder().WithScheme(esScheme(t)).WithObjects(userSecret()).Build()

	admin, failure, err := secondarystorageconfig.ElasticsearchAdmin(t.Context(), reader, esContract(false))

	require.NoError(t, err)
	require.Nil(t, failure)
	assert.NotNil(t, admin)
}

func TestElasticsearchAdminRejectsAnEmptyCABundle(t *testing.T) {
	objs := []client.Object{userSecret(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "es-ca", Namespace: "my-ns"},
		Data:       map[string][]byte{"ca.crt": {}},
	}}
	reader := fake.NewClientBuilder().WithScheme(esScheme(t)).WithObjects(objs...).Build()

	admin, failure, err := secondarystorageconfig.ElasticsearchAdmin(t.Context(), reader, esContract(true))

	// An unusable bundle is a state the user corrects, so it is a failure
	// with a reason, not an error that a retry fixes.
	require.NoError(t, err)
	assert.Nil(t, admin)
	require.NotNil(t, failure)
	assert.Equal(t, v1.ReasonInvalidReference, failure.Reason)
	assert.Contains(t, failure.Message, "my-ns/es-ca")
	assert.Contains(t, failure.Message, `"ca.crt"`)
	assert.Contains(t, failure.Message, "empty")
}
