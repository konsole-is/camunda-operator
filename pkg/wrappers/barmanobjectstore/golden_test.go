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

package barmanobjectstore

import (
	"flag"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/testing/golden"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

// updateGolden refreshes the golden manifests with the rendered output:
// go test ./pkg/wrappers/barmanobjectstore/ -run Golden -update-golden
var updateGolden = flag.Bool("update-golden", false, "update golden files")

// TestObjectStoreGolden pins the rendered ObjectStore of an S3 archive with
// static credentials. The hand-written types of this package are the only
// description of the schema the plugin serves, so the snapshot is what shows
// a field name changing under a caller.
func TestObjectStoreGolden(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, AddToScheme(scheme))

	res, err := NewBuilder(testObject()).Build()
	require.NoError(t, err)

	golden.AssertYAML(
		t, "testdata/golden/objectstore.yaml", res,
		golden.WithScheme(scheme), golden.Update(*updateGolden),
	)
}
