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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	restorepkg "github.com/konsole-is/camunda-operator/pkg/restore"
)

var _ = Describe("LogicalRestoreElasticsearch admission", func() {
	It("holds a restore whose target still runs", func() {
		w := newWorld(func(cluster *v1.CamundaCluster) { cluster.Spec.Suspend = false })
		backup := createBackup(w)

		restore := createRestore(w, backup.Name)

		reached := expectReason(restore, v1.LogicalRestorePending, v1.ReasonClusterNotSuspended)
		Expect(reached.Status.BackupID).To(BeZero())

		By("touching nothing while it waits")
		Consistently(func(g Gomega) {
			g.Expect(w.search.RepositoryPuts(w.repository)).To(BeZero())
			g.Expect(w.search.IndexDeleteCalls()).To(BeZero())
			var jobs batchv1.JobList
			g.Expect(k8sClient.List(ctx, &jobs, client.InNamespace(w.namespace))).To(Succeed())
			g.Expect(jobs.Items).To(BeEmpty())
		}, "2s", interval).Should(Succeed())
	})

	It("holds a restore whose backup does not exist", func() {
		w := newWorld()

		restore := createRestore(w, "no-such-backup")

		expectReason(restore, v1.LogicalRestorePending, v1.ReasonInvalidReference)
	})

	It("holds a restore whose backup is not completed", func() {
		w := newWorld()
		backup := createBackup(w, func(b *v1.LogicalBackupElasticsearch) {
			b.Status.Phase = v1.LogicalBackupRunning
		})

		restore := createRestore(w, backup.Name)

		reached := expectReason(restore, v1.LogicalRestorePending, v1.ReasonInvalidReference)
		Expect(readyCondition(reached).Message).To(ContainSubstring("Completed backup"))
	})

	It("holds a restore whose target cluster does not exist", func() {
		w := newWorld()
		backup := createBackup(w)

		restore := &v1.LogicalRestoreElasticsearch{
			ObjectMeta: metav1.ObjectMeta{Name: "lres-no-cluster", Namespace: w.namespace},
			Spec: v1.LogicalRestoreElasticsearchSpec{
				BackupRef:        v1.LogicalBackupRef{Name: backup.Name},
				TargetClusterRef: v1.ClusterRef{Name: "no-such-cluster"},
			},
		}
		Expect(k8sClient.Create(ctx, restore)).To(Succeed())

		expectReason(restore, v1.LogicalRestorePending, v1.ReasonInvalidReference)
	})

	It("pins what it reads once the target is suspended and the backup is completed", func() {
		w := newWorld()
		backup := createBackup(w)

		restore := createRestore(w, backup.Name)

		Eventually(func(g Gomega) {
			current := latest(g, restore)
			g.Expect(current.Status.BackupID).To(Equal(backupID))
			g.Expect(current.Status.TargetClusterUID).To(Equal(w.cluster.UID))
		}, timeout, interval).Should(Succeed())
	})

	It("wakes a waiting restore through the cluster watch when the target is suspended", func() {
		w := newWorld(func(cluster *v1.CamundaCluster) { cluster.Spec.Suspend = false })
		backup := createBackup(w)

		restore := createRestore(w, backup.Name)
		expectReason(restore, v1.LogicalRestorePending, v1.ReasonClusterNotSuspended)

		By("suspending the target")
		w.suspend(true)

		// The window is shorter than the retry interval of the suite, so only
		// the watch can move the restore in time.
		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.BackupID).To(Equal(backupID))
		}, watchWindow, interval).Should(Succeed())
	})

	It("wakes a waiting restore through the backup watch when the backup completes", func() {
		w := newWorld()
		backup := createBackup(w, func(b *v1.LogicalBackupElasticsearch) {
			b.Status.Phase = v1.LogicalBackupRunning
		})

		restore := createRestore(w, backup.Name)
		expectReason(restore, v1.LogicalRestorePending, v1.ReasonInvalidReference)

		By("completing the backup")
		Eventually(func(g Gomega) {
			var current v1.LogicalBackupElasticsearch
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &current)).To(Succeed())
			current.Status.Phase = v1.LogicalBackupCompleted
			g.Expect(k8sClient.Status().Update(ctx, &current)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// The window is shorter than the retry interval of the suite, so only
		// the watch can move the restore in time.
		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.BackupID).To(Equal(backupID))
		}, watchWindow, interval).Should(Succeed())
	})
})

