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

package controller

import (
	"flag"
	"strings"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/testing/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

var update = flag.Bool("update", false, "update golden files")

// goldenStorageConfigName is the SecondaryStorageConfig name in the full
// golden fixture.
const goldenStorageConfigName = "my-storage-config"

// goldenScheme registers exactly the kinds the bindings component renders.
func goldenScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	s := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(s))
	require.NoError(t, v1.AddToScheme(s))

	return s
}

// goldenDatabase returns the doc's minimal example with a fixed name, so the
// rendered bindings are stable golden input.
func goldenDatabase() *v1.Database {
	return &v1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "my-camunda-db"},
		Spec: v1.DatabaseSpec{
			ServerRef:       "my-db-server",
			DatabaseName:    "camunda",
			TargetNamespace: "my-cluster-ns",
		},
	}
}

// goldenFullDatabase returns the doc's realistic example with a fixed name.
func goldenFullDatabase() *v1.Database {
	db := goldenDatabase()
	db.Spec.ApplicationCredentials = &v1.CredentialsSpec{
		SecretName:      "my-camunda-db-app",
		SecretNamespace: "my-secret-ns",
	}
	db.Spec.BackupCredentials = &v1.BackupCredentialsSpec{
		CredentialsSpec: v1.CredentialsSpec{SecretName: "my-camunda-db-backup"},
	}
	db.Spec.DatabaseConfig = "my-database-config"
	db.Spec.SecondaryStorageConfig = goldenStorageConfigName
	return db
}

func TestResolveBindings(t *testing.T) {
	t.Run("defaults derive from the CR name and targetNamespace", func(t *testing.T) {
		rb := resolveBindings(goldenDatabase())

		assert.Equal(t, "my-camunda-db-credentials", rb.AppSecret.Name)
		assert.Equal(t, "my-cluster-ns", rb.AppSecret.Namespace)
		assert.Equal(t, "my-camunda-db-backup-credentials", rb.BackupSecret.Name)
		assert.Equal(t, "my-cluster-ns", rb.BackupSecret.Namespace)
		assert.Equal(t, "my-camunda-db", rb.DatabaseConfigName)
		assert.Equal(t, "camunda", rb.AppUser)
		assert.Equal(t, "camunda_backup", rb.BackupUser)
		assert.True(t, rb.BackupEnabled)
	})

	t.Run("explicit names and namespaces win over defaults", func(t *testing.T) {
		rb := resolveBindings(goldenFullDatabase())

		assert.Equal(t, "my-camunda-db-app", rb.AppSecret.Name)
		assert.Equal(t, "my-secret-ns", rb.AppSecret.Namespace)
		assert.Equal(t, "my-camunda-db-backup", rb.BackupSecret.Name)
		assert.Equal(t, "my-cluster-ns", rb.BackupSecret.Namespace)
		assert.Equal(t, "my-database-config", rb.DatabaseConfigName)
	})

	t.Run("disabled backup credentials disable the backup binding", func(t *testing.T) {
		db := goldenDatabase()
		db.Spec.BackupCredentials = &v1.BackupCredentialsSpec{Disabled: true}

		assert.False(t, resolveBindings(db).BackupEnabled)
	})
}

func TestBackupUserName(t *testing.T) {
	assert.Equal(t, "camunda_backup", backupUserName("camunda"))

	long := "d" + strings.Repeat("b", 62)
	truncated := backupUserName(long)
	assert.LessOrEqual(t, len(truncated), 63, "backup role must stay a valid identifier")
	assert.True(t, strings.HasSuffix(truncated, "_backup"))
}

func TestDatabaseBindingsGolden(t *testing.T) {
	tests := []struct {
		name   string
		db     *v1.Database
		golden string
	}{
		{"minimal", goldenDatabase(), "testdata/golden/database/minimal.yaml"},
		{"full", goldenFullDatabase(), "testdata/golden/database/full.yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb := resolveBindings(tt.db)
			rb.AppPassword = "app-password"
			rb.BackupPassword = "backup-password"

			comp, err := databaseBindingsComponent(tt.db, rb)
			require.NoError(t, err)

			golden.AssertComponentYAML(t, tt.golden, comp,
				golden.WithScheme(goldenScheme(t)), golden.Update(*update))
		})
	}
}

func TestDatabaseBindingsGoldenBackupDisabled(t *testing.T) {
	db := goldenDatabase()
	db.Spec.BackupCredentials = &v1.BackupCredentialsSpec{Disabled: true}

	rb := resolveBindings(db)
	rb.AppPassword = "app-password"

	comp, err := databaseBindingsComponent(db, rb)
	require.NoError(t, err)

	golden.AssertComponentYAML(t, "testdata/golden/database/backup-disabled.yaml", comp,
		golden.WithScheme(goldenScheme(t)), golden.Update(*update))
}
