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

// Command helmcli exposes the camunda-operator-cli image as its own chart
// value. The kubebuilder helm plugin projects the CAMUNDA_OPERATOR_CLI_IMAGE
// environment variable of the manager as one entry of manager.env. This tool
// runs right after the plugin and turns that entry into
//
//	manager:
//	  cliImage:
//	    repository: ghcr.io/konsole-is/camunda-operator-cli
//	    tag: 0.1.0
//
// rendered as the --camunda-operator-cli-image argument of the manager, so
// `--set manager.cliImage.tag=x` works exactly like `manager.image.tag`. It is
// a step of `make helm-generate`, not a hand edit. The chart is still
// regenerated from config/ every time. If the generated chart does not have
// the shape that it expects, it fails loudly. A plugin upgrade that changes
// that shape therefore breaks generation visibly and does not ship a chart
// without the value.
//
// Usage: helmcli [chart-dir]
//
// chart-dir defaults to dist/chart.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// envName is the environment variable that the plugin projected from
// config/manager/manager.yaml.
const envName = "CAMUNDA_OPERATOR_CLI_IMAGE"

// cliImageArg is the manager flag that the chart renders instead of the
// variable.
const cliImageArg = "--camunda-operator-cli-image"

func main() {
	dir := "dist/chart"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if err := run(dir); err != nil {
		fmt.Fprintln(os.Stderr, "helmcli:", err)
		os.Exit(1)
	}
}

func run(dir string) error {
	valuesPath := filepath.Join(dir, "values.yaml")
	templatePath := filepath.Join(dir, "templates", "manager", "manager.yaml")

	values, err := os.ReadFile(valuesPath)
	if err != nil {
		return err
	}
	template, err := os.ReadFile(templatePath)
	if err != nil {
		return err
	}

	newValues, image, err := rewriteValues(string(values))
	if err != nil {
		return fmt.Errorf("%s: %w", valuesPath, err)
	}
	newTemplate, err := rewriteTemplate(string(template))
	if err != nil {
		return fmt.Errorf("%s: %w", templatePath, err)
	}

	if err := os.WriteFile(valuesPath, []byte(newValues), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(templatePath, []byte(newTemplate), 0o600); err != nil {
		return err
	}
	fmt.Printf("helmcli: exposed %s as manager.cliImage\n", image)

	return nil
}

// rewriteValues removes the projected env entry from manager.env and adds the
// cliImage block after the image block. It returns the image that the entry
// named.
func rewriteValues(values string) (string, string, error) {
	lines := strings.Split(values, "\n")

	entry := indexOf(lines, "    - name: "+envName)
	if entry < 0 || entry+1 >= len(lines) {
		return "", "", fmt.Errorf("no manager.env entry for %s; is it set in config/manager/manager.yaml?", envName)
	}
	valueLine := strings.TrimSpace(lines[entry+1])
	image, ok := strings.CutPrefix(valueLine, "value: ")
	if !ok {
		return "", "", fmt.Errorf("the %s entry has no value line, found %q", envName, valueLine)
	}
	repository, tag, err := splitImage(strings.TrimSpace(image))
	if err != nil {
		return "", "", err
	}

	// Remove the entry. An env list that is then empty becomes the literal
	// empty list that the plugin writes when there is none.
	lines = append(lines[:entry], lines[entry+2:]...)
	envLine := indexOf(lines, "  env:")
	if envLine < 0 {
		return "", "", errors.New("no manager.env list")
	}
	if envLine+1 >= len(lines) || !strings.HasPrefix(lines[envLine+1], "    - ") {
		lines[envLine] = "  env: []"
	}

	pull := indexOf(lines, "    pullPolicy: ")
	if pull < 0 {
		return "", "", errors.New("no manager.image.pullPolicy line to anchor cliImage after")
	}
	block := []string{
		"",
		"  ## camunda-operator-cli image that the operator's Jobs run, passed to the",
		"  ## manager as --camunda-operator-cli-image. Set at release time.",
		"  ##",
		"  cliImage:",
		"    repository: " + repository,
		"    tag: " + tag,
	}
	lines = append(lines[:pull+1], append(block, lines[pull+1:]...)...)

	return strings.Join(lines, "\n"), image, nil
}

// rewriteTemplate renders the flag from the cliImage values, right after the
// health probe argument that the plugin keeps fixed.
func rewriteTemplate(template string) (string, error) {
	lines := strings.Split(template, "\n")
	anchor := indexOf(lines, "- --health-probe-bind-address=")
	if anchor < 0 {
		return "", errors.New("no --health-probe-bind-address argument to anchor the flag after")
	}
	if indexOf(lines, "- "+cliImageArg+"=") >= 0 {
		return "", fmt.Errorf("the template already renders %s", cliImageArg)
	}

	indent := lines[anchor][:strings.Index(lines[anchor], "-")]
	flag := indent + "- " + cliImageArg +
		"={{ .Values.manager.cliImage.repository }}:{{ .Values.manager.cliImage.tag }}"
	lines = append(lines[:anchor+1], append([]string{flag}, lines[anchor+1:]...)...)

	return strings.Join(lines, "\n"), nil
}

// splitImage separates repository and tag. It rejects a digest reference, a
// tagless image, and an empty repository or tag. The chart exposes exactly
// repository and tag, like the manager image. "repo:" or ":tag" renders an
// invalid image, and the generation must fail instead.
func splitImage(image string) (string, string, error) {
	colon := strings.LastIndex(image, ":")
	if colon < 0 || strings.Contains(image[colon:], "/") || strings.Contains(image, "@") {
		return "", "", fmt.Errorf("image %q must be <repository>:<tag>", image)
	}
	repository, tag := image[:colon], image[colon+1:]
	if repository == "" || tag == "" {
		return "", "", fmt.Errorf("image %q must be <repository>:<tag>, both non-empty", image)
	}

	return repository, tag, nil
}

// indexOf returns the first line that starts with prefix. A prefix without
// indentation matches after the leading spaces of the line are trimmed. A
// prefix with indentation matches only a line with the same indentation, so
// a nested key never matches a top-level anchor.
func indexOf(lines []string, prefix string) int {
	trimmed := strings.TrimLeft(prefix, " ")
	indent := len(prefix) - len(trimmed)
	for i, line := range lines {
		if indent > 0 {
			if strings.HasPrefix(line, prefix) {
				return i
			}
			continue
		}
		if strings.HasPrefix(strings.TrimLeft(line, " "), trimmed) {
			return i
		}
	}

	return -1
}
