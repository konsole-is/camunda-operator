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

package camundaoptimize

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// envValueNamed returns the literal value of the named variable.
func envValueNamed(t *testing.T, env []corev1.EnvVar, name string) string {
	t.Helper()

	for _, e := range env {
		if e.Name == name {
			return e.Value
		}
	}
	require.Failf(t, "missing variable", "%s is not in the rendered environment", name)

	return ""
}

// envNames returns the names of every rendered variable.
func envNames(env []corev1.EnvVar) []string {
	names := make([]string, 0, len(env))
	for _, e := range env {
		names = append(names, e.Name)
	}

	return names
}

// The importer imports the exported records; every other Zeebe setting is the
// same on both workloads.
func TestBaseEnvImporterEnablesImport(t *testing.T) {
	t.Parallel()

	env := baseEnv(newInput(t, func(in *Input) { in.Partitions = 3 }), true)

	assert.Equal(t, "true", envValueNamed(t, env, envZeebeEnabled))
	assert.Equal(t, "zeebe-record", envValueNamed(t, env, envZeebeName))
	assert.Equal(t, "3", envValueNamed(t, env, envZeebePartitionCount))
}

// The webapp serves the user interface and must not import, or two instances
// write the same Optimize indices.
func TestBaseEnvWebappDisablesImport(t *testing.T) {
	t.Parallel()

	env := baseEnv(fixtureMinimal(t), false)

	assert.Equal(t, "false", envValueNamed(t, env, envZeebeEnabled))
	assert.Equal(t, "ccsm", envValueNamed(t, env, "SPRING_PROFILES_ACTIVE"))
	assert.Equal(t, "elasticsearch", envValueNamed(t, env, envDatabase))
}

// Optimize takes the Elasticsearch host and port apart, so the endpoint is
// split and the scheme becomes the TLS switch.
func TestBaseEnvSplitsTheElasticsearchEndpoint(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		endpoint string
		host     string
		port     string
		ssl      string
	}{
		"explicit port": {"http://es.camunda.svc:9200", "es.camunda.svc", "9200", "false"},
		"tls":           {"https://es.example.com:9243", "es.example.com", "9243", "true"},
		"tls no port":   {"https://es.example.com", "es.example.com", "443", "true"},
		"plain no port": {"http://es.example.com", "es.example.com", "80", "false"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			env := baseEnv(newInput(t, func(in *Input) { in.Storage.Endpoint = tc.endpoint }), false)
			assert.Equal(t, tc.host, envValueNamed(t, env, envElasticsearchHost))
			assert.Equal(t, tc.port, envValueNamed(t, env, envElasticsearchPort))
			assert.Equal(t, tc.ssl, envValueNamed(t, env, envElasticsearchSSLEnabled))
		})
	}
}

// A storage contract with a CA mounts it and points Optimize at the mounted
// file. Without one, neither the variables nor the volume are rendered.
func TestBaseEnvTrustsTheMountedCA(t *testing.T) {
	t.Parallel()

	in := fixtureRealistic(t)
	env := baseEnv(in, false)
	assert.Equal(t, "/etc/camunda/es-ca/ca.crt", envValueNamed(t, env, envElasticsearchCAs))
	assert.Equal(t, "false", envValueNamed(t, env, envElasticsearchSelfSigned))
	require.Len(t, caVolumes(in), 1)
	assert.Equal(t, "es-ca", caVolumes(in)[0].Secret.SecretName)
	require.Len(t, caMounts(in), 1)
	assert.Equal(t, "/etc/camunda/es-ca", caMounts(in)[0].MountPath)

	minimal := fixtureMinimal(t)
	assert.NotContains(t, envNames(baseEnv(minimal, false)), envElasticsearchCAs)
	assert.Empty(t, caVolumes(minimal))
	assert.Empty(t, caMounts(minimal))
}

// The backend issuer URL is what the container reaches from inside the
// Kubernetes cluster; the contract lets it fall back to the browser issuer.
func TestIssuerBackendURLDefaultsToIssuer(t *testing.T) {
	t.Parallel()

	env := baseEnv(fixtureMinimal(t), false)
	assert.Equal(
		t, "https://identity.example.com/realms/camunda", envValueNamed(t, env, envIdentityIssuerBackendURL),
	)

	env = baseEnv(fixtureRealistic(t), false)
	assert.Equal(
		t, "http://identity.camunda.svc:8080/realms/camunda", envValueNamed(t, env, envIdentityIssuerBackendURL),
	)
}

// Every credential arrives through a Secret reference, never as a literal.
func TestBaseEnvReadsCredentialsFromSecrets(t *testing.T) {
	t.Parallel()

	env := baseEnv(fixtureRealistic(t), false)
	sources := map[string]*corev1.SecretKeySelector{}
	for _, e := range env {
		if e.ValueFrom != nil {
			sources[e.Name] = e.ValueFrom.SecretKeyRef
		}
	}

	assert.Equal(
		t,
		&corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "es-credentials"}, Key: "password",
		},
		sources[envElasticsearchPassword],
	)
	assert.Equal(
		t,
		&corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "optimize-client"}, Key: "client-secret",
		},
		sources[envIdentityClientSecret],
	)
	assert.Equal(
		t,
		&corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "camunda-license"}, Key: "license",
		},
		sources["CAMUNDA_LICENSE_KEY"],
	)
}

// A platform without a license leaves the license variable out; a platform
// registry prefixes the image.
func TestImageUsesRegistryPrefix(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "camunda/optimize:8.9.4", Image(fixtureMinimal(t)))
	assert.NotContains(t, envNames(baseEnv(fixtureMinimal(t), false)), "CAMUNDA_LICENSE_KEY")

	assert.Equal(t, "registry.example.com/mirror/camunda/optimize:8.9.4", Image(fixtureRealistic(t)))

	trailing := newInput(t, func(in *Input) {
		in.Platform = v1.CamundaPlatformConfigSpec{ImageRegistry: "registry.example.com/"}
	})
	assert.Equal(t, "registry.example.com/camunda/optimize:8.9.4", Image(trailing))
}

// The workload names come from the CamundaOptimize, so two instances on one
// cluster never collide.
func TestWorkloadName(t *testing.T) {
	t.Parallel()

	in := fixtureMinimal(t)
	assert.Equal(t, "my-optimize-webapp", WorkloadName(in.Optimize, ComponentWebapp))
	assert.Equal(t, "my-optimize-importer", WorkloadName(in.Optimize, ComponentImporter))
}

// The webapp replica count follows the spec; the importer is always one.
func TestReplicas(t *testing.T) {
	t.Parallel()

	minimal := fixtureMinimal(t)
	assert.Equal(t, int32(1), minimal.replicas(ComponentWebapp))
	assert.Equal(t, int32(1), minimal.replicas(ComponentImporter))

	realistic := fixtureRealistic(t)
	assert.Equal(t, int32(2), realistic.replicas(ComponentWebapp))
	assert.Equal(t, int32(1), realistic.replicas(ComponentImporter))
}
