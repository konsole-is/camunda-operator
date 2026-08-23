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

package mirror

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
)

// testCluster is the cluster that the rule resolves against.
func testCluster() *v1.CamundaCluster {
	return &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "cluster-ns"},
	}
}

func TestLocalSecretName(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		source    string
		purpose   camundacluster.MirrorPurpose
		want      string
	}{
		{
			name:      "source in the cluster namespace is read where it is",
			namespace: "cluster-ns",
			source:    "db-credentials",
			purpose:   camundacluster.MirrorPurposeDBCredentials,
			want:      "db-credentials",
		},
		{
			name:      "source in another namespace is read through its copy",
			namespace: "elsewhere",
			source:    "db-credentials",
			purpose:   camundacluster.MirrorPurposeDBCredentials,
			want:      "cluster-camunda-db-credentials",
		},
		{
			name:      "the copy is named by the purpose, not by the source",
			namespace: "elsewhere",
			source:    "db-credentials",
			purpose:   camundacluster.MirrorPurposeBackupCredentials,
			want:      "cluster-camunda-backup-credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LocalSecretName(testCluster(), tt.namespace, tt.source, tt.purpose)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNeeded(t *testing.T) {
	assert.False(t, Needed(testCluster(), "cluster-ns"))
	assert.True(t, Needed(testCluster(), "elsewhere"))
}

// The copy that a reader resolves for a source in another namespace is, for
// every purpose, the Secret that the writer renders: the two sides agree by
// construction, not by coincidence.
func TestLocalSecretNameIsRendered(t *testing.T) {
	cluster := testCluster()
	mirrors := map[camundacluster.MirrorPurpose]map[string][]byte{}
	for _, purpose := range camundacluster.MirrorPurposes {
		mirrors[purpose] = map[string][]byte{"key": []byte("value")}
	}
	comp, err := camundacluster.MirroredSecretComponent(cluster, mirrors)
	require.NoError(t, err)
	objects, err := comp.Preview()
	require.NoError(t, err)

	rendered := make([]string, 0, len(objects))
	for _, obj := range objects {
		rendered = append(rendered, obj.GetName())
	}
	for _, purpose := range camundacluster.MirrorPurposes {
		assert.Contains(t, rendered, LocalSecretName(cluster, "elsewhere", "source", purpose))
	}
}

func TestCheckLocalSecret(t *testing.T) {
	present := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "db-credentials", Namespace: "cluster-ns"},
		Data:       map[string][]byte{"username": []byte("camunda")},
	}

	t.Run("every key present is no failure", func(t *testing.T) {
		reader := fake.NewClientBuilder().WithObjects(present).Build()

		failure, err := CheckLocalSecret(
			context.Background(), reader, "cluster-ns", "db-credentials",
			v1.ReasonMissingSecret, "database", "username",
		)
		require.NoError(t, err)
		assert.Nil(t, failure)
	})

	t.Run("a missing key fails with the given reason", func(t *testing.T) {
		reader := fake.NewClientBuilder().WithObjects(present).Build()

		failure, err := CheckLocalSecret(
			context.Background(), reader, "cluster-ns", "db-credentials",
			v1.ReasonMissingSecret, "database", "username", "password",
		)
		require.NoError(t, err)
		require.NotNil(t, failure)
		assert.Equal(t, v1.ReasonMissingSecret, failure.Reason)
		assert.Equal(
			t,
			`Secret "cluster-ns/db-credentials" is missing key "password". The CamundaCluster `+
				"controller keeps the local copy of database credentials that live outside the "+
				"cluster namespace",
			failure.Message,
		)
	})

	t.Run("a missing Secret carries the reason of the caller", func(t *testing.T) {
		reader := fake.NewClientBuilder().Build()

		failure, err := CheckLocalSecret(
			context.Background(), reader, "cluster-ns", "cluster-camunda-backup-credentials",
			v1.ReasonMissingCredentials, "bucket", "accessKeyId",
		)
		require.NoError(t, err)
		require.NotNil(t, failure)
		assert.Equal(t, v1.ReasonMissingCredentials, failure.Reason)
		assert.Contains(t, failure.Message, `Secret "cluster-ns/cluster-camunda-backup-credentials" not found`)
		assert.Contains(t, failure.Message, "the local copy of bucket credentials")
	})
}
