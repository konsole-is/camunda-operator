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

package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
	components "github.com/konsole-is/camunda-operator/pkg/components/database"
)

// newDatabaseNamespace creates a uniquely named Namespace for one spec and
// registers its deletion.
func newDatabaseNamespace() string {
	GinkgoHelper()
	return newDatabaseNamespacePrefixed("db-ns")
}

// newDatabaseNamespacePrefixed is newDatabaseNamespace with a caller-chosen
// prefix. A spec whose two Databases are created within one second needs its
// two namespaces to sort in a known order, because Kubernetes records a
// creationTimestamp to the second and the collision rule then falls back to
// "<namespace>/<name>".
func newDatabaseNamespacePrefixed(prefix string) string {
	GinkgoHelper()
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: prefix + "-" + utilrand.String(8)},
	}
	Expect(k8sClient.Create(ctx, ns)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, ns) })
	return ns.Name
}

// createDatabaseServer creates a DatabaseServerConfig in namespace that
// points at the shared PostgreSQL container under its own host, with its
// admin credentials Secret beside it. It registers deletion of both and
// returns the name of the contract.
func createDatabaseServer(namespace string) string {
	GinkgoHelper()

	pg, err := testPostgres()
	Expect(err).NotTo(HaveOccurred())

	return createDatabaseServerAt(namespace, pg.Host)
}

// createDatabaseServerAt is createDatabaseServer with an explicit host.
func createDatabaseServerAt(namespace, host string) string {
	GinkgoHelper()

	return probedServer(namespace, createAdminSecret(namespace), host).Name
}

// testPostgresHost is the host of the shared PostgreSQL container.
func testPostgresHost() string {
	GinkgoHelper()

	pg, err := testPostgres()
	Expect(err).NotTo(HaveOccurred())

	return pg.Host
}

// createAdminSecret writes the admin credentials of the shared container into
// namespace and returns the name of the Secret.
func createAdminSecret(namespace string) string {
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

	return secret.Name
}

// probedServer is unprobedServer with the record of a probe published,
// because the DatabaseServerConfig controller does not run in this suite and
// the identity of the server is what the collision rule keys on.
func probedServer(namespace, secretName, host string) *v1.DatabaseServerConfig {
	GinkgoHelper()

	server := unprobedServer(namespace, secretName, host)
	publishProbe(server)

	return server
}

// publishProbe records a probe of the endpoint and the admin credentials that
// the spec of server names now, with the identity of the shared container. It
// refreshes server from the API, so a caller can go on updating it.
func publishProbe(server *v1.DatabaseServerConfig) {
	GinkgoHelper()

	identifier := serverSystemIdentifier()

	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), server)).To(Succeed())
		ref := server.Spec.AdminCredentialsSecretRef
		server.Status.SystemIdentifier = identifier
		server.Status.ProbedAt = &metav1.Time{Time: time.Now()}
		server.Status.ProbedEndpoint = fmt.Sprintf("%s:%d", server.Spec.Host, server.Spec.Port)
		server.Status.ProbedSecretName = ref.Name
		server.Status.ProbedSecretKeys = ref.UsernameKey + "/" + ref.PasswordKey
		g.Expect(k8sClient.Status().Update(ctx, server)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// unprobedServer creates a DatabaseServerConfig in namespace that reaches the
// shared PostgreSQL container at host, with its admin credentials Secret
// secretName beside it. Its status stays empty, so it is a contract that has
// not reached its server yet.
func unprobedServer(namespace, secretName, host string) *v1.DatabaseServerConfig {
	GinkgoHelper()

	pg, err := testPostgres()
	Expect(err).NotTo(HaveOccurred())

	server := fixtures.DatabaseServerConfig(namespace)
	server.Spec.Host = host
	server.Spec.Port = pg.Port
	server.Spec.AdminCredentialsSecretRef = v1.LocalCredentialsSecretRef{
		Name: secretName, UsernameKey: "username", PasswordKey: "password",
	}
	Expect(k8sClient.Create(ctx, server)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, server) })

	return server
}