var _ = Describe("LogicalRestoreElasticsearch compatibility", func() {
	It("walks a compatible pair into the secondary storage phase", func() {
		w := newWorld()
		backup := createBackup(w)

		restore := createRestore(w, backup.Name)

		expectPhase(restore, v1.LogicalRestoreRestoringSecondaryStorage)
	})

	It("fails a pair whose partition counts differ", func() {
		w := newWorld()
		backup := createBackup(w, func(b *v1.LogicalBackupElasticsearch) {
			b.Status.PartitionsCount = worldPartitions + 4
		})

		restore := createRestore(w, backup.Name)

		reached := expectReason(restore, v1.LogicalRestoreFailed, v1.ReasonIncompatibleTarget)
		Expect(reached.Status.FailureMessage).To(ContainSubstring("7 partitions"))
		Expect(reached.Status.FailureMessage).To(ContainSubstring("runs 3"))
		Expect(reached.Status.CompletionTime).NotTo(BeNil())
	})

	It("fails a backup that recorded no Camunda version", func() {
		w := newWorld()
		backup := createBackup(w, func(b *v1.LogicalBackupElasticsearch) {
			b.Status.Version = ""
		})

		restore := createRestore(w, backup.Name)

		reached := expectReason(restore, v1.LogicalRestoreFailed, v1.ReasonIncompatibleTarget)
		Expect(reached.Status.FailureMessage).To(ContainSubstring("did not record"))
	})

	It("fails a backup that was taken with another patch version", func() {
		w := newWorld()
		backup := createBackup(w, func(b *v1.LogicalBackupElasticsearch) {
			b.Status.Version = "8.9.10"
		})

		restore := createRestore(w, backup.Name)

		reached := expectReason(restore, v1.LogicalRestoreFailed, v1.ReasonIncompatibleTarget)
		Expect(reached.Status.FailureMessage).To(ContainSubstring("exact version"))
	})

	It("fails a target that backs up through another bucket", func() {
		w := newWorld()
		backup := createBackup(w, func(b *v1.LogicalBackupElasticsearch) {
			b.Status.Storage.BucketRef = "another-bucket"
		})

		restore := createRestore(w, backup.Name)

		reached := expectReason(restore, v1.LogicalRestoreFailed, v1.ReasonIncompatibleTarget)
		Expect(reached.Status.FailureMessage).To(ContainSubstring("another-bucket"))
	})
})

