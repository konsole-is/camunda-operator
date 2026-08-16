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
