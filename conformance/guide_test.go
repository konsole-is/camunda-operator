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

package conformance_test

import (
	"errors"
	"io/fs"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/konsole-is/camunda-operator/conformance"
)

// The guide lists the checks for plugin authors. It must list each check
// of the registry, in run order, with its description and whether a run can
// skip it.
func TestPluginGuideListsEveryCheckOfTheRegistry(t *testing.T) {
	page, err := os.ReadFile("../docs/guides/writing-a-plugin.md")
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip("the guide is outside this module, so only a checkout of the repository has it")
	}
	require.NoError(t, err)

	row := regexp.MustCompile("(?m)^\\| `([A-Z]+(?:-[A-Z]+)+)` \\| (.+) \\| (Yes|No) \\|$")
	rows := row.FindAllStringSubmatch(string(page), -1)
	got := make([]string, 0, len(rows))
	for _, m := range rows {
		got = append(got, m[1]+" | "+strings.ReplaceAll(m[2], "`", "")+" | "+m[3])
	}

	checks := conformance.Checks()
	want := make([]string, 0, len(checks))
	for _, c := range checks {
		skippable := "No"
		if c.Skippable {
			skippable = "Yes"
		}
		want = append(want, string(c.ID)+" | "+c.Description+" | "+skippable)
	}

	assert.Equal(t, want, got)
}
