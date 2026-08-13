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
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/secret"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/databaseconfig"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/secondarystorageconfig"
)

const (
	// databaseBindingsComponentName is the single ocf component publishing
	// the Database's bindings; its condition type is BindingsReady.
	databaseBindingsComponentName = "bindings"
	// databaseBindingsConditionType is the component condition the bindings
	// component reports on the Database.
	databaseBindingsConditionType = component.ConditionType("BindingsReady")

	// credentialUsernameKey and credentialPasswordKey are the keys of every
	// credential Secret the Database controller publishes.
	credentialUsernameKey = "username"
	credentialPasswordKey = "password"

	// appSecretSuffix and backupSecretSuffix derive the default credential
	// Secret names from the CR name.
	appSecretSuffix    = "-credentials"
	backupSecretSuffix = "-backup-credentials"

	// backupUserSuffix derives the backup SQL role name from the database
	// name.
	backupUserSuffix = "_backup"

	// maxSQLIdentifierLength is PostgreSQL's identifier length limit.
	maxSQLIdentifierLength = 63
)

// resolvedBindings carries the fully defaulted inputs the bindings component
// renders: object names and namespaces, the SQL role names, and — filled in
// by the reconciler after credential resolution — the role passwords.
type resolvedBindings struct {
	// AppSecret locates the application credentials Secret.
	AppSecret types.NamespacedName
	// BackupSecret locates the backup credentials Secret.
	BackupSecret types.NamespacedName
	// DatabaseConfigName names the DatabaseConfig in targetNamespace.
	DatabaseConfigName string
	// AppUser is the application SQL role, named after the logical database.
	AppUser string
	// BackupUser is the backup SQL role.
	BackupUser string
	// BackupEnabled reports whether the backup user and Secret are managed.
	BackupEnabled bool
	// AppPassword is the application role's resolved password.
	AppPassword string
	// BackupPassword is the backup role's resolved password.
	BackupPassword string
}

// resolveBindings applies the documented defaults to db's binding names: the
// credential Secrets default to "<name>-credentials" and
// "<name>-backup-credentials" in targetNamespace, the DatabaseConfig to the
// CR name, and the SQL roles derive from the database name. Passwords are
// left empty for the reconciler to fill.
func resolveBindings(db *v1.Database) resolvedBindings {
	rb := resolvedBindings{
		AppSecret: types.NamespacedName{
			Namespace: db.Spec.TargetNamespace,
			Name:      db.Name + appSecretSuffix,
		},
		BackupSecret: types.NamespacedName{
			Namespace: db.Spec.TargetNamespace,
			Name:      db.Name + backupSecretSuffix,
		},
		DatabaseConfigName: db.Name,
		AppUser:            db.Spec.DatabaseName,
		BackupUser:         backupUserName(db.Spec.DatabaseName),
		BackupEnabled:      true,
	}

	if app := db.Spec.ApplicationCredentials; app != nil {
		if app.SecretName != "" {
			rb.AppSecret.Name = app.SecretName
		}
		if app.SecretNamespace != "" {
			rb.AppSecret.Namespace = app.SecretNamespace
		}
	}

	if backup := db.Spec.BackupCredentials; backup != nil {
		rb.BackupEnabled = !backup.Disabled
		if backup.SecretName != "" {
			rb.BackupSecret.Name = backup.SecretName
		}
		if backup.SecretNamespace != "" {
			rb.BackupSecret.Namespace = backup.SecretNamespace
		}
	}

	if db.Spec.DatabaseConfig != "" {
		rb.DatabaseConfigName = db.Spec.DatabaseConfig
	}

	return rb
}

// backupUserName derives the backup SQL role from the database name,
// truncating the base so the result stays within PostgreSQL's 63-character
// identifier limit. Database names sharing their first 56 characters on one
// server would collide on the same backup role; the collision rule makes that
// combination unreachable for all but pathological name pairs.
func backupUserName(databaseName string) string {
	base := databaseName
	if max := maxSQLIdentifierLength - len(backupUserSuffix); len(base) > max {
		base = base[:max]
	}

	return base + backupUserSuffix
}