// serverSystemIdentifier reads the system identifier of the shared container.
func serverSystemIdentifier() string {
	GinkgoHelper()

	pg, err := testPostgres()
	Expect(err).NotTo(HaveOccurred())
	conn, err := pgConnect(ctx, pg.AdminUser, pg.AdminPassword, "postgres")
	Expect(err).NotTo(HaveOccurred())
	defer func() { _ = conn.Close(context.Background()) }()

	var id string
	Expect(
		conn.QueryRow(ctx, "SELECT system_identifier::text FROM pg_control_system()").Scan(&id),
	).To(Succeed())
	Expect(id).NotTo(BeEmpty())

	return id
}

// databaseFor returns a Database of namespace that is bound to server and
// claims a unique logical database name.
func databaseFor(server, namespace string) *v1.Database {
	db := validDatabase()
	db.Namespace = namespace
	db.Spec.ServerRef = server
	db.Spec.DatabaseName = "db_" + utilrand.String(8)
	return db
}

// createDatabase creates db and registers its deletion.
func createDatabase(db *v1.Database) {
	GinkgoHelper()
	Expect(k8sClient.Create(ctx, db)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, db) })
}

// expectDatabaseReady polls until the Ready condition of db matches the given
// status and reason and its message contains messagePart.
func expectDatabaseReady(db *v1.Database, status metav1.ConditionStatus, reason, messagePart string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.Database
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(db), &latest)).To(Succeed())
		ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
		g.Expect(ready).NotTo(BeNil())
		g.Expect(ready.Status).To(Equal(status))
		g.Expect(ready.Reason).To(Equal(reason))
		g.Expect(ready.Message).To(ContainSubstring(messagePart))
	}, timeout, interval).Should(Succeed())
}

