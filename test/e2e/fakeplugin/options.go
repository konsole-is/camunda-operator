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
	"errors"
	"flag"
	"fmt"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/conformance/echo"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
)

// config is the configuration of one run of the fake plugin.
type config struct {
	Echo                   echo.Options
	HealthProbeBindAddress string
}

// parseOptions reads the flags in args. It returns one error that names each
// required flag that args does not set.
func parseOptions(args []string) (config, error) {
	flags := flag.NewFlagSet("fakeplugin", flag.ContinueOnError)
	registration := flags.String("registration", "fleet-harness", "the name of the PluginRegistration")
	saNamespace := flags.String("service-account-namespace", "", "the namespace of the ServiceAccount of the plugin")
	saName := flags.String("service-account-name", "", "the name of the ServiceAccount of the plugin")
	externalURL := flags.String("access-external-url", "", "the externalUrl output of each access request")
	objectStorageConfig := flags.String(
		"backup-object-storage-config",
		"",
		"the objectStorageConfig output of each backup-storage request",
	)
	probeAddress := flags.String("health-probe-bind-address", ":8081", "the address of the health probes")

	if err := flags.Parse(args); err != nil {
		return config{}, fmt.Errorf("parsing the flags: %w", err)
	}

	var errs []error
	for _, required := range []struct{ name, value string }{
		{"service-account-namespace", *saNamespace},
		{"service-account-name", *saName},
		{"access-external-url", *externalURL},
		{"backup-object-storage-config", *objectStorageConfig},
	} {
		if required.value == "" {
			errs = append(errs, fmt.Errorf("flag --%s is required", required.name))
		}
	}

	if err := errors.Join(errs...); err != nil {
		return config{}, err
	}

	return config{
		Echo: echo.Options{
			Registration:   *registration,
			Sections:       []string{pluginstartup.SectionAccess, pluginstartup.SectionBackupStorage},
			ServiceAccount: pluginsv1alpha1.ServiceAccountRef{Namespace: *saNamespace, Name: *saName},
			Outputs: []echo.Output{
				{
					Key:     pluginstartup.OutputExternalURL,
					Value:   *externalURL,
					Section: pluginstartup.SectionAccess,
				},
				{
					Key:     pluginstartup.OutputObjectStorageConfig,
					Value:   *objectStorageConfig,
					Section: pluginstartup.SectionBackupStorage,
				},
			},
		},
		HealthProbeBindAddress: *probeAddress,
	}, nil
}