// databaseBindingsComponent assembles the single bindings component: the
// application credentials Secret, the backup credentials Secret (gated on
// backup being enabled), the DatabaseConfig, and — only when
// spec.secondaryStorageConfig is set — the SecondaryStorageConfig wiring the
// database up as rdbms secondary storage. All children land in
// spec.targetNamespace (Secret namespaces overridable per Secret) and receive
// an owner reference to the Database by the framework.
func databaseBindingsComponent(db *v1.Database, rb resolvedBindings) (*component.Component, error) {
	appSecret, err := secret.NewBuilder(
		credentialSecret(rb.AppSecret, rb.AppUser, rb.AppPassword),
	).Build()
	if err != nil {
		return nil, err
	}

	backupSecret, err := secret.NewBuilder(
		credentialSecret(rb.BackupSecret, rb.BackupUser, rb.BackupPassword),
	).Build()
	if err != nil {
		return nil, err
	}

	dbConfig, err := databaseconfig.NewBuilder(&v1.DatabaseConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rb.DatabaseConfigName,
			Namespace: db.Spec.TargetNamespace,
		},
		Spec: v1.DatabaseConfigSpec{
			ServerRef:            db.Spec.ServerRef,
			DatabaseName:         db.Spec.DatabaseName,
			CredentialsSecretRef: credentialsSecretRef(rb.AppSecret),
		},
	}).WithMutation(backupCredentialsRefMutation(rb)).Build()
	if err != nil {
		return nil, err
	}

	builder := component.NewComponentBuilder().
		WithName(databaseBindingsComponentName).
		WithConditionType(databaseBindingsConditionType).
		WithResource(appSecret).
		WithResource(backupSecret, component.GatedBy(feature.NewBooleanGate(rb.BackupEnabled))).
		WithResource(dbConfig)

	// The SecondaryStorageConfig has no name when the field is unset, so it
	// is omitted rather than gated: clearing the field leaves an existing
	// contract in place until the Database is deleted and owner-reference
	// garbage collection removes it.
	if db.Spec.SecondaryStorageConfig != "" {
		ssc, err := secondarystorageconfig.NewBuilder(&v1.SecondaryStorageConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      db.Spec.SecondaryStorageConfig,
				Namespace: db.Spec.TargetNamespace,
			},
			Spec: v1.SecondaryStorageConfigSpec{
				Type:  v1.SecondaryStorageTypeRDBMS,
				RDBMS: &v1.RDBMSStorage{DatabaseConfigRef: rb.DatabaseConfigName},
			},
		}).Build()
		if err != nil {
			return nil, err
		}
		builder = builder.WithResource(ssc)
	}

	return builder.Build()
}

// credentialSecret builds the baseline for a published credential Secret.
func credentialSecret(key types.NamespacedName, username, password string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			credentialUsernameKey: []byte(username),
			credentialPasswordKey: []byte(password),
		},
	}
}

// credentialsSecretRef points a contract at a published credential Secret.
func credentialsSecretRef(key types.NamespacedName) v1.CredentialsSecretRef {
	return v1.CredentialsSecretRef{
		Name:        key.Name,
		Namespace:   key.Namespace,
		UsernameKey: credentialUsernameKey,
		PasswordKey: credentialPasswordKey,
	}
}

// backupCredentialsRefMutation wires the backup credentials Secret into the
// DatabaseConfig while backup is enabled.
func backupCredentialsRefMutation(rb resolvedBindings) databaseconfig.Mutation {
	return databaseconfig.Mutation{
		Name:    "BackupCredentialsRef",
		Feature: feature.NewBooleanGate(rb.BackupEnabled),
		Mutate: func(m *databaseconfig.Mutator) error {
			m.Edit(func(cfg *v1.DatabaseConfig) error {
				ref := credentialsSecretRef(rb.BackupSecret)
				cfg.Spec.BackupCredentialsSecretRef = &ref
				return nil
			})
			return nil
		},
	}
}
