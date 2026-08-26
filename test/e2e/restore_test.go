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
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/restore"
	"github.com/konsole-is/camunda-operator/test/utils"
)

const (
	// esRestoreBackup and rdbmsRestoreBackup are the backups that the restores
	// read. Each restore takes a backup of its own: the backup specs delete
	// theirs, and with it the snapshots and the dump that a restore needs.
	esRestoreBackup    = "camunda-es-restore-source"
	rdbmsRestoreBackup = "camunda-rdbms-restore-source"
	// esRestore and rdbmsRestore are the two logical restores.
	esRestore    = "camunda-es-restore"
	rdbmsRestore = "camunda-rdbms-restore"
	// pitrRefused asks for a point that the live database is ahead of.
	// pitrCurrent asks for the point that the suspended database already
	// holds. pitrUncovered asks for the point the database holds too, but
	// that point lies past every primary-storage backup.
	pitrRefused   = "camunda-pitr-unrestored"
	pitrCurrent   = "camunda-pitr-current"
	pitrUncovered = "camunda-pitr-uncovered"
	// pitrDatabaseServer asks a server that the operator runs to roll itself
	// back, which is the whole flow a user gets from a DatabaseServer.
	pitrDatabaseServer = "camunda-pitr-databaseserver"

	lresResource    = "logicalrestoreelasticsearches.core.camunda.io"
	lrrdbmsResource = "logicalrestorerdbmses.core.camunda.io"
	pitrResource    = "pointintimerestores.core.camunda.io"

	// restoreTimeout bounds one restore: the snapshot recovery or the
	// pg_restore Job, the recreated broker volumes, and the restore
	// application of every broker.
	restoreTimeout = 15 * time.Minute
	// pitrRetentionDays is the retention period that the e2e database server
	// declares. Nothing archives a write-ahead log here, so the number only
	// has to hold the point that the refusing restore asks for.
	pitrRetentionDays = 7
	// pitrRefusedAge is how far the refusing restore reaches back. It is far
	// enough that the last export of the cluster is always later, whatever the
	// run took: a point after the last export would pass the check and prove
	// nothing.
	pitrRefusedAge = 24 * time.Hour
	// pitrHold is how long the refusing restore must keep its hold.
	pitrHold = 30 * time.Second
	// pitrClockGap separates the point a restore asks for from the write that
	// must not survive it. The suite and PostgreSQL read two clocks, and the
	// gap keeps that write on the later side of the point whichever of the
	// two runs ahead.
	pitrClockGap = 10 * time.Second
	// pitrBackupCoverage bounds the wait for a primary-storage backup that
	// reaches past the point. That backup is where the cluster comes back, so
	// the flow waits for it before it writes what must not survive. The worst
	// case is two minutes for the schedule of the flow, one for its checkpoint
	// interval, and the duration of one backup. The bound leaves room above
	// that.
	pitrBackupCoverage = 6 * time.Minute
	// databaseRecoveryTimeout bounds the database half of an operator-driven
	// restore: CloudNativePG bootstraps a cluster from the archive, the
	// contract moves to it, and the operator reaches the new endpoint before
	// the restore goes on.
	//
	// The bound is shorter than two probe cycles of the contract, which run
	// ten minutes apart. It holds because the recovery moves the host and the
	// admin Secret of the contract, and probedAnotherServer answers true on
	// either. The recorded probe is stale from that moment, so the reconcile
	// that the move triggers probes the new endpoint at once.
	databaseRecoveryTimeout = 15 * time.Minute
	// watchStopTimeout bounds how long a restore Job watch waits for its last
	// look to land. One look is one kubectl call, so the bound only fires when
	// that call hangs.
	watchStopTimeout = time.Minute
)

// publicTableCount counts the tables of the schema that a Camunda cluster
// writes into. It answers zero for an erased schema, where a count over a
// Camunda table would report an error instead.
const publicTableCount = `SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'`

// sqlTimestampLayout writes an instant the way the exporter position column
// holds one: a wall clock without a zone. The RDBMS exporter writes the local
// time of the broker into it, and the brokers run in UTC.
const sqlTimestampLayout = "2006-01-02 15:04:05"

// rolledBackPosition is the exporter position of a database that was rolled
// back to the start of the log. Zeebe numbers the first record of a partition
// 1, and the earliest checkpoint of a cluster covers it.
const rolledBackPosition = 1

// rollBackSQL rolls the logical database back to one exporter position, the
// way a point-in-time recovery of the server would: it erases every row that
// the exporter wrote, and it sets the position row to the position and the
// time given. The schema and its migration log stay, as they do after a
// recovery to a point that lies after the exporter created them.
//
// The rows must go with the position. The exporter resumes from the position
// it reads here and inserts again what it finds in the log. A row that stayed
// collides with that insert on its key, and the exporter then flushes nothing
// ever again.
//
// The block is quoted with a named tag. The SQL travels as a container
// command, and Kubernetes reduces "$$" in a command to one "$" as its escape
// for variable expansion, which leaves "DO $" behind.
const rollBackSQL = `DO $rollback$
DECLARE t text;
BEGIN
  FOR t IN
    SELECT table_name FROM information_schema.tables
    WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
      AND lower(table_name) NOT LIKE '%%exporter_position'
      AND lower(table_name) NOT LIKE '%%databasechangelog%%'
  LOOP
    EXECUTE format('TRUNCATE TABLE %%I CASCADE', t);
  END LOOP;
  UPDATE exporter_position SET last_exported_position = %d, last_updated = '%s';
END $rollback$`

