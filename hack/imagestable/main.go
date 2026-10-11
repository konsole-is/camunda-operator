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

// Command imagestable writes the images table: for each Camunda minor of the
// e2e matrix, the default repository and the tested version of each image.
// The fleet manager reads it to show default images and to fill the versions
// of a new management plane. The versions do not follow one line inside a
// minor, so the table cannot come from the release version.
//
// Each variable of a matrix entry is in imageVariables, versionOnly, or
// skipped. A variable in none of them fails the run, so a new image of the
// matrix cannot be left out of the table.
//
// Usage: imagestable -matrix <dir> -out <file>
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/konsole-is/camunda-operator/hack/internal/matrix"
	"github.com/konsole-is/camunda-operator/pkg/images"
)

// schemaVersion changes only when the format of the table changes, so that a
// reader can refuse a format it does not know.
const schemaVersion = 1

// table is the file that the command writes.
type table struct {
	SchemaVersion int `json:"schemaVersion"`
	// Minors holds the images of each Camunda minor, by the name of the
	// image.
	Minors map[string]map[string]entry `json:"minors"`
}

// entry is one image of one minor. The pull reference is
// repository:tagPrefix+version. A version-only entry has no repository,
// because another operator picks its image.
type entry struct {
	Repository string `json:"repository,omitempty"`
	TagPrefix  string `json:"tagPrefix,omitempty"`
	Version    string `json:"version"`
}

// imageVariables maps each variable of a matrix entry to the images of
// pkg/images that run its version. The websockets image has no variable of
// its own: the e2e suite runs it at the Web Modeler version.
var imageVariables = map[string][]images.Image{
	"CAMUNDA_VERSION":             {images.Camunda},
	"CAMUNDA_CONNECTORS_VERSION":  {images.Connectors},
	"CAMUNDA_OPTIMIZE_VERSION":    {images.Optimize},
	"CAMUNDA_IDENTITY_VERSION":    {images.Identity},
	"CAMUNDA_CONSOLE_VERSION":     {images.Console},
	"CAMUNDA_WEB_MODELER_VERSION": {images.WebModelerRestapi, images.WebModelerWebsockets},
	"KEYCLOAK_VERSION":            {images.Keycloak},
	"POSTGRES_VERSION":            {images.Postgres},
}

// versionOnly maps a variable to the table name of a version whose image
// pkg/images does not resolve. ECK picks the Elasticsearch image.
var versionOnly = map[string]string{
	"ELASTICSEARCH_VERSION": "elasticsearch",
}

// skipped is every variable that is not the version of a Camunda image: the
// releases of the operators that the suite installs, and the flow filter.
var skipped = []string{"CNPG_VERSION", "BARMAN_PLUGIN_VERSION", "E2E_LABELS"}

// minorFile matches the name of a matrix entry and captures its minor.
var minorFile = regexp.MustCompile(`^(\d+\.\d+)\.env$`)

func main() {
	dir := flag.String("matrix", "test/e2e/matrix", "the directory of the matrix entries")
	out := flag.String("out", "pkg/images/table.json", "the file to write")
	flag.Parse()

	t, err := build(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	raw, err := encode(t)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := os.WriteFile(*out, raw, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "writing the images table: %v\n", err)
		os.Exit(1)
	}
}

// build reads every entry of dir into a table.
func build(dir string) (table, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.env"))
	if err != nil {
		return table{}, fmt.Errorf("listing the matrix entries: %w", err)
	}
	if len(paths) == 0 {
		return table{}, fmt.Errorf("%s holds no matrix entry", dir)
	}

	t := table{SchemaVersion: schemaVersion, Minors: map[string]map[string]entry{}}
	for _, path := range paths {
		match := minorFile.FindStringSubmatch(filepath.Base(path))
		if match == nil {
			return table{}, fmt.Errorf("%s: the name of a matrix entry is <major>.<minor>.env", path)
		}

		values, err := matrix.Read(path)
		if err != nil {
			return table{}, err
		}

		entries, err := minorEntries(values)
		if err != nil {
			return table{}, fmt.Errorf("%s: %w", path, err)
		}

		t.Minors[match[1]] = entries
	}

	return t, nil
}

// minorEntries turns the variables of one matrix entry into its images.
func minorEntries(values map[string]string) (map[string]entry, error) {
	var unknown []string
	for name := range values {
		_, image := imageVariables[name]
		_, plain := versionOnly[name]
		if !image && !plain && !slices.Contains(skipped, name) {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		slices.Sort(unknown)
		return nil, fmt.Errorf(
			"hack/imagestable maps no image to %s. Add each to imageVariables, versionOnly, or skipped",
			strings.Join(unknown, ", "),
		)
	}

	entries := map[string]entry{}
	for _, name := range slices.Sorted(maps.Keys(imageVariables)) {
		version := values[name]
		if version == "" {
			return nil, fmt.Errorf("the matrix entry holds no %s", name)
		}

		for _, img := range imageVariables[name] {
			repository := images.DefaultRepository(img, version)
			if repository == "" {
				return nil, fmt.Errorf("pkg/images has no default repository for %s", img)
			}

			entries[string(img)] = entry{Repository: repository, TagPrefix: images.TagPrefix(img), Version: version}
		}
	}

	for _, name := range slices.Sorted(maps.Keys(versionOnly)) {
		version := values[name]
		if version == "" {
			return nil, fmt.Errorf("the matrix entry holds no %s", name)
		}

		entries[versionOnly[name]] = entry{Version: version}
	}

	return entries, nil
}

// encode writes t with sorted keys and a final newline, so that two runs on
// the same matrix write the same bytes.
func encode(t table) ([]byte, error) {
	raw, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encoding the images table: %w", err)
	}

	return append(raw, '\n'), nil
}
