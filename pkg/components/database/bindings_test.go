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
	"reflect"
	"strings"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/testing/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/credentials"
)

var update = flag.Bool("update", false, "update golden files")

// goldenStorageConfigName is the SecondaryStorageConfig name in the full
// golden fixture.
const goldenStorageConfigName = "my-storage-config"

// goldenScheme registers exactly the kinds that the bindings component renders.
func goldenScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	s := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(s))
	require.NoError(t, v1.AddToScheme(s))

	return s
}

// goldenDatabase returns the minimal example of the CRD doc with a fixed name,
// so the rendered bindings are stable golden input.
func goldenDatabase() *v1.Database {
	return &v1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "my-camunda-db", Namespace: "my-cluster-ns"},
		Spec: v1.DatabaseSpec{
			ServerRef:    "my-db-server",
			DatabaseName: "camunda",
		},
	}
}

// goldenFullDatabase returns the realistic example of the CRD doc with a fixed
// name.
func goldenFullDatabase() *v1.Database {
	db := goldenDatabase()
	db.Spec.ApplicationCredentials = &v1.CredentialsSpec{SecretName: "my-camunda-db-app"}
	db.Spec.BackupCredentials = &v1.BackupCredentialsSpec{
		CredentialsSpec: v1.CredentialsSpec{SecretName: "my-camunda-db-backup"},
	}
	db.Spec.DatabaseConfig = "my-database-config"
	db.Spec.SecondaryStorageConfig = goldenStorageConfigName
	return db
}

func TestResolveBindings(t *testing.T) {
	t.Run("defaults derive from the CR name and its namespace", func(t *testing.T) {
		rb := ResolveBindings(goldenDatabase())

		assert.Equal(t, "my-camunda-db-credentials", rb.AppSecret.Name)
		assert.Equal(t, "my-cluster-ns", rb.AppSecret.Namespace)
		assert.Equal(t, "my-camunda-db-backup-credentials", rb.BackupSecret.Name)
		assert.Equal(t, "my-cluster-ns", rb.BackupSecret.Namespace)
		assert.Equal(t, "my-camunda-db", rb.DatabaseConfigName)
		assert.Equal(t, "camunda", rb.AppUser)
		assert.Equal(t, "camunda_backup", rb.BackupUser)
		assert.True(t, rb.BackupEnabled)
	})

	t.Run("explicit names win over defaults, and every Secret stays local", func(t *testing.T) {
		rb := ResolveBindings(goldenFullDatabase())

		assert.Equal(t, "my-camunda-db-app", rb.AppSecret.Name)
		assert.Equal(t, "my-cluster-ns", rb.AppSecret.Namespace)
		assert.Equal(t, "my-camunda-db-backup", rb.BackupSecret.Name)
		assert.Equal(t, "my-cluster-ns", rb.BackupSecret.Namespace)
		assert.Equal(t, "my-database-config", rb.DatabaseConfigName)
	})

	t.Run("disabled backup credentials disable the backup binding", func(t *testing.T) {
		db := goldenDatabase()
		db.Spec.BackupCredentials = &v1.BackupCredentialsSpec{Disabled: true}

		assert.False(t, ResolveBindings(db).BackupEnabled)
	})
}

// previewNames renders the component and returns "<kind>/<name>" for every
// resource it registered.
func previewNames(t *testing.T, comp *component.Component) []string {
	t.Helper()

	objects, err := comp.Preview()
	require.NoError(t, err)

	// Preview returns typed objects, whose TypeMeta the API server would have
	// filled in, so the kind comes from the Go type.
	names := make([]string, 0, len(objects))
	for _, obj := range objects {
		names = append(names, reflect.TypeOf(obj).Elem().Name()+"/"+obj.GetName())
	}

	return names
}

// TestWithdrawnBindingsComponentRegistersWhatWasPublished pins which objects
// the withdrawal touches. It registers the names it is given and no other. One
// edit can rename a binding, so the object on the cluster carries the name from
// before the edit, and the names of the spec reach nothing. A binding this
// Database does not own is not in the set at all, because the component would
// otherwise delete an object that belongs to the Database that won the claim.
func TestWithdrawnBindingsComponentRegistersWhatWasPublished(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		published PublishedBindings
		want      []string
	}{
		{
			name: "the names from before a rename, not the names of the spec",
			published: PublishedBindings{
				Secrets:                 []string{"old-app", "old-backup"},
				DatabaseConfigs:         []string{"old-config"},
				SecondaryStorageConfigs: []string{"old-storage"},
			},
			want: []string{
				"Secret/old-app",
				"Secret/old-backup",
				"DatabaseConfig/old-config",
				"SecondaryStorageConfig/old-storage",
			},
		},
		{
			name:      "nothing published",
			published: PublishedBindings{},
			want:      []string{},
		},
		{
			name:      "only the application Secret",
			published: PublishedBindings{Secrets: []string{"my-camunda-db-app"}},
			want:      []string{"Secret/my-camunda-db-app"},
		},
		{
			name: "the contract of another Database is left out",
			published: PublishedBindings{
				Secrets:                 []string{"my-camunda-db-app", "my-camunda-db-backup"},
				SecondaryStorageConfigs: []string{goldenStorageConfigName},
			},
			want: []string{
				"Secret/my-camunda-db-app",
				"Secret/my-camunda-db-backup",
				"SecondaryStorageConfig/" + goldenStorageConfigName,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			comp, err := WithdrawnBindingsComponent(goldenFullDatabase(), tt.published)
			require.NoError(t, err)

			assert.ElementsMatch(t, tt.want, previewNames(t, comp))
		})
	}
}

