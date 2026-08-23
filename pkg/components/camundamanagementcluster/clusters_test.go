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
	corev1 "k8s.io/api/core/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// envMap turns rendered environment entries into a map of plain values.
func envMap(env []corev1.EnvVar) map[string]string {
	values := map[string]string{}
	for _, e := range env {
		values[e.Name] = e.Value
	}

	return values
}

// An oidc cluster takes the token of the person who is signed in. The gateway
// binding publishes a host and a port for gRPC, and Web Modeler wants a URL.
func TestClustersEnvRendersAnOIDCCluster(t *testing.T) {
	t.Parallel()

	env := ClustersEnv(fixtureAttachedClusters()[:1])

	assert.Equal(
		t, map[string]string{
			"CAMUNDA_MODELER_CLUSTERS_0_ID":                     string(fixtureClusterOIDCUID),
			"CAMUNDA_MODELER_CLUSTERS_0_NAME":                   "prod-ns/prod",
			"CAMUNDA_MODELER_CLUSTERS_0_VERSION":                "8.9.9",
			"CAMUNDA_MODELER_CLUSTERS_0_AUTHENTICATION":         "BEARER_TOKEN",
			"CAMUNDA_MODELER_CLUSTERS_0_URL_GRPC":               "grpc://prod-gateway.prod-ns.svc:26500",
			"CAMUNDA_MODELER_CLUSTERS_0_URL_REST":               "http://prod-gateway.prod-ns.svc:8080",
			"CAMUNDA_MODELER_CLUSTERS_0_URL_WEBAPP":             "https://prod.example.com",
			"CAMUNDA_MODELER_CLUSTERS_0_AUTHORIZATIONS_ENABLED": "true",
		}, envMap(env),
	)
}

// A basic-auth cluster takes a user name and a password that Web Modeler asks
// the person for. No setting carries them, so the block names the method
// alone, and a cluster that publishes no browser URL leaves that entry out.
func TestClustersEnvRendersABasicAuthCluster(t *testing.T) {
	t.Parallel()

	env := ClustersEnv(fixtureAttachedClusters()[1:2])

	assert.Equal(
		t, map[string]string{
			"CAMUNDA_MODELER_CLUSTERS_0_ID":                     string(fixtureClusterBasicUID),
			"CAMUNDA_MODELER_CLUSTERS_0_NAME":                   "staging-ns/staging",
			"CAMUNDA_MODELER_CLUSTERS_0_VERSION":                "8.9.9",
			"CAMUNDA_MODELER_CLUSTERS_0_AUTHENTICATION":         "BASIC",
			"CAMUNDA_MODELER_CLUSTERS_0_URL_GRPC":               "grpc://staging-gateway.staging-ns.svc:26500",
			"CAMUNDA_MODELER_CLUSTERS_0_URL_REST":               "http://staging-gateway.staging-ns.svc:8080",
			"CAMUNDA_MODELER_CLUSTERS_0_AUTHORIZATIONS_ENABLED": "true",
		}, envMap(env),
	)
}

// Web Modeler stops reading at the first index it does not find, so a cluster
// that publishes no endpoints must not leave a gap behind it.
func TestClustersEnvNumbersOverAClusterWithoutEndpoints(t *testing.T) {
	t.Parallel()

	clusters := fixtureAttachedClusters()
	env := envMap(ClustersEnv([]AttachedCluster{clusters[2], clusters[0], clusters[1]}))

	assert.Equal(t, "prod-ns/prod", env["CAMUNDA_MODELER_CLUSTERS_0_NAME"])
	assert.Equal(t, "staging-ns/staging", env["CAMUNDA_MODELER_CLUSTERS_1_NAME"])
	assert.NotContains(t, env, "CAMUNDA_MODELER_CLUSTERS_2_NAME")
}

// A cluster that publishes a gRPC address with a scheme of its own keeps it.
func TestClustersEnvKeepsAGRPCSchemeThatIsAlreadyThere(t *testing.T) {
	t.Parallel()

	env := envMap(ClustersEnv([]AttachedCluster{{
		Name:         "prod",
		Namespace:    "prod-ns",
		GRPCEndpoint: "grpcs://prod.example.com:26500",
		RESTEndpoint: "https://prod.example.com",
		AuthMethod:   v1.AuthenticationMethodOIDC,
	}}))

	assert.Equal(t, "grpcs://prod.example.com:26500", env["CAMUNDA_MODELER_CLUSTERS_0_URL_GRPC"])
}

// Without a cluster Web Modeler can model but not deploy, and it needs no
// entry to say so.
func TestClustersEnvRendersNothingWithoutACluster(t *testing.T) {
	t.Parallel()

	assert.Empty(t, ClustersEnv(nil))
}
