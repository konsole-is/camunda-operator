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

// Command helmfleet adds the fleet manager to the generated Helm chart. It
// copies the .yaml and .tpl files of the templates directory into
// templates/fleet-manager/ of the chart, and it appends the fleetManager
// values to values.yaml with the image of -image. It is a step of
// `make helm-generate`, after helmcli and before helmschema, not a hand edit.
// It fails when values.yaml already has fleetManager, because then the helm
// plugin did not write values.yaml again before it.
//
// Usage: helmfleet -image <repository>:<tag> [-templates dir] [chart-dir]
//
// -templates defaults to fleet-manager/deploy/chart and chart-dir to
// dist/chart.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/konsole-is/camunda-operator/hack/internal/imageref"
)

// valuesKey is the top-level key of the values that helmfleet appends.
const valuesKey = "fleetManager:"

// valuesBlock is the text that helmfleet appends to values.yaml. The two
// verbs are the quoted repository and tag. Quoted, so that YAML reads a tag
// such as 8 as a string and the values schema types it as one.
const valuesBlock = `
## Fleet manager: a web app that shows and changes the Camunda clusters of
## this Kubernetes cluster. It has no login. Reach it with kubectl port-forward.
##
fleetManager:
  ## Set to true to install the fleet manager
  ##
  enabled: false

  image:
    repository: %s
    tag: %s
    pullPolicy: IfNotPresent

  ## Resource requests and limits. The memory grows with the number of
  ## resources in the cluster.
  ##
  resources:
    limits:
      memory: 512Mi
    requests:
      cpu: 50m
      memory: 256Mi

  ## Settings that the fleet manager starts with.
  ##
  settings:
    kubernetesClusterName: ""
    git:
      repository: ""
      branch: ""
      path: ""
      credentialsSecretRef: ""
    secretStore:
      kind: ""
      name: ""
    prometheus:
      url: ""
      credentialsSecretRef: ""
`

func main() {
	image := flag.String("image", "", "the fleet manager image, <repository>:<tag>")
	templates := flag.String(
		"templates",
		"fleet-manager/deploy/chart",
		"the directory of the chart templates of the app",
	)
	flag.Parse()

	dir := "dist/chart"
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}
	if err := run(dir, *templates, *image); err != nil {
		fmt.Fprintln(os.Stderr, "helmfleet:", err)
		os.Exit(1)
	}
}

func run(chartDir, templatesDir, image string) error {
	repository, tag, err := imageref.Split(image)
	if err != nil {
		return fmt.Errorf("-image: %w", err)
	}

	valuesPath := filepath.Join(chartDir, "values.yaml")
	values, err := os.ReadFile(valuesPath)
	if err != nil {
		return err
	}
	newValues, err := appendValues(string(values), repository, tag)
	if err != nil {
		return fmt.Errorf("%s: %w", valuesPath, err)
	}

	copied, err := copyTemplates(templatesDir, filepath.Join(chartDir, "templates", "fleet-manager"))
	if err != nil {
		return err
	}

	if err := os.WriteFile(valuesPath, []byte(newValues), 0o600); err != nil {
		return err
	}
	fmt.Printf("helmfleet: copied %d templates and exposed %s as fleetManager.image\n", copied, image)

	return nil
}

// appendValues returns values with the fleetManager block at the end. It
// fails when values already has a top-level fleetManager key.
func appendValues(values, repository, tag string) (string, error) {
	for line := range strings.SplitSeq(values, "\n") {
		if line == valuesKey {
			return "", errors.New(
				"values.yaml already has fleetManager, so the helm plugin did not run before helmfleet",
			)
		}
	}
	block := fmt.Sprintf(valuesBlock, strconv.Quote(repository), strconv.Quote(tag))

	return strings.TrimRight(values, "\n") + "\n" + block, nil
}

// copyTemplates replaces target with the .yaml and .tpl files of source and
// returns how many it copied. It fails when source has none.
func copyTemplates(source, target string) (int, error) {
	entries, err := os.ReadDir(source)
	if err != nil {
		return 0, err
	}
	var names []string
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if !e.IsDir() && (ext == ".yaml" || ext == ".tpl") {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return 0, fmt.Errorf("no .yaml or .tpl file in %s", source)
	}

	// A template deleted from source must not stay in the chart.
	if err := os.RemoveAll(target); err != nil {
		return 0, err
	}
	if err := os.MkdirAll(target, 0o750); err != nil {
		return 0, err
	}
	for _, name := range names {
		content, err := os.ReadFile(filepath.Join(source, name))
		if err != nil {
			return 0, err
		}
		if err := os.WriteFile(filepath.Join(target, name), content, 0o600); err != nil {
			return 0, err
		}
	}

	return len(names), nil
}
