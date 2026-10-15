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

package conformance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// validConfig returns a config that passes validate, for the tests to break one field at a time.
func validConfig() Config {
	return Config{
		Registration: "echo",
		Namespace:    "conformance-echo",
		Cases:        []Case{{Name: "default"}},
	}
}

func TestTimeoutsWithDefaultsFillsOnlyZeroFields(t *testing.T) {
	assert.Equal(
		t,
		Timeouts{OutputsReady: 5 * time.Minute, Ready: 10 * time.Minute, Deletion: 5 * time.Minute},
		Timeouts{}.withDefaults(),
	)

	set := Timeouts{OutputsReady: time.Minute, Ready: 2 * time.Minute, Deletion: 3 * time.Minute}
	assert.Equal(t, set, set.withDefaults())
}

func TestTimeoutsValidateNamesNegativeFieldsInAFixedOrder(t *testing.T) {
	err := Timeouts{OutputsReady: -time.Second, Deletion: -2 * time.Second}.validate()

	assert.EqualError(t, err, "timeout OutputsReady is negative: -1s\ntimeout Deletion is negative: -2s")
}

func TestConfigAcceptsValidFields(t *testing.T) {
	cfg := validConfig()
	cfg.Cases = append(
		cfg.Cases,
		Case{Name: "delete", DeletionPolicy: pluginsv1alpha1.DeletionPolicyDelete},
		Case{Name: "orphan", DeletionPolicy: pluginsv1alpha1.DeletionPolicyOrphan},
	)

	require.NoError(t, cfg.validate())
}

func TestConfigRefusesInvalidFields(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		message string
	}{
		{"no registration", func(c *Config) { c.Registration = "" }, "registration is empty"},
		{"no namespace", func(c *Config) { c.Namespace = "" }, "namespace is empty"},
		{"no cases", func(c *Config) { c.Cases = nil }, "at least one case"},
		{"case without a name", func(c *Config) { c.Cases = []Case{{}} }, "case 0 has no name"},
		{
			"two cases with one name",
			func(c *Config) { c.Cases = []Case{{Name: "a"}, {Name: "a"}} },
			`case name "a" is used more than once`,
		},
		{
			"unknown deletion policy",
			func(c *Config) { c.Cases[0].DeletionPolicy = "Retain" },
			`case "default" has the deletion policy "Retain"`,
		},
		{"negative timeout", func(c *Config) { c.Timeouts.Ready = -time.Second }, "timeout Ready is negative"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(&cfg)

			assert.ErrorContains(t, cfg.validate(), tt.message)
		})
	}
}