// TestWithdrawnBindingsComponentKeepsTheNamespaceOfTheDatabase pins that the
// withdrawal deletes in the namespace of the Database. Every binding lands
// there, and a name alone reaches an object of any namespace.
func TestWithdrawnBindingsComponentKeepsTheNamespaceOfTheDatabase(t *testing.T) {
	t.Parallel()

	comp, err := WithdrawnBindingsComponent(
		goldenFullDatabase(), PublishedBindings{Secrets: []string{"old-app"}},
	)
	require.NoError(t, err)

	objects, err := comp.Preview()
	require.NoError(t, err)
	require.Len(t, objects, 1)
	assert.Equal(t, "my-cluster-ns", objects[0].GetNamespace())
}

func TestBackupUserName(t *testing.T) {
	t.Run("short names keep the plain suffix form", func(t *testing.T) {
		assert.Equal(t, "camunda_backup", BackupUserName("camunda"))
	})

	t.Run("long names sharing a prefix derive distinct roles", func(t *testing.T) {
		prefix := strings.Repeat("a", 56)
		one := BackupUserName(prefix + "1xyz")
		two := BackupUserName(prefix + "2xyz")

		assert.NotEqual(
			t, one, two,
			"database names differing past the truncation point must not share a backup role",
		)
		assert.LessOrEqual(t, len(one), 63, "backup role must stay a valid identifier")
		assert.LessOrEqual(t, len(two), 63)
		assert.True(t, strings.HasSuffix(one, "_backup"))
		assert.True(t, strings.HasSuffix(two, "_backup"))
		assert.Equal(
			t, one, BackupUserName(prefix+"1xyz"),
			"the disambiguated role must be deterministic across reconciles",
		)
		assert.Regexp(t, "^[a-z_][a-z0-9_]{0,62}$", one)
	})
}

func TestDatabaseBindingsGolden(t *testing.T) {
	tests := []struct {
		name   string
		db     *v1.Database
		golden string
	}{
		{"minimal", goldenDatabase(), "testdata/golden/minimal/bindings.yaml"},
		{"full", goldenFullDatabase(), "testdata/golden/full/bindings.yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb := ResolveBindings(tt.db)
			rb.AppPassword = credentials.Password{Value: "app-password"}
			rb.BackupPassword = credentials.Password{Value: "backup-password"}

			comp, err := BindingsComponent(tt.db, rb)
			require.NoError(t, err)

			golden.AssertComponentYAML(
				t, tt.golden, comp,
				golden.WithScheme(goldenScheme(t)), golden.Update(*update),
			)
		})
	}
}

func TestDatabaseBindingsGoldenBackupDisabled(t *testing.T) {
	db := goldenDatabase()
	db.Spec.BackupCredentials = &v1.BackupCredentialsSpec{Disabled: true}

	rb := ResolveBindings(db)
	rb.AppPassword = credentials.Password{Value: "app-password"}

	comp, err := BindingsComponent(db, rb)
	require.NoError(t, err)

	golden.AssertComponentYAML(
		t, "testdata/golden/backup-disabled/bindings.yaml", comp,
		golden.WithScheme(goldenScheme(t)), golden.Update(*update),
	)
}

// A password that came from an existing Secret must bind its apply to that
// Secret, or a delete between the read and the apply recreates the Secret with
// the old password and the delete rotates nothing. Each credential Secret
// carries the precondition of its own password.
func TestCredentialSecretsCarryTheApplyPrecondition(t *testing.T) {
	t.Parallel()

	db := goldenDatabase()
	rb := ResolveBindings(db)
	rb.AppPassword = credentials.Password{Value: "app-password", SourceUID: "uid-app"}
	rb.BackupPassword = credentials.Password{Value: "backup-password"}

	comp, err := BindingsComponent(db, rb)
	require.NoError(t, err)

	objects, err := comp.Preview()
	require.NoError(t, err)

	annotations := map[string]map[string]string{}
	for _, obj := range objects {
		if _, ok := obj.(*corev1.Secret); ok {
			annotations[obj.GetName()] = obj.GetAnnotations()
		}
	}
	assert.Equal(
		t,
		map[string]string{credentials.PreconditionAnnotation: "uid-app"},
		annotations[rb.AppSecret.Name],
	)
	// A new password has no source object, so the apply must be free to
	// create the Secret.
	assert.Empty(t, annotations[rb.BackupSecret.Name])
}