// itRestoresTheElasticsearchCluster registers the restore specs of the
// Elasticsearch flow. The CamundaCluster flow calls it while its cluster is
// healthy and has exported a process instance, so the specs prove the round
// trip: the state of the cluster goes into a backup, both halves of it are
// erased, and the backup alone brings the process instance back.
//
// The specs are ordered. Each one leaves the cluster in the state the next one
// starts from.
func itRestoresTheElasticsearchCluster(cluster *v1.CamundaCluster, elasticsearch, storageConfig string) {
	var (
		// backup is the completed backup that the restore reads.
		backup v1.LogicalBackupElasticsearch
		// contract is the storage contract of the target, the way this suite
		// reaches its Elasticsearch.
		contract v1.SecondaryStorageConfig
		// patterns are the index patterns that the restore replaces.
		patterns []string
		// claims are the broker volumes as they were bound before the wipe.
		claims map[string]corev1.PersistentVolumeClaim
	)

	It("completes the LogicalBackupElasticsearch that the restore reads", func() {
		By("creating the LogicalBackupElasticsearch")
		Expect(apply(&v1.LogicalBackupElasticsearch{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1.GroupVersion.String(),
				Kind:       "LogicalBackupElasticsearch",
			},
			ObjectMeta: metav1.ObjectMeta{Name: esRestoreBackup, Namespace: cluster.Namespace},
			Spec: v1.LogicalBackupElasticsearchSpec{
				ClusterRef: v1.ClusterRef{Name: cluster.Name},
			},
		})).To(Succeed())

		By("waiting for Ready Completed")
		Eventually(func(g Gomega) {
			expectReady(g, lbesResource, esRestoreBackup, cluster.Namespace, v1.ReasonCompleted)
		}, backupTimeout, 5*time.Second).Should(Succeed())

		By("reading the identity of the backup set")
		Expect(utils.Get(lbesResource, esRestoreBackup, cluster.Namespace, &backup)).To(Succeed())
		Expect(backup.Status.BackupID).NotTo(BeZero())
		Expect(backup.Status.Repository).To(Equal(elasticsearch))
		Expect(backup.Status.HistorySnapshots).NotTo(BeEmpty())
	})

	// A user never does this. The spec erases the state of the cluster to
	// prove that the restore rebuilds it from the backup alone, and only a
	// suspended cluster loses its storage without a workload writing over the
	// gap. The suspension is given back before the restore is created, so the
	// restore is what suspends the cluster from then on.
	It("suspends the cluster so the spec can erase its state", func() {
		claims = brokerClaims(cluster)
		Expect(claims).NotTo(BeEmpty(), "the cluster has no broker volume")

		suspend(cluster)
	})

	It("erases the Camunda indices and the broker volumes of the cluster", func() {
		By("reading the storage contract of the cluster")
		Expect(utils.Get(sscResource, storageConfig, cluster.Namespace, &contract)).To(Succeed())
		Expect(contract.Spec.Elasticsearch).NotTo(BeNil())

		// The patterns are the ones the restore itself replaces, so the wipe
		// erases exactly what the restore puts back.
		patterns = logicalbackup.CamundaIndexPatterns(
			logicalbackup.HasOptimizeSnapshot(backup.Status.HistorySnapshots),
		)

		By("deleting every Camunda index of the target")
		names, err := resolveIndices(&contract, patterns)
		Expect(err).NotTo(HaveOccurred())
		Expect(names).NotTo(BeEmpty(), "the cluster holds no Camunda index to erase")
		Expect(deleteIndices(&contract, names)).To(Succeed())

		By("deleting the broker volumes")
		for name := range claims {
			_, err := utils.Kubectl("delete", "pvc", name, "-n", cluster.Namespace, "--wait=false")
			Expect(err).NotTo(HaveOccurred())
		}

		By("checking that nothing of the cluster state is left")
		names, err = resolveIndices(&contract, patterns)
		Expect(err).NotTo(HaveOccurred())
		Expect(names).To(BeEmpty(), "the target still holds Camunda indices")
		Eventually(func(g Gomega) {
			for name := range claims {
				expectGone(g, "pvc", name, cluster.Namespace)
			}
		}, 3*time.Minute, 5*time.Second).Should(Succeed())
	})

	It("brings the cluster back from the backup alone", func() {
		letTheRestoreTakeOver(cluster)

		By("creating the LogicalRestoreElasticsearch")
		Expect(apply(&v1.LogicalRestoreElasticsearch{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1.GroupVersion.String(),
				Kind:       "LogicalRestoreElasticsearch",
			},
			ObjectMeta: metav1.ObjectMeta{Name: esRestore, Namespace: cluster.Namespace},
			Spec: v1.LogicalRestoreElasticsearchSpec{
				BackupRef:        v1.LogicalBackupRef{Name: esRestoreBackup},
				TargetClusterRef: v1.ClusterRef{Name: cluster.Name},
			},
		})).To(Succeed())

		// The Jobs of a completed restore are gone by the time the wait below
		// returns, and they carry the backup id. The watch starts here, before
		// anything waits, so it sees them while they run.
		jobs := watchRestoreJobs(
			lresResource, esRestore, cluster.Namespace, labels.LogicalRestoreElasticsearch(esRestore),
		)

		expectRestoreCompleted(
			lresResource, esRestore, cluster.Namespace, string(v1.LogicalRestoreFailed),
		)

		By("reading what the restore recorded")
		var restored v1.LogicalRestoreElasticsearch
		Expect(utils.Get(lresResource, esRestore, cluster.Namespace, &restored)).To(Succeed())
		Expect(restored.Status.Phase).To(Equal(v1.LogicalRestoreCompleted))
		Expect(restored.Status.BackupID).To(Equal(backup.Status.BackupID))
		Expect(restored.Status.Repository).To(Equal(backup.Status.Repository))
		Expect(restored.Status.RestoredSnapshots).To(ContainElements(
			toAny(backup.Status.HistorySnapshots)...,
		))
		Expect(restored.Status.RestoredSnapshots).To(ContainElement(
			logicalbackup.RecordsSnapshotName(backup.Status.BackupID),
		))
		Expect(restored.Status.RecreatedClaims).To(ConsistOf(toAny(slices.Sorted(maps.Keys(claims)))...))

		By("finding the Camunda indices in Elasticsearch again")
		names, err := resolveIndices(&contract, patterns)
		Expect(err).NotTo(HaveOccurred())
		Expect(names).NotTo(BeEmpty())

		By("running the restore application once per broker with the backup id")
		jobs.expect(
			len(claims), ConsistOf("--backupId="+strconv.FormatInt(backup.Status.BackupID, 10)),
		)
	})

	It("finds the process instance of the backup after the cluster runs again", func() {
		expectUnsuspended(cluster)

		By("searching the instance that was started before the backup")
		expectInstanceSearchable(cluster)
	})

	It("collects its restore Jobs when it completes", func() {
		expectRestoreCollectedItsJobs(
			cluster.Namespace, lresResource, esRestore, labels.LogicalRestoreElasticsearch(esRestore),
		)
	})
}

