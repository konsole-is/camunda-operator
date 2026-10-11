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

package v1alpha1_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The api module exists so that a consumer gets the CRD types without the
// dependencies of the operator. The free-form JSON fields of this group need
// the apiextensions types, and the API server must not come with them.
func TestModuleDependencies(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	require.NoError(t, err)

	deps := strings.Split(strings.TrimSpace(string(out)), "\n")
	require.NotEmpty(t, deps)

	for _, dep := range deps {
		for _, forbidden := range []string{
			"sigs.k8s.io/controller-runtime",
			"github.com/sourcehawk/operator-component-framework",
			"github.com/konsole-is/camunda-operator/pkg",
			"github.com/konsole-is/camunda-operator/internal",
		} {
			assert.False(t, strings.HasPrefix(dep, forbidden), "api module must not depend on %s", dep)
		}

		assert.False(
			t,
			dep == "k8s.io/apiserver" || strings.HasPrefix(dep, "k8s.io/apiserver/"),
			"api module must not depend on %s",
			dep,
		)
	}
}
