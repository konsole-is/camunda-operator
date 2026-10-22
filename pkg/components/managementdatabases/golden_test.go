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

package managementdatabases

import (
	"flag"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/testing/golden"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// updateGolden refreshes the golden manifests with the rendered output:
// go test ./pkg/components/managementdatabases/ -run Golden -update-golden
var updateGolden = flag.Bool("update-golden", false, "update golden files")

func TestBackingGoldenAllComponents(t *testing.T) {
	t.Parallel()

	assertBackingGolden(t, "testdata/golden/all.yaml", goldenValues(Components...))
}

func TestBackingGoldenIdentityOnly(t *testing.T) {
	t.Parallel()

	assertBackingGolden(t, "testdata/golden/identity.yaml", goldenValues(Identity))
}

func TestBackingGoldenStorageClass(t *testing.T) {
	t.Parallel()

	v := goldenValues(Identity)
	v.StorageClassName = "encrypted"
	assertBackingGolden(t, "testdata/golden/storage-class.yaml", v)
}

func assertBackingGolden(t *testing.T, path string, v Values) {
	t.Helper()

	b, err := NewBacking(v, "Ready")
	require.NoError(t, err)

	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))
	golden.AssertComponentYAML(t, path, b.Component, golden.WithScheme(scheme), golden.Update(*updateGolden))
}
