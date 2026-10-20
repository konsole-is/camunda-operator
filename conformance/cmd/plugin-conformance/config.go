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
	"fmt"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	sigsjson "sigs.k8s.io/json"
	"sigs.k8s.io/yaml"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/conformance"
)

// fileConfig is the config file of the binary. It holds the fields of
// conformance.Config.
type fileConfig struct {
	Registration string       `json:"registration"`
	Namespace    string       `json:"namespace"`
	Cases        []fileCase   `json:"cases"`
	Timeouts     fileTimeouts `json:"timeouts"`
	Skip         []fileSkip   `json:"skip"`
}

type fileCase struct {
	Name           string                         `json:"name"`
	Inputs         map[string]string              `json:"inputs"`
	Parameters     *apiextensionsv1.JSON          `json:"parameters"`
	Target         pluginsv1alpha1.TargetRef      `json:"target"`
	DeletionPolicy pluginsv1alpha1.DeletionPolicy `json:"deletionPolicy"`
}

// fileTimeouts holds Go duration strings. An empty string takes the default
// of the library.
type fileTimeouts struct {
	OutputsReady string `json:"outputsReady"`
	Ready        string `json:"ready"`
	Deletion     string `json:"deletion"`
}

type fileSkip struct {
	ID     conformance.CheckID `json:"id"`
	Reason string              `json:"reason"`
}

// parseConfig reads a config file into the config of the library. It
// refuses a duplicate key, an unknown key, a key with the wrong case, and a
// timeout that is not a duration. It checks nothing that the library checks.
func parseConfig(data []byte) (conformance.Config, error) {
	raw, err := yaml.YAMLToJSONStrict(data)
	if err != nil {
		return conformance.Config{}, fmt.Errorf("reading the YAML: %w", err)
	}

	// encoding/json matches a key to a field without case, so it would
	// accept deletionpolicy and drop its value.
	var f fileConfig
	strictErrs, err := sigsjson.UnmarshalStrict(raw, &f, sigsjson.DisallowUnknownFields)
	if err := errors.Join(append([]error{err}, strictErrs...)...); err != nil {
		return conformance.Config{}, err
	}

	timeouts, err := f.Timeouts.parse()
	if err != nil {
		return conformance.Config{}, err
	}

	cfg := conformance.Config{
		Registration: f.Registration,
		Namespace:    f.Namespace,
		Timeouts:     timeouts,
	}
	for _, c := range f.Cases {
		cfg.Cases = append(cfg.Cases, conformance.Case{
			Name:           c.Name,
			Inputs:         c.Inputs,
			Parameters:     c.Parameters,
			Target:         c.Target,
			DeletionPolicy: c.DeletionPolicy,
		})
	}
	for _, s := range f.Skip {
		cfg.Skip = append(cfg.Skip, conformance.Skip{ID: s.ID, Reason: s.Reason})
	}

	return cfg, nil
}

func (t fileTimeouts) parse() (conformance.Timeouts, error) {
	var out conformance.Timeouts
	var errs []error
	for _, f := range []struct {
		key   string
		value string
		into  *time.Duration
	}{
		{"outputsReady", t.OutputsReady, &out.OutputsReady},
		{"ready", t.Ready, &out.Ready},
		{"deletion", t.Deletion, &out.Deletion},
	} {
		if f.value == "" {
			continue
		}

		d, err := time.ParseDuration(f.value)
		if err != nil {
			errs = append(errs, fmt.Errorf("timeouts.%s: %q is not a duration such as 5m: %w", f.key, f.value, err))
			continue
		}
		*f.into = d
	}

	return out, errors.Join(errs...)
}
