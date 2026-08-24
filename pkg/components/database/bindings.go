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

// Package database renders the resources that a Database CR publishes. It
// resolves the documented naming defaults, assembles the ocf bindings
// component, and holds the collision rule. Everything here is pure: spec in,
// resources out, no API calls. The controller in internal/controller/database
// drives it.
package database

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/secret"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/credentials"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/databaseconfig"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/secondarystorageconfig"
)

const (
	// bindingsComponentLabel is the labels.ComponentKey value on every binding
	// that the Database publishes.
	bindingsComponentLabel = "database"

	// ConditionBindings is the condition type of the bindings component, the
	// single ocf component that publishes the bindings of the Database.
	ConditionBindings = component.ConditionType("BindingsReady")

	// CredentialUsernameKey and CredentialPasswordKey are the keys of every
	// credential Secret that the Database controller publishes.
	CredentialUsernameKey = "username"
	CredentialPasswordKey = "password"

	// appSecretSuffix and backupSecretSuffix derive the default credential
	// Secret names from the CR name.
	appSecretSuffix    = "-credentials"
	backupSecretSuffix = "-backup-credentials"

	// backupUserSuffix derives the backup SQL role name from the database
	// name.
	backupUserSuffix = "_backup"

	// maxSQLIdentifierLength is the identifier length limit of PostgreSQL.
	maxSQLIdentifierLength = 63

	// backupUserHashLength is the number of hex characters of the SHA-256 of
	// the database name that disambiguate a backup role. It applies when the
	// plain suffixed form is longer than maxSQLIdentifierLength.
	backupUserHashLength = 8
)

// Bindings carries the fully defaulted inputs that the bindings
// component renders: object names and namespaces, the SQL role names, and the
// role passwords. The reconciler fills the passwords after it resolves the
// credentials.
type Bindings struct {
	// AppSecret locates the application credentials Secret.
	AppSecret types.NamespacedName
	// BackupSecret locates the backup credentials Secret.
	BackupSecret types.NamespacedName
	// DatabaseConfigName names the DatabaseConfig in the namespace of the Database.
	DatabaseConfigName string
	// AppUser is the application SQL role. It has the name of the logical
	// database.
	AppUser string
	// BackupUser is the backup SQL role.
	BackupUser string
	// BackupEnabled reports whether the backup user and Secret are managed.
	BackupEnabled bool
	// AppPassword is the resolved password of the application role.
	AppPassword credentials.Password
	// BackupPassword is the resolved password of the backup role.
	BackupPassword credentials.Password
}

// ResolveBindings applies the documented defaults to the binding names of db.
// The credential Secrets default to "<name>-credentials" and
// "<name>-backup-credentials". The DatabaseConfig defaults to the CR name.
// The SQL roles derive from the database name. Every binding lands in the
// namespace of db. Passwords stay empty for the reconciler to fill.
func ResolveBindings(db *v1.Database) Bindings {
	rb := Bindings{
		AppSecret: types.NamespacedName{
			Namespace: db.Namespace,
			Name:      db.Name + appSecretSuffix,
		},
		BackupSecret: types.NamespacedName{
			Namespace: db.Namespace,
			Name:      db.Name + backupSecretSuffix,
		},
		DatabaseConfigName: db.Name,
		AppUser:            db.Spec.DatabaseName,
		BackupUser:         BackupUserName(db.Spec.DatabaseName),
		BackupEnabled:      true,
	}

	if app := db.Spec.ApplicationCredentials; app != nil && app.SecretName != "" {
		rb.AppSecret.Name = app.SecretName
	}

	if backup := db.Spec.BackupCredentials; backup != nil {
		rb.BackupEnabled = !backup.Disabled
		if backup.SecretName != "" {
			rb.BackupSecret.Name = backup.SecretName
		}
	}

	if db.Spec.DatabaseConfig != "" {
		rb.DatabaseConfigName = db.Spec.DatabaseConfig
	}

	return rb
}

