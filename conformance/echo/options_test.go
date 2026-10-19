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

package echo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

func validOptions() Options {
	return Options{
		Registration:   "echo",
		Sections:       []string{"echo"},
		ServiceAccount: pluginsv1alpha1.ServiceAccountRef{Namespace: "plugins", Name: "echo"},
		Inputs:         []pluginsv1alpha1.InputDecl{{Name: "source", From: "echo-source/value"}},
		Outputs:        []Output{{Key: "message", FromInput: "source"}},
	}
}

func TestNewRefusesAnIncompleteOption(t *testing.T) {
	tests := []struct {
		name    string
		change  func(o *Options)
		message string
	}{
		{"no registration", func(o *Options) { o.Registration = "" }, "Registration"},
		{"no sections", func(o *Options) { o.Sections = nil }, "Sections"},
		{"no ServiceAccount name", func(o *Options) { o.ServiceAccount.Name = "" }, "ServiceAccount.Name"},
		{
			"no ServiceAccount namespace",
			func(o *Options) { o.ServiceAccount.Namespace = "" },
			"ServiceAccount.Namespace",
		},
		{"an output without a key", func(o *Options) { o.Outputs[0].Key = "" }, "Outputs[0].Key"},
		{
			"an output from an unknown input",
			func(o *Options) { o.Outputs[0].FromInput = "other" },
			`Outputs[0].FromInput`,
		},
		{
			"an output of a section that is not served",
			func(o *Options) { o.Outputs[0].Section = "access" },
			"Outputs[0].Section",
		},
		{"an unknown defect", func(o *Options) { o.Defect = "Broken" }, `Defect "Broken"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := validOptions()
			tt.change(&opts)

			c, err := New(opts)

			assert.ErrorContains(t, err, tt.message)
			assert.Nil(t, c)
		})
	}
}

func TestNewAcceptsEveryDefect(t *testing.T) {
	for _, d := range defects {
		opts := validOptions()
		opts.Defect = d

		_, err := New(opts)

		assert.NoError(t, err, "defect %s", d)
	}
}

func TestNewAcceptsTheOptionsOfTheAppHarness(t *testing.T) {
	opts := validOptions()
	opts.Sections = []string{"access", "backup-storage"}
	opts.Inputs = nil
	opts.Outputs = []Output{
		{Key: "externalUrl", Value: "https://camunda.example.com"},
		{Key: "bucket", Value: "backups"},
	}

	c, err := New(opts)

	require.NoError(t, err)
	assert.NotNil(t, c)
}

// otherRegistration is a registration that echo does not serve.
const otherRegistration = "other"

func TestServesReadsTheRegistrationOfTheSpecNotTheLabel(t *testing.T) {
	c, err := New(validOptions())
	require.NoError(t, err)

	relabelled := &pluginsv1alpha1.PluginRequest{}
	relabelled.Spec.Registration = "echo"
	relabelled.Labels = map[string]string{pluginsv1alpha1.LabelRegistration: otherRegistration}
	other := &pluginsv1alpha1.PluginRequest{}
	other.Spec.Registration = otherRegistration
	other.Labels = map[string]string{pluginsv1alpha1.LabelRegistration: "echo"}

	assert.True(t, c.serves(relabelled))
	assert.False(t, c.serves(other))
}

func TestOutputsGiveARequestOnlyTheOutputsOfItsSection(t *testing.T) {
	opts := validOptions()
	opts.Sections = []string{"echo", "access"}
	opts.Outputs = append(opts.Outputs, Output{Key: "externalUrl", Section: "access", Value: "https://x"})
	opts.Outputs[0].Section = "echo"
	c, err := New(opts)
	require.NoError(t, err)

	req := &pluginsv1alpha1.PluginRequest{}
	req.Spec.Section = "access"

	assert.Equal(t, map[string]string{"externalUrl": "https://x"}, c.outputs(req))
}