// itRestoresTheRelationalCluster registers the restore specs of the RDBMS
// flow. It is the same round trip as the Elasticsearch flow, over the other
// storage path: the secondary store is a logical database, and the wipe drops
// the schema that holds the Camunda tables.
func itRestoresTheRelationalCluster(cluster *v1.CamundaCluster) {
	var (
		// backup is the completed backup that the restore reads.
		backup v1.LogicalBackupRDBMS
		// credentials are the application credentials of the logical database,
		// the role that owns the database and every object in it.
		credentials v1.CredentialsSecretRef
		// claims are the broker volumes as they were bound before the wipe.
		claims map[string]corev1.PersistentVolumeClaim
	)

	It("completes the LogicalBackupRDBMS that the restore reads", func() {
		By("creating the LogicalBackupRDBMS")
		Expect(apply(&v1.LogicalBackupRDBMS{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1.GroupVersion.String(),
				Kind:       "LogicalBackupRDBMS",
			},
			ObjectMeta: metav1.ObjectMeta{Name: rdbmsRestoreBackup, Namespace: cluster.Namespace},
			Spec: v1.LogicalBackupRDBMSSpec{
				ClusterRef: v1.ClusterRef{Name: cluster.Name},
			},
		})).To(Succeed())

		By("waiting for Ready Completed")
		Eventually(func(g Gomega) {
			expectReady(g, lbrdbmsResource, rdbmsRestoreBackup, cluster.Namespace, v1.ReasonCompleted)
		}, backupTimeout, 5*time.Second).Should(Succeed())

		By("reading the identity of the backup")
		Expect(utils.Get(lbrdbmsResource, rdbmsRestoreBackup, cluster.Namespace, &backup)).To(Succeed())
		Expect(backup.Status.BackupID).NotTo(BeZero())
		Expect(backup.Status.ObjectKey).NotTo(BeEmpty())
	})

	// A user never does this. The spec erases the state of the cluster to
	// prove that the restore rebuilds it from the backup alone, and only a
	// suspended cluster loses its storage without a workload writing over the
	// gap. The suspension is given back before the restore is created, so the
	// restore is what suspends the cluster from then on.
	It("suspends the cluster so the spec can erase its state", func() {
		claims = brokerClaims(cluster)
		Expect(claims).NotTo(BeEmpty(), "the cluster has no broker volume")

		suspend(cluster)
	})

	It("erases the schema and the broker volumes of the cluster", func() {
		credentials = applicationCredentials(cluster)

		// The schema goes as the application role, which owns the database. On
		// PostgreSQL 15 and later the schema public belongs to
		// pg_database_owner, so the new schema belongs to that same role, which
		// is the role the restore Job connects as.
		By("dropping the schema of the logical database as the application role")
		_, err := psql(
			cluster.Namespace, "wipe", credentials,
			"DROP SCHEMA public CASCADE; CREATE SCHEMA public;",
		)
		Expect(err).NotTo(HaveOccurred())

		By("deleting the broker volumes")
		for name := range claims {
			_, err := utils.Kubectl("delete", "pvc", name, "-n", cluster.Namespace, "--wait=false")
			Expect(err).NotTo(HaveOccurred())
		}

		By("checking that nothing of the cluster state is left")
		out, err := psql(cluster.Namespace, "wiped", credentials, publicTableCount)
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(out)).To(Equal("0"), "the schema still holds tables")
		Eventually(func(g Gomega) {
			for name := range claims {
				expectGone(g, "pvc", name, cluster.Namespace)
			}
		}, 3*time.Minute, 5*time.Second).Should(Succeed())
	})

	It("brings the cluster back from the backup alone", func() {
		letTheRestoreTakeOver(cluster)

		By("creating the LogicalRestoreRDBMS")
		Expect(apply(&v1.LogicalRestoreRDBMS{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1.GroupVersion.String(),
				Kind:       "LogicalRestoreRDBMS",
			},
			ObjectMeta: metav1.ObjectMeta{Name: rdbmsRestore, Namespace: cluster.Namespace},
			Spec: v1.LogicalRestoreRDBMSSpec{
				BackupRef:        v1.LogicalBackupRef{Name: rdbmsRestoreBackup},
				TargetClusterRef: v1.ClusterRef{Name: cluster.Name},
			},
		})).To(Succeed())

		// The Jobs of a completed restore are gone by the time the wait below
		// returns. The watch starts here, before anything waits, so it sees
		// them while they run.
		jobs := watchRestoreJobs(
			lrrdbmsResource, rdbmsRestore, cluster.Namespace, labels.LogicalRestoreRDBMS(rdbmsRestore),
		)

		expectRestoreCompleted(
			lrrdbmsResource, rdbmsRestore, cluster.Namespace, string(v1.LogicalRestoreFailed),
		)

		By("reading what the restore recorded")
		var restored v1.LogicalRestoreRDBMS
		Expect(utils.Get(lrrdbmsResource, rdbmsRestore, cluster.Namespace, &restored)).To(Succeed())
		Expect(restored.Status.Phase).To(Equal(v1.LogicalRestoreCompleted))
		Expect(restored.Status.BackupID).NotTo(BeZero())
		Expect(restored.Status.SecondaryJobName).NotTo(BeEmpty())
		Expect(restored.Status.RecreatedClaims).To(ConsistOf(toAny(slices.Sorted(maps.Keys(claims)))...))

		By("finding the Camunda tables in the logical database again")
		out, err := psql(cluster.Namespace, "restored", credentials, publicTableCount)
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(out)).NotTo(Equal("0"), "the schema holds no table")

		// The restore application reads the exporter position from the restored
		// database and picks the primary-storage backups itself, so it takes no
		// argument on this path.
		By("running the restore application once per broker without an argument")
		jobs.expect(len(claims), BeEmpty())
	})

	It("finds the process instance of the backup after the cluster runs again", func() {
		expectUnsuspended(cluster)

		By("searching the instance that was started before the backup")
		expectInstanceSearchable(cluster)
	})

	It("collects its restore Jobs when it completes", func() {
		expectRestoreCollectedItsJobs(
			cluster.Namespace, lrrdbmsResource, rdbmsRestore, labels.LogicalRestoreRDBMS(rdbmsRestore),
		)
	})
}

