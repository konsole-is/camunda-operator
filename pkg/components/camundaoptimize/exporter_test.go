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
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// exporterStorage is the secondary storage contract that the exporter patch is
// rendered from.
func exporterStorage() v1.ElasticsearchStorage {
	return v1.ElasticsearchStorage{
		Endpoint: "http://elasticsearch.camunda.svc:9200",
		CredentialsSecretRef: v1.CredentialsSecretRef{
			Name:        "es-credentials",
			Namespace:   "camunda",
			UsernameKey: "username",
			PasswordKey: "password",
		},
	}
}

// The exporter entries are the unified configuration keys only. A legacy
// zeebe.broker.exporters.* entry next to them stops the broker from starting.
func TestExporterEnvUsesTheUnifiedKeys(t *testing.T) {
	t.Parallel()

	env := ExporterEnv(exporterStorage())

	assert.Equal(
		t,
		[]corev1.EnvVar{
			{
				Name:  "CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_CLASSNAME",
				Value: "io.camunda.zeebe.exporter.ElasticsearchExporter",
			},
			{
				Name:  "CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_ARGS_URL",
				Value: "http://elasticsearch.camunda.svc:9200",
			},
			{
				Name:  "CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_ARGS_INDEX_PREFIX",
				Value: "zeebe-record",
			},
			{
				Name: "CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_ARGS_AUTHENTICATION_USERNAME",
				ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "es-credentials"},
					Key:                  "username",
				}},
			},
			{
				Name: "CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_ARGS_AUTHENTICATION_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "es-credentials"},
					Key:                  "password",
				}},
			},
		},
		env,
	)
	for _, e := range env {
		assert.NotContains(t, e.Name, "ZEEBE_BROKER_EXPORTERS", "legacy key in the exporter patch")
	}
}

// The index prefix of the exporter is the prefix that Optimize reads. The two
// must agree, so both come from one constant.
func TestExporterEnvPrefixMatchesTheOptimizeImportPrefix(t *testing.T) {
	t.Parallel()

	env := ExporterEnv(exporterStorage())
	prefix := ""
	for _, e := range env {
		if e.Name == "CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_ARGS_INDEX_PREFIX" {
			prefix = e.Value
		}
	}

	assert.Equal(t, envValue(t, baseEnv(fixtureMinimal(t), true), envZeebeName), prefix)
}

// The patch object carries the identity of the cluster and its exporter
// entries, and nothing else. Any other field would take ownership of that
// field away from whoever set it.
func TestExporterPatchCarriesNothingElse(t *testing.T) {
	t.Parallel()

	patch := ExporterPatch(
		types.NamespacedName{Namespace: "camunda", Name: "my-cluster"},
		ExporterEnv(exporterStorage()),
	)

	raw, err := json.Marshal(patch)
	require.NoError(t, err)

	var fields map[string]any
	require.NoError(t, json.Unmarshal(raw, &fields))
	assert.Equal(t, []string{"apiVersion", "kind", "metadata", "spec"}, sortedKeys(fields))
	assert.Equal(
		t,
		map[string]any{"name": "my-cluster", "namespace": "camunda"},
		fields["metadata"],
	)

	spec, ok := fields["spec"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []string{"zeebe"}, sortedKeys(spec))

	zeebe, ok := spec["zeebe"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []string{"extraEnv"}, sortedKeys(zeebe))
	assert.Len(t, zeebe["extraEnv"], 5)
}

// The finalizer applies the same object with no entries, which removes the
// entries this field manager owns and leaves every other entry alone.
func TestExporterPatchWithoutEntriesCarriesAnEmptyList(t *testing.T) {
	t.Parallel()

	patch := ExporterPatch(types.NamespacedName{Namespace: "camunda", Name: "my-cluster"}, nil)

	require.NotNil(t, patch.Spec.Zeebe)
	assert.Empty(t, patch.Spec.Zeebe.ExtraEnv)
	assert.Equal(t, "CamundaCluster", patch.Kind)
	assert.Equal(t, v1.GroupVersion.String(), patch.APIVersion)
}

// sortedKeys returns the keys of a decoded JSON object, sorted.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	return keys
}
