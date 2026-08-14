//go:build !no_docker

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
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// newDatabaseNamespace creates a uniquely named Namespace for one spec and
// registers its deletion.
func newDatabaseNamespace() string {
	GinkgoHelper()
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "db-ns-" + utilrand.String(8)},
	}
	Expect(k8sClient.Create(ctx, ns)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, ns) })
	return ns.Name
}

// createDatabaseServer creates a DatabaseServerConfig pointing at the shared
// PostgreSQL container, with its admin credentials Secret in namespace, and
// registers deletion of both. It returns the server's name.
func createDatabaseServer(namespace string) string {
	GinkgoHelper()

	pg, err := testPostgres()
	Expect(err).NotTo(HaveOccurred())

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "admin-creds", Namespace: namespace},
		Data: map[string][]byte{
			"username": []byte(pg.AdminUser),
			"password": []byte(pg.AdminPassword),
		},
	}
	Expect(k8sClient.Create(ctx, secret)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })

	server := validDatabaseServerConfig()
	server.Spec.Host = pg.Host
	server.Spec.Port = pg.Port
	server.Spec.AdminCredentialsSecretRef = v1.CredentialsSecretRef{
		Name: secret.Name, Namespace: namespace,
		UsernameKey: "username", PasswordKey: "password",
	}
	Expect(k8sClient.Create(ctx, server)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, server) })

	return server.Name
}

// databaseFor returns a Database bound to server, publishing into namespace,
// claiming a unique logical database name.
func databaseFor(server, namespace string) *v1.Database {
	db := validDatabase()
	db.Spec.ServerRef = server
	db.Spec.TargetNamespace = namespace
	db.Spec.DatabaseName = "db_" + utilrand.String(8)
	return db
}

// createDatabase creates db and registers its deletion.
func createDatabase(db *v1.Database) {
	GinkgoHelper()
	Expect(k8sClient.Create(ctx, db)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, db) })
}

// expectDatabaseReady polls until db's Ready condition matches the given
// status and reason, with a message containing messagePart, and returns the
// matched condition.
func expectDatabaseReady(db *v1.Database, status metav1.ConditionStatus, reason, messagePart string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.Database
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(db), &latest)).To(Succeed())
		ready := meta.FindStatusCondition(latest.Status.Conditions, conditions.TypeReady)
		g.Expect(ready).NotTo(BeNil())
		g.Expect(ready.Status).To(Equal(status))
		g.Expect(ready.Reason).To(Equal(reason))
		g.Expect(ready.Message).To(ContainSubstring(messagePart))
	}, timeout, interval).Should(Succeed())
}

// expectOwnedByDatabase asserts obj carries the controller owner reference to
// db, the link garbage collection uses when the Database is deleted.
func expectOwnedByDatabase(obj client.Object, db *v1.Database) {
	GinkgoHelper()

	var latest v1.Database
	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(db), &latest)).To(Succeed())

	refs := obj.GetOwnerReferences()
	Expect(refs).To(HaveLen(1))
	Expect(refs[0].APIVersion).To(Equal("core.camunda.io/v1"))
	Expect(refs[0].Kind).To(Equal("Database"))
	Expect(refs[0].Name).To(Equal(db.Name))
	Expect(refs[0].UID).To(Equal(latest.UID))
	Expect(refs[0].Controller).To(HaveValue(BeTrue()))
}

// sqlDatabaseExists reports whether the logical database exists on the shared
// server.
func sqlDatabaseExists(name string) bool {
	GinkgoHelper()

	pg, err := testPostgres()
	Expect(err).NotTo(HaveOccurred())
	conn, err := pgConnect(ctx, pg.AdminUser, pg.AdminPassword, "postgres")
	Expect(err).NotTo(HaveOccurred())
	defer func() { _ = conn.Close(context.Background()) }()

	var exists bool
	Expect(conn.QueryRow(ctx,
		"SELECT EXISTS (SELECT FROM pg_database WHERE datname = $1)", name,
	).Scan(&exists)).To(Succeed())
	return exists
}