// itRefusesAPointInTimeRestoreOfAnUnrestoredDatabase registers the specs that
// prove the guarantee of a point-in-time restore: a database that nobody
// rolled back holds the restore, and the restore touches no broker volume
// while it waits.
//
// The cluster is suspended by hand here, and it stays that way for the
// point-in-time specs that follow. That is the one flow where the spec keeps
// the suspension: it rolls the database back by hand, and brokers that run
// would export past that point again at once. The restore therefore records
// no suspension of its own, and it unsuspends nothing at the end.
func itRefusesAPointInTimeRestoreOfAnUnrestoredDatabase(cluster *v1.CamundaCluster) {
	// claims are the broker volumes as the restore found them.
	var claims map[string]corev1.PersistentVolumeClaim

	It("declares point-in-time recovery on the database server", func() {
		By("setting spec.pitr on the DatabaseServerConfig")
		_, err := utils.Kubectl(
			"patch", dbServerResource, ccRDBMSServer, "-n", ccRDBMSNamespace, "--type=merge",
			"-p", fmt.Sprintf(
				`{"spec":{"pitr":{"enabled":true,"retentionPeriodDays":%d}}}`, pitrRetentionDays,
			),
		)
		Expect(err).NotTo(HaveOccurred())

		// The edit raises the generation of the server. A restore that reads
		// the server before its controller caught up sees a contract that
		// nothing validated.
		By("waiting for the server to report the new spec as Ready Healthy")
		Eventually(func(g Gomega) {
			var server v1.DatabaseServerConfig
			g.Expect(utils.Get(dbServerResource, ccRDBMSServer, ccRDBMSNamespace, &server)).To(Succeed())
			g.Expect(server.Spec.PITR).NotTo(BeNil())
			g.Expect(server.Status.ObservedGeneration).To(Equal(server.Generation))
			expectReady(g, dbServerResource, ccRDBMSServer, ccRDBMSNamespace, v1.ReasonHealthy)
		}, 3*time.Minute, 5*time.Second).Should(Succeed())
	})

	It("holds in Pending while the database is ahead of the requested point", func() {
		suspend(cluster)
		claims = brokerClaims(cluster)
		Expect(claims).NotTo(BeEmpty(), "the cluster has no broker volume")

		By("creating a PointInTimeRestore for a point the database is ahead of")
		Expect(apply(&v1.PointInTimeRestore{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1.GroupVersion.String(),
				Kind:       "PointInTimeRestore",
			},
			ObjectMeta: metav1.ObjectMeta{Name: pitrRefused, Namespace: cluster.Namespace},
			Spec: v1.PointInTimeRestoreSpec{
				ClusterRef: v1.ClusterRef{Name: cluster.Name},
				Timestamp:  metav1.NewTime(time.Now().UTC().Add(-pitrRefusedAge)),
			},
		})).To(Succeed())

		By("waiting for the DatabaseNotRestored hold")
		Eventually(func(g Gomega) {
			expectPhase(
				g, pitrResource, pitrRefused, cluster.Namespace, string(v1.PointInTimeRestorePending),
			)
			expectConditionFalse(
				g, pitrResource, pitrRefused, cluster.Namespace,
				v1.ConditionReady, v1.ReasonDatabaseNotRestored,
			)
		}, 5*time.Minute, 5*time.Second).Should(Succeed())

		By("checking that the hold stands")
		Consistently(func(g Gomega) {
			expectPhase(
				g, pitrResource, pitrRefused, cluster.Namespace, string(v1.PointInTimeRestorePending),
			)
			expectConditionFalse(
				g, pitrResource, pitrRefused, cluster.Namespace,
				v1.ConditionReady, v1.ReasonDatabaseNotRestored,
			)
		}, pitrHold, 5*time.Second).Should(Succeed())
	})

	It("leaves the broker volumes and the primary storage untouched", func() {
		By("reading the broker volumes again")
		current := brokerClaims(cluster)
		Expect(slices.Sorted(maps.Keys(current))).To(Equal(slices.Sorted(maps.Keys(claims))))
		for name, before := range claims {
			Expect(current[name].UID).To(Equal(before.UID), "broker volume %q was replaced", name)
			Expect(current[name].CreationTimestamp).To(
				Equal(before.CreationTimestamp), "broker volume %q was created again", name,
			)
		}

		By("checking that the restore ran no Job")
		jobs, err := restoreJobs(cluster.Namespace, labels.PointInTimeRestore(pitrRefused))
		Expect(err).NotTo(HaveOccurred())
		Expect(jobs).To(BeEmpty())

		var refused v1.PointInTimeRestore
		Expect(utils.Get(pitrResource, pitrRefused, cluster.Namespace, &refused)).To(Succeed())
		Expect(refused.Status.RecreatedClaims).To(BeEmpty())
		Expect(refused.Status.PrimaryJobNames).To(BeEmpty())
		Expect(refused.Status.ObservedPositions).NotTo(
			BeEmpty(), "the restore read no exporter position at all",
		)
	})

	// A restore holds the claim on its cluster from admission on, and
	// admission takes it before it reads the database. The refused restore
	// therefore holds the cluster for as long as it exists, and the next
	// restore of this flow would report ClusterClaimed.
	It("gives the cluster claim back when it is deleted", func() {
		_, err := utils.Kubectl(
			"delete", pitrResource, pitrRefused, "-n", cluster.Namespace, "--wait=false",
		)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func(g Gomega) {
			expectGone(g, pitrResource, pitrRefused, cluster.Namespace)
		}, 3*time.Minute, 5*time.Second).Should(Succeed())
	})
}

// itRunsAPointInTimeRestoreAtTheCurrentDatabaseState registers the specs of a
// point-in-time restore that the database state admits. The database of the
// suspended cluster holds no state after the requested point, which is what a
// caller who restored the server to that point presents. The operator never
// restores the database itself, so this is the whole of its side.
//
// This is also the second restore of this cluster. The LogicalRestoreRDBMS of
// the earlier flow is still there, and no spec deleted it. Its Jobs are
// collected, so its pods hold no broker volume and this restore can empty
// them.
func itRunsAPointInTimeRestoreAtTheCurrentDatabaseState(cluster *v1.CamundaCluster) {
	// at is the point the restore asks for. The cluster is suspended, so
	// nothing exports after it.
	var at time.Time

	// jobs keeps a copy of the restore Jobs while they run. The restore
	// removes them when it completes, and the spec that reads their arguments
	// runs after that.
	var jobs *restoreJobWatch

	It("runs against a cluster whose earlier restore is still in place", func() {
		exists, err := utils.Exists(lrrdbmsResource, rdbmsRestore, cluster.Namespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(exists).To(
			BeTrue(),
			"LogicalRestoreRDBMS %q is gone, so this flow proves no sequence of two restores",
			rdbmsRestore,
		)
	})

	// The operator never rolls a database back. It reads the exporter position
	// of a database that somebody else already restored, and it aligns the
	// broker volumes with that position. Nothing archives a write-ahead log in
	// kind, so this spec plays that other party and rolls the database back.
	//
	// The rollback is what makes the point restorable, not a convenience. The
	// restore application takes a primary-storage checkpoint whose log covers
	// the exporter position of the database, first position and last position
	// both. The brokers of a running cluster export past the last position of
	// every checkpoint that exists, so a live database is always outside that
	// window. rolledBackPosition is the first position of the log, which the
	// earliest checkpoint of the cluster always covers.
	It("presents a database that somebody rolled back to the requested point", func() {
		at = time.Now().UTC().Truncate(time.Second)

		By("rolling the logical database back to the start of the log")
		_, err := psql(
			cluster.Namespace, "rollback", applicationCredentials(cluster),
			fmt.Sprintf(rollBackSQL, rolledBackPosition, at.Format(sqlTimestampLayout)),
		)
		Expect(err).NotTo(HaveOccurred())
	})

	It("reaches the broker volumes with the database at the requested point", func() {
		By("creating a PointInTimeRestore for the point the database holds")
		Expect(apply(&v1.PointInTimeRestore{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1.GroupVersion.String(),
				Kind:       "PointInTimeRestore",
			},
			ObjectMeta: metav1.ObjectMeta{Name: pitrCurrent, Namespace: cluster.Namespace},
			Spec: v1.PointInTimeRestoreSpec{
				ClusterRef: v1.ClusterRef{Name: cluster.Name},
				Timestamp:  metav1.NewTime(at),
			},
		})).To(Succeed())

		jobs = watchRestoreJobs(
			pitrResource, pitrCurrent, cluster.Namespace, labels.PointInTimeRestore(pitrCurrent),
		)

		// ValidatingDatabaseState is visible only while the database is
		// unreachable, so the restore is past the check once it reports either
		// of the phases that follow it.
		By("waiting for the restore to pass the database-state check")
		Eventually(func(g Gomega) {
			var running v1.PointInTimeRestore
			g.Expect(utils.Get(pitrResource, pitrCurrent, cluster.Namespace, &running)).To(Succeed())
			g.Expect(running.Status.Phase).To(BeElementOf(
				v1.PointInTimeRestoreRestoringPrimaryStorage,
				v1.PointInTimeRestoreCompleted,
			), running.Status.FailureMessage)
			g.Expect(running.Status.ObservedPositions).NotTo(BeEmpty())
			g.Expect(running.Status.Storage).NotTo(BeNil())
		}, 5*time.Minute, 5*time.Second).Should(Succeed())
	})

	It("runs the restore application with the requested point once per broker", func() {
		expectRestoreCompleted(
			pitrResource, pitrCurrent, cluster.Namespace, string(v1.PointInTimeRestoreFailed),
		)

		jobs.expect(len(brokerClaims(cluster)), ConsistOf("--to="+at.Format(time.RFC3339)))
	})

	// The spec suspended this cluster by hand, so the restore recorded no
	// suspension of its own and withdrew none. The spec gives it back.
	//
	// The rollback erased the instance from the database. The exporter
	// resumes from the rolled-back position and writes it again, so the
	// search proves the export after the restore, and not a row that
	// survived.
	It("converges when the cluster runs again", func() {
		unsuspend(cluster)

		By("searching the instance that the exporter writes again")
		expectInstanceSearchable(cluster)
	})
}

