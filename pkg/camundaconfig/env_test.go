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

package camundaconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestKeyEnv(t *testing.T) {
	t.Parallel()

	tests := map[Key]string{
		"camunda.data.secondary-storage.type":               "CAMUNDA_DATA_SECONDARYSTORAGE_TYPE",
		"camunda.security.authentication.oidc.jwk-set-uri":  "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_JWKSETURI",
		"camunda.security.initialization.users[0].username": "CAMUNDA_SECURITY_INITIALIZATION_USERS_0_USERNAME",
		"camunda.security.initialization.default-roles.admin.users[0]": "CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_" +
			"ADMIN_USERS_0",
		"zeebe.broker.gateway.enable": "ZEEBE_BROKER_GATEWAY_ENABLE",
		"server.port":                 "SERVER_PORT",
	}
	for k, want := range tests {
		assert.Equal(t, want, k.Env(), k)
	}
}

func TestIndex(t *testing.T) {
	t.Parallel()

	assert.Equal(
		t,
		Key("camunda.security.initialization.users[1].email"),
		Index(KeyInitializationUsers, 1, "email"),
	)
	assert.Equal(
		t,
		Key("camunda.security.initialization.default-roles.admin.users[0]"),
		Index(KeyDefaultRolesAdminUsers, 0, ""),
	)
}

func TestVar(t *testing.T) {
	t.Parallel()

	assert.Equal(
		t,
		corev1.EnvVar{Name: "CAMUNDA_CLUSTER_NAME", Value: "my-cluster"},
		Var(KeyClusterName, "my-cluster"),
	)

	source := &corev1.EnvVarSource{
		SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "es-user"},
			Key:                  "password",
		},
	}
	assert.Equal(
		t,
		corev1.EnvVar{Name: "CAMUNDA_DATA_SECONDARYSTORAGE_ELASTICSEARCH_PASSWORD", ValueFrom: source},
		VarFrom(KeyElasticsearchPassword, source),
	)
}

func TestIsDeclared(t *testing.T) {
	t.Parallel()

	assert.True(t, IsDeclared("CAMUNDA_SECURITY_INITIALIZATION_USERS_0_USERNAME"))
	assert.True(t, IsDeclared("CAMUNDA_SECURITY_INITIALIZATION_USERS_12_EMAIL"))
	assert.True(t, IsDeclared("CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_ADMIN_USERS_0"))
	assert.True(t, IsDeclared("CAMUNDA_CLUSTER_NAME"))
	assert.True(t, IsDeclared("SPRING_PROFILES_ACTIVE"))
	assert.True(t, IsDeclared("JAVA_TOOL_OPTIONS"))
	assert.True(t, IsDeclared("CAMUNDA_LICENSE_KEY"))
	assert.False(t, IsDeclared("CAMUNDA_MODE"))
	assert.False(t, IsDeclared("CAMUNDA_WEBAPPS_ENABLED"))
}

// Declared is sorted so that a renderer that walks it, or a test that prints
// it, is stable.
func TestDeclaredIsSortedAndComplete(t *testing.T) {
	t.Parallel()

	declared := Declared()
	require.NotEmpty(t, declared)
	assert.IsIncreasing(t, declared)
	assert.Contains(t, declared, KeyClusterName)
	assert.Contains(t, declared, KeyClientMode)
}
