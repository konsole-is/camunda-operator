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

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A render of helm template: source comments, an empty document of a
// template that rendered nothing, and two objects.
const rendered = `---
# Source: camunda-operator/templates/fleet-manager/service.yaml
---
# Source: camunda-operator/templates/rbac/controller-manager.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: verify-camunda-operator-controller-manager
---
# Source: camunda-operator/templates/fleet-manager/serviceaccount.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: verify-camunda-operator-fleet-manager
  labels:
    app.kubernetes.io/component: fleet-manager
`

func TestReadObjectsSkipsEmptyDocuments(t *testing.T) {
	t.Parallel()

	objects, err := readObjects(strings.NewReader(rendered))
	require.NoError(t, err)

	require.Len(t, objects, 2)
	assert.Equal(t, "verify-camunda-operator-controller-manager", objects[0].GetName())
	assert.Equal(t, "verify-camunda-operator-fleet-manager", objects[1].GetName())
}

func TestReadOperatorKindsReadsEachCRDBase(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	crd := "apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\n" +
		"spec:\n  group: core.camunda.io\n  names:\n    plural: camundaclusters\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "core.camunda.io_camundaclusters.yaml"), []byte(crd), 0o600))

	kinds, err := readOperatorKinds(dir)
	require.NoError(t, err)

	assert.Equal(t, []groupResource{{"core.camunda.io", "camundaclusters"}}, kinds)
}

func TestReadOperatorKindsFailsWithoutCRDBases(t *testing.T) {
	t.Parallel()

	_, err := readOperatorKinds(t.TempDir())

	require.ErrorContains(t, err, "no CRD base")
}