// itFailsAPointInTimeRestoreAheadOfEveryBackup registers the specs that
// provoke the one refusal of the restore application that the operator names,
// against the real Camunda image. The operator recognizes that refusal by a
// line in the log of the failed Job. Only a real run can tell whether the line
// still reads the way the operator expects: the unit tests of the match feed
// it their own copy of the line.
//
// The refusal needs a database whose exporter position lies past the last
// position of every primary-storage backup. That is the state of a cluster
// that exported after its last backup. The specs therefore export once more,
// and they skip the rollback that the restore before them did.
//
// The refusal arrives after the broker volumes are erased, so the specs leave
// the cluster suspended and without primary storage. They run last in their
// flow.
func itFailsAPointInTimeRestoreAheadOfEveryBackup(cluster *v1.CamundaCluster) {
	It("exports a record past every primary-storage backup", func() {
		key := startInstance(cluster)
		expectInstanceExported(cluster, key)
	})

	It("reaches Failed with reason ExporterPositionNotCovered", func() {
		suspend(cluster)

		By("creating a PointInTimeRestore for the point the live database holds")
		Expect(apply(&v1.PointInTimeRestore{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1.GroupVersion.String(),
				Kind:       "PointInTimeRestore",
			},
			ObjectMeta: metav1.ObjectMeta{Name: pitrUncovered, Namespace: cluster.Namespace},
			Spec: v1.PointInTimeRestoreSpec{
				ClusterRef: v1.ClusterRef{Name: cluster.Name},
				Timestamp:  metav1.Now(),
			},
		})).To(Succeed())

		// Both terminal phases end the wait. A restore that completes proves
		// that the database was not ahead of every backup, and a restore that
		// fails for another reason is the generic fallback. Waiting longer
		// changes neither.
		By("waiting for a terminal phase")
		var terminal v1.PointInTimeRestore
		Eventually(func(g Gomega) {
			g.Expect(utils.Get(pitrResource, pitrUncovered, cluster.Namespace, &terminal)).To(Succeed())
			g.Expect(terminal.Status.Phase).To(BeElementOf(
				v1.PointInTimeRestoreFailed,
				v1.PointInTimeRestoreCompleted,
			), terminal.Status.FailureMessage)
		}, restoreTimeout, 5*time.Second).Should(Succeed())

		Expect(terminal.Status.Phase).To(
			Equal(v1.PointInTimeRestoreFailed),
			"the restore completed, so the database was not ahead of every backup and the restore "+
				"application refused nothing",
		)

		ready := meta.FindStatusCondition(terminal.Status.Conditions, v1.ConditionReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Reason).To(
			Equal(v1.ReasonExporterPositionNotCovered),
			"the operator did not recognize the refusal in the Job log and reported the generic "+
				"failure instead: %s",
			terminal.Status.FailureMessage,
		)
	})
}

