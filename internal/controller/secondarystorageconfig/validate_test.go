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

package secondarystorageconfig

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestSecondaryStorageConfigValidate(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	credentialsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "es-credentials", Namespace: "camunda"},
		Data:       map[string][]byte{"username": []byte("u"), "password": []byte("p")},
	}
	partialSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "es-credentials", Namespace: "camunda"},
		Data:       map[string][]byte{"username": []byte("u")},
	}
	database := &v1.DatabaseConfig{ObjectMeta: metav1.ObjectMeta{Name: "camunda-db", Namespace: "camunda"}}
	databaseElsewhere := &v1.DatabaseConfig{ObjectMeta: metav1.ObjectMeta{Name: "camunda-db", Namespace: "other"}}

	caSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "es-ca", Namespace: "camunda"},
		Data:       map[string][]byte{"ca.crt": []byte("pem")},
	}
	caSecretNoKey := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "es-ca", Namespace: "camunda"},
		Data:       map[string][]byte{"tls.crt": []byte("pem")},
	}

	elasticsearch := v1.SecondaryStorageConfigSpec{
		Type: v1.SecondaryStorageTypeElasticsearch,
		Elasticsearch: &v1.ElasticsearchStorage{
			Endpoint: "https://es:9200",
			CredentialsSecretRef: v1.CredentialsSecretRef{
				Name: "es-credentials", Namespace: "camunda",
				UsernameKey: "username", PasswordKey: "password",
			},
		},
	}
	elasticsearchWithCA := *elasticsearch.DeepCopy()
	elasticsearchWithCA.Elasticsearch.CASecretRef = &v1.SecretKeyRef{
		Name: "es-ca", Namespace: "camunda", Key: "ca.crt",
	}
	rdbms := v1.SecondaryStorageConfigSpec{
		Type:  v1.SecondaryStorageTypeRDBMS,
		RDBMS: &v1.RDBMSStorage{DatabaseConfigRef: "camunda-db"},
	}

	tests := []struct {
		name        string
		spec        v1.SecondaryStorageConfigSpec
		objects     []client.Object
		wantStatus  metav1.ConditionStatus
		wantReason  string
		wantMessage string
		wantErr     string
	}{
		{
			name:        "elasticsearch with complete secret is healthy",
			spec:        elasticsearch,
			objects:     []client.Object{credentialsSecret},
			wantStatus:  metav1.ConditionTrue,
			wantReason:  v1.ReasonHealthy,
			wantMessage: "All checks passed",
		},
		{
			name:        "elasticsearch with missing secret reports MissingSecret",
			spec:        elasticsearch,
			wantStatus:  metav1.ConditionFalse,
			wantReason:  v1.ReasonMissingSecret,
			wantMessage: `Secret "camunda/es-credentials" not found`,
		},
		{
			name:        "elasticsearch with secret lacking a key names the key",
			spec:        elasticsearch,
			objects:     []client.Object{partialSecret},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  v1.ReasonMissingSecret,
			wantMessage: `Secret "camunda/es-credentials" is missing key "password"`,
		},
		{
			name:        "elasticsearch with credentials and CA secrets is healthy",
			spec:        elasticsearchWithCA,
			objects:     []client.Object{credentialsSecret, caSecret},
			wantStatus:  metav1.ConditionTrue,
			wantReason:  v1.ReasonHealthy,
			wantMessage: "All checks passed",
		},
		{
			name:        "elasticsearch with missing CA secret reports MissingSecret",
			spec:        elasticsearchWithCA,
			objects:     []client.Object{credentialsSecret},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  v1.ReasonMissingSecret,
			wantMessage: `Secret "camunda/es-ca" not found`,
		},
		{
			name:        "elasticsearch with CA secret lacking the key names the key",
			spec:        elasticsearchWithCA,
			objects:     []client.Object{credentialsSecret, caSecretNoKey},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  v1.ReasonMissingSecret,
			wantMessage: `Secret "camunda/es-ca" is missing key "ca.crt"`,
		},
		{
			name:        "rdbms with a same-namespace DatabaseConfig is healthy",
			spec:        rdbms,
			objects:     []client.Object{database},
			wantStatus:  metav1.ConditionTrue,
			wantReason:  v1.ReasonHealthy,
			wantMessage: "All checks passed",
		},
		{
			name:        "rdbms with dangling DatabaseConfig reports InvalidReference",
			spec:        rdbms,
			wantStatus:  metav1.ConditionFalse,
			wantReason:  v1.ReasonInvalidReference,
			wantMessage: `DatabaseConfig "camunda-db" not found`,
		},
		{
			name:        "rdbms ignores a same-named DatabaseConfig in another namespace",
			spec:        rdbms,
			objects:     []client.Object{databaseElsewhere},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  v1.ReasonInvalidReference,
			wantMessage: `DatabaseConfig "camunda-db" not found`,
		},
		{
			name:    "elasticsearch type without elasticsearch block errors",
			spec:    v1.SecondaryStorageConfigSpec{Type: v1.SecondaryStorageTypeElasticsearch},
			wantErr: `spec.type "elasticsearch" has no matching configuration block`,
		},
		{
			name:    "rdbms type without rdbms block errors",
			spec:    v1.SecondaryStorageConfigSpec{Type: v1.SecondaryStorageTypeRDBMS},
			wantErr: `spec.type "rdbms" has no matching configuration block`,
		},
		{
			name:    "unknown type errors",
			spec:    v1.SecondaryStorageConfigSpec{Type: "opensearch"},
			wantErr: `spec.type "opensearch" has no matching configuration block`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.objects...).Build()
			r := &SecondaryStorageConfigReconciler{Client: c, APIReader: c, Scheme: scheme}
			secondaryStorage := &v1.SecondaryStorageConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "storage", Namespace: "camunda", Generation: 3},
				Spec:       tt.spec,
			}

			cond, err := r.validate(context.Background(), secondaryStorage)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, v1.ConditionReady, cond.Type)
			assert.Equal(t, tt.wantStatus, cond.Status)
			assert.Equal(t, tt.wantReason, cond.Reason)
			assert.Equal(t, tt.wantMessage, cond.Message)
			assert.Equal(t, secondaryStorage.Generation, cond.ObservedGeneration)
		})
	}
}
