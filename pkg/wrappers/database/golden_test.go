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

package database

import (
	"flag"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/testing/golden"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	apiv1 "github.com/konsole-is/camunda-operator/api/v1"
)

// updateGolden refreshes the golden manifests with the rendered output:
// go test ./pkg/wrappers/database/ -run Golden -update-golden
var updateGolden = flag.Bool("update-golden", false, "update golden files")

// goldenDatabase returns the Database that the database-server adapter
// applies for a claim: the logical database camunda on the server of the
// claim, with both bindings.
func goldenDatabase() *apiv1.Database {
	return &apiv1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: "team-a"},
		Spec: apiv1.DatabaseSpec{
			ServerRef:              "orders-pg",
			DatabaseName:           "camunda",
			DatabaseConfig:         "orders",
			SecondaryStorageConfig: "orders-pg",
		},
	}
}

// TestDatabaseGolden pins the rendered Database.
func TestDatabaseGolden(t *testing.T) {
	t.Parallel()

	res, err := NewBuilder(goldenDatabase()).Build()
	require.NoError(t, err)

	golden.AssertYAML(
		t, "testdata/golden/database.yaml", res,
		golden.WithScheme(goldenScheme(t)), golden.Update(*updateGolden),
	)
}

func goldenScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, apiv1.AddToScheme(scheme))

	return scheme
}