// itRunsAPointInTimeRestoreThroughTheDatabaseServer registers the specs of the
// restore a user gets when a DatabaseServer runs the server: they create one
// resource, and the operator rolls the database back, aligns the brokers, and
// starts the cluster again.
//
// The specs write on both sides of one point. A restore brings the cluster
// back at the first primary-storage backup taken at or after that point. Zeebe
// then re-exports every record up to the checkpoint of that backup. kept ran
// before the point and comes back. lost starts after that backup. It is in
// neither the recovered database nor the restored Zeebe log, so it never
// comes back.
//
// A marker checkpoint between the point and a write does not keep that write
// out. One CI run wrote a marker there, and the write came back with the
// backup that followed it.
//
// Nothing here suspends the cluster. The restore suspends it, recovers the
// server under a new name, restores the broker volumes, and withdraws its own
// suspension.
func itRunsAPointInTimeRestoreThroughTheDatabaseServer(cluster *v1.CamundaCluster, server string) {
	var (
		// at is the point the restore asks for.
		at time.Time
		// kept ran before at and survives the restore. lost ran after it and
		// does not.
		kept, lost string
		// requestID is the uid of the restore, which is what it writes on the
		// contract to tell its request from an earlier one.
		requestID string
		// pinnedIdentifier is the identity of the PostgreSQL instance that the
		// restore pins at admission. A recovery restores the pg_control of the
		// base backup it reads, so the recovered instance reports this same
		// value and only the endpoint moves.
		pinnedIdentifier string
	)

	It("starts one process instance before the point and one after it", func() {
		kept = startInstance(cluster)
		expectInstanceExported(cluster, kept)

		at = time.Now().UTC().Truncate(time.Second)

		// Two reasons to wait for a backup past the point. The cluster comes
		// back at the first primary-storage backup taken at or after the
		// point, so lost has to start after that backup to stay gone. A point
		// that no backup reached yet also fails the restore, after it erased
		// the broker volumes.
		By("waiting until every partition holds a backup past the point")
		partitions := int(components.NewEffective(cluster.Spec).Partitions())
		Eventually(func(g Gomega) {
			backups, err := latestBackups(cluster)
			g.Expect(err).NotTo(HaveOccurred())

			var behind []int
			for partition := 1; partition <= partitions; partition++ {
				if taken, ok := backups[partition]; !ok || !taken.After(at) {
					behind = append(behind, partition)
				}
			}
			g.Expect(behind).To(
				BeEmpty(),
				"of the %d partitions, %v hold no backup past the point", partitions, behind,
			)
		}, pitrBackupCoverage, 10*time.Second).Should(Succeed())

		time.Sleep(pitrClockGap)

		lost = startInstance(cluster)
		expectInstanceExported(cluster, lost)
	})

	It("asks the server to roll itself back to the point", func() {
		// No wait here. The backup that covers the point is already taken. A
		// later backup carries lost back with it, and a restore created now is
		// unlikely to find one.
		By("reading the identity that the restore pins")
		var pinned v1.DatabaseServerConfig
		Expect(utils.Get(dbServerResource, server, cluster.Namespace, &pinned)).To(Succeed())
		pinnedIdentifier = pinned.Status.SystemIdentifier
		Expect(pinnedIdentifier).NotTo(BeEmpty())

		By("creating the PointInTimeRestore")
		Expect(apply(&v1.PointInTimeRestore{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1.GroupVersion.String(),
				Kind:       "PointInTimeRestore",
			},
			ObjectMeta: metav1.ObjectMeta{Name: pitrDatabaseServer, Namespace: cluster.Namespace},
			Spec: v1.PointInTimeRestoreSpec{
				ClusterRef: v1.ClusterRef{Name: cluster.Name},
				Timestamp:  metav1.NewTime(at),
			},
		})).To(Succeed())

		By("reading the request that the restore wrote on the contract")
		Eventually(func(g Gomega) {
			var restore v1.PointInTimeRestore
			g.Expect(utils.Get(pitrResource, pitrDatabaseServer, cluster.Namespace, &restore)).To(Succeed())
			// The request stays on the contract, but RestoringDatabase does
			// not: a recovery that finishes between two polls has already
			// left it.
			g.Expect(restore.Status.Phase).To(BeElementOf(
				v1.PointInTimeRestoreRestoringDatabase,
				v1.PointInTimeRestoreValidatingDatabaseState,
				v1.PointInTimeRestoreRestoringPrimaryStorage,
				v1.PointInTimeRestoreCompleted,
			), restore.Status.FailureMessage)
			requestID = string(restore.UID)

			var contract v1.DatabaseServerConfig
			g.Expect(utils.Get(dbServerResource, server, cluster.Namespace, &contract)).To(Succeed())
			g.Expect(contract.Spec.Recovery).NotTo(BeNil())
			g.Expect(contract.Spec.Recovery.RequestID).To(Equal(requestID))
			g.Expect(contract.Spec.Recovery.RequestedBy).To(
				Equal(cluster.Namespace + "/" + pitrDatabaseServer),
			)
			g.Expect(contract.Spec.Recovery.TargetTime).To(Equal(at.Format(time.RFC3339)))
		}, 5*time.Minute, 5*time.Second).Should(Succeed())
	})

	It("recovers the server into a new cluster and moves the contract to it", func() {
		By("waiting for the restore to leave the database recovery")
		Eventually(func(g Gomega) {
			var restore v1.PointInTimeRestore
			g.Expect(utils.Get(pitrResource, pitrDatabaseServer, cluster.Namespace, &restore)).To(Succeed())
			g.Expect(restore.Status.Phase).To(BeElementOf(
				v1.PointInTimeRestoreValidatingDatabaseState,
				v1.PointInTimeRestoreRestoringPrimaryStorage,
				v1.PointInTimeRestoreCompleted,
			), restore.Status.FailureMessage)
		}, databaseRecoveryTimeout, 5*time.Second).Should(Succeed())

		// The restore moves on from the answer the server published on the
		// contract. The server writes its own status at the end of that
		// reconcile, so a look right after the move can miss it.
		By("reading what the server recorded about the recovery")
		Eventually(func(g Gomega) {
			var current v1.DatabaseServer
			g.Expect(utils.Get(dsResource, server, cluster.Namespace, &current)).To(Succeed())
			g.Expect(current.Status.Cluster).To(Equal(server + "-r1"))
			g.Expect(current.Status.Recovery).NotTo(BeNil())
			g.Expect(current.Status.Recovery.RequestID).To(Equal(requestID))
			g.Expect(current.Status.Recovery.PreviousCluster).To(Equal(server))
			g.Expect(current.Status.Recovery.Result).To(Equal(v1.RecoveryResultCompleted))
			g.Expect(current.Status.SystemIdentifier).To(
				Equal(pinnedIdentifier), "the recovered instance reports another identity",
			)
			g.Expect(current.Status.Archive.History).NotTo(BeEmpty())
			g.Expect(current.Status.Archive.History[0].To).NotTo(
				BeNil(), "the archive of the superseded cluster is still open",
			)
		}, 2*time.Minute, 5*time.Second).Should(Succeed())

		By("keeping the identity the restore pinned, on the contract and in the record")
		Eventually(func(g Gomega) {
			var current v1.DatabaseServerConfig
			g.Expect(utils.Get(dbServerResource, server, cluster.Namespace, &current)).To(Succeed())
			g.Expect(current.Status.SystemIdentifier).To(Equal(pinnedIdentifier))

			var restore v1.PointInTimeRestore
			g.Expect(utils.Get(pitrResource, pitrDatabaseServer, cluster.Namespace, &restore)).To(Succeed())
			g.Expect(restore.Status.Storage).NotTo(BeNil())
			g.Expect(restore.Status.Storage.SystemIdentifier).To(Equal(pinnedIdentifier))
		}, 2*time.Minute, 5*time.Second).Should(Succeed())

		By("reading the contract that now names the recovered cluster")
		var contract v1.DatabaseServerConfig
		Expect(utils.Get(dbServerResource, server, cluster.Namespace, &contract)).To(Succeed())
		Expect(contract.Spec.Host).To(Equal(server + "-r1-rw." + cluster.Namespace + ".svc"))
		Expect(contract.Spec.AdminCredentialsSecretRef.Name).To(Equal(server + "-r1-superuser"))
		Expect(contract.Spec.PITR.LastRecovery).NotTo(BeNil())
		Expect(contract.Spec.PITR.LastRecovery.RequestID).To(Equal(requestID))
		Expect(contract.Spec.PITR.LastRecovery.Result).To(Equal(v1.RecoveryResultCompleted))
	})

	It("completes and leaves the cluster with the state of the point", func() {
		expectRestoreCompleted(
			pitrResource, pitrDatabaseServer, cluster.Namespace, string(v1.PointInTimeRestoreFailed),
		)

		expectUnsuspended(cluster)

		By("searching the instance that ran before the point")
		expectInstanceExported(cluster, kept)

		By("checking that the instance that ran after the point does not come back")
		Consistently(func(g Gomega) {
			expectInstanceGone(g, cluster, lost)
		}, pitrHold, 15*time.Second).Should(Succeed())
	})

	It("archives the recovered cluster and reports Ready Healthy again", func() {
		Eventually(func(g Gomega) {
			expectReady(g, dsResource, server, cluster.Namespace, v1.ReasonHealthy)

			var current v1.DatabaseServer
			g.Expect(utils.Get(dsResource, server, cluster.Namespace, &current)).To(Succeed())
			g.Expect(current.Status.Archive.History).To(HaveLen(2))
			g.Expect(current.Status.Archive.History[1].ServerName).To(Equal(server + "-r1"))
			g.Expect(current.Status.Archive.History[1].To).To(BeNil())
		}, dsReadyTimeout, 5*time.Second).Should(Succeed())
	})
}

