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
	"errors"
	"fmt"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// Run runs every registered check against cfg, in registry order, each as a
// subtest of t named by its ID, on the cluster of cfg.RESTConfig. A check
// that cfg skips, or that skips itself, is reported as skipped with its
// reason. A failed check fails t, and the later checks still run. When cfg
// is not valid, for example because it skips a mandatory check, or when Run
// cannot connect to the cluster, Run fails t and runs no check.
func Run(t *testing.T, cfg Config) {
	t.Helper()
	runChecks(t, cfg, checks, connect)
}

func runChecks(
	t *testing.T,
	cfg Config,
	registry []Check,
	connect func(*rest.Config) (*rest.Config, client.Client, error),
) {
	t.Helper()

	steps, err := plan(cfg, registry)
	if err != nil {
		t.Errorf("conformance: the config of the run is not valid: %v", err)
		return
	}

	restConfig, c, err := connect(cfg.RESTConfig)
	if err != nil {
		t.Errorf("conformance: connecting to the cluster: %v", err)
		return
	}

	s := newSuite(cfg, restConfig, c)

	for _, st := range steps {
		t.Run(string(st.check.ID), func(t *testing.T) {
			if st.skipReason != "" {
				t.Skip("skipped by the config: " + st.skipReason)
			}

			err := st.check.run(t.Context(), s)
			if reason, skip := skipReasonOf(err); skip {
				t.Skip(reason)
			}

			if err != nil {
				t.Error(err)
			}
		})
	}
}

// connect returns the REST config of the run and a client for it. A nil
// restConfig is resolved from the kubeconfig, then from the in-cluster
// config.
func connect(restConfig *rest.Config) (*rest.Config, client.Client, error) {
	if restConfig == nil {
		var err error
		if restConfig, err = config.GetConfig(); err != nil {
			return nil, nil, err
		}
	}

	scheme, err := newScheme()
	if err != nil {
		return nil, nil, err
	}

	c, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		return nil, nil, err
	}

	return restConfig, c, nil
}

// newScheme holds the Kubernetes kinds of client-go, the core kinds of the
// operator, and the plugin kinds.
func newScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		clientgoscheme.AddToScheme,
		v1.AddToScheme,
		pluginsv1alpha1.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			return nil, err
		}
	}

	return scheme, nil
}

// step is one check of a run, with the reason from the config when the run
// skips it.
type step struct {
	check      Check
	skipReason string
}

// plan checks cfg and its skips against registry. It returns one step for
// each check of registry in its order, or an error that names every problem.
func plan(cfg Config, registry []Check) ([]step, error) {
	reasons, skipErr := skipReasons(cfg.Skip, registry)
	if err := errors.Join(cfg.validate(), skipErr); err != nil {
		return nil, err
	}

	steps := make([]step, 0, len(registry))
	for _, c := range registry {
		steps = append(steps, step{check: c, skipReason: reasons[c.ID]})
	}

	return steps, nil
}

func skipReasons(skips []Skip, registry []Check) (map[CheckID]string, error) {
	byID := make(map[CheckID]Check, len(registry))
	for _, c := range registry {
		byID[c.ID] = c
	}

	reasons := make(map[CheckID]string, len(skips))
	var errs []error
	for _, s := range skips {
		c, known := byID[s.ID]
		_, seen := reasons[s.ID]

		switch {
		case !known:
			errs = append(errs, fmt.Errorf("skip %q names no check", s.ID))
		case !c.Skippable:
			errs = append(errs, fmt.Errorf(
				"skip %q names a mandatory check, and only a skippable check can be skipped",
				s.ID,
			))
		case strings.TrimSpace(s.Reason) == "":
			errs = append(errs, fmt.Errorf("skip %q has no reason", s.ID))
		case seen:
			errs = append(errs, fmt.Errorf("check %q is skipped more than once", s.ID))
		default:
			reasons[s.ID] = s.Reason
		}
	}

	return reasons, errors.Join(errs...)
}
