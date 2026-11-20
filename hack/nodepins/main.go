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

// Command nodepins proves that the fleet manager pins one Node.js release. The
// check fails when fleet-manager/.node-version, engines.node of
// fleet-manager/package.json, the build stage of fleet-manager/Dockerfile, and
// nodejs of .tool-versions do not all name the same release, or when one of
// them names none. pnpm refuses to install with a release other than the one
// of engines.node.
//
// Usage: nodepins [root]
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// pin is one place that names the Node.js release. read returns the release
// that the content of file names, or an empty string when it names none.
type pin struct {
	file  string
	field string
	read  func(content []byte) (string, error)
}

// buildStage is the line that the custom manager of renovate.json5 reads, so a
// line in another shape fails here too.
var buildStage = regexp.MustCompile(`(?m)^FROM node:(\d+\.\d+\.\d+)-\S+ AS build$`)

var toolVersion = regexp.MustCompile(`(?m)^nodejs\s+(\S+)\s*$`)

var pins = []pin{
	{file: "fleet-manager/.node-version", field: "release", read: nodeVersion},
	{file: "fleet-manager/package.json", field: "engines.node", read: enginesNode},
	{file: "fleet-manager/Dockerfile", field: "build stage", read: submatch(buildStage)},
	{file: ".tool-versions", field: "nodejs", read: submatch(toolVersion)},
}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	if err := check(root); err != nil {
		fmt.Fprintln(os.Stderr, "nodepins:", err)
		os.Exit(1)
	}
}

// check returns an error that names each pin and its release when the pins
// below root do not all name one release.
func check(root string) error {
	releases := make([]string, len(pins))
	for i, p := range pins {
		content, err := os.ReadFile(filepath.Join(root, p.file))
		if err != nil {
			return fmt.Errorf("reading %s: %w", p.file, err)
		}

		release, err := p.read(content)
		if err != nil {
			return fmt.Errorf("reading %s %s: %w", p.file, p.field, err)
		}
		releases[i] = release
	}

	differs := func(release string) bool { return release != releases[0] }
	if releases[0] != "" && !slices.ContainsFunc(releases, differs) {
		return nil
	}

	var msg strings.Builder
	msg.WriteString("the Node.js pins of the fleet manager must name one release, because pnpm refuses any other:")
	for i, p := range pins {
		release := releases[i]
		if release == "" {
			release = "(none)"
		}
		fmt.Fprintf(&msg, "\n  %s %s: %s", p.file, p.field, release)
	}
	return errors.New(msg.String())
}

func nodeVersion(content []byte) (string, error) {
	return strings.TrimSpace(string(content)), nil
}

func enginesNode(content []byte) (string, error) {
	var pkg struct {
		Engines struct {
			Node string `json:"node"`
		} `json:"engines"`
	}
	if err := json.Unmarshal(content, &pkg); err != nil {
		return "", err
	}
	return pkg.Engines.Node, nil
}

func submatch(re *regexp.Regexp) func(content []byte) (string, error) {
	return func(content []byte) (string, error) {
		m := re.FindSubmatch(content)
		if m == nil {
			return "", nil
		}
		return string(m[1]), nil
	}
}