// latestBackups returns the time of the newest primary-storage backup that
// each partition of cluster holds, by partition id. Camunda serves it on the
// runtime state endpoint of the broker management port:
// https://docs.camunda.io/docs/self-managed/operational-guides/backup-restore/zeebe-backup-and-restore/#request-runtime-state
//
// Zeebe numbers the partitions from one. The ids come from the answer, so a
// cluster of any partition count is read the same way, and a partition that
// holds no backup yet is absent from the map.
func latestBackups(cluster *v1.CamundaCluster) (map[int]time.Time, error) {
	url := fmt.Sprintf(
		"http://%s.%s.svc:%d/actuator/backupRuntime/state",
		components.WorkloadName(cluster, components.ComponentZeebe),
		cluster.Namespace,
		components.PortManagement,
	)

	out, err := utils.RunPod(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "curl-backup-state-" + utilrand.String(5),
			Namespace: cluster.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "curl",
				Image: utils.CurlImage,
				Args:  []string{"-fsS", url},
			}},
		},
	}, podTimeout)
	if err != nil {
		return nil, err
	}

	var state struct {
		BackupStates []struct {
			PartitionID int    `json:"partitionId"`
			Timestamp   string `json:"checkpointTimestamp"`
		} `json:"backupStates"`
	}
	if err := json.Unmarshal([]byte(out), &state); err != nil {
		return nil, fmt.Errorf(
			"decoding the backup state of %s/%s: %w", cluster.Namespace, cluster.Name, err,
		)
	}

	backups := map[int]time.Time{}
	for _, backup := range state.BackupStates {
		taken, err := utils.ParseCheckpointTime(backup.Timestamp)
		if err != nil {
			return nil, err
		}
		if current, ok := backups[backup.PartitionID]; !ok || taken.After(current) {
			backups[backup.PartitionID] = taken
		}
	}

	return backups, nil
}

// applicationCredentials returns the application credentials of the logical
// database of the cluster, resolved through the storage chain the way the
// operator resolves it: the contract of the cluster names the DatabaseConfig,
// and the DatabaseConfig names the Secret.
func applicationCredentials(cluster *v1.CamundaCluster) v1.CredentialsSecretRef {
	var contract v1.SecondaryStorageConfig
	Expect(utils.Get(sscResource, cluster.Spec.StorageRef, cluster.Namespace, &contract)).To(Succeed())
	Expect(contract.Spec.RDBMS).NotTo(BeNil())

	var config v1.DatabaseConfig
	Expect(utils.Get(
		dbConfigResource, contract.Spec.RDBMS.DatabaseConfigRef, cluster.Namespace, &config,
	)).To(Succeed())

	return config.Spec.CredentialsSecretRef
}

// expectRestoreCompleted waits until the restore reports Ready Completed.
//
// Failed is terminal, so a restore that reports it never reaches Completed and
// the wait ends there with the recorded message. Without that, a failed
// restore costs the whole timeout and reports a stale condition instead of the
// cause.
func expectRestoreCompleted(resource, name, namespace, failed string) {
	By("waiting for Ready Completed")
	Eventually(func(g Gomega) {
		var restored struct {
			Status struct {
				Phase          string `json:"phase"`
				FailureMessage string `json:"failureMessage"`
			} `json:"status"`
		}
		g.Expect(utils.Get(resource, name, namespace, &restored)).To(Succeed())
		if restored.Status.Phase == failed {
			StopTrying(fmt.Sprintf(
				"%s %q reached %s: %s", resource, name, failed, restored.Status.FailureMessage,
			)).Now()
		}

		expectReady(g, resource, name, namespace, v1.ReasonCompleted)
	}, restoreTimeout, 5*time.Second).Should(Succeed())
}

// expectRestoreCollectedItsJobs asserts that a restore which reached Completed
// removed its own Jobs, and that nobody had to delete the restore for it.
//
// A pod that completed still counts as a user of its volume under the
// kubernetes.io/pvc-protection finalizer. The Job pods of a restore mount the
// broker volumes, so a volume never terminates while such a pod exists.
// Whatever deletes the volume next then waits without end: another restore of
// the cluster, or the garbage collection of the cluster itself.
func expectRestoreCollectedItsJobs(namespace, resource, name string, owner labels.Owner) {
	By("keeping the finished restore in place")
	exists, err := utils.Exists(resource, name, namespace)
	Expect(err).NotTo(HaveOccurred())
	Expect(exists).To(
		BeTrue(), "%s %q is gone, so this spec proves nothing about its Jobs", resource, name,
	)

	By("waiting for its Jobs and their pods to be gone")
	Eventually(func(g Gomega) {
		jobs, err := restoreJobs(namespace, owner)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(jobs).To(BeEmpty())

		var pods corev1.PodList
		g.Expect(utils.List(
			"pods", namespace, k8slabels.SelectorFromSet(restore.JobSelector(owner)).String(), &pods,
		)).To(Succeed())
		g.Expect(pods.Items).To(BeEmpty())
	}, 3*time.Minute, 5*time.Second).Should(Succeed())
}

// restoreJobWatch keeps a copy of the restore Jobs of one restore while they
// exist.
//
// A restore that completes removes its Jobs, and a Job object is the only
// place the arguments of the restore application appear. A spec that reads the
// Jobs after the restore is terminal therefore finds nothing. The watch runs
// from the moment the restore is created, over the whole primary-storage run,
// and it keeps the first copy it sees of each Job.
//
// A sample every second is enough, and no Kubernetes watch is needed. A Job of
// a restore lives for as long as its pod schedules, starts a JVM, reads a
// primary-storage backup, and exits, which is tens of seconds at least. The
// operator also needs several reconciles before the first Job exists, and it
// creates every Job of the restore in one pass. expectRestoreCompleted then
// reads the phase on a five-second poll, so the suite cannot even see
// Completed before the samples below have run many times.
type restoreJobWatch struct {
	resource  string
	name      string
	namespace string
	owner     labels.Owner

	once sync.Once
	done chan struct{}
	// stopped closes when the goroutine has left its loop, so close can wait
	// for the last look to land before expect reads the record.
	stopped chan struct{}

	mu   sync.Mutex
	jobs map[string]batchv1.Job
}

