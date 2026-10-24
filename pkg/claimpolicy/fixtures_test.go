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

package claimpolicy

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	strictjson "sigs.k8s.io/json"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// make claimpolicy-fixtures sets update.
var update = flag.Bool("update", false, "write the result of each case from the function of its directory")

// schemaVersion is the format version of the cases. It changes only when the
// format changes.
const schemaVersion = 1

// fixturesRoot holds one directory of cases for each function.
const fixturesRoot = "../../test/fixtures/claimpolicy"

// fixtureDirs are the case directories that have a runner. Each table test
// adds its directory here.
var fixtureDirs = []string{
	"environment-of",
	"resolve",
	"check-allowed",
	"order",
	"inputs",
	"target-service-account",
	"slot-fields",
	"check-slot-conflicts",
	"check-served",
	"check-deletion-policy",
}

// fixture is one case: the input of a function and its result. The field
// order is the order that writeFixture writes.
type fixture struct {
	SchemaVersion int             `json:"schemaVersion"`
	Name          string          `json:"name"`
	Input         json.RawMessage `json:"input"`
	Result        json.RawMessage `json:"result,omitempty"`

	// Path is the file that holds the case.
	Path string `json:"-"`
}

// result is the output of a function, a refusal, or another error. Exactly
// one field is set.
type result struct {
	Value   any      `json:"value,omitempty"`
	Refusal *refusal `json:"refusal,omitempty"`
	Error   string   `json:"error,omitempty"`
}

type refusal struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

func TestCheckResult(t *testing.T) {
	t.Parallel()

	f := fixture{Path: "case.json", Result: json.RawMessage(`{"value":{"a":1,"b":[1,2]}}`)}

	assert.NoError(
		t,
		checkResult(f, []byte(`{"value":{"b":[1,2],"a":1.0}}`)),
		"key order and number form do not matter",
	)
	assert.ErrorContains(t, checkResult(f, []byte(`{"value":{"a":2,"b":[1,2]}}`)), "case.json")

	f.Result = nil
	assert.ErrorContains(t, checkResult(f, []byte(`{"value":{}}`)), "make claimpolicy-fixtures")
}

func TestResultOf(t *testing.T) {
	t.Parallel()

	assert.Equal(
		t,
		result{Refusal: &refusal{Reason: "NotAllowed", Message: "m"}},
		resultOf(nil, fmt.Errorf("wrapped: %w", &conditions.PreCheckFailure{Reason: "NotAllowed", Message: "m"})),
	)
	assert.Equal(t, result{Error: "broken"}, resultOf(nil, errors.New("broken")))
	assert.Equal(t, result{Value: 3}, resultOf(3, nil))
}

// Not parallel: with -update, the table tests rewrite the files that this
// test lists.
func TestEveryFixtureDirectoryHasARunner(t *testing.T) {
	entries, err := os.ReadDir(fixturesRoot)
	require.NoError(t, err)

	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}

	assert.ElementsMatch(t, fixtureDirs, dirs, "each directory under test/fixtures/claimpolicy needs a table test")
}

// Not parallel, for the reason of TestEveryFixtureDirectoryHasARunner.
func TestEveryRefusalReasonHasACase(t *testing.T) {
	reasons := map[string]bool{}
	for _, dir := range fixtureDirs {
		fixtures, err := loadFixtures(dir)
		require.NoError(t, err)

		for _, f := range fixtures {
			// A case without a result is a new case: the table test fails on
			// it, and with -update it gets its result after this test.
			if len(f.Result) == 0 {
				continue
			}

			var r result
			require.NoError(t, json.Unmarshal(f.Result, &r), f.Path)
			if r.Refusal != nil {
				reasons[r.Refusal.Reason] = true
			}
		}
	}

	for _, reason := range []string{
		fleetv1alpha1.ReasonNoEnvironment,
		fleetv1alpha1.ReasonUnknownEnvironment,
		fleetv1alpha1.ReasonNotAllowed,
		fleetv1alpha1.ReasonMissingInput,
		fleetv1alpha1.ReasonSlotConflict,
		fleetv1alpha1.ReasonWiringCycle,
		v1.ReasonInvalidReference,
	} {
		assert.True(t, reasons[reason], "no case under test/fixtures/claimpolicy has the refusal reason %s", reason)
	}
}

