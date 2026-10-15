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
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The report, a skip, and the docs name a check by its ID, so an ID is
// unique, has the shape of the IDs in the spec, and comes with a
// description and a run function.
func TestRegistryChecksHaveUniqueIDsAndDescriptions(t *testing.T) {
	idShape := regexp.MustCompile(`^[A-Z]+(-[A-Z]+)+$`)
	seen := map[CheckID]bool{}

	for _, c := range Checks() {
		assert.Regexp(t, idShape, string(c.ID))
		assert.False(t, seen[c.ID], "check %s is registered more than once", c.ID)
		assert.NotEmpty(t, c.Description, "check %s has no description", c.ID)
		assert.NotNil(t, c.run, "check %s has no run function", c.ID)
		seen[c.ID] = true
	}
}

func TestChecksReturnsACopyOfTheRegistry(t *testing.T) {
	saved := checks
	t.Cleanup(func() { checks = saved })
	checks = []Check{{ID: "TEST-COPY", Description: "A check."}}

	got := Checks()
	got[0].ID = "TEST-CHANGED"

	assert.Equal(t, CheckID("TEST-COPY"), checks[0].ID)
}
