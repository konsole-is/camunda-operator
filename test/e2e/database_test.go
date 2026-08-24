//go:build e2e
// +build e2e

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

package e2e

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/test/utils"
)

const (
	dbNamespace = "database-e2e"
	// postgresService and postgresAdminSecret are the Service and the admin
	// Secret that testdata/postgres.yaml creates; dbServer is the
	// DatabaseServerConfig that describes that server.
	postgresService     = "postgres"
	postgresAdminSecret = "postgres-admin"
	dbServer            = "camunda-postgres"
	// dbName is the Database CR; dbDatabaseName is the logical database and
	// the application role it bootstraps.
	dbName         = "camunda-db"
	dbDatabaseName = "camunda"
	// dbStorageConfig is the SecondaryStorageConfig that the Database
	// publishes.
	dbStorageConfig = "camunda-db-storage"
	// dbTable is the table that the application user creates and the backup
	// user reads.
	dbTable = "documents"

	dbResource       = "databases.core.camunda.io"
	dbServerResource = "databaseserverconfigs.core.camunda.io"
	dbConfigResource = "databaseconfigs.core.camunda.io"
	sscResource      = "secondarystorageconfigs.core.camunda.io"
)

var _ = Describe("Database", Ordered, Label(labelDatabase), func() {
	var (
		server = &v1.DatabaseServerConfig{
			TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "DatabaseServerConfig"},
			ObjectMeta: metav1.ObjectMeta{Name: dbServer, Namespace: dbNamespace},
			Spec: v1.DatabaseServerConfigSpec{
				Engine: v1.DatabaseEnginePostgres,
				Host:   postgresService + "." + dbNamespace + ".svc",
				Port:   5432,
				AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
					Name:        postgresAdminSecret,
					UsernameKey: "username",
					PasswordKey: "password",
				},
			},
		}
		database = &v1.Database{
			TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "Database"},
			ObjectMeta: metav1.ObjectMeta{Name: dbName, Namespace: dbNamespace},
			Spec: v1.DatabaseSpec{
				ServerRef:              dbServer,
				DatabaseName:           dbDatabaseName,
				SecondaryStorageConfig: dbStorageConfig,
			},
		}
		// config is the DatabaseConfig binding as published; the credential
		// checks read the Secret references from it.
		config v1.DatabaseConfig
	)

	BeforeAll(func() {
		By("creating the test namespace")
		_, err := utils.Kubectl("create", "ns", dbNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("deploying PostgreSQL")
		_, err = utils.Kubectl("apply", "-n", dbNamespace, "-f", "test/e2e/testdata/postgres.yaml")
		Expect(err).NotTo(HaveOccurred())
		_, err = utils.Kubectl(
			"rollout", "status", "deployment/postgres", "-n", dbNamespace, "--timeout", "5m",
		)
		Expect(err).NotTo(HaveOccurred())

		By("describing the server")
		Expect(apply(server)).To(Succeed())
	})

	AfterAll(func() {
		By("removing the test namespace, which holds the whole chain")
		_, _ = utils.Kubectl("delete", "ns", dbNamespace, "--wait=false")
	})

	AfterEach(func() {
		dumpDiagnostics(dbNamespace)
	})

	It("reaches Ready Healthy and publishes its bindings in its own namespace", func() {
		By("waiting for the contract to publish the identity of the server")
		Eventually(func(g Gomega) {
			var contract v1.DatabaseServerConfig
			g.Expect(utils.Get(dbServerResource, dbServer, dbNamespace, &contract)).To(Succeed())
			g.Expect(contract.Status.SystemIdentifier).NotTo(BeEmpty())
		}, 3*time.Minute).Should(Succeed())

		By("creating the Database")
		Expect(apply(database)).To(Succeed())

		By("waiting for Ready Healthy")
		Eventually(func(g Gomega) {
			expectReady(g, dbResource, dbName, dbNamespace, v1.ReasonHealthy)
		}, 3*time.Minute).Should(Succeed())

		By("reading the DatabaseConfig")
		Expect(utils.Get(dbConfigResource, dbName, dbNamespace, &config)).To(Succeed())
		Expect(config.OwnerReferences).To(ContainElement(HaveField("Name", dbName)))
		Expect(config.Spec.ServerRef).To(Equal(dbServer))
		Expect(config.Spec.DatabaseName).To(Equal(dbDatabaseName))
		Expect(config.Spec.BackupCredentialsSecretRef).NotTo(BeNil())

		By("reading the credential Secrets")
		app := config.Spec.CredentialsSecretRef
		username, err := utils.SecretValue(app.Namespace, app.Name, app.UsernameKey)
		Expect(err).NotTo(HaveOccurred())
		Expect(username).To(Equal(dbDatabaseName))
		backup := config.Spec.BackupCredentialsSecretRef
		username, err = utils.SecretValue(backup.Namespace, backup.Name, backup.UsernameKey)
		Expect(err).NotTo(HaveOccurred())
		Expect(username).To(Equal(dbDatabaseName + "_backup"))

		By("reading the SecondaryStorageConfig")
		var contract v1.SecondaryStorageConfig
		Expect(utils.Get(sscResource, dbStorageConfig, dbNamespace, &contract)).To(Succeed())
		Expect(contract.Spec.Type).To(Equal(v1.SecondaryStorageTypeRDBMS))
		Expect(contract.Spec.RDBMS).NotTo(BeNil())
		Expect(contract.Spec.RDBMS.DatabaseConfigRef).To(Equal(dbName))

		By("waiting for the bindings to validate their references")
		Eventually(func(g Gomega) {
			expectReady(g, dbConfigResource, dbName, dbNamespace, v1.ReasonHealthy)
			expectReady(g, sscResource, dbStorageConfig, dbNamespace, v1.ReasonHealthy)
		}, 2*time.Minute).Should(Succeed())
	})

	It("lets the application user own the database and the backup user read it", func() {
		By("creating a table and a row as the application user")
		_, err := psql(
			dbNamespace, "app", config.Spec.CredentialsSecretRef,
			"CREATE TABLE "+dbTable+" (id int PRIMARY KEY); INSERT INTO "+dbTable+" VALUES (1);",
		)
		Expect(err).NotTo(HaveOccurred())

		By("counting the rows as the backup user")
		out, err := psql(
			dbNamespace, "backup", *config.Spec.BackupCredentialsSecretRef,
			"SELECT count(*) FROM "+dbTable,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(out)).To(Equal("1"))
	})

	It("garbage-collects the bindings on deletion and keeps the logical database", func() {
		_, err := utils.Kubectl("delete", dbResource, dbName, "-n", dbNamespace, "--wait=false")
		Expect(err).NotTo(HaveOccurred())

		By("waiting for the bindings to be gone")
		Eventually(func(g Gomega) {
			expectGone(g, dbResource, dbName, dbNamespace)
			expectGone(g, dbConfigResource, dbName, dbNamespace)
			expectGone(g, sscResource, dbStorageConfig, dbNamespace)
			expectGone(g, "secret", config.Spec.CredentialsSecretRef.Name, dbNamespace)
			expectGone(g, "secret", config.Spec.BackupCredentialsSecretRef.Name, dbNamespace)
		}, 3*time.Minute).Should(Succeed())

		By("reading the row as the server admin")
		out, err := psql(
			dbNamespace,
			"admin",
			v1.CredentialsSecretRef{Name: postgresAdminSecret, UsernameKey: "username", PasswordKey: "password"},
			"SELECT count(*) FROM "+dbTable,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(out)).To(Equal("1"))
	})
})
