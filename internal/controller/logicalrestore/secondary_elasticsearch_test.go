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

package logicalrestore

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
)

// targetIndices are the Camunda indices that the target holds before the
// restore. They are what the restore deletes, and the Optimize one is what it
// keeps unless the backup can put it back.
var targetIndices = []string{
	"camunda-list-view-8.9.0_",
	"operate-flownode-instance-8.9.0_",
	"tasklist-task-8.9.0_",
	"zeebe-record_process-instance_8.9.0_2026-08-20",
	"optimize-process-instance_8.9.0",
}

var _ = Describe("LogicalRestore of Elasticsearch secondary storage", func() {
	It("registers the repository, empties the target, and restores every snapshot", func() {
		w := newElasticsearchWorld()
		backup := createElasticsearchBackup(w)
		w.seedSnapshots(elasticsearchSnapshots...)
		w.search.SetIndices(targetIndices...)
		w.search.SetRecoveryActive(true)

		restore := createRestore(w, v1.LogicalBackupKindElasticsearch, backup.Name)

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
			"base_path", logicalbackup.ClusterPrefix("clusters", w.namespace, w.repository),
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
		w := newElasticsearchWorld()
		backup := createElasticsearchBackup(w)
		w.seedSnapshots(elasticsearchSnapshots...)
		w.search.SetIndices(targetIndices...)
		w.search.SetRecoveryActive(true)

		// The Elasticsearch of a target can be a cluster that this operator
		// does not manage, where an operator registered the repository by
		// hand. Overwriting it would point it at another prefix of another
		// bucket.
		w.registerRepositoryAt("someone-elses/prefix")

		restore := createRestore(w, v1.LogicalBackupKindElasticsearch, backup.Name)

		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.RestoredSnapshots).NotTo(BeEmpty())
		}, timeout, interval).Should(Succeed())

		Expect(w.search.RepositoryPuts(w.repository)).To(Equal(1), "only the hand registration")
		Expect(w.search.Repository(w.repository).Settings).To(
			HaveKeyWithValue("base_path", "someone-elses/prefix"),
		)
	})

	It("deletes the Optimize indices when the backup holds an Optimize snapshot", func() {
		w := newElasticsearchWorld()
		snapshots := append(append([]string{}, elasticsearchSnapshots...), optimizeSnapshot)
		backup := createElasticsearchBackup(w, func(b *v1.LogicalBackupElasticsearch) {
			b.Status.HistorySnapshots = snapshots
		})
		w.seedSnapshots(snapshots...)
		w.search.SetIndices(targetIndices...)
		w.search.SetRecoveryActive(true)

		restore := createRestore(w, v1.LogicalBackupKindElasticsearch, backup.Name)

		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.RestoredSnapshots).To(HaveLen(len(snapshots) + 1))
		}, timeout, interval).Should(Succeed())

		Expect(w.search.DeletedIndices()).To(ConsistOf(targetIndices))
		Expect(w.search.Indices()).To(BeEmpty())
	})

	It("waits for the restored indices before it calls the restore finished", func() {
		w := newElasticsearchWorld()
		backup := createElasticsearchBackup(w)
		w.seedSnapshots(elasticsearchSnapshots...)
		// The target holds nothing, and no shard recovers. A restore that read
		// that as a finished recovery would walk on with empty secondary
		// storage.
		w.search.SetRecoveryActive(false)

		restore := createRestore(w, v1.LogicalBackupKindElasticsearch, backup.Name)

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

	It("holds an unreachable Elasticsearch and fails after the grace", func() {
		w := newElasticsearchWorld()
		backup := createElasticsearchBackup(w)
		w.seedSnapshots(elasticsearchSnapshots...)
		w.search.Close()

		restore := createRestore(w, v1.LogicalBackupKindElasticsearch, backup.Name)

		expectReason(restore, v1.LogicalRestoreRestoringSecondaryStorage, v1.ReasonConnectionFailed)

		reached := expectReason(restore, v1.LogicalRestoreFailed, v1.ReasonFailed)
		Expect(reached.Status.FailureMessage).To(ContainSubstring("did not recover"))
	})
})
