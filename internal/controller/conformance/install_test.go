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

package conformance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// A controller that reconciles a kind whose CRD the install leaves out stops the
// manager at the cache sync on a real cluster. envtest loads every CRD base,
// so no controller suite shows it. This test reads PROJECT, where each kind
// with a controller has controller: true.
func TestInstallListsTheCRDOfEveryReconciledKind(t *testing.T) {
	t.Parallel()

	root := moduleRoot()

	var project struct {
		Resources []struct {
			Group      string `json:"group"`
			Domain     string `json:"domain"`
			Kind       string `json:"kind"`
			Controller bool   `json:"controller"`
		} `json:"resources"`
	}
	readYAMLFile(t, filepath.Join(root, "PROJECT"), &project)

	var install struct {
		Resources []string `json:"resources"`
	}
	readYAMLFile(t, filepath.Join(root, "config", "crd", "kustomization.yaml"), &install)

	bases, err := filepath.Glob(filepath.Join(root, "config", "crd", "bases", "*.yaml"))
	require.NoError(t, err)

	files := map[string]string{}
	for _, base := range bases {
		var crd struct {
			Spec struct {
				Group string `json:"group"`
				Names struct {
					Kind string `json:"kind"`
				} `json:"names"`
			} `json:"spec"`
		}
		readYAMLFile(t, base, &crd)
		files[crd.Spec.Group+"/"+crd.Spec.Names.Kind] = "bases/" + filepath.Base(base)
	}

	for _, r := range project.Resources {
		if !r.Controller {
			continue
		}

		file, ok := files[r.Group+"."+r.Domain+"/"+r.Kind]
		if assert.True(t, ok, "config/crd/bases has no CRD of %s", r.Kind) {
			assert.Contains(t, install.Resources, file, "config/crd/kustomization.yaml must list the CRD of %s", r.Kind)
		}
	}
}

func readYAMLFile(t *testing.T, path string, into any) {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean(path))
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(data, into))
}
