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

package controller

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
	"github.com/konsole-is/camunda-operator/pkg/conditions"
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
	database := &v1.DatabaseConfig{ObjectMeta: metav1.ObjectMeta{Name: "camunda-db"}}

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
			wantReason:  conditions.ReasonHealthy,
			wantMessage: "All checks passed",
		},
		{
			name:        "elasticsearch with missing secret reports MissingSecret",
			spec:        elasticsearch,
			wantStatus:  metav1.ConditionFalse,
			wantReason:  conditions.ReasonMissingSecret,
			wantMessage: `Secret "camunda/es-credentials" not found`,
		},
		{
			name:        "elasticsearch with secret lacking a key names the key",
			spec:        elasticsearch,
			objects:     []client.Object{partialSecret},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  conditions.ReasonMissingSecret,
			wantMessage: `Secret "camunda/es-credentials" is missing key "password"`,
		},
		{
			name:        "rdbms with existing DatabaseConfig is healthy",
			spec:        rdbms,
			objects:     []client.Object{database},
			wantStatus:  metav1.ConditionTrue,
			wantReason:  conditions.ReasonHealthy,
			wantMessage: "All checks passed",
		},
		{
			name:        "rdbms with dangling DatabaseConfig reports InvalidReference",
			spec:        rdbms,
			wantStatus:  metav1.ConditionFalse,
			wantReason:  conditions.ReasonInvalidReference,
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
			cfg := &v1.SecondaryStorageConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "storage", Generation: 3},
				Spec:       tt.spec,
			}

			cond, err := r.validate(context.Background(), cfg)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, conditions.TypeReady, cond.Type)
			assert.Equal(t, tt.wantStatus, cond.Status)
			assert.Equal(t, tt.wantReason, cond.Reason)
			assert.Equal(t, tt.wantMessage, cond.Message)
			assert.Equal(t, cfg.Generation, cond.ObservedGeneration)
		})
	}
}
