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

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/conformance"
)

// sampleConfig is the library config of testdata/config.yaml.
func sampleConfig() conformance.Config {
	return conformance.Config{
		Registration: "s3-backup-bucket",
		Namespace:    "conformance-s3-backup-bucket",
		Timeouts: conformance.Timeouts{
			OutputsReady: 2 * time.Minute,
			Ready:        5 * time.Minute,
			Deletion:     90 * time.Second,
		},
		Skip: []conformance.Skip{{ID: "ORDER-REGISTRATION-FIRST", Reason: "the plugin has no Deployment"}},
		Cases: []conformance.Case{
			{
				Name:       "default",
				Inputs:     map[string]string{"bucket-region": "eu-west-1"},
				Parameters: &apiextensionsv1.JSON{Raw: []byte(`{"retentionDays":30,"versioning":true}`)},
				Target: pluginsv1alpha1.TargetRef{
					Kind:               "CamundaCluster",
					Name:               "orders",
					ServiceAccountName: "orders-camunda",
				},
			},
			{
				Name:           "orphan",
				Target:         pluginsv1alpha1.TargetRef{Kind: "CamundaCluster", Name: "orders"},
				DeletionPolicy: pluginsv1alpha1.DeletionPolicyOrphan,
			},
		},
	}
}

// assertSampleConfig asserts that got is sampleConfig. Parameters are
// compared as JSON, because the key order of the raw bytes is not part of
// the contract.
func assertSampleConfig(t *testing.T, got conformance.Config) {
	t.Helper()

	want := sampleConfig()
	if assert.Len(t, got.Cases, len(want.Cases)) && assert.NotNil(t, got.Cases[0].Parameters) {
		assert.JSONEq(t, string(want.Cases[0].Parameters.Raw), string(got.Cases[0].Parameters.Raw))
		want.Cases[0].Parameters, got.Cases[0].Parameters = nil, nil
	}

	assert.Equal(t, want, got)
}

func TestParseConfigReadsTheSampleIntoTheLibraryConfig(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	got, err := parseConfig(data)
	require.NoError(t, err)

	assertSampleConfig(t, got)
}

func TestParseConfigLeavesMissingTimeoutsForTheDefaults(t *testing.T) {
	got, err := parseConfig([]byte("registration: echo\ncases:\n  - name: default\n"))
	require.NoError(t, err)

	assert.Equal(t, conformance.Timeouts{}, got.Timeouts)
	require.Len(t, got.Cases, 1)
	assert.Nil(t, got.Cases[0].Parameters, "a case without parameters has none")
}

func TestParseConfigRefusesUnknownAndMiscasedKeys(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		key  string
	}{
		{"typo at the top", "registraton: echo\n", "registraton"},
		{"wrong case in a case", "cases:\n  - name: default\n    deletionpolicy: Orphan\n", "deletionpolicy"},
		{"singular of timeouts", "timeout:\n  ready: 5m\n", "timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseConfig([]byte(tt.yaml))

			assert.ErrorContains(t, err, tt.key)
		})
	}
}

func TestParseConfigRefusesDuplicateKeys(t *testing.T) {
	_, err := parseConfig([]byte("registration: echo\nregistration: other\n"))

	assert.ErrorContains(t, err, "registration")
}

func TestParseConfigRefusesATimeoutThatIsNotADuration(t *testing.T) {
	for _, value := range []string{"300", "5 minutes"} {
		t.Run(value, func(t *testing.T) {
			_, err := parseConfig([]byte("timeouts:\n  ready: " + value + "\n"))

			assert.ErrorContains(t, err, "timeouts.ready")
		})
	}
}
