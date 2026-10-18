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
	"context"
	"errors"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// CheckID names one check. The report, a skip, and the docs use the same ID.
type CheckID string

// Check is one entry of the registry. Run reports it as a subtest named by
// its ID. A run can skip a check only when Skippable is true.
type Check struct {
	ID          CheckID
	Description string
	Skippable   bool
	// run returns nil when the check passes, an error that says what is
	// wrong when it fails, and the error of SkipCheck to skip.
	run func(ctx context.Context, s *suite) error
}

// suite is what a check gets: the config of the run, with the timeout
// defaults filled in, and the connection to the cluster of the run. It also
// holds the state that the checks of one run share, and the cleanups that run
// after the last check.
type suite struct {
	cfg        Config
	restConfig *rest.Config
	client     client.Client
	life       *lifecycle
	cleanups   []func(ctx context.Context) error
}

func newSuite(cfg Config, restConfig *rest.Config, c client.Client) *suite {
	cfg.Timeouts = cfg.Timeouts.withDefaults()

	return &suite{cfg: cfg, restConfig: restConfig, client: c}
}

// onClose adds a cleanup that close runs after the last check of the run.
func (s *suite) onClose(fn func(ctx context.Context) error) {
	s.cleanups = append(s.cleanups, fn)
}

// close runs the cleanups in the reverse order of onClose. It runs each of
// them, also after one fails, and returns the errors of all.
func (s *suite) close(ctx context.Context) error {
	errs := make([]error, 0, len(s.cleanups))
	for _, fn := range slices.Backward(s.cleanups) {
		errs = append(errs, fn(ctx))
	}
	s.cleanups = nil

	return errors.Join(errs...)
}

// registration gets the PluginRegistration that the run checks. The error
// names it.
func (s *suite) registration(ctx context.Context) (*pluginsv1alpha1.PluginRegistration, error) {
	var reg pluginsv1alpha1.PluginRegistration
	if err := s.client.Get(ctx, types.NamespacedName{Name: s.cfg.Registration}, &reg); err != nil {
		return nil, fmt.Errorf("PluginRegistration %q: %w", s.cfg.Registration, err)
	}

	return &reg, nil
}

// checks is the registry, in run order.
var checks = slices.Concat(staticChecks, lifecycleChecks, deletionChecks)

// Checks returns the registered checks in run order. A change to the result
// does not change the registry.
func Checks() []Check {
	return slices.Clone(checks)
}

// SkipCheck returns the error that a check returns to skip itself at run
// time. The report shows reason.
func SkipCheck(reason string) error {
	return &skipError{reason: reason}
}

type skipError struct {
	reason string
}

func (e *skipError) Error() string {
	return "check skipped: " + e.reason
}

// skipReasonOf returns the reason of the SkipCheck error in the chain of
// err, and false when err holds none.
func skipReasonOf(err error) (string, bool) {
	var skip *skipError
	if !errors.As(err, &skip) {
		return "", false
	}

	return skip.reason, true
}
