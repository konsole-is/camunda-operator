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

func TestDatabaseConfigValidate(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	server := &v1.DatabaseServerConfig{ObjectMeta: metav1.ObjectMeta{Name: "server"}}
	appSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "app-creds", Namespace: "ns"},
		Data:       map[string][]byte{"username": []byte("u"), "password": []byte("p")},
	}
	appSecretNoPassword := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "app-creds", Namespace: "ns"},
		Data:       map[string][]byte{"username": []byte("u")},
	}
	backupRef := &v1.CredentialsSecretRef{
		Name: "backup-creds", Namespace: "ns",
		UsernameKey: "username", PasswordKey: "password",
	}

	tests := []struct {
		name        string
		objects     []client.Object
		backupRef   *v1.CredentialsSecretRef
		wantStatus  metav1.ConditionStatus
		wantReason  string
		wantMessage string
	}{
		{
			name:        "missing server wins over missing secrets",
			objects:     nil,
			backupRef:   backupRef,
			wantStatus:  metav1.ConditionFalse,
			wantReason:  conditions.ReasonInvalidReference,
			wantMessage: `DatabaseServerConfig "server" not found`,
		},
		{
			name:        "app secret checked once the server exists",
			objects:     []client.Object{server},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  conditions.ReasonMissingSecret,
			wantMessage: `Secret "ns/app-creds" not found`,
		},
		{
			name:        "broken app secret wins over broken backup secret",
			objects:     []client.Object{server},
			backupRef:   backupRef,
			wantStatus:  metav1.ConditionFalse,
			wantReason:  conditions.ReasonMissingSecret,
			wantMessage: `Secret "ns/app-creds" not found`,
		},
		{
			name:        "missing key names the secret and key",
			objects:     []client.Object{server, appSecretNoPassword},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  conditions.ReasonMissingSecret,
			wantMessage: `Secret "ns/app-creds" is missing key "password"`,
		},
		{
			name:        "absent backup ref skips the backup check",
			objects:     []client.Object{server, appSecret},
			wantStatus:  metav1.ConditionTrue,
			wantReason:  conditions.ReasonHealthy,
			wantMessage: "All checks passed",
		},
		{
			name:        "present but broken backup ref fails after the app check",
			objects:     []client.Object{server, appSecret},
			backupRef:   backupRef,
			wantStatus:  metav1.ConditionFalse,
			wantReason:  conditions.ReasonMissingSecret,
			wantMessage: `Secret "ns/backup-creds" not found`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.objects...).Build()
			r := &DatabaseConfigReconciler{Client: c, APIReader: c, Scheme: scheme}

			cfg := &v1.DatabaseConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "db", Generation: 3},
				Spec: v1.DatabaseConfigSpec{
					ServerRef:    "server",
					DatabaseName: "camunda",
					CredentialsSecretRef: v1.CredentialsSecretRef{
						Name: "app-creds", Namespace: "ns",
						UsernameKey: "username", PasswordKey: "password",
					},
					BackupCredentialsSecretRef: tt.backupRef,
				},
			}

			cond, err := r.validate(context.Background(), cfg)
			require.NoError(t, err)

			assert.Equal(t, conditions.TypeReady, cond.Type)
			assert.Equal(t, tt.wantStatus, cond.Status)
			assert.Equal(t, tt.wantReason, cond.Reason)
			assert.Equal(t, tt.wantMessage, cond.Message)
			assert.Equal(t, cfg.Generation, cond.ObservedGeneration)
		})
	}
}
