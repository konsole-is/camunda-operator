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
	"encoding/json"
	"errors"
	"flag"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

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
// required flag that args does not set, and --grant-template when its value is
// not a JSON object.
func parseOptions(args []string) (config, error) {
	flags := flag.NewFlagSet("fakeplugin", flag.ContinueOnError)
	registration := flags.String("registration", "fleet-harness", "the name of the PluginRegistration")
	saNamespace := flags.String("service-account-namespace", "", "the namespace of the ServiceAccount of the plugin")
	saName := flags.String("service-account-name", "", "the name of the ServiceAccount of the plugin")
	externalURL := flags.String(
		"access-external-url",
		"",
		"the externalUrl output of each access request, and serve the access section only when it is set",
	)
	objectStorageConfig := flags.String(
		"backup-object-storage-config",
		"",
		"the objectStorageConfig output of each backup-storage request",
	)
	grantTemplate := flags.String(
		"grant-template",
		"",
		"a JSON object, the template of the one grant template read of the registration",
	)
	probeAddress := flags.String("health-probe-bind-address", ":8081", "the address of the health probes")

	if err := flags.Parse(args); err != nil {
		return config{}, fmt.Errorf("parsing the flags: %w", err)
	}

	var errs []error
	for _, required := range []struct{ name, value string }{
		{"service-account-namespace", *saNamespace},
		{"service-account-name", *saName},
		{"backup-object-storage-config", *objectStorageConfig},
	} {
		if required.value == "" {
			errs = append(errs, fmt.Errorf("flag --%s is required", required.name))
		}
	}

	if *grantTemplate != "" {
		var object map[string]any
		if err := json.Unmarshal([]byte(*grantTemplate), &object); err != nil || object == nil {
			errs = append(errs, fmt.Errorf("flag --grant-template is not a JSON object: %q", *grantTemplate))
		}
	}

	if err := errors.Join(errs...); err != nil {
		return config{}, err
	}

	opts := echo.Options{
		Registration:   *registration,
		ServiceAccount: pluginsv1alpha1.ServiceAccountRef{Namespace: *saNamespace, Name: *saName},
	}
	if *externalURL != "" {
		opts.Sections = append(opts.Sections, pluginstartup.SectionAccess)
		opts.Outputs = append(opts.Outputs, echo.Output{
			Key:     pluginstartup.OutputExternalURL,
			Value:   *externalURL,
			Section: pluginstartup.SectionAccess,
		})
	}
	opts.Sections = append(opts.Sections, pluginstartup.SectionBackupStorage)
	opts.Outputs = append(opts.Outputs, echo.Output{
		Key:     pluginstartup.OutputObjectStorageConfig,
		Value:   *objectStorageConfig,
		Section: pluginstartup.SectionBackupStorage,
	})
	if *grantTemplate != "" {
		opts.GrantTemplates = []pluginsv1alpha1.GrantTemplate{{
			Name:     "read",
			Format:   pluginsv1alpha1.GrantFormatAWSIAMStatement,
			Template: &apiextensionsv1.JSON{Raw: []byte(*grantTemplate)},
		}}
	}

	return config{Echo: opts, HealthProbeBindAddress: *probeAddress}, nil
}
