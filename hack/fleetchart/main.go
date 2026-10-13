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

// Command fleetchart checks the fleet manager objects of a rendered chart.
// `make helm-verify` pipes `helm template` into it, once with
// fleetManager.enabled=false and once with true, and pipes dist/install.yaml
// into it with -enabled=false.
//
// With -enabled=false it fails on each object of the fleet manager. With
// -enabled=true it checks that the chart renders each object of the app,
// that the ClusterRole grants the RBAC list of the app (contract section 6.2)
// and a read of each kind in -crds, that the settings ConfigMap rules are in
// a Role in the app namespace, that the chart never renders the settings
// ConfigMap, and that the NetworkPolicy refuses all ingress to the app pod.
//
// Usage: helm template ... | fleetchart -enabled=<bool> [-crds dir]
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"
)

// groupResource is a kind of the operator, by its API group and plural.
type groupResource struct {
	group    string
	resource string
}

func main() {
	enabled := flag.Bool("enabled", false, "whether the render set fleetManager.enabled=true")
	crds := flag.String("crds", "config/crd/bases", "the directory of the CRD bases; the app reads each kind")
	flag.Parse()

	if err := run(os.Stdin, *enabled, *crds); err != nil {
		fmt.Fprintln(os.Stderr, "fleetchart:", err)
		os.Exit(1)
	}
}

func run(input io.Reader, enabled bool, crds string) error {
	objects, err := readObjects(input)
	if err != nil {
		return err
	}

	var problems []string
	if enabled {
		kinds, err := readOperatorKinds(crds)
		if err != nil {
			return err
		}
		problems = checkEnabled(objects, kinds)
	} else {
		problems = checkDisabled(objects)
	}
	for _, p := range problems {
		fmt.Fprintln(os.Stderr, "fleetchart:", p)
	}
	if len(problems) > 0 {
		return fmt.Errorf("%d problems with fleetManager.enabled=%t", len(problems), enabled)
	}
	fmt.Printf("fleetchart: checked %d objects with fleetManager.enabled=%t\n", len(objects), enabled)

	return nil
}

// readObjects decodes a stream of YAML documents. It skips an empty
// document, which helm template writes for a template that renders nothing.
func readObjects(input io.Reader) ([]*unstructured.Unstructured, error) {
	decoder := utilyaml.NewYAMLOrJSONDecoder(input, 4096)
	var objects []*unstructured.Unstructured
	for {
		var content map[string]any
		err := decoder.Decode(&content)
		if errors.Is(err, io.EOF) {
			return objects, nil
		}
		if err != nil {
			return nil, fmt.Errorf("decoding the rendered chart: %w", err)
		}
		if len(content) > 0 {
			objects = append(objects, &unstructured.Unstructured{Object: content})
		}
	}
}

// readOperatorKinds returns the group and plural of each CRD base in dir.
func readOperatorKinds(dir string) ([]groupResource, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no CRD base in %s", dir)
	}

	kinds := make([]groupResource, 0, len(files))
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		var crd struct {
			Spec struct {
				Group string `json:"group"`
				Names struct {
					Plural string `json:"plural"`
				} `json:"names"`
			} `json:"spec"`
		}
		if err := yaml.Unmarshal(content, &crd); err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		kinds = append(kinds, groupResource{crd.Spec.Group, crd.Spec.Names.Plural})
	}

	return kinds, nil
}