var _ = Describe("LogicalRestoreElasticsearch of primary storage", func() {
	It("recreates the broker volumes, runs one Job per broker, and completes", func() {
		w := newWorld()
		backup := createBackup(w)
		restore := startedRestore(w, backup)

		By("recording the broker count and the recreated volumes")
		Eventually(func(g Gomega) {
			current := latest(g, restore)
			g.Expect(current.Status.Brokers).To(Equal(worldBrokers))
			g.Expect(current.Status.RecreatedClaims).To(ConsistOf(w.claimNames()))
		}, timeout, interval).Should(Succeed())

		By("running one restore Job per broker, labelled as the restore of this cluster")
		owner := labels.LogicalRestoreElasticsearch(restore.Name)
		names := []string{restorepkg.JobName(owner, 0), restorepkg.JobName(owner, 1)}
		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.PrimaryJobNames).To(Equal(names))
		}, timeout, interval).Should(Succeed())

		By("sizing every volume from the recorded restore size, with no owner reference")
		for _, name := range w.claimNames() {
			claim := claimNamed(w.namespace, name)
			Expect(claim.Spec.Resources.Requests[corev1.ResourceStorage]).To(
				Equal(resource.MustParse(worldRestoreSize)),
			)
			Expect(claim.OwnerReferences).To(BeEmpty())
		}

		job := jobNamed(w.namespace, names[0])
		Expect(job.Labels).To(HaveKeyWithValue("camunda.io/component", "restore"))
		Expect(job.Labels).To(HaveKeyWithValue("camunda.io/logical-restore-elasticsearch", restore.Name))
		Expect(job.Labels).To(HaveKeyWithValue("camunda.io/cluster", w.cluster.Name))
		Expect(job.Spec.Template.Spec.Containers[0].Args).To(
			ContainElement("--backupId=1772001869309"),
		)
		// envtest runs no garbage collector, so the controller reference is
		// what proves that deleting the restore removes its Jobs.
		Expect(job.OwnerReferences).To(HaveLen(1))
		Expect(job.OwnerReferences[0].Name).To(Equal(restore.Name))
		Expect(job.OwnerReferences[0].Controller).NotTo(BeNil())

		By("completing when every Job completed")
		for _, name := range names {
			markJob(w.namespace, name, batchv1.JobComplete)
		}
		reached := expectReason(restore, v1.LogicalRestoreCompleted, v1.ReasonCompleted)
		Expect(readyCondition(reached).Status).To(Equal(metav1.ConditionTrue))
		Expect(reached.Status.CompletionTime).NotTo(BeNil())

		By("leaving the recreated volumes behind when the restore is deleted")
		Expect(k8sClient.Delete(ctx, reached)).To(Succeed())
		Consistently(func(g Gomega) {
			for _, name := range w.claimNames() {
				g.Expect(k8sClient.Get(
					ctx, types.NamespacedName{Namespace: w.namespace, Name: name},
					&corev1.PersistentVolumeClaim{},
				)).To(Succeed())
			}
		}, "2s", interval).Should(Succeed())
	})

	// A Job that completed keeps its pod, and a pod that mounts a broker
	// volume holds that volume under the pvc-protection finalizer. The next
	// operation on the cluster then waits on that volume without end.
	It("removes its restore Jobs when it completes", func() {
		w := newWorld()
		backup := createBackup(w)
		restore := startedRestore(w, backup)

		owner := labels.LogicalRestoreElasticsearch(restore.Name)
		names := []string{restorepkg.JobName(owner, 0), restorepkg.JobName(owner, 1)}
		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.PrimaryJobNames).To(Equal(names))
		}, timeout, interval).Should(Succeed())

		for _, name := range names {
			markJob(w.namespace, name, batchv1.JobComplete)
		}
		expectReason(restore, v1.LogicalRestoreCompleted, v1.ReasonCompleted)

		expectJobsCollected(w.namespace, names)
	})

	// The logs of a failed Job are the diagnosis of the failure, and only the
	// pod keeps them readable. A failed restore therefore holds the broker
	// volumes until somebody deletes the restore.
	It("keeps its restore Jobs when it fails", func() {
		w := newWorld()
		backup := createBackup(w)
		restore := startedRestore(w, backup)

		owner := labels.LogicalRestoreElasticsearch(restore.Name)
		names := []string{restorepkg.JobName(owner, 0), restorepkg.JobName(owner, 1)}
		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.PrimaryJobNames).To(Equal(names))
		}, timeout, interval).Should(Succeed())

		markJob(w.namespace, names[1], batchv1.JobFailed)
		expectReason(restore, v1.LogicalRestoreFailed, v1.ReasonFailed)

		expectJobsKept(w.namespace, names)
	})

	It("sizes the volumes from the claim template when the backup recorded no size", func() {
		w := newWorld()
		backup := createBackup(w, func(b *v1.LogicalBackupElasticsearch) {
			b.Status.StorageSizes = v1.LogicalBackupStorageSizes{}
		})
		restore := startedRestore(w, backup)

		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.PrimaryJobNames).To(HaveLen(int(worldBrokers)))
		}, timeout, interval).Should(Succeed())

		claim := claimNamed(w.namespace, w.claimNames()[0])
		Expect(claim.Spec.Resources.Requests[corev1.ResourceStorage]).To(
			Equal(resource.MustParse(worldClaimSize)),
		)
	})

	It("reports a restore pod that cannot start and fails after the grace", func() {
		w := newWorld()
		backup := createBackup(w)
		restore := startedRestore(w, backup)

		owner := labels.LogicalRestoreElasticsearch(restore.Name)
		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.PrimaryJobNames).To(HaveLen(int(worldBrokers)))
		}, timeout, interval).Should(Succeed())

		By("standing in for a kubelet that cannot mount a Secret of the pod")
		stuckPod(w, restorepkg.JobName(owner, 0), restorepkg.JobSelector(owner))

		expectReason(restore, v1.LogicalRestoreRestoringPrimaryStorage, v1.ReasonMissingSecret)

		// The grace runs out only when the clock survives a look that resolved
		// every reference again. A restore that cleared it on each look would
		// hold here for ever.
		reached := expectReason(restore, v1.LogicalRestoreFailed, v1.ReasonFailed)
		Expect(reached.Status.FailureMessage).To(ContainSubstring("did not recover"))
	})

	It("stops when the target is unsuspended while the restore runs", func() {
		w := newWorld()
		backup := createBackup(w)
		restore := startedRestore(w, backup)

		By("starting the workloads of the target again")
		w.suspend(false)

		// The restore deletes the data volumes of the brokers in this phase.
		// A cluster whose workloads run again must hold it, and the grace
		// then ends it.
		expectReason(
			restore, v1.LogicalRestoreRestoringPrimaryStorage, v1.ReasonClusterNotSuspended,
		)

		reached := expectReason(restore, v1.LogicalRestoreFailed, v1.ReasonFailed)
		Expect(reached.Status.FailureMessage).To(ContainSubstring("did not recover"))
	})

	It("refuses a backup that was replaced under its name", func() {
		w := newWorld()
		backup := createBackup(w)
		restore := createRestore(w, backup.Name)

		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.BackupID).To(Equal(backupID))
		}, timeout, interval).Should(Succeed())

		By("replacing the backup with another completed one of the same name")
		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		replacement := &v1.LogicalBackupElasticsearch{
			ObjectMeta: metav1.ObjectMeta{Name: backup.Name, Namespace: w.namespace},
			Spec:       v1.LogicalBackupElasticsearchSpec{ClusterRef: v1.ClusterRef{Name: w.cluster.Name}},
		}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Create(ctx, replacement)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		replacement.Status = v1.LogicalBackupElasticsearchStatus{
			Phase:           v1.LogicalBackupCompleted,
			BackupID:        backupID + 1,
			PartitionsCount: worldPartitions,
			Version:         worldVersion,
			Repository:      w.repository,
		}
		Expect(k8sClient.Status().Update(ctx, replacement)).To(Succeed())

		// The snapshots of the replacement carry other names. A restore that
		// followed the name would restore them and call them the backup it
		// pinned.
		reached := expectReason(restore, v1.LogicalRestoreFailed, v1.ReasonFailed)
		Expect(reached.Status.FailureMessage).To(ContainSubstring("another backup"))
	})

	It("refuses a Job that another restore left behind under its name", func() {
		w := newWorld()
		backup := createBackup(w)

		// The name of a restore Job comes from the name of the restore and the
		// broker ordinal, so a restore that somebody deleted and created again
		// finds the Jobs of its predecessor. Only the controller reference
		// tells the two apart.
		restore := &v1.LogicalRestoreElasticsearch{
			ObjectMeta: metav1.ObjectMeta{Name: "lres-successor", Namespace: w.namespace},
			Spec: v1.LogicalRestoreElasticsearchSpec{
				BackupRef:        v1.LogicalBackupRef{Name: backup.Name},
				TargetClusterRef: v1.ClusterRef{Name: w.cluster.Name},
			},
		}
		name := restorepkg.JobName(labels.LogicalRestoreElasticsearch(restore.Name), 0)
		createForeignJob(w, name)

		w.seedSnapshots(elasticsearchSnapshots...)
		w.search.SetRecoveryActive(false)
		Expect(k8sClient.Create(ctx, restore)).To(Succeed())
		serveRestoredIndices(w, restore)

		reached := expectReason(restore, v1.LogicalRestoreFailed, v1.ReasonFailed)
		Expect(reached.Status.FailureMessage).To(ContainSubstring(name))
		Expect(reached.Status.FailureMessage).To(ContainSubstring("Remove the Job"))

		By("writing nothing to the Job of the other restore")
		foreign := jobNamed(w.namespace, name)
		Expect(foreign.OwnerReferences).To(BeEmpty())
		for _, entry := range foreign.ManagedFields {
			Expect(entry.Manager).NotTo(
				Equal(string(restorepkg.FieldManagerLogicalRestoreElasticsearch)),
			)
		}
	})

	It("fails and names the broker when one restore Job fails", func() {
		w := newWorld()
		backup := createBackup(w)
		restore := startedRestore(w, backup)

		owner := labels.LogicalRestoreElasticsearch(restore.Name)
		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.PrimaryJobNames).To(HaveLen(int(worldBrokers)))
		}, timeout, interval).Should(Succeed())

		markJob(w.namespace, restorepkg.JobName(owner, 0), batchv1.JobComplete)
		markJob(w.namespace, restorepkg.JobName(owner, 1), batchv1.JobFailed)

		reached := expectReason(restore, v1.LogicalRestoreFailed, v1.ReasonFailed)
		Expect(reached.Status.FailureMessage).To(ContainSubstring("broker 1"))
	})
})

