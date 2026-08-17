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

package logicalbackup_test

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
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
)

func managementScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	return scheme
}

func clusterWithBinding(binding *v1.ManagementBinding) *v1.CamundaCluster {
	return &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cluster", Namespace: "my-ns"},
		Status:     v1.CamundaClusterStatus{Management: binding},
	}
}

func TestManagementClientHoldsOnAnUnpublishedBinding(t *testing.T) {
	reader := fake.NewClientBuilder().WithScheme(managementScheme(t)).Build()

	for _, binding := range []*v1.ManagementBinding{nil, {Endpoint: ""}} {
		admin, failure, err := logicalbackup.ManagementClient(
			t.Context(), reader, clusterWithBinding(binding),
		)

		require.NoError(t, err)
		assert.Nil(t, admin)
		require.NotNil(t, failure)
		assert.Equal(t, v1.ReasonProgressing, failure.Reason)
		assert.Contains(t, failure.Message, "has not published its management binding")
	}
}

func TestManagementClientBuildsWithoutAuth(t *testing.T) {
	reader := fake.NewClientBuilder().WithScheme(managementScheme(t)).Build()

	admin, failure, err := logicalbackup.ManagementClient(
		t.Context(),
		reader,
		clusterWithBinding(&v1.ManagementBinding{
			Endpoint: "http://my-cluster-zeebe.my-ns.svc:9600",
			Version:  "8.9.9",
			Auth:     v1.ManagementAuth{Method: v1.ManagementAuthMethodNone},
		}),
	)

	require.NoError(t, err)
	require.Nil(t, failure)
	assert.NotNil(t, admin)
}

func TestManagementClientRejectsBasicAuthWithoutARef(t *testing.T) {
	reader := fake.NewClientBuilder().WithScheme(managementScheme(t)).Build()

	admin, failure, err := logicalbackup.ManagementClient(
		t.Context(),
		reader,
		clusterWithBinding(&v1.ManagementBinding{
			Endpoint: "http://my-cluster-zeebe.my-ns.svc:9600",
			Version:  "8.9.9",
			Auth:     v1.ManagementAuth{Method: v1.ManagementAuthMethodBasic},
		}),
	)

	require.NoError(t, err)
	assert.Nil(t, admin)
	require.NotNil(t, failure)
	assert.Equal(t, v1.ReasonMissingSecret, failure.Reason)
	assert.Contains(t, failure.Message, "names no credentials Secret")
}

func TestManagementClientReportsAMissingCredentialsSecret(t *testing.T) {
	reader := fake.NewClientBuilder().WithScheme(managementScheme(t)).Build()

	admin, failure, err := logicalbackup.ManagementClient(
		t.Context(),
		reader,
		clusterWithBinding(&v1.ManagementBinding{
			Endpoint: "http://my-cluster-zeebe.my-ns.svc:9600",
			Version:  "8.9.9",
			Auth: v1.ManagementAuth{
				Method: v1.ManagementAuthMethodBasic,
				CredentialsSecretRef: &v1.CredentialsSecretRef{
					Name:        "management-auth",
					Namespace:   "my-ns",
					UsernameKey: "username",
					PasswordKey: "password",
				},
			},
		}),
	)

	require.NoError(t, err)
	assert.Nil(t, admin)
	require.NotNil(t, failure)
	assert.Equal(t, v1.ReasonMissingSecret, failure.Reason)
	assert.Contains(t, failure.Message, "not found")
}

func TestManagementClientResolvesBasicAuth(t *testing.T) {
	reader := fake.NewClientBuilder().WithScheme(managementScheme(t)).WithObjects(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "management-auth", Namespace: "my-ns"},
		Data: map[string][]byte{
			"username": []byte("admin"),
			"password": []byte("secret"),
		},
	}).Build()

	admin, failure, err := logicalbackup.ManagementClient(
		t.Context(),
		reader,
		clusterWithBinding(&v1.ManagementBinding{
			Endpoint: "http://my-cluster-zeebe.my-ns.svc:9600",
			Version:  "8.9.9",
			Auth: v1.ManagementAuth{
				Method: v1.ManagementAuthMethodBasic,
				CredentialsSecretRef: &v1.CredentialsSecretRef{
					Name:        "management-auth",
					Namespace:   "my-ns",
					UsernameKey: "username",
					PasswordKey: "password",
				},
			},
		}),
	)

	require.NoError(t, err)
	require.Nil(t, failure)
	assert.NotNil(t, admin)
}

func TestManagementClientRejectsAnUnsupportedVersion(t *testing.T) {
	reader := fake.NewClientBuilder().WithScheme(managementScheme(t)).Build()

	admin, failure, err := logicalbackup.ManagementClient(
		t.Context(),
		reader,
		clusterWithBinding(&v1.ManagementBinding{
			Endpoint: "http://my-cluster-zeebe.my-ns.svc:9600",
			Version:  "8.10.0",
			Auth:     v1.ManagementAuth{Method: v1.ManagementAuthMethodNone},
		}),
	)

	require.NoError(t, err)
	assert.Nil(t, admin)
	require.NotNil(t, failure)
	assert.Equal(t, v1.ReasonInvalidReference, failure.Reason)
	assert.Contains(t, failure.Message, "8.10.0")

	_ = conditions.PreCheckFailure{}
}
