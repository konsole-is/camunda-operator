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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	dscomponents "github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
	"github.com/konsole-is/camunda-operator/test/utils"
)

const (
	// dsNamespace holds the whole chain of this flow: the server, the logical
	// database, and the orchestration cluster that runs on it.
	dsNamespace = "databaseserver-e2e"
	// dsServer names the DatabaseServer, the CloudNativePG cluster it creates,
	// and the DatabaseServerConfig it publishes. One name for the three is
	// what a user writes, and it makes every derived name readable.
	dsServer = "camunda-cnpg"
	// dsDatabase and dsStorageConfig are the Database on that server and the
	// SecondaryStorageConfig it publishes for the cluster.
	dsDatabase      = "camunda-cnpg-db"
	dsStorageConfig = "camunda-cnpg-storage"
	// dsPlatform is the cluster-scoped platform config of this flow.
	dsPlatform = "databaseserver-e2e"
	// dsVersion is the PostgreSQL major the server runs.
	dsVersion = "17"
	// dsRetentionDays is how far back the archive of the server reaches. The
	// point this flow restores to is minutes old, so one day is enough, and a
	// short retention keeps the bucket small.
	dsRetentionDays = 1
	// dsArchiveSegment separates the archives of a DatabaseServer from the
	// backup layout inside one bucket. The operator writes the archive under
	// <basePath>/<segment>/<namespace>/<server>/<cluster>.
	dsArchiveSegment = "databaseserver"

	dsResource = "databaseservers.core.camunda.io"

	// dsReadyTimeout covers the pull of the PostgreSQL image, the bootstrap of
	// the instance, and the first base backup, which the archive waits for
	// before it reports ready.
	dsReadyTimeout = 10 * time.Minute
)

