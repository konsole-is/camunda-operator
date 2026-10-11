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

package merge

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	strictjson "sigs.k8s.io/json"

	"github.com/konsole-is/camunda-operator/test/utils"
)

// schemaVersion is the format version of the cases. It changes only when
// the format changes.
const schemaVersion = 1

// mergeRulesHeading starts the section of a preset page whose table lists
// the merge rules.
const mergeRulesHeading = "## Merge rules"

// fixture is one case under <kind>/: the layers of a merge and the spec that
// the merge function of the kind returns for them. The field order is the
// order that writeFixture writes.
type fixture struct {
	SchemaVersion int             `json:"schemaVersion"`
	Name          string          `json:"name"`
	Rule          string          `json:"rule"`
	Preset        json.RawMessage `json:"preset,omitempty"`
	Release       json.RawMessage `json:"release,omitempty"`
	Spec          json.RawMessage `json:"spec"`
	Merged        json.RawMessage `json:"merged,omitempty"`

	// Path is the file that holds the case.
	Path string `json:"-"`
}

// loadFixtures reads every *.json case in dir, in file name order. A field
// that the format does not have is an error that names the file.
func loadFixtures(dir string) ([]fixture, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("listing merge fixtures in %q: %w", dir, err)
	}

	fixtures := make([]fixture, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading merge fixture: %w", err)
		}

		var f fixture
		if err := decodeStrict(data, &f); err != nil {
			return nil, fmt.Errorf("decoding merge fixture %q: %w", path, err)
		}
		f.Path = path
		fixtures = append(fixtures, f)
	}

	return fixtures, nil
}

// decodeLayer decodes one layer of a case with the field names of the API:
// a field with the wrong case, an unknown field, or a duplicate field is an
// error. An absent or null layer is nil, which the merge functions read as
// an empty layer.
func decodeLayer[T any](raw json.RawMessage) (*T, error) {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}

	layer := new(T)
	if err := decodeStrict(raw, layer); err != nil {
		return nil, err
	}

	return layer, nil
}

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
		return fmt.Errorf("encoding merge fixture %q: %w", f.Path, err)
	}

	if err := os.WriteFile(f.Path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("writing merge fixture: %w", err)
	}

	return nil
}

// mergeRules returns the first cell of each row of the table in the
// "Merge rules" section of docs/crds/<page>.
func mergeRules(page string) ([]string, error) {
	root, err := utils.ModuleRoot()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filepath.Join(root, "docs", "crds", page))
	if err != nil {
		return nil, fmt.Errorf("reading merge rules: %w", err)
	}

	rules, err := parseMergeRules(data)
	if err != nil {
		return nil, fmt.Errorf("reading merge rules of %s: %w", page, err)
	}

	return rules, nil
}

// parseMergeRules reads the first table of the section only. A first cell
// that holds an escaped pipe is not supported.
func parseMergeRules(markdown []byte) ([]string, error) {
	_, section, found := strings.Cut(string(markdown), "\n"+mergeRulesHeading+"\n")
	if !found {
		return nil, fmt.Errorf("no %q section", mergeRulesHeading)
	}

	var rows []string
	for line := range strings.SplitSeq(section, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || (len(rows) > 0 && !strings.HasPrefix(line, "|")) {
			break
		}
		if strings.HasPrefix(line, "|") {
			rows = append(rows, line)
		}
	}

	// The first two lines of a table are its header and its separator.
	if len(rows) < 3 {
		return nil, fmt.Errorf("the %q section has no table with rows", mergeRulesHeading)
	}

	rules := make([]string, 0, len(rows)-2)
	for _, row := range rows[2:] {
		rules = append(rules, strings.TrimSpace(strings.Split(row, "|")[1]))
	}

	return rules, nil
}

// ruleCoverage compares the rows of a "Merge rules" table with the rule of
// each case. It returns the rows that no case names, and the file names of
// the cases whose rule is not a row.
func ruleCoverage(rules []string, fixtures []fixture) (uncovered, unknown []string) {
	named := make(map[string]bool, len(fixtures))
	for _, f := range fixtures {
		named[f.Rule] = true
		if !slices.Contains(rules, f.Rule) {
			unknown = append(unknown, filepath.Base(f.Path))
		}
	}

	for _, rule := range rules {
		if !named[rule] {
			uncovered = append(uncovered, rule)
		}
	}

	return uncovered, unknown
}
