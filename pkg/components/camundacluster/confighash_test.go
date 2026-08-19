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

package camundacluster

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigHashStableAndSensitive(t *testing.T) {
	t.Parallel()

	zeebe := func(in Input) Process { return Resolve(in.Effective)[0] }
	hash := func(in Input) string { return ConfigHash(in, zeebe(in)) }

	base := hash(fixtureDefault(t))
	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{16}$`), base)
	assert.Equal(t, base, hash(fixtureDefault(t)), "same input, same hash")

	// The order of the hash inputs does not matter; the controller may pass
	// them in any order.
	reordered := fixtureDefault(t)
	reordered.HashInputs[0], reordered.HashInputs[1] = reordered.HashInputs[1], reordered.HashInputs[0]
	assert.Equal(t, base, hash(reordered))

	bumped := fixtureDefault(t)
	bumped.HashInputs[0] = "Secret/my-cluster-ns/es-user=13"
	assert.NotEqual(t, base, hash(bumped), "a resource version change rolls the pods")

	url := fixtureDefault(t)
	url.Storage.Elasticsearch.Endpoint = "https://other:9200"
	assert.NotEqual(t, base, hash(url), "an env value change rolls the pods")

	secretName := fixtureDefault(t)
	secretName.Storage.Elasticsearch.CredentialsSecretRef.Name = "other-user"
	assert.NotEqual(t, base, hash(secretName), "a Secret reference change rolls the pods")

	envFrom := fixtureDefault(t)
	envFrom.Cluster.Spec.ExtraEnvFrom = nil
	envFrom.Effective = NewEffective(MergePreset(envFrom.Cluster.Spec, mediumPreset()))
	assert.NotEqual(t, base, hash(envFrom), "an envFrom change rolls the pods")
}

// The active admin password feeds the hash of connectors only: connectors
// authenticate every call with it at runtime, while the unified processes
// read it once as the create-once initial user seed, so rolling them on a
// rotation would restart the brokers for nothing.
func TestConfigHashAdminPassword(t *testing.T) {
	t.Parallel()

	in := fixtureDefault(t)
	processes := Resolve(in.Effective)
	connectors := processes[len(processes)-1]
	require.Equal(t, ComponentConnectors, connectors.Component)
	zeebe := processes[0]

	base := ConfigHash(in, connectors)
	zeebeBase := ConfigHash(in, zeebe)

	rotated := fixtureDefault(t)
	rotated.AdminPasswordHash = PasswordHash("the-new-password")
	assert.NotEqual(t, base, ConfigHash(rotated, connectors), "a password change rolls connectors")
	assert.Equal(t, zeebeBase, ConfigHash(rotated, zeebe), "a password change does not roll the brokers")

	same := fixtureDefault(t)
	same.AdminPasswordHash = rotated.AdminPasswordHash
	assert.Equal(t, ConfigHash(rotated, connectors), ConfigHash(same, connectors), "same password, same hash")
}

// PasswordHash never returns the password itself, and equal passwords hash
// equal so the config hash stays stable across reconciles.
func TestPasswordHash(t *testing.T) {
	t.Parallel()

	hash := PasswordHash("s3cret")
	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{16}$`), hash)
	assert.NotContains(t, hash, "s3cret")
	assert.Equal(t, hash, PasswordHash("s3cret"))
	assert.NotEqual(t, hash, PasswordHash("other"))
	assert.Empty(t, PasswordHash(""), "no password, no hash input")
}