var _ = Describe("DatabaseServer", Ordered, Label(labelDatabaseServer), func() {
	var (
		server = &v1.DatabaseServer{
			TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "DatabaseServer"},
			ObjectMeta: metav1.ObjectMeta{Name: dsServer, Namespace: dsNamespace},
			Spec: v1.DatabaseServerSpec{
				Version:              dsVersion,
				Instances:            new(int32(1)),
				Resources:            requests("500m", "512Mi"),
				StorageSize:          new(resource.MustParse("2Gi")),
				DatabaseServerConfig: dsServer,
				Archive: &v1.DatabaseServerArchiveSpec{
					ObjectStorageRef:    backupStorage,
					RetentionPeriodDays: dsRetentionDays,
				},
			},
		}
		database = &v1.Database{
			TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "Database"},
			ObjectMeta: metav1.ObjectMeta{Name: dsDatabase, Namespace: dsNamespace},
			Spec: v1.DatabaseSpec{
				ServerRef:              dsServer,
				DatabaseName:           dbDatabaseName,
				SecondaryStorageConfig: dsStorageConfig,
			},
		}
		cluster = newCluster(dsNamespace, dsPlatform, dsStorageConfig, backupStorage, false)
	)

	BeforeAll(func() {
		// Zeebe takes a primary-storage backup every schedule and writes a
		// marker checkpoint every checkpointInterval. The restore application
		// aligns the brokers to the newest checkpoint the backups cover, so
		// the defaults of one hour and fifteen minutes would leave the point
		// of this flow, minutes old, outside every backup.
		cluster.Spec.Backup = &v1.ClusterBackupSpec{
			PrimaryStorage: &v1.PrimaryStorageBackupSpec{
				Schedule:           "PT2M",
				CheckpointInterval: "PT1M",
			},
		}

		By("creating the test namespace")
		_, err := utils.Kubectl("create", "ns", dsNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("creating the DatabaseServer")
		Expect(apply(server)).To(Succeed())

		By("creating the platform config")
		Expect(apply(basicPlatform(dsPlatform))).To(Succeed())
	})

	AfterAll(func() {
		By("removing the restore, the cluster, the chain, the platform config, and the namespace")
		_, _ = utils.Kubectl(
			"delete", pitrResource, pitrDatabaseServer,
			"-n", dsNamespace, "--ignore-not-found", "--wait=false",
		)
		_, _ = utils.Kubectl("delete", ccResource, ccName, "-n", dsNamespace, "--ignore-not-found", "--wait=false")
		_, _ = utils.Kubectl("delete", dbResource, dsDatabase, "-n", dsNamespace, "--ignore-not-found")
		_, _ = utils.Kubectl("delete", dsResource, dsServer, "-n", dsNamespace, "--ignore-not-found", "--wait=false")
		_, _ = utils.Kubectl("delete", ccPlatformResource, dsPlatform, "--ignore-not-found")
		_, _ = utils.Kubectl("delete", "ns", dsNamespace, "--wait=false")
	})

	AfterEach(func() {
		dumpDiagnostics(dsNamespace)
	})

	It("reaches Ready Healthy and publishes a contract that offers operator recovery", func() {
		By("waiting for Ready Healthy")
		Eventually(func(g Gomega) {
			expectReady(g, dsResource, dsServer, dsNamespace, v1.ReasonHealthy)
		}, dsReadyTimeout, 5*time.Second).Should(Succeed())

		By("reading what the server records about the instance it runs")
		var current v1.DatabaseServer
		Expect(utils.Get(dsResource, dsServer, dsNamespace, &current)).To(Succeed())
		Expect(current.Status.Cluster).To(Equal(dsServer))
		Expect(current.Status.SystemIdentifier).NotTo(BeEmpty())
		Expect(current.Status.Archive).NotTo(BeNil())
		Expect(current.Status.Archive.History).To(HaveLen(1))
		Expect(current.Status.Archive.History[0].ServerName).To(Equal(dsServer))
		Expect(current.Status.Archive.History[0].To).To(BeNil(), "the archive the server writes to is closed")
		Expect(current.Status.Volumes).NotTo(BeEmpty())

		By("reading the published DatabaseServerConfig")
		var contract v1.DatabaseServerConfig
		Expect(utils.Get(dbServerResource, dsServer, dsNamespace, &contract)).To(Succeed())
		Expect(contract.OwnerReferences).To(ContainElement(HaveField("Name", dsServer)))
		Expect(contract.Spec.Engine).To(Equal(v1.DatabaseEnginePostgres))
		Expect(contract.Spec.Host).To(Equal(dscomponents.ReadWriteHost(&current)))
		Expect(contract.Spec.Port).To(BeEquivalentTo(dscomponents.PostgresPort))
		Expect(contract.Spec.AdminCredentialsSecretRef.Name).To(Equal(dscomponents.SuperuserSecretName(&current)))
		Expect(contract.Spec.PITR).NotTo(BeNil())
		Expect(contract.Spec.PITR.Enabled).To(BeTrue())
		Expect(contract.Spec.PITR.Recovery).To(Equal(v1.RecoveryModeOperator))
		Expect(contract.Spec.PITR.RetentionPeriodDays).To(HaveValue(BeEquivalentTo(dsRetentionDays)))

		By("waiting for the contract to report the identity of the instance behind it")
		Eventually(func(g Gomega) {
			var probed v1.DatabaseServerConfig
			g.Expect(utils.Get(dbServerResource, dsServer, dsNamespace, &probed)).To(Succeed())
			g.Expect(probed.Status.SystemIdentifier).To(Equal(current.Status.SystemIdentifier))
			g.Expect(probed.Status.ServerVersion).To(Equal(dsVersion))
			expectReady(g, dbServerResource, dsServer, dsNamespace, v1.ReasonHealthy)
		}, 3*time.Minute, 5*time.Second).Should(Succeed())
	})

	It("shows its readiness, its reason, and its version in the resource table", func() {
		out, err := utils.Kubectl("get", dsResource, dsServer, "-n", dsNamespace)
		Expect(err).NotTo(HaveOccurred())

		lines := strings.Split(strings.TrimSpace(out), "\n")
		Expect(lines).To(HaveLen(2), out)
		Expect(strings.Fields(lines[0])).To(Equal([]string{"NAME", "READY", "REASON", "VERSION", "AGE"}), out)
		Expect(strings.Fields(lines[1])).To(
			HaveExactElements(dsServer, "True", v1.ReasonHealthy, dsVersion, Not(BeEmpty())), out,
		)
	})

	It("holds a base backup in the archive prefix of the bucket", func() {
		var current v1.DatabaseServer
		Expect(utils.Get(dsResource, dsServer, dsNamespace, &current)).To(Succeed())

		prefix := strings.Join([]string{
			backupBasePath, dsArchiveSegment, dsNamespace, dsServer, dscomponents.ClusterName(&current), "base",
		}, "/") + "/"

		objects, err := utils.MinIOObjectsWithPrefix(minioNamespace, prefix, storeTimeout)
		Expect(err).NotTo(HaveOccurred())
		Expect(objects).NotTo(BeEmpty(), "the archive holds no base backup under %q", prefix)
	})

	It("bootstraps a logical database on the server", func() {
		By("creating the Database")
		Expect(apply(database)).To(Succeed())

		By("waiting for the bindings to be Ready Healthy")
		Eventually(func(g Gomega) {
			expectReady(g, dbResource, dsDatabase, dsNamespace, v1.ReasonHealthy)
			expectReady(g, dbConfigResource, dsDatabase, dsNamespace, v1.ReasonHealthy)
			expectReady(g, sscResource, dsStorageConfig, dsNamespace, v1.ReasonHealthy)
		}, 5*time.Minute, 5*time.Second).Should(Succeed())

		var config v1.DatabaseConfig
		Expect(utils.Get(dbConfigResource, dsDatabase, dsNamespace, &config)).To(Succeed())
		Expect(config.Spec.ServerRef).To(Equal(dsServer))
		Expect(config.Spec.DatabaseName).To(Equal(dbDatabaseName))
	})

	itRunsTheOrchestrationCluster(cluster)
	itRunsAPointInTimeRestoreThroughTheDatabaseServer(cluster, dsServer)
})
