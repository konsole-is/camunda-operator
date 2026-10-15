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

package conformance_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A plugin repository imports this module. It must get the api module and the
// Kubernetes libraries, never the controllers and the dependencies of the
// operator.
func TestModuleDoesNotDependOnTheOperatorModule(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "-f", "{{with .Module}}{{.Path}}{{end}}", "./...").Output()
	require.NoError(t, err)

	modules := strings.Fields(string(out))
	require.Contains(
		t,
		modules,
		"github.com/konsole-is/camunda-operator/api",
		"the list holds the modules of the imports",
	)
	assert.NotContains(t, modules, "github.com/konsole-is/camunda-operator")
}