var _ = Describe("LogicalRestoreElasticsearch cluster claim", func() {
	// One backup or restore of a cluster runs at a time. A restore that
	// started while a backup runs would erase the volumes under it.
	It("holds a restore whose cluster another operation holds, and touches nothing", func() {
		w := newWorld()
		holder := holdCluster(w)
		backup := createBackup(w)

		restore := createRestore(w, backup.Name)

		reached := expectReason(restore, v1.LogicalRestorePending, v1.ReasonClusterClaimed)
		Expect(readyCondition(reached).Message).To(ContainSubstring(holder.Name))
		Expect(claimHolder(w)).To(Equal("LogicalBackupElasticsearch/" + holder.Name))
		Expect(w.search.IndexDeleteCalls()).To(BeZero())
	})

	// Nothing bounds the hold, and no spec change ends it. The restore takes
	// the claim over as soon as the holder reaches a terminal phase.
	It("starts on its own once the holder finishes", func() {
		w := newWorld()
		holder := holdCluster(w)
		backup := createBackup(w)

		restore := createRestore(w, backup.Name)
		expectReason(restore, v1.LogicalRestorePending, v1.ReasonClusterClaimed)

		Eventually(func(g Gomega) {
			var current v1.LogicalBackupElasticsearch
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(holder), &current)).To(Succeed())
			current.Status.Phase = v1.LogicalBackupCompleted
			g.Expect(k8sClient.Status().Update(ctx, &current)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func() string {
			return claimHolder(w)
		}, timeout, interval).Should(Equal("LogicalRestoreElasticsearch/" + restore.Name))
	})

	It("gives the claim back when it completes", func() {
		w := newWorld()
		backup := createBackup(w)
		restore := startedRestore(w, backup)

		owner := labels.LogicalRestoreElasticsearch(restore.Name)
		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.PrimaryJobNames).To(HaveLen(int(worldBrokers)))
		}, timeout, interval).Should(Succeed())
		Expect(claimHolder(w)).To(Equal("LogicalRestoreElasticsearch/" + restore.Name))

		for ordinal := range worldBrokers {
			markJob(w.namespace, restorepkg.JobName(owner, ordinal), batchv1.JobComplete)
		}

		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.Phase).To(Equal(v1.LogicalRestoreCompleted))
			g.Expect(claimHolder(w)).To(BeEmpty())
		}, timeout, interval).Should(Succeed())
	})

	It("gives the claim back when it fails", func() {
		w := newWorld()
		backup := createBackup(w, func(b *v1.LogicalBackupElasticsearch) {
			b.Status.Version = ""
		})

		restore := createRestore(w, backup.Name)

		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.Phase).To(Equal(v1.LogicalRestoreFailed))
			g.Expect(claimHolder(w)).To(BeEmpty())
		}, timeout, interval).Should(Succeed())
	})
})
