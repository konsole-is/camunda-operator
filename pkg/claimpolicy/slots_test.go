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

package claimpolicy

import (
	"testing"

	"github.com/stretchr/testify/assert"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
)

func TestTargetServiceAccountFixtures(t *testing.T) {
	t.Parallel()

	type input struct {
		Kind    pluginsv1alpha1.ClaimKind      `json:"kind"`
		Claim   string                         `json:"claim"`
		Section *pluginsv1alpha1.PluginSection `json:"section"`
	}
	type value struct {
		ServiceAccountName string `json:"serviceAccountName"`
	}

	runFixtures(t, "target-service-account", func(in input) (any, error) {
		return value{ServiceAccountName: TargetServiceAccount(in.Kind, in.Claim, in.Section)}, nil
	})
}

func TestSlotFieldsFixtures(t *testing.T) {
	t.Parallel()

	type input struct {
		Kind       pluginsv1alpha1.ClaimKind       `json:"kind"`
		Sections   []pluginsv1alpha1.PluginSection `json:"sections,omitempty"`
		Selections []fleetv1alpha1.PluginSelection `json:"selections,omitempty"`
		Requests   []pluginsv1alpha1.PluginRequest `json:"requests,omitempty"`
	}

	runFixtures(t, "slot-fields", func(in input) (any, error) {
		return SlotFields(in.Kind, in.Sections, in.Selections, in.Requests)
	})
}

func TestOutputKeysAreTheOutputsOfTheCoreSections(t *testing.T) {
	t.Parallel()

	assert.Equal(t, pluginstartup.OutputServiceAccountAnnotations, outputServiceAccountAnnotations)
	assert.Equal(t, pluginstartup.OutputStorageClassName, outputStorageClassName)
	assert.Equal(t, pluginstartup.OutputObjectStorageConfig, outputObjectStorageConfig)
	assert.Equal(t, pluginstartup.OutputSecondaryStorageConfig, outputSecondaryStorageConfig)
	assert.Equal(t, pluginstartup.OutputExternalURL, outputExternalURL)
	assert.Equal(t, pluginstartup.OutputOptimizeURL, outputOptimizeURL)
	assert.Equal(t, pluginstartup.OutputManagementHost, outputManagementHost)
}
