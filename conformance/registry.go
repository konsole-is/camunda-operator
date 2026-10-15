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
	"slices"
	"testing"
)

// CheckID names one check. The report, a skip, and the docs use the same ID.
type CheckID string

// Check is one entry of the registry. Run reports it as a subtest named by
// its ID. A run can skip a check only when Skippable is true.
type Check struct {
	ID          CheckID
	Description string
	Skippable   bool
	run         func(t *testing.T, s *suite)
}

// suite is what a check gets: the config of the run, with the timeout
// defaults filled in.
type suite struct {
	cfg Config
}

// checks is the registry, in run order.
var checks []Check

// Checks returns the registered checks in run order. A change to the result
// does not change the registry.
func Checks() []Check {
	return slices.Clone(checks)
}
