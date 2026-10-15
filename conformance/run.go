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
)

// Run runs every registered check against cfg, in registry order, each as a
// subtest of t named by its ID. A check that cfg skips is reported as
// skipped with its reason. A failed check fails t, and the later checks
// still run. When cfg is not valid, for example because it skips a
// mandatory check, Run fails t and runs no check.
func Run(t *testing.T, cfg Config) {
	t.Helper()
	runChecks(t, cfg, checks)
}

func runChecks(t *testing.T, cfg Config, registry []Check) {
	t.Helper()

	steps, err := plan(cfg, registry)
	if err != nil {
		t.Errorf("conformance: the config of the run is not valid: %v", err)
		return
	}

	cfg.Timeouts = cfg.Timeouts.withDefaults()
	s := &suite{cfg: cfg}

	for _, st := range steps {
		t.Run(string(st.check.ID), func(t *testing.T) {
			if st.skipReason != "" {
				t.Skip("skipped by the config: " + st.skipReason)
			}

			st.check.run(t, s)
		})
	}
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