// expectOwnedByDatabase asserts that obj carries the controller owner
// reference to db. Garbage collection uses this link when the Database is
// deleted.
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
	Expect(conn.QueryRow(
		ctx,
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
	Expect(conn.QueryRow(
		ctx,
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

// publishedAnyBinding reports whether the Database published anything of its
// own: the DatabaseConfig, or the credentials Secret that it writes first. Any
// other error than a missing object fails the caller.
func publishedAnyBinding(g Gomega, namespace, name string) bool {
	published := false
	for _, get := range []func() error{
		func() error {
			return k8sClient.Get(
				ctx, types.NamespacedName{Namespace: namespace, Name: name}, &v1.DatabaseConfig{},
			)
		},
		func() error {
			return k8sClient.Get(
				ctx,
				types.NamespacedName{Namespace: namespace, Name: name + "-credentials"},
				&corev1.Secret{},
			)
		},
	} {
		err := get()
		if err == nil {
			published = true
			continue
		}
		g.Expect(err).To(MatchError(ContainSubstring("not found")))
	}

	return published
}

var _ = Describe("Database controller", func() {
	It("bootstraps the database and publishes the bindings", func() {
		namespace := newDatabaseNamespace()
		server := createDatabaseServer(namespace)
		db := databaseFor(server, namespace)
		db.Spec.SecondaryStorageConfig = "storage-config"
		createDatabase(db)

		expectDatabaseReady(db, metav1.ConditionTrue, v1.ReasonHealthy, "bindings: Component is healthy.")

		By("creating the SQL objects")
		Expect(sqlDatabaseExists(db.Spec.DatabaseName)).To(BeTrue())
		Expect(sqlRoleExists(db.Spec.DatabaseName)).To(BeTrue())
		Expect(sqlRoleExists(db.Spec.DatabaseName + "_backup")).To(BeTrue())

		By("publishing owner-referenced credential Secrets in the namespace of the Database")
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
		Expect(
			k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "storage-config"}, &storage),
		).To(Succeed())
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

		expectDatabaseReady(db, metav1.ConditionTrue, v1.ReasonHealthy, "bindings: Component is healthy.")

		appKey := types.NamespacedName{Namespace: namespace, Name: db.Name + "-credentials"}
		var original corev1.Secret
		Expect(k8sClient.Get(ctx, appKey, &original)).To(Succeed())
		originalPassword := string(original.Data["password"])
		Expect(k8sClient.Delete(ctx, &original)).To(Succeed())

		var rotated string
		Eventually(func(g Gomega) {
			var secret corev1.Secret
			g.Expect(k8sClient.Get(ctx, appKey, &secret)).To(Succeed())
			g.Expect(secret.UID).NotTo(Equal(original.UID))
			g.Expect(secret.Data).To(HaveKey("password"))
			// A reconcile that was in flight when the delete landed must not
			// republish the old password onto a new object: the delete is the
			// only rotation trigger the contract offers.
			g.Expect(string(secret.Data["password"])).NotTo(Equal(originalPassword))
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

		expectDatabaseReady(db, metav1.ConditionTrue, v1.ReasonHealthy, "bindings: Component is healthy.")

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

		expectDatabaseReady(
			db, metav1.ConditionFalse, v1.ReasonInvalidReference,
			fmt.Sprintf("DatabaseServerConfig %s/no-such-server not found", namespace),
		)
	})

	It("reports MissingSecret when the admin credentials Secret lacks a key", func() {
		namespace := newDatabaseNamespace()

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "half-admin-creds", Namespace: namespace},
			Data:       map[string][]byte{"username": []byte("postgres")},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })

		server := probedServer(namespace, secret.Name, testPostgresHost())

		db := databaseFor(server.Name, namespace)
		createDatabase(db)

		expectDatabaseReady(
			db, metav1.ConditionFalse, v1.ReasonMissingSecret,
			fmt.Sprintf("Secret %s/%s is missing key \"password\"", namespace, secret.Name),
		)
	})

	// The identity of the server is the key of the uniqueness rule. Until the
	// contract publishes it, the operator cannot tell which instance the
	// endpoint reaches, so it claims nothing and touches no server.
	It("waits with ServerIdentityUnknown while the contract publishes no identity", func() {
		namespace := newDatabaseNamespace()

		server := unprobedServer(namespace, createAdminSecret(namespace), testPostgresHost())

		db := databaseFor(server.Name, namespace)
		createDatabase(db)

		expectDatabaseReady(
			db, metav1.ConditionFalse, v1.ReasonServerIdentityUnknown,
			fmt.Sprintf(
				"DatabaseServerConfig %s/%s has not published its system identifier yet. Wait "+
					"until it reports Ready",
				namespace, server.Name,
			),
		)

		Consistently(func() bool {
			return sqlDatabaseExists(db.Spec.DatabaseName)
		}, "3s", interval).Should(BeFalse(), "a Database that claims nothing must run no SQL")
	})

	// A contract keeps the identity it published while its spec moves to
	// another endpoint, until the operator reaches that endpoint. The Database
	// keys its claim on the identity and connects to the endpoint of the spec,
	// so the two must describe one server before it acts.
	It("waits with ServerIdentityUnknown while the identity belongs to the endpoint before a move", func() {
		namespace := newDatabaseNamespace()

		host := testPostgresHost()
		previous := "127.0.0.1"
		if host == previous {
			previous = "localhost"
		}
		server := probedServer(namespace, createAdminSecret(namespace), previous)

		By("moving the endpoint of the contract with no probe of the new one")
		server.Spec.Host = host
		Expect(k8sClient.Update(ctx, server)).To(Succeed())

		db := databaseFor(server.Name, namespace)
		createDatabase(db)

		expectDatabaseReady(
			db, metav1.ConditionFalse, v1.ReasonServerIdentityUnknown,
			fmt.Sprintf(
				"DatabaseServerConfig %s/%s has not reached the server its spec names now",
				namespace, server.Name,
			),
		)

		Consistently(func() bool {
			return sqlDatabaseExists(db.Spec.DatabaseName)
		}, "3s", interval).Should(BeFalse(), "a Database that cannot key its claim must run no SQL")

		By("recording a probe of the endpoint the spec names now")
		publishProbe(server)

		expectDatabaseReady(db, metav1.ConditionTrue, v1.ReasonHealthy, "bindings: Component is healthy.")
		Expect(sqlDatabaseExists(db.Spec.DatabaseName)).To(BeTrue())
	})

	It("reports ConnectionFailed for an unreachable server", func() {
		namespace := newDatabaseNamespace()

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "unreachable-admin-creds", Namespace: namespace},
			Data:       map[string][]byte{"username": []byte("postgres"), "password": []byte("pw")},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })

		server := unprobedServer(namespace, secret.Name, "127.0.0.1")
		server.Spec.Port = 1
		Expect(k8sClient.Update(ctx, server)).To(Succeed())
		publishProbe(server)

		db := databaseFor(server.Name, namespace)
		createDatabase(db)

		expectDatabaseReady(
			db, metav1.ConditionFalse, v1.ReasonConnectionFailed,
			fmt.Sprintf("Connecting to DatabaseServerConfig %s/%s", namespace, server.Name),
		)
	})

	It("derives distinct backup roles for long database names sharing a prefix", func() {
		namespace := newDatabaseNamespace()
		server := createDatabaseServer(namespace)

		// The two 60-character names are identical up to the truncation point.
		// Plain truncation collapses both onto one backup role.
		prefix := "db_" + utilrand.String(5) + strings.Repeat("x", 48)
		first := databaseFor(server, namespace)
		first.Spec.DatabaseName = prefix + "aaaa"
		second := databaseFor(server, namespace)
		second.Spec.DatabaseName = prefix + "bbbb"
		createDatabase(first)
		createDatabase(second)

		expectDatabaseReady(first, metav1.ConditionTrue, v1.ReasonHealthy, "bindings: Component is healthy.")
		expectDatabaseReady(second, metav1.ConditionTrue, v1.ReasonHealthy, "bindings: Component is healthy.")

		By("creating one backup role per database")
		firstRole := components.BackupUserName(first.Spec.DatabaseName)
		secondRole := components.BackupUserName(second.Spec.DatabaseName)
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
		Expect(err).To(
			HaveOccurred(),
			"one database's backup role must not reach the sibling database",
		)
	})

	It("rejects a later Database claiming an already claimed logical database", func() {
		namespace := newDatabaseNamespace()
		server := createDatabaseServer(namespace)

		winner := databaseFor(server, namespace)
		winner.Name = "colla-" + utilrand.String(8)
		createDatabase(winner)
		expectDatabaseReady(winner, metav1.ConditionTrue, v1.ReasonHealthy, "bindings: Component is healthy.")

		loser := databaseFor(server, namespace)
		loser.Name = "collb-" + utilrand.String(8)
		loser.Spec.DatabaseName = winner.Spec.DatabaseName
		createDatabase(loser)

		expectDatabaseReady(
			loser, metav1.ConditionFalse, v1.ReasonInvalidReference,
			fmt.Sprintf(
				"Database %s/%s already claims database %q",
				namespace, winner.Name, winner.Spec.DatabaseName,
			),
		)
	})

	// Two Databases that name one logical database on one server can reach
	// their first reconcile together, before either has recorded a claim.
	// The cached index answers both with an empty list, so the rule alone
	// lets both of them bootstrap. The claim that the API server serializes
	// is what decides between them.
	It("serializes the claim of two Databases that reconcile together", func() {
		namespace := newDatabaseNamespace()
		server := unprobedServer(namespace, createAdminSecret(namespace), testPostgresHost())

		first := databaseFor(server.Name, namespace)
		first.Name = "racea-" + utilrand.String(8)
		createDatabase(first)

		second := databaseFor(server.Name, namespace)
		second.Name = "racez-" + utilrand.String(8)
		second.Spec.DatabaseName = first.Spec.DatabaseName
		createDatabase(second)

		// Neither can key a claim while the contract has no identity, so the
		// probe below enqueues both of them at once with an empty index.
		expectDatabaseReady(
			first, metav1.ConditionFalse, v1.ReasonServerIdentityUnknown, "system identifier",
		)
		expectDatabaseReady(
			second, metav1.ConditionFalse, v1.ReasonServerIdentityUnknown, "system identifier",
		)

		publishProbe(server)

		By("publishing no second set of bindings, from the first reconcile on")
		Consistently(func(g Gomega) {
			published := 0
			for _, db := range []*v1.Database{first, second} {
				if publishedAnyBinding(g, namespace, db.Name) {
					published++
				}
			}
			g.Expect(published).To(
				BeNumerically("<=", 1), "only the holder of the claim may publish bindings",
			)
		}, "5s", 100*time.Millisecond).Should(Succeed())

		// Which of the two holds the claim is the order in which they reached
		// the API server, and nothing here depends on it. The Lease names the
		// holder, and the other one reports it.
		By("leaving one holder of the claim Lease")
		var lease coordinationv1.Lease
		leaseKey := types.NamespacedName{
			Namespace: testClaimNamespace,
			Name: claimLeaseName(
				components.CollisionKey(serverSystemIdentifier(), first.Spec.DatabaseName),
			),
		}
		Eventually(func() error {
			return k8sClient.Get(ctx, leaseKey, &lease)
		}, timeout, interval).Should(Succeed())

		winner, loser := first, second
		if lease.Annotations[claimHolderNameAnnotation] == second.Name {
			winner, loser = second, first
		}

		expectDatabaseReady(winner, metav1.ConditionTrue, v1.ReasonHealthy, "bindings: Component is healthy.")
		expectDatabaseReady(
			loser, metav1.ConditionFalse, v1.ReasonInvalidReference,
			fmt.Sprintf(
				"Database %s/%s already claims database %q",
				namespace, winner.Name, winner.Spec.DatabaseName,
			),
		)
	})

	// A Database can lose a claim after it published under it, when its spec
	// names a logical database that another Database already holds. The
	// holder owns that logical database and resets the shared role password,
	// so the credentials the loser published open nothing. It must withdraw
	// them rather than leave a Secret that authenticates nowhere.
	It("withdraws the bindings of a Database that loses its claim", func() {
		namespace := newDatabaseNamespace()
		server := createDatabaseServer(namespace)

		holder := databaseFor(server, namespace)
		holder.Name = "losta-" + utilrand.String(8)
		createDatabase(holder)
		expectDatabaseReady(holder, metav1.ConditionTrue, v1.ReasonHealthy, "bindings: Component is healthy.")

		mover := databaseFor(server, namespace)
		mover.Name = "lostz-" + utilrand.String(8)
		mover.Spec.SecondaryStorageConfig = "lost-storage"
		createDatabase(mover)
		expectDatabaseReady(mover, metav1.ConditionTrue, v1.ReasonHealthy, "bindings: Component is healthy.")

		By("publishing the bindings while it holds a claim of its own")
		appKey := types.NamespacedName{Namespace: namespace, Name: mover.Name + "-credentials"}
		Expect(k8sClient.Get(ctx, appKey, &corev1.Secret{})).To(Succeed())

		By("pointing it at the logical database that the other one holds")
		Eventually(func(g Gomega) {
			var latest v1.Database
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(mover), &latest)).To(Succeed())
			latest.Spec.DatabaseName = holder.Spec.DatabaseName
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		expectDatabaseReady(
			mover, metav1.ConditionFalse, v1.ReasonInvalidReference,
			fmt.Sprintf(
				"Database %s/%s already claims database %q",
				namespace, holder.Name, holder.Spec.DatabaseName,
			),
		)

		By("withdrawing every binding it published")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, appKey, &corev1.Secret{})).To(
				MatchError(ContainSubstring("not found")),
			)
			g.Expect(k8sClient.Get(
				ctx, types.NamespacedName{
					Namespace: namespace, Name: mover.Name + "-backup-credentials",
				}, &corev1.Secret{},
			)).To(MatchError(ContainSubstring("not found")))
			g.Expect(k8sClient.Get(
				ctx, types.NamespacedName{
					Namespace: namespace, Name: mover.Name,
				}, &v1.DatabaseConfig{},
			)).To(MatchError(ContainSubstring("not found")))
			g.Expect(k8sClient.Get(
				ctx, types.NamespacedName{
					Namespace: namespace, Name: "lost-storage",
				}, &v1.SecondaryStorageConfig{},
			)).To(MatchError(ContainSubstring("not found")))
		}, timeout, interval).Should(Succeed())

		By("owning the BindingsReady condition of the loser")
		var latest v1.Database
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(mover), &latest)).To(Succeed())
		bindings := meta.FindStatusCondition(latest.Status.Conditions, string(components.ConditionBindings))
		Expect(bindings).NotTo(BeNil())
		Expect(bindings.Reason).To(Equal("Disabled"))

		By("keeping the bindings of the Database that holds the claim")
		Expect(k8sClient.Get(
			ctx, types.NamespacedName{Namespace: namespace, Name: holder.Name}, &v1.DatabaseConfig{},
		)).To(Succeed())
	})

	// Two contracts that describe one PostgreSQL instance under different
	// hosts are one server. The claim follows the instance, so the second
	// Database loses it even though it names another contract in another
	// namespace.
	It("rejects a Database of another namespace claiming the same logical database", func() {
		pg, err := testPostgres()
		Expect(err).NotTo(HaveOccurred())

		first := newDatabaseNamespacePrefixed("db-ns-a")
		firstServer := createDatabaseServerAt(first, pg.Host)

		second := newDatabaseNamespacePrefixed("db-ns-z")
		otherHost := "127.0.0.1"
		if pg.Host == otherHost {
			otherHost = "localhost"
		}
		secondServer := createDatabaseServerAt(second, otherHost)

		winner := databaseFor(firstServer, first)
		winner.Name = "xnsa-" + utilrand.String(8)
		createDatabase(winner)
		expectDatabaseReady(winner, metav1.ConditionTrue, v1.ReasonHealthy, "bindings: Component is healthy.")

		loser := databaseFor(secondServer, second)
		loser.Name = "xnsb-" + utilrand.String(8)
		loser.Spec.DatabaseName = winner.Spec.DatabaseName
		createDatabase(loser)

		expectDatabaseReady(
			loser, metav1.ConditionFalse, v1.ReasonInvalidReference,
			fmt.Sprintf(
				"Database %s/%s already claims database %q",
				first, winner.Name, winner.Spec.DatabaseName,
			),
		)

		By("publishing no bindings for the Database that lost the claim")
		Consistently(func() error {
			return k8sClient.Get(
				ctx,
				types.NamespacedName{Namespace: second, Name: loser.Name},
				&v1.DatabaseConfig{},
			)
		}, "3s", interval).Should(MatchError(ContainSubstring("not found")))
	})

	// A claim stays with the Database that took it. An older Database whose
	// contract reaches its server later meets a logical database that is in
	// use, with published Secrets that carry the passwords of its holder. To
	// hand it on would reset those passwords under a running cluster.
	It("keeps the claim with the Database that took it first", func() {
		pg, err := testPostgres()
		Expect(err).NotTo(HaveOccurred())

		older := newDatabaseNamespacePrefixed("db-ns-a")
		olderServer := unprobedServer(older, createAdminSecret(older), pg.Host)
		olderDB := databaseFor(olderServer.Name, older)
		olderDB.Name = "lateta-" + utilrand.String(8)
		createDatabase(olderDB)
		expectDatabaseReady(
			olderDB, metav1.ConditionFalse, v1.ReasonServerIdentityUnknown, "system identifier",
		)

		By("letting the newer Database claim while the older one waits")
		newer := newDatabaseNamespacePrefixed("db-ns-z")
		newerServer := createDatabaseServerAt(newer, pg.Host)
		newerDB := databaseFor(newerServer, newer)
		newerDB.Name = "latetz-" + utilrand.String(8)
		newerDB.Spec.DatabaseName = olderDB.Spec.DatabaseName
		createDatabase(newerDB)
		expectDatabaseReady(newerDB, metav1.ConditionTrue, v1.ReasonHealthy, "bindings: Component is healthy.")

		By("giving the older contract its identity")
		publishProbe(olderServer)

		By("leaving the claim with the Database that holds it")
		expectDatabaseReady(
			olderDB, metav1.ConditionFalse, v1.ReasonInvalidReference,
			fmt.Sprintf(
				"Database %s/%s already claims database %q",
				newer, newerDB.Name, newerDB.Spec.DatabaseName,
			),
		)
		expectDatabaseReady(newerDB, metav1.ConditionTrue, v1.ReasonHealthy, "bindings: Component is healthy.")

		By("publishing no bindings for the Database that lost the claim")
		Consistently(func() error {
			return k8sClient.Get(
				ctx, types.NamespacedName{Namespace: older, Name: olderDB.Name}, &v1.DatabaseConfig{},
			)
		}, "3s", interval).Should(MatchError(ContainSubstring("not found")))

		By("keeping the bindings of the Database that holds the claim")
		Expect(k8sClient.Get(
			ctx, types.NamespacedName{Namespace: newer, Name: newerDB.Name}, &v1.DatabaseConfig{},
		)).To(Succeed())
	})

	// Two Databases of one namespace can name one DatabaseConfig explicitly.
	// Only the one that owns it may delete it, so the loser withdraws its own
	// Secrets and leaves the contract of the winner where it is.
	It("leaves a binding it does not own in place when it withdraws", func() {
		namespace := newDatabaseNamespace()
		server := createDatabaseServer(namespace)
		shared := "shared-config-" + utilrand.String(6)

		holder := databaseFor(server, namespace)
		holder.Name = "ownera-" + utilrand.String(8)
		holder.Spec.DatabaseConfig = shared
		createDatabase(holder)
		expectDatabaseReady(holder, metav1.ConditionTrue, v1.ReasonHealthy, "bindings: Component is healthy.")

		sharedKey := types.NamespacedName{Namespace: namespace, Name: shared}
		var owned v1.DatabaseConfig
		Expect(k8sClient.Get(ctx, sharedKey, &owned)).To(Succeed())
		Expect(metav1.GetControllerOf(&owned).Name).To(Equal(holder.Name))

		By("creating a later Database that names the same DatabaseConfig and loses the claim")
		loser := databaseFor(server, namespace)
		loser.Name = "ownerz-" + utilrand.String(8)
		loser.Spec.DatabaseName = holder.Spec.DatabaseName
		loser.Spec.DatabaseConfig = shared
		createDatabase(loser)

		expectDatabaseReady(
			loser, metav1.ConditionFalse, v1.ReasonInvalidReference,
			"These bindings belong to another Database and stay in place: "+sharedKey.String(),
		)

		By("keeping the DatabaseConfig of the holder, with its owner reference intact")
		Consistently(func(g Gomega) {
			var still v1.DatabaseConfig
			g.Expect(k8sClient.Get(ctx, sharedKey, &still)).To(Succeed())
			g.Expect(metav1.GetControllerOf(&still).Name).To(Equal(holder.Name))
		}, "3s", interval).Should(Succeed())

		By("keeping the holder Ready on its own bindings")
		expectDatabaseReady(holder, metav1.ConditionTrue, v1.ReasonHealthy, "bindings: Component is healthy.")
	})

	It("gives its claim back on deletion, leaving the SQL database and users intact", func() {
		namespace := newDatabaseNamespace()
		server := createDatabaseServer(namespace)
		db := databaseFor(server, namespace)
		createDatabase(db)

		expectDatabaseReady(db, metav1.ConditionTrue, v1.ReasonHealthy, "bindings: Component is healthy.")

		By("holding the claim Lease of the logical database")
		var claimed v1.Database
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(db), &claimed)).To(Succeed())
		lease := types.NamespacedName{
			Namespace: testClaimNamespace, Name: claimLeaseName(claimed.Status.CollisionKey),
		}
		Expect(k8sClient.Get(ctx, lease, &coordinationv1.Lease{})).To(Succeed())

		Expect(k8sClient.Delete(ctx, db)).To(Succeed())
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(db), &v1.Database{})
		}, timeout, interval).Should(
			MatchError(ContainSubstring("not found")),
			"the claim finalizer must release the CR",
		)

		By("giving the claim back, so the logical database is free again")
		Eventually(func() error {
			return k8sClient.Get(ctx, lease, &coordinationv1.Lease{})
		}, timeout, interval).Should(MatchError(ContainSubstring("not found")))

		Expect(sqlDatabaseExists(db.Spec.DatabaseName)).To(
			BeTrue(),
			"deleting the CR must never drop the logical database",
		)
		Expect(sqlRoleExists(db.Spec.DatabaseName)).To(BeTrue())
		Expect(sqlRoleExists(db.Spec.DatabaseName + "_backup")).To(BeTrue())
	})
})