// runFixtures calls run on the input of every case in dir and compares the
// result with the result of the case. With -update, it writes the result to
// the case instead.
func runFixtures[I any](t *testing.T, dir string, run func(I) (any, error)) {
	t.Helper()

	fixtures, err := loadFixtures(dir)
	require.NoError(t, err)
	require.NotEmpty(t, fixtures, "no cases in %s/", dir)

	for _, f := range fixtures {
		t.Run(filepath.Base(f.Path), func(t *testing.T) {
			t.Parallel()

			require.NoError(t, checkFixture(f), f.Path)

			var in I
			require.NoError(t, decodeStrict(f.Input, &in), "decoding input of %s", f.Path)

			got, err := json.Marshal(resultOf(run(in)))
			require.NoError(t, err)

			if *update {
				f.Result = got
				require.NoError(t, writeFixture(f))
				return
			}

			require.NoError(t, checkResult(f, got))
		})
	}
}

func resultOf(value any, err error) result {
	var failure *conditions.PreCheckFailure
	switch {
	case errors.As(err, &failure):
		return result{Refusal: &refusal{Reason: failure.Reason, Message: failure.Message}}
	case err != nil:
		return result{Error: err.Error()}
	default:
		return result{Value: value}
	}
}

// checkResult compares the results as JSON values, so that the key order and
// the number form do not matter.
func checkResult(f fixture, got []byte) error {
	if len(f.Result) == 0 {
		return fmt.Errorf("%s has no result: run make claimpolicy-fixtures", f.Path)
	}

	var want, have any
	if err := json.Unmarshal(f.Result, &want); err != nil {
		return fmt.Errorf("decoding the result of %s: %w", f.Path, err)
	}
	if err := json.Unmarshal(got, &have); err != nil {
		return fmt.Errorf("decoding the new result of %s: %w", f.Path, err)
	}

	if !reflect.DeepEqual(want, have) {
		return fmt.Errorf("%s: the result is %s, and the function now gives %s", f.Path, f.Result, got)
	}

	return nil
}

// loadFixtures reads every *.json case in the directory dir of fixturesRoot,
// in file name order. A field that the format does not have is an error that
// names the file.
func loadFixtures(dir string) ([]fixture, error) {
	paths, err := filepath.Glob(filepath.Join(fixturesRoot, dir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("listing claim policy fixtures in %q: %w", dir, err)
	}

	fixtures := make([]fixture, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading claim policy fixture: %w", err)
		}

		var f fixture
		if err := decodeStrict(data, &f); err != nil {
			return nil, fmt.Errorf("decoding claim policy fixture %q: %w", path, err)
		}
		f.Path = path
		fixtures = append(fixtures, f)
	}

	return fixtures, nil
}

func checkFixture(f fixture) error {
	switch {
	case f.SchemaVersion != schemaVersion:
		return fmt.Errorf("schemaVersion is %d, not %d", f.SchemaVersion, schemaVersion)
	case f.Name == "":
		return errors.New("a case needs a name")
	case len(f.Input) == 0:
		return errors.New("a case needs an input")
	}

	return nil
}

// decodeStrict decodes with the field names of the API: a field with the
// wrong case, an unknown field, or a duplicate field is an error.
func decodeStrict(data []byte, v any) error {
	strictErrs, err := strictjson.UnmarshalStrict(data, v)
	if err != nil {
		return err
	}

	return errors.Join(strictErrs...)
}

// writeFixture writes f to its Path with a two-space indent, no HTML
// escaping, and a final newline, so that two runs write the same bytes.
func writeFixture(f fixture) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(f); err != nil {
		return fmt.Errorf("encoding claim policy fixture %q: %w", f.Path, err)
	}

	if err := os.WriteFile(f.Path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("writing claim policy fixture: %w", err)
	}

	return nil
}
