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

package builtin

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/yaml"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// The conformance check REG-RBAC and the Grant admission policy both need the
// manager to hold serve on each registration that it serves.
func TestManagerRoleGrantsServeOnEachServedRegistration(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "config", "rbac", "role.yaml"))
	require.NoError(t, err)

	var role rbacv1.ClusterRole
	require.NoError(t, yaml.Unmarshal(data, &role))

	var granted []string
	for _, rule := range role.Rules {
		if slices.Contains(rule.APIGroups, pluginsv1alpha1.GroupVersion.Group) &&
			slices.Contains(rule.Resources, "pluginregistrations") &&
			slices.Contains(rule.Verbs, "serve") {
			granted = append(granted, rule.ResourceNames...)
		}
	}

	slices.Sort(granted)
	assert.Equal(t, Served(), granted)
}