// sqlRoleExists reports whether the SQL role exists on the shared server.
func sqlRoleExists(name string) bool {
	GinkgoHelper()

	pg, err := testPostgres()
	Expect(err).NotTo(HaveOccurred())
	conn, err := pgConnect(ctx, pg.AdminUser, pg.AdminPassword, "postgres")
	Expect(err).NotTo(HaveOccurred())
	defer func() { _ = conn.Close(context.Background()) }()

	var exists bool
	Expect(conn.QueryRow(ctx,
		"SELECT EXISTS (SELECT FROM pg_roles WHERE rolname = $1)", name,
	).Scan(&exists)).To(Succeed())
	return exists
}

// publishedPassword reads the password published in the credentials Secret at
// key.
func publishedPassword(key types.NamespacedName) string {
	GinkgoHelper()
	var secret corev1.Secret
	Expect(k8sClient.Get(ctx, key, &secret)).To(Succeed())
	Expect(secret.Data).To(HaveKey("password"))
	return string(secret.Data["password"])
}

var _ = Describe("Database controller", func() {
	It("bootstraps the database and publishes the bindings", func() {
		namespace := newDatabaseNamespace()
		server := createDatabaseServer(namespace)
		db := databaseFor(server, namespace)
		db.Spec.SecondaryStorageConfig = "storage-config"
		createDatabase(db)

		expectDatabaseReady(db, metav1.ConditionTrue, conditions.ReasonHealthy, "All components ready")

		By("creating the SQL objects")
		Expect(sqlDatabaseExists(db.Spec.DatabaseName)).To(BeTrue())
		Expect(sqlRoleExists(db.Spec.DatabaseName)).To(BeTrue())
		Expect(sqlRoleExists(db.Spec.DatabaseName + "_backup")).To(BeTrue())

		By("publishing owner-referenced credential Secrets in targetNamespace")
		var appSecret corev1.Secret
		appKey := types.NamespacedName{Namespace: namespace, Name: db.Name + "-credentials"}
		Expect(k8sClient.Get(ctx, appKey, &appSecret)).To(Succeed())
		Expect(appSecret.Data).To(HaveKeyWithValue("username", []byte(db.Spec.DatabaseName)))
		Expect(appSecret.Data).To(HaveKey("password"))
		expectOwnedByDatabase(&appSecret, db)

		var backupSecret corev1.Secret
		backupKey := types.NamespacedName{Namespace: namespace, Name: db.Name + "-backup-credentials"}
		Expect(k8sClient.Get(ctx, backupKey, &backupSecret)).To(Succeed())
		Expect(backupSecret.Data).To(HaveKeyWithValue("username", []byte(db.Spec.DatabaseName+"_backup")))
		Expect(backupSecret.Data).To(HaveKey("password"))
		expectOwnedByDatabase(&backupSecret, db)

		By("publishing an owner-referenced DatabaseConfig wired to the Secrets")
		var dbConfig v1.DatabaseConfig
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: db.Name}, &dbConfig)).To(Succeed())
		Expect(dbConfig.Spec.ServerRef).To(Equal(server))
		Expect(dbConfig.Spec.DatabaseName).To(Equal(db.Spec.DatabaseName))
		Expect(dbConfig.Spec.CredentialsSecretRef.Name).To(Equal(appKey.Name))
		Expect(dbConfig.Spec.CredentialsSecretRef.Namespace).To(Equal(namespace))
		Expect(dbConfig.Spec.BackupCredentialsSecretRef).NotTo(BeNil())
		Expect(dbConfig.Spec.BackupCredentialsSecretRef.Name).To(Equal(backupKey.Name))
		expectOwnedByDatabase(&dbConfig, db)

		By("publishing an owner-referenced rdbms SecondaryStorageConfig")
		var storage v1.SecondaryStorageConfig
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "storage-config"}, &storage)).To(Succeed())
		Expect(storage.Spec.Type).To(Equal(v1.SecondaryStorageTypeRDBMS))
		Expect(storage.Spec.RDBMS).NotTo(BeNil())
		Expect(storage.Spec.RDBMS.DatabaseConfigRef).To(Equal(db.Name))
		expectOwnedByDatabase(&storage, db)

		By("publishing credentials that authenticate against the server")
		appConn, err := pgConnect(ctx, db.Spec.DatabaseName, publishedPassword(appKey), db.Spec.DatabaseName)
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = appConn.Close(context.Background()) }()
		Expect(appConn.Ping(ctx)).To(Succeed())

		By("tracking the reconciled generation")
		var latest v1.Database
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(db), &latest)).To(Succeed())
		Expect(latest.Status.ObservedGeneration).To(Equal(latest.Generation))
	})

	It("regenerates a working password when the credentials Secret is deleted", func() {
		namespace := newDatabaseNamespace()
		server := createDatabaseServer(namespace)
		db := databaseFor(server, namespace)
		createDatabase(db)

		expectDatabaseReady(db, metav1.ConditionTrue, conditions.ReasonHealthy, "All components ready")

		appKey := types.NamespacedName{Namespace: namespace, Name: db.Name + "-credentials"}
		var original corev1.Secret
		Expect(k8sClient.Get(ctx, appKey, &original)).To(Succeed())
		Expect(k8sClient.Delete(ctx, &original)).To(Succeed())

		var rotated string
		Eventually(func(g Gomega) {
			var secret corev1.Secret
			g.Expect(k8sClient.Get(ctx, appKey, &secret)).To(Succeed())
			g.Expect(secret.UID).NotTo(Equal(original.UID))
			g.Expect(secret.Data).To(HaveKey("password"))
			rotated = string(secret.Data["password"])
		}, timeout, interval).Should(Succeed())

		conn, err := pgConnect(ctx, db.Spec.DatabaseName, rotated, db.Spec.DatabaseName)
		Expect(err).NotTo(HaveOccurred(), "the regenerated password must already be set on the server")
		defer func() { _ = conn.Close(context.Background()) }()
		Expect(conn.Ping(ctx)).To(Succeed())
	})

	It("omits the backup user, Secret, and reference when backups are disabled", func() {
		namespace := newDatabaseNamespace()
		server := createDatabaseServer(namespace)
		db := databaseFor(server, namespace)
		db.Spec.BackupCredentials = &v1.BackupCredentialsSpec{Disabled: true}
		createDatabase(db)

		expectDatabaseReady(db, metav1.ConditionTrue, conditions.ReasonHealthy, "All components ready")

		var secret corev1.Secret
		backupKey := types.NamespacedName{Namespace: namespace, Name: db.Name + "-backup-credentials"}
		Expect(k8sClient.Get(ctx, backupKey, &secret)).To(MatchError(ContainSubstring("not found")))

		var dbConfig v1.DatabaseConfig
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: db.Name}, &dbConfig)).To(Succeed())
		Expect(dbConfig.Spec.BackupCredentialsSecretRef).To(BeNil())

		Expect(sqlRoleExists(db.Spec.DatabaseName + "_backup")).To(BeFalse())
	})

	It("reports InvalidReference for a dangling serverRef", func() {
		namespace := newDatabaseNamespace()
		db := databaseFor("no-such-server", namespace)
		createDatabase(db)

		expectDatabaseReady(db, metav1.ConditionFalse, conditions.ReasonInvalidReference,
			`DatabaseServerConfig "no-such-server" not found`)
	})

	It("reports MissingSecret when the admin credentials Secret lacks a key", func() {
		namespace := newDatabaseNamespace()

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "half-admin-creds", Namespace: namespace},
			Data:       map[string][]byte{"username": []byte("postgres")},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })

		server := validDatabaseServerConfig()
		server.Spec.AdminCredentialsSecretRef = v1.CredentialsSecretRef{
			Name: secret.Name, Namespace: namespace,
			UsernameKey: "username", PasswordKey: "password",
		}
		Expect(k8sClient.Create(ctx, server)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, server) })

		db := databaseFor(server.Name, namespace)
		createDatabase(db)

		expectDatabaseReady(db, metav1.ConditionFalse, conditions.ReasonMissingSecret,
			fmt.Sprintf(`Secret "%s/%s" is missing key "password"`, namespace, secret.Name))
	})

	It("reports ConnectionFailed for an unreachable server", func() {
		namespace := newDatabaseNamespace()

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "unreachable-admin-creds", Namespace: namespace},
			Data:       map[string][]byte{"username": []byte("postgres"), "password": []byte("pw")},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })

		server := validDatabaseServerConfig()
		server.Spec.Host = "127.0.0.1"
		server.Spec.Port = 1
		server.Spec.AdminCredentialsSecretRef = v1.CredentialsSecretRef{
			Name: secret.Name, Namespace: namespace,
			UsernameKey: "username", PasswordKey: "password",
		}
		Expect(k8sClient.Create(ctx, server)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, server) })

		db := databaseFor(server.Name, namespace)
		createDatabase(db)

		expectDatabaseReady(db, metav1.ConditionFalse, conditions.ReasonConnectionFailed,
			fmt.Sprintf("Connecting to DatabaseServerConfig %q", server.Name))
	})

	It("derives distinct backup roles for long database names sharing a prefix", func() {
		namespace := newDatabaseNamespace()
		server := createDatabaseServer(namespace)

		// Two 60-character names identical through the truncation point, so
		// plain truncation would collapse both onto one backup role.
		prefix := "db_" + utilrand.String(5) + strings.Repeat("x", 48)
		first := databaseFor(server, namespace)
		first.Spec.DatabaseName = prefix + "aaaa"
		second := databaseFor(server, namespace)
		second.Spec.DatabaseName = prefix + "bbbb"
		createDatabase(first)
		createDatabase(second)

		expectDatabaseReady(first, metav1.ConditionTrue, conditions.ReasonHealthy, "All components ready")
		expectDatabaseReady(second, metav1.ConditionTrue, conditions.ReasonHealthy, "All components ready")

		By("creating one backup role per database")
		firstRole := backupUserName(first.Spec.DatabaseName)
		secondRole := backupUserName(second.Spec.DatabaseName)
		Expect(firstRole).NotTo(Equal(secondRole))
		Expect(sqlRoleExists(firstRole)).To(BeTrue())
		Expect(sqlRoleExists(secondRole)).To(BeTrue())

		By("granting each backup role access to its own database only")
		password := publishedPassword(types.NamespacedName{
			Namespace: namespace, Name: first.Name + "-backup-credentials",
		})
		own, err := pgConnect(ctx, firstRole, password, first.Spec.DatabaseName)
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = own.Close(context.Background()) }()
		Expect(own.Ping(ctx)).To(Succeed())

		_, err = pgConnect(ctx, firstRole, password, second.Spec.DatabaseName)
		Expect(err).To(HaveOccurred(),
			"one database's backup role must not reach the sibling database")
	})

	It("rejects a later Database claiming an already claimed logical database", func() {
		namespace := newDatabaseNamespace()
		server := createDatabaseServer(namespace)

		winner := databaseFor(server, namespace)
		winner.Name = "colla-" + utilrand.String(8)
		createDatabase(winner)
		expectDatabaseReady(winner, metav1.ConditionTrue, conditions.ReasonHealthy, "All components ready")

		loser := databaseFor(server, namespace)
		loser.Name = "collb-" + utilrand.String(8)
		loser.Spec.DatabaseName = winner.Spec.DatabaseName
		createDatabase(loser)

		expectDatabaseReady(loser, metav1.ConditionFalse, conditions.ReasonInvalidReference,
			fmt.Sprintf("Database %q already claims database %q", winner.Name, winner.Spec.DatabaseName))
	})

	It("deletes without a finalizer, leaving the SQL database and users intact", func() {
		namespace := newDatabaseNamespace()
		server := createDatabaseServer(namespace)
		db := databaseFor(server, namespace)
		createDatabase(db)

		expectDatabaseReady(db, metav1.ConditionTrue, conditions.ReasonHealthy, "All components ready")

		Expect(k8sClient.Delete(ctx, db)).To(Succeed())
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(db), &v1.Database{})
		}, timeout, interval).Should(MatchError(ContainSubstring("not found")),
			"deletion must complete without a finalizer holding the CR")

		Expect(sqlDatabaseExists(db.Spec.DatabaseName)).To(BeTrue(),
			"deleting the CR must never drop the logical database")
		Expect(sqlRoleExists(db.Spec.DatabaseName)).To(BeTrue())
		Expect(sqlRoleExists(db.Spec.DatabaseName + "_backup")).To(BeTrue())
	})
})
