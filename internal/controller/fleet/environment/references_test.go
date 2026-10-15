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

package environment

import (
	"testing"

	"github.com/stretchr/testify/assert"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
)

func TestReferencesListsEveryNameOfTheSpecInKindOrder(t *testing.T) {
	spec := fleetv1alpha1.EnvironmentSpec{
		PlatformConfigRef: "production",
		Presets: fleetv1alpha1.EnvironmentPresets{
			CamundaCluster:       []string{"small", "large"},
			ElasticsearchCluster: []string{"es-small"},
			DatabaseServer:       []string{"pg-small"},
		},
		Releases: &fleetv1alpha1.EnvironmentReleases{Allowed: []string{"camunda-8-9"}},
		Sections: map[string]fleetv1alpha1.SectionPolicy{
			"backup-storage": {Allowed: []string{"s3-backups"}},
			"access":         {},
		},
		Parameters: map[string]fleetv1alpha1.ParameterPolicies{
			"aws-connectors": {"policies": {ClaimEditable: true}},
		},
	}

	assert.Equal(
		t, []reference{
			{kindPlatformConfig, "production"},
			{kindClusterPreset, "large"},
			{kindClusterPreset, "small"},
			{kindElasticsearchPreset, "es-small"},
			{kindDatabaseServerPreset, "pg-small"},
			{kindRelease, "camunda-8-9"},
			{kindSection, "access"},
			{kindSection, "backup-storage"},
			{kindRegistration, "aws-connectors"},
			{kindRegistration, "s3-backups"},
		}, references(&spec),
	)
}

func TestReferencesNamesEachObjectOnce(t *testing.T) {
	spec := fleetv1alpha1.EnvironmentSpec{
		Releases: &fleetv1alpha1.EnvironmentReleases{Allowed: []string{"camunda-8-9"}, Default: "camunda-8-9"},
		Sections: map[string]fleetv1alpha1.SectionPolicy{
			"backup-storage": {Default: "s3-backups", Allowed: []string{"s3-backups"}},
		},
		Parameters: map[string]fleetv1alpha1.ParameterPolicies{
			"s3-backups": {"retentionDays": {ClaimEditable: true}},
		},
	}

	assert.Equal(
		t, []reference{
			{kindRelease, "camunda-8-9"},
			{kindSection, "backup-storage"},
			{kindRegistration, "s3-backups"},
		}, references(&spec),
	)
}

func TestReferencesOfAnEmptySpecIsEmpty(t *testing.T) {
	assert.Empty(t, references(&fleetv1alpha1.EnvironmentSpec{}))
}

func TestEveryReferencedKindHasALookupObject(t *testing.T) {
	for _, kind := range kindOrder {
		assert.Contains(t, lookupObjects, kind)
	}
}

func TestInvalidReferenceMessageNamesTheKindAndTheNameOfEachObject(t *testing.T) {
	assert.Equal(
		t,
		"The spec names objects that do not exist: CamundaRelease camunda-8-9, PluginSection backup-storage",
		invalidReferenceMessage([]reference{{kindRelease, "camunda-8-9"}, {kindSection, "backup-storage"}}),
	)
}