// watchRestoreJobs starts the watch of one restore. The caller starts it right
// after it creates the restore, so the window is the whole run and not a phase
// edge. The watch stops when expect reads it, and after restoreTimeout in any
// case, so it never outlives the flow.
func watchRestoreJobs(resource, name, namespace string, owner labels.Owner) *restoreJobWatch {
	w := &restoreJobWatch{
		resource:  resource,
		name:      name,
		namespace: namespace,
		owner:     owner,
		done:      make(chan struct{}),
		stopped:   make(chan struct{}),
		jobs:      map[string]batchv1.Job{},
	}

	go func() {
		defer close(w.stopped)

		deadline := time.After(restoreTimeout)
		for {
			w.capture()

			select {
			case <-w.done:
				return
			case <-deadline:
				return
			case <-time.After(time.Second):
			}
		}
	}()

	return w
}

// capture records every restore Job that exists now and that the watch has not
// seen yet.
//
// It reports no error and it asserts nothing. A list that fails is a look that
// saw nothing, and it runs on a goroutine that no spec owns. expect is where a
// watch that saw too little fails.
func (w *restoreJobWatch) capture() {
	jobs, err := restoreJobs(w.namespace, w.owner)
	if err != nil {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	for _, job := range jobs {
		if _, seen := w.jobs[job.Name]; !seen {
			w.jobs[job.Name] = job
		}
	}
}

// close stops the watch and waits for its last look to land. It is safe to
// call more than once.
//
// The wait is what completes the record. A look that listed the Jobs before
// the stop still holds its result, and it writes that result after the list
// returns. expect reads the record right after this call, so without the wait
// the last look can land behind the read and the record can lack a Job that
// the watch really saw.
//
// The wait is bounded. It gives up after watchStopTimeout, because a kubectl
// call that never returns must not hold a spec open for ever.
func (w *restoreJobWatch) close() {
	w.once.Do(func() { close(w.done) })

	select {
	case <-w.stopped:
	case <-time.After(watchStopTimeout):
	}
}

// captured returns the Jobs the watch kept, by name.
func (w *restoreJobWatch) captured() map[string]batchv1.Job {
	w.mu.Lock()
	defer w.mu.Unlock()

	return maps.Clone(w.jobs)
}

// expect asserts that the restore ran the restore application once per broker,
// and that every Job carried the arguments of its path.
//
// The count comes from status.primaryJobNames. The operator records those
// names before it applies the Jobs and it never removes them, so the record
// answers after the restore is terminal, whatever happened to the Jobs
// themselves.
//
// The arguments come from the Jobs that the watch kept. The names it captured
// must be exactly the names the restore recorded, so a watch that missed a Job
// and a watch that never ran both fail here. A spec that asserts over an empty
// capture proves nothing about the restore application.
func (w *restoreJobWatch) expect(brokers int, args gomegatypes.GomegaMatcher) {
	Expect(w).NotTo(BeNil(), "no spec of this flow started a watch of the restore Jobs")

	// close waits for the last look, so the record is complete when it
	// returns. A look of its own here would only read the cluster again, after
	// the restore already collected its Jobs, and it would add a kubectl call
	// that nothing bounds.
	w.close()

	var recorded struct {
		Status struct {
			PrimaryJobNames []string `json:"primaryJobNames"`
		} `json:"status"`
	}
	Expect(utils.Get(w.resource, w.name, w.namespace, &recorded)).To(Succeed())
	Expect(recorded.Status.PrimaryJobNames).To(
		HaveLen(brokers),
		"%s %q recorded no restore Job for every one of its %d brokers", w.resource, w.name, brokers,
	)

	captured := w.captured()
	names := slices.Sorted(maps.Keys(captured))
	Expect(names).To(
		Equal(slices.Sorted(slices.Values(recorded.Status.PrimaryJobNames))),
		"the watch of %s %q kept %d of its %d restore Jobs. A restore that completes removes its "+
			"Jobs, so the arguments of the restore application can only be read from a Job that the "+
			"watch kept while the restore ran",
		w.resource, w.name, len(captured), len(recorded.Status.PrimaryJobNames),
	)

	for _, name := range names {
		containers := captured[name].Spec.Template.Spec.Containers
		Expect(containers).To(HaveLen(1), "Job %q runs more than the restore container", name)
		Expect(containers[0].Name).To(Equal(restore.ComponentRestore))
		Expect(containers[0].Command).To(ConsistOf(restore.RestoreEntrypoint))
		Expect(containers[0].Args).To(args, "Job %q", name)
	}
}

// restoreJobs returns the restore Jobs of one restore, in name order.
//
// The selector comes from pkg/restore. The owner label value is bounded to a
// DNS label, and a restore name can be longer, so a hand-built selector misses
// every Job of a restore with a long name.
func restoreJobs(namespace string, owner labels.Owner) ([]batchv1.Job, error) {
	selector := k8slabels.SelectorFromSet(restore.JobSelector(owner)).String()

	var jobs batchv1.JobList
	if err := utils.List("jobs", namespace, selector, &jobs); err != nil {
		return nil, err
	}

	slices.SortFunc(jobs.Items, func(a, b batchv1.Job) int { return strings.Compare(a.Name, b.Name) })

	return jobs.Items, nil
}

// resolveIndices returns the concrete names of the indices that patterns
// match, in name order. An empty answer means that the target holds none of
// them.
//
// A delete by pattern is not an option. Elasticsearch refuses a destructive
// call that names no concrete index since 8.0, because
// action.destructive_requires_name defaults to true. The names therefore come
// first, and the caller deletes those.
//
// expand_wildcards=open,closed matters. The get-index API expands a pattern to
// open indices alone, and a delete expands to open and closed ones, so the
// default would leave a closed index behind and report the target as empty.
func resolveIndices(contract *v1.SecondaryStorageConfig, patterns []string) ([]string, error) {
	path := "/" + strings.Join(patterns, ",") +
		"?ignore_unavailable=true&allow_no_indices=true&expand_wildcards=open,closed"

	out, err := curlElasticsearch(contract, "resolve-indices", path, "-g")
	if err != nil {
		return nil, err
	}

	var indices map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &indices); err != nil {
		return nil, fmt.Errorf("decoding the indices of %s: %w", strings.Join(patterns, ","), err)
	}

	return slices.Sorted(maps.Keys(indices)), nil
}

// deleteIndices deletes the named indices in one call. An empty list deletes
// nothing: an empty target names every index of the cluster.
func deleteIndices(contract *v1.SecondaryStorageConfig, names []string) error {
	if len(names) == 0 {
		return nil
	}

	_, err := curlElasticsearch(
		contract, "delete-indices", "/"+strings.Join(names, ","), "-g", "-X", "DELETE",
	)

	return err
}
