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

// Package merge runs the merge cases in its kind directories against the
// merge functions of the operator. The package holds tests only.
package merge

import (
	"encoding/json"
	"flag"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
	"github.com/konsole-is/camunda-operator/pkg/components/elasticsearchcluster"
)

// make merge-fixtures sets update.
var update = flag.Bool("update", false, "write the merged field of each case from the merge function of its kind")

func TestCamundaClusterFixtures(t *testing.T) {
	t.Parallel()

	runFixtures(t, "camundacluster", camundacluster.MergeSpec)
}

func TestElasticsearchClusterFixtures(t *testing.T) {
	t.Parallel()

	runFixtures(t, "elasticsearchcluster", elasticsearchcluster.MergeSpec)
}

func TestDatabaseServerFixtures(t *testing.T) {
	t.Parallel()

	runFixtures(t, "databaseserver", databaseserver.MergeSpec)
}

// Not parallel: with -update, the fixture tests rewrite the files that this
// test reads, and Go runs them only after every sequential test ends.
func TestFixturesCoverEveryMergeRule(t *testing.T) {
	tests := []struct {
		kind string
		page string
	}{
		{"camundacluster", "camundaclusterpreset.md"},
		{"elasticsearchcluster", "elasticsearchclusterpreset.md"},
		{"databaseserver", "databaseserverpreset.md"},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			t.Parallel()

			rules, err := mergeRules(tt.page)
			require.NoError(t, err)
			fixtures, err := loadFixtures(tt.kind)
			require.NoError(t, err)

			uncovered, unknown := ruleCoverage(rules, fixtures)
			assert.Empty(
				t,
				uncovered,
				"rows of the Merge rules table of docs/crds/%s without a case in %s/",
				tt.page,
				tt.kind,
			)
			assert.Empty(t, unknown, "cases in %s/ whose rule is not a row of docs/crds/%s", tt.kind, tt.page)
		})
	}
}

// runFixtures runs merge on the layers of every case in the kind directory
// and compares the result with its merged field. With -update, it writes the
// result to the merged field instead.
func runFixtures[S, P, R any](t *testing.T, kind string, merge func(S, *P, *R) S) {
	t.Helper()

	fixtures, err := loadFixtures(kind)
	require.NoError(t, err)
	require.NotEmpty(t, fixtures, "no cases in %s/", kind)

	for _, f := range fixtures {
		t.Run(filepath.Base(f.Path), func(t *testing.T) {
			t.Parallel()

			require.NoError(t, checkFixture(f, kind), f.Path)

			spec, err := decodeLayer[S](f.Spec)
			require.NoError(t, err, "decoding spec")
			require.NotNil(t, spec, "a case needs a spec")
			preset, err := decodeLayer[P](f.Preset)
			require.NoError(t, err, "decoding preset")
			release, err := decodeLayer[R](f.Release)
			require.NoError(t, err, "decoding release")

			got, err := json.Marshal(merge(*spec, preset, release))
			require.NoError(t, err)

			if *update {
				f.Merged = got
				require.NoError(t, writeFixture(f))
				return
			}

			require.NotEmpty(t, f.Merged, "%s has no merged field: run make merge-fixtures", f.Path)
			assert.JSONEq(t, string(f.Merged), string(got), "%s: %s", f.Path, f.Name)
		})
	}
}
