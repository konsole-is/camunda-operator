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

// Package matrix reads an entry of the version matrix of the e2e suite, a
// file test/e2e/matrix/<minor>.env.
package matrix

import (
	"fmt"
	"os"
	"strings"
)

// Read returns the NAME=value lines of the matrix entry at path. It skips
// blank lines, comments, and lines without an equals sign, and it trims the
// spaces around a name and a value.
func Read(path string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading the matrix entry: %w", err)
	}

	values := map[string]string{}
	for line := range strings.SplitSeq(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		name, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}

		values[strings.TrimSpace(name)] = strings.TrimSpace(value)
	}

	return values, nil
}
