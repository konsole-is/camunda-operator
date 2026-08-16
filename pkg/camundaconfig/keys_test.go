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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A key without a source pointer does not exist: every declared key names
// the 8.9.9 file (or the Helm chart 14.8.3 path) that declares it.
func TestEveryDeclaredKeyHasASource(t *testing.T) {
	t.Parallel()

	for _, k := range Declared() {
		source, ok := sources[k]
		assert.True(t, ok, "key %q is not in the sources table", k)
		assert.NotEmpty(t, strings.TrimSpace(source), "key %q has an empty source", k)
	}
	assert.Len(t, Declared(), len(sources))
}

// The plain environment variables and the profiles carry a source too.
func TestPlainEnvAndProfilesHaveASource(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		EnvSpringProfilesActive, EnvJavaToolOptions, EnvLicenseKeyConnectors,
		ProfileBroker, ProfileGateway, ProfileOperate, ProfileTasklist, ProfileAdmin, ProfileConsolidatedAuth,
	} {
		assert.NotEmpty(t, strings.TrimSpace(plainSources[name]), "%q has no source", name)
	}
}
