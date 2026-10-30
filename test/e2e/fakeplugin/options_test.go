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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/conformance/echo"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
)

func TestParseOptionsServesAccessAndBackupStorage(t *testing.T) {
	t.Parallel()

	cfg, err := parseOptions([]string{
		"--service-account-namespace=fleet-harness",
		"--service-account-name=fakeplugin",
		"--access-external-url=https://camundacluster-sample-fleet-smoke.fleet-harness.test",
		"--backup-object-storage-config=fleet-harness-backups",
	})
	require.NoError(t, err)
	opts := cfg.Echo

	assert.Equal(t, ":8081", cfg.HealthProbeBindAddress)
	assert.Equal(t, "fleet-harness", opts.Registration)
	assert.Equal(t, []string{pluginstartup.SectionAccess, pluginstartup.SectionBackupStorage}, opts.Sections)
	wantSA := pluginsv1alpha1.ServiceAccountRef{Namespace: "fleet-harness", Name: "fakeplugin"}
	assert.Equal(t, wantSA, opts.ServiceAccount)
	want := []echo.Output{
		{
			Key:     pluginstartup.OutputExternalURL,
			Value:   "https://camundacluster-sample-fleet-smoke.fleet-harness.test",
			Section: pluginstartup.SectionAccess,
		},
		{
			Key:     pluginstartup.OutputObjectStorageConfig,
			Value:   "fleet-harness-backups",
			Section: pluginstartup.SectionBackupStorage,
		},
	}
	assert.ElementsMatch(t, want, opts.Outputs)
	assert.Equal(t, echo.DefectNone, opts.Defect, "the fake plugin keeps the contract")

	_, err = echo.New(opts)
	assert.NoError(t, err, "echo accepts the options")
}

func TestParseOptionsTakesTheRegistrationAndProbeAddress(t *testing.T) {
	t.Parallel()

	cfg, err := parseOptions([]string{
		"--registration=other",
		"--health-probe-bind-address=:9000",
		"--service-account-namespace=fleet-harness",
		"--service-account-name=fakeplugin",
		"--access-external-url=https://example.test",
		"--backup-object-storage-config=backups",
	})
	require.NoError(t, err)

	assert.Equal(t, "other", cfg.Echo.Registration)
	assert.Equal(t, ":9000", cfg.HealthProbeBindAddress)
}

func TestParseOptionsNamesEachMissingFlag(t *testing.T) {
	t.Parallel()

	_, err := parseOptions(nil)

	require.Error(t, err)
	for _, flag := range []string{
		"--service-account-namespace", "--service-account-name", "--backup-object-storage-config",
	} {
		assert.ErrorContains(t, err, flag)
	}
	assert.NotContains(t, err.Error(), "--access-external-url", "access is optional")
}

func TestParseOptionsWithoutAccessServesBackupStorageOnly(t *testing.T) {
	t.Parallel()

	cfg, err := parseOptions([]string{
		"--registration=e2e-echo",
		"--service-account-namespace=builtin-registrations-echo",
		"--service-account-name=echo",
		"--backup-object-storage-config=echo-bucket",
	})
	require.NoError(t, err)

	assert.Equal(t, []string{pluginstartup.SectionBackupStorage}, cfg.Echo.Sections)
	assert.Equal(t, []echo.Output{{
		Key:     pluginstartup.OutputObjectStorageConfig,
		Value:   "echo-bucket",
		Section: pluginstartup.SectionBackupStorage,
	}}, cfg.Echo.Outputs)
	assert.Empty(t, cfg.Echo.GrantTemplates)

	_, err = echo.New(cfg.Echo)
	assert.NoError(t, err, "echo accepts the options")
}

func TestParseOptionsPublishesTheGrantTemplate(t *testing.T) {
	t.Parallel()

	template := `{"Effect":"Allow","Action":["s3:GetObject"],"Resource":"arn:aws:s3:::{{ .outputs.objectStorageConfig }}/*"}`
	cfg, err := parseOptions([]string{
		"--service-account-namespace=builtin-registrations-echo",
		"--service-account-name=echo",
		"--backup-object-storage-config=echo-bucket",
		"--grant-template=" + template,
	})
	require.NoError(t, err)

	require.Len(t, cfg.Echo.GrantTemplates, 1)
	got := cfg.Echo.GrantTemplates[0]
	assert.Equal(t, "read", got.Name)
	assert.Equal(t, pluginsv1alpha1.GrantFormatAWSIAMStatement, got.Format)
	assert.Empty(t, got.RequiresInputs)
	require.NotNil(t, got.Template)
	assert.JSONEq(t, template, string(got.Template.Raw))

	_, err = echo.New(cfg.Echo)
	assert.NoError(t, err, "echo accepts the options")
}

func TestParseOptionsRefusesAGrantTemplateThatIsNotAJSONObject(t *testing.T) {
	t.Parallel()

	for _, value := range []string{`not json`, `["s3:GetObject"]`, `null`} {
		_, err := parseOptions([]string{
			"--service-account-namespace=ns",
			"--service-account-name=echo",
			"--backup-object-storage-config=bucket",
			"--grant-template=" + value,
		})
		assert.ErrorContains(t, err, "--grant-template", "value %q", value)
	}
}

func TestParseOptionsRefusesAnUnknownFlag(t *testing.T) {
	t.Parallel()

	_, err := parseOptions([]string{"--hostname-pattern=x"})

	assert.Error(t, err)
}