// BackupUserName derives the backup SQL role from the database name. A short
// name keeps the plain "<databaseName>_backup" form. If the suffixed form is
// longer than the 63-character identifier limit of PostgreSQL, the role
// becomes "<first 47 characters>_<first 8 hex characters of the SHA-256 of
// the name>_backup". This form is deterministic across reconciles. Two
// database names that share a long prefix can never share a backup role.
// With plain truncation they can, and the backup grants of one database then
// extend to the role of another.
func BackupUserName(databaseName string) string {
	if len(databaseName)+len(backupUserSuffix) <= maxSQLIdentifierLength {
		return databaseName + backupUserSuffix
	}

	digest := sha256.Sum256([]byte(databaseName))
	hash := hex.EncodeToString(digest[:])[:backupUserHashLength]
	prefix := databaseName[:maxSQLIdentifierLength-len(backupUserSuffix)-backupUserHashLength-1]

	return prefix + "_" + hash + backupUserSuffix
}

// BindingsComponent assembles the single bindings component. It holds
// the application credentials Secret, the backup credentials Secret (gated on
// backup enabled), the DatabaseConfig, and the SecondaryStorageConfig. The
// SecondaryStorageConfig is present only when spec.secondaryStorageConfig is
// set. It registers the database as rdbms secondary storage. All children
// land in the namespace of the Database, which gives each of them an owner
// reference to it.
func BindingsComponent(db *v1.Database, rb Bindings) (*component.Component, error) {
	appSecret, err := secret.NewBuilder(
		credentialSecret(db, rb.AppSecret, rb.AppUser, rb.AppPassword),
	).Build()
	if err != nil {
		return nil, err
	}

	backupSecret, err := secret.NewBuilder(
		credentialSecret(db, rb.BackupSecret, rb.BackupUser, rb.BackupPassword),
	).Build()
	if err != nil {
		return nil, err
	}

	dbConfig, err := databaseconfig.NewBuilder(&v1.DatabaseConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rb.DatabaseConfigName,
			Namespace: db.Namespace,
			Labels:    bindingLabels(db),
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
		WithName("bindings").
		WithConditionType(ConditionBindings).
		WithResource(appSecret).
		WithResource(backupSecret, component.GatedBy(feature.NewBooleanGate(rb.BackupEnabled))).
		WithResource(dbConfig)

	// The SecondaryStorageConfig has no name when the field is unset, so it is
	// omitted rather than gated. If the field is cleared, the existing contract
	// stays in place until the Database is deleted and owner-reference garbage
	// collection removes it.
	if db.Spec.SecondaryStorageConfig != "" {
		ssc, err := secondarystorageconfig.NewBuilder(&v1.SecondaryStorageConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      db.Spec.SecondaryStorageConfig,
				Namespace: db.Namespace,
				Labels:    bindingLabels(db),
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

// bindingLabels returns the labels of every binding that db publishes.
func bindingLabels(db *v1.Database) map[string]string {
	return labels.Managed(labels.Database(db.Name), bindingsComponentLabel)
}

// credentialSecret builds the baseline for a published credential Secret. A
// reused password carries its apply precondition onto the Secret, so a delete
// of the Secret always rotates the password.
func credentialSecret(
	db *v1.Database,
	key types.NamespacedName,
	username string,
	password credentials.Password,
) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        key.Name,
			Namespace:   key.Namespace,
			Labels:      bindingLabels(db),
			Annotations: password.PreconditionAnnotations(),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			CredentialUsernameKey: []byte(username),
			CredentialPasswordKey: []byte(password.Value),
		},
	}
}

// credentialsSecretRef points a contract at a published credential Secret.
func credentialsSecretRef(key types.NamespacedName) v1.CredentialsSecretRef {
	return v1.CredentialsSecretRef{
		Name:        key.Name,
		Namespace:   key.Namespace,
		UsernameKey: CredentialUsernameKey,
		PasswordKey: CredentialPasswordKey,
	}
}

// backupCredentialsRefMutation sets the backup credentials Secret reference on
// the DatabaseConfig while backup is enabled.
func backupCredentialsRefMutation(rb Bindings) databaseconfig.Mutation {
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
