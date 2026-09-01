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

package logicalrestoreelasticsearch

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
)

var _ = Describe("LogicalRestoreElasticsearch of secondary storage", func() {
	It("registers the repository, empties the target, and restores every snapshot", func() {
		w := newWorld()
		backup := createBackup(w)
		w.seedSnapshots(elasticsearchSnapshots...)
		w.search.SetIndices(targetIndices...)
		w.search.SetRecoveryActive(true)

		restore := createRestore(w, backup.Name)

		By("registering the repository over the prefix that the backup wrote under")
		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.Repository).To(Equal(w.repository))
		}, timeout, interval).Should(Succeed())

		repository := w.search.Repository(w.repository)
		Expect(repository).NotTo(BeNil())
		Expect(w.search.RepositoryPuts(w.repository)).To(Equal(1), "the restore registered it once")
		Expect(repository.Type).To(Equal("s3"))
		Expect(repository.Settings).To(HaveKeyWithValue("bucket", "camunda-backups"))
		Expect(repository.Settings).To(HaveKeyWithValue(
			"base_path", logicalbackup.ClusterPrefix("clusters", w.esNamespace, w.esCluster),
		))

		By("deleting the Camunda indices of the target in one request, and keeping Optimize")
		Expect(w.search.IndexDeleteCalls()).To(Equal(1))
		Expect(w.search.DeletedIndices()).To(ConsistOf(targetIndices[:4]))
		Expect(w.search.Indices()).To(ConsistOf(targetIndices[4]))

		By("restoring every snapshot of the backup and recording them")
		wanted := append(
			append([]string{}, elasticsearchSnapshots...),
			logicalbackup.RecordsSnapshotName(backupID),
		)
		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.RestoredSnapshots).To(Equal(wanted))
		}, timeout, interval).Should(Succeed())

		restored := make([]string, 0, len(wanted))
		for _, request := range w.search.RestoreRequests() {
			Expect(request.Repo).To(Equal(w.repository))
			Expect(request.WaitForCompletion).To(BeFalse())
			restored = append(restored, request.Name)
		}
		Expect(restored).To(ConsistOf(wanted))

		By("waiting while Elasticsearch recovers the restored indices")
		w.search.SetIndices(targetIndices...)
		expectReason(restore, v1.LogicalRestoreRestoringSecondaryStorage, v1.ReasonProgressing)

		By("never deleting the indices a second time")
		Expect(w.search.IndexDeleteCalls()).To(Equal(1))

		By("moving on when no shard recovers any more")
		w.search.SetRecoveryActive(false)
		expectPhase(restore, v1.LogicalRestoreRestoringPrimaryStorage)
	})

	It("keeps a snapshot repository that the target already holds", func() {
		w := newWorld()
		backup := createBackup(w)
		w.seedSnapshots(elasticsearchSnapshots...)
		w.search.SetIndices(targetIndices...)
		w.search.SetRecoveryActive(true)

		// The Elasticsearch of a target can be a cluster that this operator
		// does not manage, where an operator registered the repository by
		// hand. Overwriting it would point it at another prefix of another
		// bucket.
		w.registerRepositoryAt("someone-elses/prefix")

		restore := createRestore(w, backup.Name)

		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.RestoredSnapshots).NotTo(BeEmpty())
		}, timeout, interval).Should(Succeed())

		Expect(w.search.RepositoryPuts(w.repository)).To(Equal(1), "only the hand registration")
		Expect(w.search.Repository(w.repository).Settings).To(
			HaveKeyWithValue("base_path", "someone-elses/prefix"),
		)
	})

	// A repository name that this operator did not produce carries no prefix.
	// Only the registration that somebody made by hand knows which prefix of
	// the bucket it reads, so a restore that cannot find it says so instead of
	// registering a prefix of its own over a name it does not own.
	It("reports a repository that it neither registered nor found", func() {
		w := newWorld()
		w.repository = "backups"
		backup := createBackup(w)
		w.seedSnapshots(elasticsearchSnapshots...)
		w.search.SetIndices(targetIndices...)

		restore := createRestore(w, backup.Name)

		held := expectReason(
			restore, v1.LogicalRestoreRestoringSecondaryStorage, v1.ReasonInvalidReference,
		)
		Expect(readyMessage(held)).To(ContainSubstring(`"backups"`))
		Expect(w.search.DeletedIndices()).To(BeEmpty(), "the destructive step never ran")
	})

	It("deletes the Optimize indices when the backup holds an Optimize snapshot", func() {
		w := newWorld()
		snapshots := append(append([]string{}, elasticsearchSnapshots...), optimizeSnapshot)
		backup := createBackup(w, func(b *v1.LogicalBackupElasticsearch) {
			b.Status.HistorySnapshots = snapshots
		})
		w.seedSnapshots(snapshots...)
		w.search.SetIndices(targetIndices...)
		w.search.SetRecoveryActive(true)

		restore := createRestore(w, backup.Name)

		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.RestoredSnapshots).To(HaveLen(len(snapshots) + 1))
		}, timeout, interval).Should(Succeed())

		Expect(w.search.DeletedIndices()).To(ConsistOf(targetIndices))
		Expect(w.search.Indices()).To(BeEmpty())
	})

	It("waits for the restored indices before it calls the restore finished", func() {
		w := newWorld()
		backup := createBackup(w)
		w.seedSnapshots(elasticsearchSnapshots...)
		// The target holds nothing, and no shard recovers. A restore that read
		// that as a finished recovery would walk on with empty secondary
		// storage.
		w.search.SetRecoveryActive(false)

		restore := createRestore(w, backup.Name)

		expectReason(restore, v1.LogicalRestoreRestoringSecondaryStorage, v1.ReasonProgressing)
		Consistently(func(g Gomega) {
			g.Expect(latest(g, restore).Status.Phase).To(
				Equal(v1.LogicalRestoreRestoringSecondaryStorage),
			)
		}, "2s", interval).Should(Succeed())

		By("moving on once the restored indices are there")
		w.search.SetIndices(targetIndices...)
		expectPhase(restore, v1.LogicalRestoreRestoringPrimaryStorage)
	})

	// The clock of the mid-run grace runs from the first outage of a started
	// restore. The indices of the target are gone once the delete succeeded.
	// A restore that measures a second full grace from a later outage holds an
	// erased cluster twice as long.
	It("keeps the mid-run grace of the first outage once the indices are deleted", func() {
		// outage is a failure count that outlasts the spec. The spec ends an
		// outage by setting the count back to zero.
		const outage = 1000

		w := newWorld()
		backup := createBackup(w)
		w.seedSnapshots(elasticsearchSnapshots...)
		w.search.SetIndices(targetIndices...)
		w.search.FailNext("snapshotRestore", outage)

		restore := createRestore(w, backup.Name)

		By("deleting the indices and then holding the failing snapshot restore")
		first := expectReason(
			restore, v1.LogicalRestoreRestoringSecondaryStorage, v1.ReasonConnectionFailed,
		)
		Expect(w.search.DeletedIndices()).NotTo(BeEmpty(), "the destructive step ran")
		Expect(first.Status.FirstFailedAt).NotTo(BeNil())
		firstFailedAt := *first.Status.FirstFailedAt

		By("keeping the clock when Elasticsearch answers again")
		w.search.FailNext("snapshotRestore", 0)
		Eventually(func(g Gomega) {
			recovered := latest(g, restore)
			g.Expect(recovered.Status.RestoredSnapshots).NotTo(BeEmpty())
			g.Expect(recovered.Status.FirstFailedAt).To(Equal(&firstFailedAt))
		}, timeout, interval).Should(Succeed())

		By("running on while the grace of the first outage passes")
		Consistently(func(g Gomega) {
			g.Expect(latest(g, restore).Status.Phase).To(
				Equal(v1.LogicalRestoreRestoringSecondaryStorage),
			)
		}, midRunGrace, interval).Should(Succeed())

		By("failing on the next outage, because that grace is spent")
		w.search.FailNext("indexResolve", outage)
		Eventually(func(g Gomega) {
			ended := latest(g, restore)
			g.Expect(ended.Status.Phase).To(Equal(v1.LogicalRestoreFailed), "Ready: %s", readyMessage(ended))
			g.Expect(ended.Status.FirstFailedAt).To(Equal(&firstFailedAt))
		}, midRunGrace-time.Second, interval).Should(Succeed())
	})

	It("holds an unreachable Elasticsearch and fails after the grace", func() {
		w := newWorld()
		backup := createBackup(w)
		w.seedSnapshots(elasticsearchSnapshots...)
		w.search.Close()

		restore := createRestore(w, backup.Name)

		expectReason(restore, v1.LogicalRestoreRestoringSecondaryStorage, v1.ReasonConnectionFailed)

		reached := expectReason(restore, v1.LogicalRestoreFailed, v1.ReasonFailed)
		Expect(reached.Status.FailureMessage).To(ContainSubstring("did not recover"))
	})
})
