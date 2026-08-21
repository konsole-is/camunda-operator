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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
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
	// holds.
	pitrRefused = "camunda-pitr-unrestored"
	pitrCurrent = "camunda-pitr-current"

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
		expectRestoreJobs(
			cluster.Namespace,
			labels.LogicalRestoreElasticsearch(esRestore),
			len(claims),
			ConsistOf("--backupId="+strconv.FormatInt(backup.Status.BackupID, 10)),
		)
	})

	It("finds the process instance of the backup after the cluster runs again", func() {
		expectUnsuspended(cluster)

		By("searching the instance that was started before the backup")
		expectInstanceSearchable(cluster)
	})

	It("releases the broker volumes when the finished restore is removed", func() {
		removeFinishedRestore(
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
		expectRestoreJobs(
			cluster.Namespace, labels.LogicalRestoreRDBMS(rdbmsRestore), len(claims), BeEmpty(),
		)
	})

	It("finds the process instance of the backup after the cluster runs again", func() {
		expectUnsuspended(cluster)

		By("searching the instance that was started before the backup")
		expectInstanceSearchable(cluster)
	})

	It("releases the broker volumes when the finished restore is removed", func() {
		removeFinishedRestore(
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
// the suspension: it rewinds the exporter position of the database by hand,
// and brokers that run would export past that position again at once. The
// restore therefore records no suspension of its own, and it unsuspends
// nothing at the end.
func itRefusesAPointInTimeRestoreOfAnUnrestoredDatabase(cluster *v1.CamundaCluster) {
	// claims are the broker volumes as the restore found them.
	var claims map[string]corev1.PersistentVolumeClaim

	It("declares point-in-time recovery on the database server", func() {
		By("setting spec.pitr on the DatabaseServerConfig")
		_, err := utils.Kubectl(
			"patch", dbServerResource, ccRDBMSServer, "--type=merge",
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
			g.Expect(utils.Get(dbServerResource, ccRDBMSServer, "", &server)).To(Succeed())
			g.Expect(server.Spec.PITR).NotTo(BeNil())
			g.Expect(server.Status.ObservedGeneration).To(Equal(server.Generation))
			expectReady(g, dbServerResource, ccRDBMSServer, "", v1.ReasonHealthy)
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
func itRunsAPointInTimeRestoreAtTheCurrentDatabaseState(cluster *v1.CamundaCluster) {
	// at is the point the restore asks for. The cluster is suspended, so
	// nothing exports after it.
	var at time.Time

	// The operator never rolls a database back. It reads the exporter position
	// of a database that somebody else already restored, and it aligns the
	// broker volumes with that position. Nothing archives a write-ahead log in
	// kind, so this spec plays that other party and rewinds the position.
	//
	// The rewind is what makes the point restorable, not a convenience. The
	// restore application takes a primary-storage checkpoint whose log covers
	// the exporter position of the database, first position and last position
	// both. The brokers of a running cluster export past the last position of
	// every checkpoint that exists, so a live database is always outside that
	// window. rolledBackPosition is the first position of the log, which the
	// earliest checkpoint of the cluster always covers.
	It("presents a database that somebody rolled back to the requested point", func() {
		at = time.Now().UTC().Truncate(time.Second)

		By("rewinding the exporter position of the logical database")
		_, err := psql(
			cluster.Namespace, "rollback", applicationCredentials(cluster),
			fmt.Sprintf(
				"UPDATE exporter_position SET last_exported_position = %d, last_updated = '%s'",
				rolledBackPosition, at.Format(sqlTimestampLayout),
			),
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

		expectRestoreJobs(
			cluster.Namespace,
			labels.PointInTimeRestore(pitrCurrent),
			len(brokerClaims(cluster)),
			ConsistOf("--to="+at.Format(time.RFC3339)),
		)
	})

	// The spec suspended this cluster by hand, so the restore recorded no
	// suspension of its own and withdrew none. The spec gives it back.
	It("converges when the cluster runs again", func() {
		unsuspend(cluster)

		By("searching the instance that the database still holds")
		expectInstanceSearchable(cluster)
	})
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

// removeFinishedRestore deletes a restore that reached a terminal phase and
// waits until its Jobs and their pods are gone.
//
// A pod that completed still counts as a user of its volume under the
// kubernetes.io/pvc-protection finalizer. The Job pods of a finished restore
// mount the broker volumes, so those volumes never terminate while the pods
// exist, and whatever deletes one next waits without end: another restore of
// the cluster, or the garbage collection of the cluster itself. A restore
// carries no finalizer, and its Jobs carry a controller reference to it, so
// deleting the restore takes the Jobs and their pods with it.
func removeFinishedRestore(namespace, resource, name string, owner labels.Owner) {
	By("deleting the finished restore")
	_, err := utils.Kubectl("delete", resource, name, "-n", namespace, "--wait=false")
	Expect(err).NotTo(HaveOccurred())

	By("waiting for its Jobs and their pods to be gone")
	Eventually(func(g Gomega) {
		expectGone(g, resource, name, namespace)

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

// expectRestoreJobs asserts that the restore ran the restore application once
// per broker, and that every Job carried the arguments of its path.
func expectRestoreJobs(
	namespace string,
	owner labels.Owner,
	brokers int,
	args gomegatypes.GomegaMatcher,
) {
	jobs, err := restoreJobs(namespace, owner)
	Expect(err).NotTo(HaveOccurred())
	Expect(jobs).To(HaveLen(brokers))

	for i := range jobs {
		containers := jobs[i].Spec.Template.Spec.Containers
		Expect(containers).To(HaveLen(1), "Job %q runs more than the restore container", jobs[i].Name)
		Expect(containers[0].Name).To(Equal(restore.ComponentRestore))
		Expect(containers[0].Command).To(ConsistOf(restore.RestoreEntrypoint))
		Expect(containers[0].Args).To(args, "Job %q", jobs[i].Name)
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
